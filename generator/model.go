package generator

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func (g *Generator) BuildIR(f *protogen.File) (*ir.File, error) {
	params := getFileParams(f)

	virtualMessages := append([]*goplain.VirtualMessage(nil), params.GetVirtualMessage()...)
	virtualMessages = append(virtualMessages, g.virtualMessages...)

	model := &ir.File{
		GoPackage:       string(f.GoPackageName),
		GoImportPath:    string(f.GoImportPath),
		PlainSuffix:     g.Settings.PlainSuffix,
		VirtualMessages: append([]*goplain.VirtualMessage(nil), virtualMessages...),
	}

	ordered := orderedMessages(f.Messages)
	virtualByMessage := make(map[protoreflect.FullName][]*goplain.VirtualFieldSpec)
	if len(g.virtualFields) > 0 {
		byFull, byGo := collectMessages(f.Messages)
		for _, vf := range g.virtualFields {
			msg, ok := resolveMessage(vf.GetMessage(), byFull, byGo)
			if !ok {
				return nil, fmt.Errorf("virtual field target not found: %s", vf.GetMessage())
			}
			field := vf.GetField()
			if field == nil {
				continue
			}
			field.Name = goSanitized(field.GetName())
			virtualByMessage[msg.Desc.FullName()] = append(virtualByMessage[msg.Desc.FullName()], field)
		}
	}

	fileOverrides := newOverrideRegistry(params.GetOverwrite())

	for _, msg := range ordered {
		msgParams := getMessageParams(msg)
		mm := &ir.Message{
			ProtoFullName: string(msg.Desc.FullName()),
			GoIdent:       irGoIdent(msg.GoIdent),
			PlainName:     msg.GoIdent.GoName + g.Settings.PlainSuffix,
			Generate:      shouldGenerateMessage(msg),
			TypeAlias:     msgParams.GetTypeAlias(),
		}

		oneofByProto := make(map[*protogen.Oneof]*ir.Oneof)
		for _, oneof := range msg.Oneofs {
			if oneof.Desc.IsSynthetic() {
				continue
			}
			oo := &ir.Oneof{
				Name:   string(oneof.Desc.Name()),
				GoName: oneof.GoName,
			}
			oneofByProto[oneof] = oo
			mm.Oneofs = append(mm.Oneofs, oo)
		}

		for _, field := range msg.Fields {
			fm := buildFieldIR(field, fileOverrides, g.overrides)
			if fm == nil {
				continue
			}
			if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
				if oo, ok := oneofByProto[field.Oneof]; ok {
					fm.Oneof = oo
					oo.Fields = append(oo.Fields, fm)
				}
			}
			mm.Fields = append(mm.Fields, fm)
		}
		if extras := virtualByMessage[msg.Desc.FullName()]; len(extras) > 0 {
			for _, spec := range extras {
				fm, err := buildVirtualFieldIR(spec)
				if err != nil {
					return nil, err
				}
				if fm != nil {
					mm.Fields = append(mm.Fields, fm)
				}
			}
		}

		if mm.TypeAlias {
			mm.AliasValueField = aliasValueFieldIR(mm)
		}

		model.Messages = append(model.Messages, mm)
	}

	return model, nil
}

func irGoIdent(id protogen.GoIdent) ir.GoIdent {
	return ir.GoIdent{
		Name:       id.GoName,
		ImportPath: string(id.GoImportPath),
	}
}

func irGoIdentFromSpec(id *goplain.GoIdent) ir.GoIdent {
	if id == nil {
		return ir.GoIdent{}
	}
	return ir.GoIdent{
		Name:       id.GetName(),
		ImportPath: id.GetImportPath(),
	}
}

