package ir

import (
	"fmt"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

type IRFile = protogen.File
type IRMessage = protogen.Message
type IRField = protogen.Field

func ConvertFile(f *protogen.File) *IRFile {
	if f == nil {
		return nil
	}
	c := newConverter(f)
	return c.convertFile(f)
}

// ConvertRequestDescriptor transforms CodeGeneratorRequest by rewriting descriptor protos
// according to goplain rules. This produces a new request suitable for protogen.Options.New.
func ConvertRequestDescriptor(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorRequest, error) {
	if req == nil {
		return nil, nil
	}
	reg := newTypeRegistry(req)
	out := proto.Clone(req).(*pluginpb.CodeGeneratorRequest)
	for i, f := range req.GetProtoFile() {
		out.ProtoFile[i] = convertFileDescriptor(f, reg)
	}
	return out, nil
}

// BuildPluginFromRequest constructs a protogen.Plugin using the provided request.
func BuildPluginFromRequest(req *pluginpb.CodeGeneratorRequest) (*protogen.Plugin, error) {
	if req == nil {
		return nil, nil
	}
	var opts protogen.Options
	return opts.New(req)
}

// ConvertPluginDescriptor rebuilds a protogen.Plugin from a transformed descriptor request.
func ConvertPluginDescriptor(p *protogen.Plugin) (*protogen.Plugin, error) {
	if p == nil || p.Request == nil {
		return nil, nil
	}
	req, err := ConvertRequestDescriptor(p.Request)
	if err != nil {
		return nil, err
	}
	return BuildPluginFromRequest(req)
}

type converter struct {
	byFull           map[protoreflect.FullName]*protogen.Message
	byGo             map[string]*protogen.Message
	virtualByMessage map[protoreflect.FullName][]*goplain.VirtualFieldSpec
	msgCache         map[*protogen.Message]*protogen.Message
}

func newConverter(f *protogen.File) *converter {
	byFull, byGo := collectMessages(f.Messages)
	virtualByMessage := make(map[protoreflect.FullName][]*goplain.VirtualFieldSpec)
	params := getFileParams(f)

	for _, vf := range params.GetVirtualField() {
		msg, ok := resolveMessage(vf.GetMessage(), byFull, byGo)
		if !ok {
			panic("virtual field target not found: " + vf.GetMessage())
		}
		spec := vf.GetField()
		if spec == nil {
			continue
		}
		spec.Name = goSanitized(spec.GetName())
		virtualByMessage[msg.Desc.FullName()] = append(virtualByMessage[msg.Desc.FullName()], spec)
	}

	return &converter{
		byFull:           byFull,
		byGo:             byGo,
		virtualByMessage: virtualByMessage,
		msgCache:         make(map[*protogen.Message]*protogen.Message),
	}
}

func (c *converter) convertFile(f *protogen.File) *protogen.File {
	out := *f
	out.Messages = c.convertMessages(f.Messages)
	return &out
}

func (c *converter) convertMessages(msgs []*protogen.Message) []*protogen.Message {
	out := make([]*protogen.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if msg.Desc.IsMapEntry() {
			out = append(out, msg)
			continue
		}
		out = append(out, c.convertMessage(msg))
	}
	return out
}

func (c *converter) convertMessage(msg *protogen.Message) *protogen.Message {
	if msg == nil {
		return nil
	}
	if cached, ok := c.msgCache[msg]; ok {
		return cached
	}
	out := *msg
	c.msgCache[msg] = &out

	out.Messages = c.convertMessages(msg.Messages)
	out.Fields = c.convertFields(msg)
	return &out
}

func (c *converter) convertFields(msg *protogen.Message) []*protogen.Field {
	out := make([]*protogen.Field, 0, len(msg.Fields))
	for _, field := range msg.Fields {
		if field == nil {
			continue
		}
		if isEmbeddedField(field) {
			out = append(out, c.flattenEmbeddedFields(field)...)
			continue
		}
		out = append(out, c.convertField(field))
	}

	if params := getMessageParams(msg); params != nil {
		out = append(out, c.virtualFields(params.GetVirtualFields(), msg)...)
	}
	if extras := c.virtualByMessage[msg.Desc.FullName()]; len(extras) > 0 {
		out = append(out, c.virtualFields(extras, msg)...)
	}

	return out
}

func (c *converter) convertField(field *protogen.Field) *protogen.Field {
	out := copyField(field)
	if out == nil {
		return nil
	}

	if isSerializedField(field) {
		out.Comments = addMarker(out.Comments, "serialized", "true")
		out.Comments = addMarker(out.Comments, "kind", "bytes")
	}

	if field.Desc.Kind() == protoreflect.MessageKind && field.Message != nil && isTypeAliasMessage(field.Message) {
		out.Comments = addMarker(out.Comments, "alias", string(field.Message.Desc.FullName()))
		if aliasVal := aliasValueField(field.Message); aliasVal != nil {
			out.Comments = addMarker(out.Comments, "alias_value_kind", aliasVal.Desc.Kind().String())
			switch aliasVal.Desc.Kind() {
			case protoreflect.MessageKind:
				if aliasVal.Message != nil {
					out.Comments = addMarker(out.Comments, "alias_value_type", string(aliasVal.Message.Desc.FullName()))
				}
			case protoreflect.EnumKind:
				if aliasVal.Enum != nil {
					out.Comments = addMarker(out.Comments, "alias_value_type", string(aliasVal.Enum.Desc.FullName()))
				}
			}
		}
	}

	return out
}

func (c *converter) flattenEmbeddedFields(field *protogen.Field) []*protogen.Field {
	if field == nil || field.Message == nil {
		return nil
	}
	embedded := c.convertMessage(field.Message)
	if embedded == nil {
		return nil
	}
	out := make([]*protogen.Field, 0, len(embedded.Fields))
	for _, ef := range embedded.Fields {
		if ef == nil || ef.Desc == nil {
			continue
		}
		copied := copyField(ef)
		copied.Comments = addMarker(copied.Comments, "embedded_from", field.GoName)
		copied.Comments = addMarker(copied.Comments, "embedded_from_full", string(field.Message.Desc.FullName()))
		out = append(out, copied)
	}
	return out
}

func (c *converter) virtualFields(specs []*goplain.VirtualFieldSpec, parent *protogen.Message) []*protogen.Field {
	if len(specs) == 0 {
		return nil
	}
	out := make([]*protogen.Field, 0, len(specs))
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		if gt := spec.GetGoType(); gt != nil && (gt.GetName() != "" || gt.GetImportPath() != "") {
			panic(fmt.Sprintf("virtual field %s must not set go_type (use proto_type only)", spec.GetName()))
		}
		name := goSanitized(spec.GetName())
		if name == "" {
			continue
		}
		protoType := strings.TrimSpace(spec.GetProtoType())
		if protoType == "" {
			panic(fmt.Sprintf("virtual field %s requires proto_type", name))
		}
		if !applyScalarProtoType(&descriptorpb.FieldDescriptorProto{}, protoType) {
			panic(fmt.Sprintf("virtual field %s must use scalar proto_type, got: %s", name, protoType))
		}
		f := &protogen.Field{
			GoName:   name,
			Parent:   parent,
			Desc:     nil,
			Enum:     nil,
			Message:  nil,
			Oneof:    nil,
			Extendee: nil,
		}
		f.Comments = addMarker(f.Comments, "virtual", "true")
		out = append(out, f)
	}
	return out
}