func buildFieldIR(field *protogen.Field, fileOverrides, globalOverrides *overrideRegistry) *ir.Field {
	if field == nil {
		return nil
	}
	ov := resolveOverride(field, fileOverrides, globalOverrides)
	fm := &ir.Field{
		ProtoName:    string(field.Desc.Name()),
		GoName:       field.GoName,
		Kind:         ir.Kind(field.Desc.Kind().String()),
		IsList:       field.Desc.IsList(),
		IsMap:        field.Desc.IsMap(),
		IsOptional:   field.Desc.HasOptionalKeyword(),
		IsEmbedded:   isEmbeddedMessage(field),
		IsSerialized: isSerializedMessage(field),
		Override:     ov,
	}
	if val, ok := irMarker(field, "virtual"); ok && val == "true" {
		fm.IsVirtual = true
		if goType, ok := irMarker(field, "go_type"); ok {
			fm.GoType.Name = goType
		}
		if goImport, ok := irMarker(field, "go_import"); ok {
			fm.GoType.ImportPath = goImport
		}
		fm.IsEmbedded = false
		fm.IsSerialized = false
	}
	if val, ok := irMarker(field, "embedded_from"); ok && val != "" {
		fm.EmbeddedFrom = val
	}
	if val, ok := irMarker(field, "embedded_from_full"); ok && val != "" {
		fm.EmbeddedFromFull = val
	}
	if val, ok := irMarker(field, "alias"); ok && val != "" {
		fm.AliasFromFull = val
	}
	if val, ok := irMarker(field, "serialized_from"); ok && val != "" {
		fm.SerializedFromFull = val
	}
	if val, ok := irMarker(field, "proto3_optional"); ok && val == "true" {
		fm.IsOptional = true
	}

	if field.Desc.Kind() == protoreflect.MessageKind {
		fm.MessageType = &ir.TypeRef{
			Kind:          ir.KindMessage,
			ProtoFullName: string(field.Message.Desc.FullName()),
			GoIdent:       irGoIdent(field.Message.GoIdent),
		}
	}
	if field.Desc.Kind() == protoreflect.EnumKind {
		fm.EnumType = &ir.TypeRef{
			Kind:          ir.KindEnum,
			ProtoFullName: string(field.Enum.Desc.FullName()),
			GoIdent:       irGoIdent(field.Enum.GoIdent),
		}
	}

	if field.Desc.IsMap() {
		keyField := field.Message.Fields[0]
		valField := field.Message.Fields[1]
		fm.MapKey = buildFieldIR(keyField, fileOverrides, globalOverrides)

		valOverride := getFieldOverwrite(field)
		if valOverride == nil {
			valOverride = resolveOverride(valField, fileOverrides, globalOverrides)
		}
		fm.MapValue = buildFieldIR(valField, fileOverrides, globalOverrides)
		if fm.MapValue != nil {
			fm.MapValue.Override = valOverride
		}
	}

	return fm
}

func resolveOverride(field *protogen.Field, fileOverrides, globalOverrides *overrideRegistry) *goplain.OverwriteType {
	if ov := getFieldOverwrite(field); ov != nil {
		return ov
	}
	if fileOverrides != nil {
		if ov := fileOverrides.matchField(protogenFieldKey(field), buildSelectorStateFromProtogen(field)); ov != nil {
			return ov
		}
	}
	if globalOverrides != nil {
		return globalOverrides.matchField(protogenFieldKey(field), buildSelectorStateFromProtogen(field))
	}
	return nil
}

type selectorState struct {
	protoType    string
	isList       bool
	isMap        bool
	isOptional   bool
	isOneof      bool
	isEmbedded   bool
	isSerialized bool
	isVirtual    bool
}

func buildSelectorStateFromProtogen(field *protogen.Field) selectorState {
	state := selectorState{}
	if field == nil {
		return state
	}
	state.protoType = protogenFieldKey(field)
	state.isList = field.Desc.IsList()
	state.isMap = field.Desc.IsMap()
	state.isOptional = field.Desc.HasOptionalKeyword()
	state.isOneof = field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
	state.isEmbedded = isEmbeddedMessage(field)
	state.isSerialized = isSerializedMessage(field)
	if val, ok := irMarker(field, "virtual"); ok && val == "true" {
		state.isVirtual = true
		state.isEmbedded = false
		state.isSerialized = false
	}
	if val, ok := irMarker(field, "embedded_from"); ok && val != "" {
		state.isEmbedded = true
	}
	if val, ok := irMarker(field, "proto3_optional"); ok && val == "true" {
		state.isOptional = true
	}
	if val, ok := irMarker(field, "serialized"); ok && val == "true" {
		state.isSerialized = true
	}
	return state
}

func protogenFieldKey(field *protogen.Field) string {
	if field == nil {
		return ""
	}
	switch field.Desc.Kind() {
	case protoreflect.MessageKind:
		if field.Message != nil {
			return string(field.Message.Desc.FullName())
		}
	case protoreflect.EnumKind:
		if field.Enum != nil {
			return string(field.Enum.Desc.FullName())
		}
	default:
		return field.Desc.Kind().String()
	}
	return ""
}