func copyField(field *protogen.Field) *protogen.Field {
	if field == nil {
		return nil
	}
	out := *field
	return &out
}

func isEmbeddedField(field *protogen.Field) bool {
	opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	return plainField.GetEmbedded()
}

func isSerializedField(field *protogen.Field) bool {
	opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	return plainField.GetSerialized()
}

func isTypeAliasMessage(msg *protogen.Message) bool {
	return getMessageParams(msg).GetTypeAlias()
}

func aliasValueField(msg *protogen.Message) *protogen.Field {
	if msg == nil || !isTypeAliasMessage(msg) {
		return nil
	}
	if len(msg.Fields) != 1 {
		panic("type_alias message " + string(msg.Desc.FullName()) + " must have exactly one field named value")
	}
	field := msg.Fields[0]
	if field.Desc.Name() != "value" {
		panic("type_alias message " + string(msg.Desc.FullName()) + " must have a single field named value")
	}
	if field.Desc.IsList() || field.Desc.IsMap() || (field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()) {
		panic("type_alias message " + string(msg.Desc.FullName()) + " value field must be a singular non-oneof field")
	}
	return field
}

func getFileParams(file *protogen.File) *goplain.PlainFileParams {
	opts := file.Desc.Options().(*descriptorpb.FileOptions)
	params, _ := proto.GetExtension(opts, goplain.E_File).(*goplain.PlainFileParams)
	if params == nil {
		return &goplain.PlainFileParams{}
	}
	return params
}