func irMarker(field *protogen.Field, key string) (string, bool) {
	if field == nil || field.Desc == nil || field.Desc.Options() == nil {
		return "", false
	}
	opts, ok := field.Desc.Options().(*descriptorpb.FieldOptions)
	if !ok || opts == nil {
		return "", false
	}
	for _, u := range opts.GetUninterpretedOption() {
		for _, part := range u.GetName() {
			if part.GetNamePart() == "goplain_ir_"+key {
				val := u.GetIdentifierValue()
				if val == "" && u.StringValue != nil {
					val = string(u.GetStringValue())
				}
				return val, true
			}
		}
	}
	return "", false
}

func buildVirtualFieldIR(spec *goplain.VirtualFieldSpec) (*ir.Field, error) {
	if spec == nil {
		return nil, nil
	}
	if gt := spec.GetGoType(); gt != nil && (gt.GetName() != "" || gt.GetImportPath() != "") {
		return nil, fmt.Errorf("virtual field %q must not set go_type (use proto_type only)", spec.GetName())
	}
	if strings.TrimSpace(spec.GetProtoType()) == "" {
		return nil, fmt.Errorf("virtual field %q requires proto_type", spec.GetName())
	}
	kind, ok := scalarKindFromProtoType(spec.GetProtoType())
	if !ok {
		return nil, fmt.Errorf("virtual field %q must use scalar proto_type, got: %s", spec.GetName(), spec.GetProtoType())
	}
	name := goSanitized(spec.GetName())
	if name == "" {
		return nil, nil
	}
	return &ir.Field{
		ProtoName: spec.GetName(),
		GoName:    name,
		Kind:      kind,
		IsVirtual: true,
	}, nil
}

func scalarKindFromProtoType(protoType string) (ir.Kind, bool) {
	switch strings.TrimPrefix(strings.TrimSpace(protoType), ".") {
	case "bool":
		return ir.KindBool, true
	case "int32":
		return ir.KindInt32, true
	case "int64":
		return ir.KindInt64, true
	case "sint32":
		return ir.KindSint32, true
	case "sint64":
		return ir.KindSint64, true
	case "uint32":
		return ir.KindUint32, true
	case "uint64":
		return ir.KindUint64, true
	case "fixed32":
		return ir.KindFixed32, true
	case "fixed64":
		return ir.KindFixed64, true
	case "sfixed32":
		return ir.KindSfixed32, true
	case "sfixed64":
		return ir.KindSfixed64, true
	case "float32", "float":
		return ir.KindFloat, true
	case "float64", "double":
		return ir.KindDouble, true
	case "string":
		return ir.KindString, true
	case "bytes":
		return ir.KindBytes, true
	default:
		return "", false
	}
}

func aliasValueFieldIR(msg *ir.Message) *ir.Field {
	if msg == nil || !msg.TypeAlias {
		return nil
	}
	if len(msg.Fields) != 1 {
		panic("type_alias message " + msg.ProtoFullName + " must have exactly one field named value")
	}
	field := msg.Fields[0]
	if field.ProtoName != "value" {
		panic("type_alias message " + msg.ProtoFullName + " must have a single field named value")
	}
	if field.IsList || field.IsMap || field.Oneof != nil {
		panic("type_alias message " + msg.ProtoFullName + " value field must be a singular non-oneof field")
	}
	return field
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

func orderedMessages(msgs []*protogen.Message) []*protogen.Message {
	ordered := make([]*protogen.Message, 0)
	var walk func(list []*protogen.Message)
	walk = func(list []*protogen.Message) {
		for _, msg := range list {
			if msg.Desc.IsMapEntry() {
				continue
			}
			ordered = append(ordered, msg)
			walk(msg.Messages)
		}
	}
	walk(msgs)
	return ordered
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

func virtualFieldType(out *protogen.GeneratedFile, field *goplain.VirtualFieldSpec) string {
	if field == nil || field.GetGoType() == nil {
		panic("virtual field go_type is required")
	}
	goType := field.GetGoType()
	if goType.GetImportPath() == "" {
		return goType.GetName()
	}
	if strings.Contains(goType.GetName(), ".") {
		panic("virtual field go_type.name must be unqualified when import_path is set")
	}
	return out.QualifiedGoIdent(protogen.GoIdent{
		GoImportPath: protogen.GoImportPath(goType.GetImportPath()),
		GoName:       goType.GetName(),
	})
}

func virtualMessageName(name string) string {
	return goSanitized(name)
}