func getMessageParams(msg *protogen.Message) *goplain.PlainMessageParams {
	opts := msg.Desc.Options().(*descriptorpb.MessageOptions)
	params, _ := proto.GetExtension(opts, goplain.E_Message).(*goplain.PlainMessageParams)
	if params == nil {
		return &goplain.PlainMessageParams{}
	}
	return params
}

func collectMessages(msgs []*protogen.Message) (map[protoreflect.FullName]*protogen.Message, map[string]*protogen.Message) {
	byFull := make(map[protoreflect.FullName]*protogen.Message)
	byGo := make(map[string]*protogen.Message)
	var walk func(list []*protogen.Message)
	walk = func(list []*protogen.Message) {
		for _, msg := range list {
			if msg.Desc.IsMapEntry() {
				continue
			}
			byFull[msg.Desc.FullName()] = msg
			byGo[msg.GoIdent.GoName] = msg
			walk(msg.Messages)
		}
	}
	walk(msgs)
	return byFull, byGo
}

func resolveMessage(name string, byFull map[protoreflect.FullName]*protogen.Message, byGo map[string]*protogen.Message) (*protogen.Message, bool) {
	if name == "" {
		return nil, false
	}
	fullName := protoreflect.FullName(strings.TrimPrefix(name, "."))
	if msg, ok := byFull[fullName]; ok {
		return msg, true
	}
	msg, ok := byGo[name]
	return msg, ok
}

func addMarker(set protogen.CommentSet, key, val string) protogen.CommentSet {
	set.Leading = addMarkerComment(set.Leading, key, val)
	return set
}

func addMarkerComment(comments protogen.Comments, key, val string) protogen.Comments {
	line := "goplain:" + key
	if val != "" {
		line += "=" + val
	}
	if comments != "" && !strings.HasSuffix(string(comments), "\n") {
		comments += "\n"
	}
	comments += protogen.Comments(line + "\n")
	return comments
}

func goSanitized(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '_'
	}, s)

	r, _ := utf8.DecodeRuneInString(s)
	if token.Lookup(s).IsKeyword() || !unicode.IsLetter(r) {
		return "_" + s
	}
	return s
}

type typeRegistry struct {
	messages map[string]*descriptorpb.DescriptorProto
	enums    map[string]*descriptorpb.EnumDescriptorProto
}

func newTypeRegistry(req *pluginpb.CodeGeneratorRequest) *typeRegistry {
	reg := &typeRegistry{
		messages: make(map[string]*descriptorpb.DescriptorProto),
		enums:    make(map[string]*descriptorpb.EnumDescriptorProto),
	}
	for _, f := range req.GetProtoFile() {
		pkg := f.GetPackage()
		for _, msg := range f.GetMessageType() {
			collectMessageTypes(reg, pkg, "", msg)
		}
		for _, enum := range f.GetEnumType() {
			full := joinFullName(pkg, "", enum.GetName())
			reg.enums[full] = enum
		}
	}
	return reg
}

func collectMessageTypes(reg *typeRegistry, pkg, parent string, msg *descriptorpb.DescriptorProto) {
	if msg == nil {
		return
	}
	full := joinFullName(pkg, parent, msg.GetName())
	reg.messages[full] = msg
	for _, nested := range msg.GetNestedType() {
		collectMessageTypes(reg, pkg, full, nested)
	}
	for _, enum := range msg.GetEnumType() {
		enumFull := joinFullName(pkg, full, enum.GetName())
		reg.enums[enumFull] = enum
	}
}

func joinFullName(pkg, parent, name string) string {
	if name == "" {
		return ""
	}
	var parts []string
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if parent != "" {
		parts = append(parts, parent)
	}
	parts = append(parts, name)
	return strings.TrimPrefix(strings.Join(parts, "."), ".")
}

func convertFileDescriptor(f *descriptorpb.FileDescriptorProto, reg *typeRegistry) *descriptorpb.FileDescriptorProto {
	if f == nil {
		return nil
	}
	out := proto.Clone(f).(*descriptorpb.FileDescriptorProto)
	ctx := &transformCtx{
		file:       out,
		registry:   reg,
		fileParams: getFileParamsFromDescriptor(out),
	}
	out.MessageType = transformMessages(out.GetMessageType(), ctx, "")
	return out
}

type transformCtx struct {
	file       *descriptorpb.FileDescriptorProto
	registry   *typeRegistry
	fileParams *goplain.PlainFileParams
}

func transformMessages(msgs []*descriptorpb.DescriptorProto, ctx *transformCtx, parent string) []*descriptorpb.DescriptorProto {
	out := make([]*descriptorpb.DescriptorProto, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		out = append(out, transformMessage(msg, ctx, parent))
	}
	return out
}

func transformMessage(msg *descriptorpb.DescriptorProto, ctx *transformCtx, parent string) *descriptorpb.DescriptorProto {
	if msg == nil {
		return nil
	}
	out := proto.Clone(msg).(*descriptorpb.DescriptorProto)
	fullName := joinFullName(ctx.file.GetPackage(), parent, out.GetName())
	out.NestedType = transformMessages(out.GetNestedType(), ctx, fullName)

	nextNumber := nextFieldNumber(out)
	fields := transformFields(out.GetField(), ctx, fullName, &nextNumber)
	fields = append(fields, virtualFieldsFromMessage(out, ctx, fullName, &nextNumber)...)
	fields = append(fields, virtualFieldsFromFile(ctx, fullName, &nextNumber)...)
	out.Field = fields
	return out
}

func transformFields(fields []*descriptorpb.FieldDescriptorProto, ctx *transformCtx, parentFull string, nextNumber *int32) []*descriptorpb.FieldDescriptorProto {
	out := make([]*descriptorpb.FieldDescriptorProto, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		if isEmbeddedFieldDesc(field) {
			out = append(out, flattenEmbeddedFields(field, ctx, parentFull, nextNumber)...)
			continue
		}
		out = append(out, transformField(field, ctx))
	}
	return out
}

func transformField(field *descriptorpb.FieldDescriptorProto, ctx *transformCtx) *descriptorpb.FieldDescriptorProto {
	out := proto.Clone(field).(*descriptorpb.FieldDescriptorProto)
	if isSerializedFieldDesc(field) {
		if out.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && out.GetTypeName() != "" {
			addMarkerOption(out, "serialized_from", normalizeTypeName(out.GetTypeName()))
		}
		out.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
		out.TypeName = nil
		addMarkerOption(out, "serialized", "true")
		addMarkerOption(out, "kind", "bytes")
	}

	if out.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE && out.GetTypeName() != "" {
		ref := normalizeTypeName(out.GetTypeName())
		if msg := ctx.registry.messages[ref]; msg != nil && isTypeAliasMessageDesc(msg) {
			addMarkerOption(out, "alias", ref)
			if aliasVal := aliasValueFieldDesc(msg); aliasVal != nil {
				addMarkerOption(out, "alias_value_kind", aliasVal.GetType().String())
				if aliasVal.GetTypeName() != "" {
					addMarkerOption(out, "alias_value_type", normalizeTypeName(aliasVal.GetTypeName()))
				}
				out.Type = aliasVal.Type
				out.TypeName = aliasVal.TypeName
			}
		}
	}

	return out
}

func flattenEmbeddedFields(field *descriptorpb.FieldDescriptorProto, ctx *transformCtx, parentFull string, nextNumber *int32) []*descriptorpb.FieldDescriptorProto {
	ref := normalizeTypeName(field.GetTypeName())
	msg := ctx.registry.messages[ref]
	if msg == nil {
		return nil
	}
	embedded := transformMessage(msg, ctx, parentFull)
	out := make([]*descriptorpb.FieldDescriptorProto, 0, len(embedded.GetField()))
	for _, ef := range embedded.GetField() {
		if ef == nil {
			continue
		}
		copied := proto.Clone(ef).(*descriptorpb.FieldDescriptorProto)
		if nextNumber != nil {
			copied.Number = proto.Int32(*nextNumber)
			*nextNumber++
		}
		if ef.GetProto3Optional() {
			addMarkerOption(copied, "proto3_optional", "true")
		}
		copied.OneofIndex = nil
		copied.Proto3Optional = nil
		addMarkerOption(copied, "embedded_from", strcase.ToCamel(field.GetName()))
		addMarkerOption(copied, "embedded_from_full", ref)
		out = append(out, copied)
	}
	return out
}

func virtualFieldsFromMessage(msg *descriptorpb.DescriptorProto, ctx *transformCtx, fullName string, nextNumber *int32) []*descriptorpb.FieldDescriptorProto {
	params := getMessageParamsFromDescriptor(msg)
	if params == nil {
		return nil
	}
	return makeVirtualFieldDescriptors(params.GetVirtualFields(), ctx, fullName, nextNumber)
}

func virtualFieldsFromFile(ctx *transformCtx, fullName string, nextNumber *int32) []*descriptorpb.FieldDescriptorProto {
	if ctx == nil || ctx.fileParams == nil {
		return nil
	}
	specs := make([]*goplain.VirtualFieldSpec, 0)
	for _, vf := range ctx.fileParams.GetVirtualField() {
		if vf == nil || vf.GetMessage() == "" || vf.GetField() == nil {
			continue
		}
		target := strings.TrimPrefix(vf.GetMessage(), ".")
		if target == fullName {
			specs = append(specs, vf.GetField())
		}
	}
	if len(specs) == 0 {
		return nil
	}
	return makeVirtualFieldDescriptors(specs, ctx, fullName, nextNumber)
}

func makeVirtualFieldDescriptors(specs []*goplain.VirtualFieldSpec, ctx *transformCtx, fullName string, nextNumber *int32) []*descriptorpb.FieldDescriptorProto {
	if len(specs) == 0 {
		return nil
	}
	out := make([]*descriptorpb.FieldDescriptorProto, 0, len(specs))
	for _, spec := range specs {
		if spec == nil {
			continue
		}
		if gt := spec.GetGoType(); gt != nil && (gt.GetName() != "" || gt.GetImportPath() != "") {
			panic(fmt.Sprintf("virtual field %s must not set go_type (use proto_type only)", spec.GetName()))
		}
		name := strcase.ToSnake(goSanitized(spec.GetName()))
		if name == "" {
			continue
		}
		protoType := strings.TrimSpace(spec.GetProtoType())
		if protoType == "" {
			panic(fmt.Sprintf("virtual field %s requires proto_type", name))
		}
		fd := &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(name),
			Number: proto.Int32(*nextNumber),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		*nextNumber++
		if !applyScalarProtoType(fd, protoType) {
			panic(fmt.Sprintf("virtual field %s must use scalar proto_type, got: %s", name, protoType))
		}
		addMarkerOption(fd, "virtual", "true")
		out = append(out, fd)
	}
	return out
}

func applyScalarProtoType(fd *descriptorpb.FieldDescriptorProto, protoType string) bool {
	protoType = normalizeTypeName(protoType)
	switch protoType {
	case "string":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
		fd.TypeName = nil
		return true
	case "bytes":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
		fd.TypeName = nil
		return true
	case "bool":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
		fd.TypeName = nil
		return true
	case "int32":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
		fd.TypeName = nil
		return true
	case "int64":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
		fd.TypeName = nil
		return true
	case "uint32":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT32.Enum()
		fd.TypeName = nil
		return true
	case "uint64":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum()
		fd.TypeName = nil
		return true
	case "sint32":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_SINT32.Enum()
		fd.TypeName = nil
		return true
	case "sint64":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_SINT64.Enum()
		fd.TypeName = nil
		return true
	case "fixed32":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_FIXED32.Enum()
		fd.TypeName = nil
		return true
	case "fixed64":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_FIXED64.Enum()
		fd.TypeName = nil
		return true
	case "sfixed32":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_SFIXED32.Enum()
		fd.TypeName = nil
		return true
	case "sfixed64":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_SFIXED64.Enum()
		fd.TypeName = nil
		return true
	case "float32", "float":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()
		fd.TypeName = nil
		return true
	case "float64", "double":
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
		fd.TypeName = nil
		return true
	}
	return false
}

func nextFieldNumber(msg *descriptorpb.DescriptorProto) int32 {
	var max int32 = 0
	if msg == nil {
		return 1
	}
	for _, field := range msg.GetField() {
		if field == nil {
			continue
		}
		if field.GetNumber() > max {
			max = field.GetNumber()
		}
	}
	return max + 1
}

func addMarkerOption(field *descriptorpb.FieldDescriptorProto, key, val string) {
	if field == nil {
		return
	}
	if field.Options == nil {
		field.Options = &descriptorpb.FieldOptions{}
	}
	field.Options.UninterpretedOption = append(field.Options.UninterpretedOption, &descriptorpb.UninterpretedOption{
		Name: []*descriptorpb.UninterpretedOption_NamePart{
			{NamePart: proto.String("goplain_ir_" + key), IsExtension: proto.Bool(false)},
		},
		IdentifierValue: proto.String(val),
	})
}

func normalizeTypeName(name string) string {
	return strings.TrimPrefix(name, ".")
}

func getFileParamsFromDescriptor(file *descriptorpb.FileDescriptorProto) *goplain.PlainFileParams {
	opts := file.GetOptions()
	params, _ := proto.GetExtension(opts, goplain.E_File).(*goplain.PlainFileParams)
	if params == nil {
		return &goplain.PlainFileParams{}
	}
	return params
}

func getMessageParamsFromDescriptor(msg *descriptorpb.DescriptorProto) *goplain.PlainMessageParams {
	opts := msg.GetOptions()
	params, _ := proto.GetExtension(opts, goplain.E_Message).(*goplain.PlainMessageParams)
	if params == nil {
		return &goplain.PlainMessageParams{}
	}
	return params
}

func isEmbeddedFieldDesc(field *descriptorpb.FieldDescriptorProto) bool {
	opts := field.GetOptions()
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	return plainField.GetEmbedded()
}

func isSerializedFieldDesc(field *descriptorpb.FieldDescriptorProto) bool {
	opts := field.GetOptions()
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	return plainField.GetSerialized()
}

func isTypeAliasMessageDesc(msg *descriptorpb.DescriptorProto) bool {
	return getMessageParamsFromDescriptor(msg).GetTypeAlias()
}

func aliasValueFieldDesc(msg *descriptorpb.DescriptorProto) *descriptorpb.FieldDescriptorProto {
	if msg == nil || !isTypeAliasMessageDesc(msg) {
		return nil
	}
	if len(msg.GetField()) != 1 {
		panic("type_alias message must have exactly one field named value")
	}
	field := msg.GetField()[0]
	if field.GetName() != "value" {
		panic("type_alias message must have a single field named value")
	}
	if field.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED || field.OneofIndex != nil {
		panic("type_alias message value field must be a singular non-oneof field")
	}
	return field
}

func isVirtualFieldDesc(field *descriptorpb.FieldDescriptorProto) bool {
	opts := field.GetOptions()
	if opts == nil {
		return false
	}
	for _, u := range opts.GetUninterpretedOption() {
		for _, part := range u.GetName() {
			if part.GetNamePart() == "goplain_ir_virtual" {
				return true
			}
		}
	}
	return false
}
