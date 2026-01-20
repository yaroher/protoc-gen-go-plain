package ir

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/typepb"
)

// BuildIR analyzes the plugin and produces a transformation plan.
// It validates goplain options and returns diagnostics on error.
func BuildIR(p *protogen.Plugin, opts IRConfig) (*IR, error) {
	if p == nil {
		return nil, fmt.Errorf("nil plugin")
	}
	if strings.TrimSpace(opts.PlainSuffix) == "" {
		opts.PlainSuffix = "Plain"
	}
	plan := &IR{
		Files:           make(map[string]*FileIR),
		Messages:        make(map[string]*MessageIR),
		Enums:           make(map[string]*EnumIR),
		Renames:         &RenameIndex{Messages: make(map[string]string), Enums: make(map[string]string)},
		TypeResolutions: &TypeIndex{Alias: make(map[string]descriptorpb.FieldDescriptorProto_Type)},
		Options:         opts,
	}

	fileGenerate := make(map[string]bool)
	for _, f := range p.Files {
		fileGenerate[f.Desc.Path()] = f.Generate
	}

	msgIndex := make(map[string]*descriptorpb.DescriptorProto)

	for _, f := range p.Request.GetProtoFile() {
		collectMessagesAndEnums(f.GetPackage(), "", f.GetMessageType(), msgIndex)
		validateFileOptions(f, &plan.Diagnostics)
		collectTypeOverrides(plan, f)
	}

	for _, f := range p.Request.GetProtoFile() {
		fileIR := &FileIR{
			Name:     f.GetName(),
			Package:  f.GetPackage(),
			Generate: fileGenerate[f.GetName()],
		}
		for _, msg := range f.GetMessageType() {
			buildMessageIR(plan, fileIR, f.GetPackage(), "", "", msg, msgIndex)
		}
		plan.Files[fileIR.Name] = fileIR
	}

	sortDiagnostics(plan.Diagnostics)
	if hasErrors(plan.Diagnostics) {
		return plan, fmt.Errorf("invalid goplain options: %+v", plan.Diagnostics)
	}
	return plan, nil
}

func collectTypeOverrides(plan *IR, file *descriptorpb.FileDescriptorProto) {
	if file == nil || file.GetOptions() == nil {
		return
	}
	ext := proto.GetExtension(file.GetOptions(), goplain.E_File)
	if ext == nil {
		return
	}
	opts, ok := ext.(*goplain.FileOptions)
	if !ok || opts == nil {
		return
	}
	for _, ov := range opts.GetGoTypesOverrides() {
		if ov == nil || ov.GetSelector() == nil || ov.GetTargetGoType() == nil {
			continue
		}
		plan.TypeResolutions.Overrides = append(plan.TypeResolutions.Overrides, TypeOverrideRule{
			Selector: TypeOverrideSelector{
				TargetFullPath:   ov.GetSelector().GetTargetFullPath(),
				FieldKind:        ov.GetSelector().FieldKind,
				FieldCardinality: ov.GetSelector().FieldCardinality,
				FieldTypeName:    ov.GetSelector().GetFieldTypeUrl(),
			},
			Target: GoIdent{
				Name:       ov.GetTargetGoType().GetName(),
				ImportPath: ov.GetTargetGoType().GetImportPath(),
			},
		})
	}
}

func collectMessagesAndEnums(pkg, parent string, msgs []*descriptorpb.DescriptorProto, msgIndex map[string]*descriptorpb.DescriptorProto) {
	for _, m := range msgs {
		full := fullName(pkg, parent, m.GetName())
		msgIndex[full] = m
		if len(m.GetNestedType()) > 0 {
			collectMessagesAndEnums(pkg, joinPath(parent, m.GetName()), m.GetNestedType(), msgIndex)
		}
	}
}

func buildMessageIR(plan *IR, fileIR *FileIR, pkg, parent, parentNew string, msg *descriptorpb.DescriptorProto, msgIndex map[string]*descriptorpb.DescriptorProto) {
	full := fullName(pkg, parent, msg.GetName())
	validateMessageOptions(msg, full, &plan.Diagnostics)

	msgOpts := getMessageOptions(msg)
	generate := msgOpts != nil && msgOpts.GetGenerate()
	newName := msg.GetName()
	if generate {
		newName = msg.GetName() + plan.Options.PlainSuffix
	}

	newFull := fullName(pkg, parentNew, newName)
	if generate && full != newFull {
		plan.Renames.Messages[full] = newFull
	}

	msgIR := &MessageIR{
		FullName:    full,
		NewFullName: newFull,
		OrigName:    msg.GetName(),
		NewName:     newName,
		Parent:      parent,
		File:        fileIR.Name,
		Generate:    generate,
		IsMapEntry:  msg.GetOptions() != nil && msg.GetOptions().GetMapEntry(),
	}

	if generate {
		buildFieldPlans(plan, msgIR, pkg, msg, msgIndex)
		buildVirtualFields(plan, msgIR, msg)
	}

	plan.Messages[full] = msgIR
	fileIR.MessageOrder = append(fileIR.MessageOrder, full)

	if len(msg.GetNestedType()) > 0 {
		nextParent := joinPath(parent, msg.GetName())
		nextParentNew := joinPath(parentNew, newName)
		for _, nested := range msg.GetNestedType() {
			buildMessageIR(plan, fileIR, pkg, nextParent, nextParentNew, nested, msgIndex)
		}
	}
}

func buildFieldPlans(plan *IR, msgIR *MessageIR, pkg string, msg *descriptorpb.DescriptorProto, msgIndex map[string]*descriptorpb.DescriptorProto) {
	fields := msg.GetField()
	oneofDecls := msg.GetOneofDecl()
	oneofFields := make(map[int32][]*descriptorpb.FieldDescriptorProto)
	oneofIndexToName := make(map[int32]string)
	for _, f := range fields {
		if f.OneofIndex != nil {
			oneofFields[f.GetOneofIndex()] = append(oneofFields[f.GetOneofIndex()], f)
		}
		validateFieldOptions(f, msgIR.FullName, &plan.Diagnostics)
	}
	for idx, oneof := range oneofDecls {
		oneofIndexToName[int32(idx)] = oneof.GetName()
		group := oneofFields[int32(idx)]
		validateOneofOptions(oneof, msgIR.FullName, group, fields, &plan.Diagnostics)
	}

	out := make([]*FieldPlan, 0, len(fields))
	maxNumber := int32(0)
	for _, f := range fields {
		if f.GetNumber() > maxNumber {
			maxNumber = f.GetNumber()
		}
	}

	flattenOneof := make(map[int32]bool)
	oneofHasEnums := make(map[int32]bool)
	oneofPlanByIndex := make(map[int32]*OneofPlan)

	for idx, oneof := range oneofDecls {
		group := oneofFields[int32(idx)]
		opts := getOneofOptions(oneof)
		embed := false
		enumDispatch := false
		if opts != nil {
			embed = opts.GetEmbed() || opts.GetEmbedWithPrefix()
			enumDispatch = opts.GetEnumDispatched() || opts.GetEnumDispatchedWithPrefix()
		}
		fieldEnums := make(map[string][]string)
		for _, f := range group {
			fopts := getFieldOptions(f)
			if fopts != nil && len(fopts.GetWithEnums()) > 0 {
				oneofHasEnums[int32(idx)] = true
				fieldEnums[f.GetName()] = append(fieldEnums[f.GetName()], fopts.GetWithEnums()...)
			}
		}
		if embed || enumDispatch || oneofHasEnums[int32(idx)] {
			flattenOneof[int32(idx)] = true
		}
		embedWithPrefix := false
		if opts != nil {
			embedWithPrefix = opts.GetEmbedWithPrefix()
		}
		useDiscriminator := !embed && !enumDispatch && oneofHasEnums[int32(idx)]
		oneofPlan := &OneofPlan{
			OrigName:        oneof.GetName(),
			NewName:         oneof.GetName(),
			Fields:          nil,
			FieldEnums:      nil,
			Embed:           embed,
			EmbedWithPrefix: embedWithPrefix,
			Discriminator:   useDiscriminator,
		}
		if len(fieldEnums) > 0 {
			oneofPlan.FieldEnums = fieldEnums
		}
		if enumDispatch {
			oneofPlan.EnumDispatch = buildEnumDispatchPlan(plan, msgIR, oneof.GetName(), opts, group)
		}
		for _, f := range group {
			oneofPlan.Fields = append(oneofPlan.Fields, f.GetName())
		}
		oneofPlanByIndex[int32(idx)] = oneofPlan
		msgIR.OneofPlan = append(msgIR.OneofPlan, oneofPlan)
	}

	for _, f := range fields {
		if f.OneofIndex != nil && flattenOneof[f.GetOneofIndex()] {
			continue
		}
		out = append(out, buildFieldPlan(plan, msgIR, f, msgIndex, &maxNumber, "", false, oneofIndexToName)...)
	}

	for idx, group := range oneofFields {
		if !flattenOneof[idx] {
			continue
		}
		oneof := oneofDecls[idx]
		opts := getOneofOptions(oneof)
		prefix := ""
		if opts != nil && (opts.GetEmbedWithPrefix() || opts.GetEnumDispatchedWithPrefix()) {
			prefix = oneof.GetName() + "_"
		}

		if opts != nil && (opts.GetEnumDispatched() || opts.GetEnumDispatchedWithPrefix()) {
			if oneofPlan := oneofPlanByIndex[idx]; oneofPlan != nil && oneofPlan.EnumDispatch != nil {
				maxNumber++
				enumFieldName := oneof.GetName() + "_type"
				if prefix != "" {
					enumFieldName = prefix + oneof.GetName() + "_type"
				}
				out = append(out, &FieldPlan{
					OrigField: nil,
					NewField: FieldSpec{
						Name:     enumFieldName,
						Number:   maxNumber,
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL,
						Type:     descriptorpb.FieldDescriptorProto_TYPE_ENUM,
						TypeName: oneofPlan.EnumDispatch.EnumFullName,
					},
					Origin: FieldOrigin{IsVirtual: true},
					Ops:    []FieldOp{{Kind: OpEmbed, Reason: "oneof enum dispatch"}},
				})
			}
		}
		embed := false
		enumDispatch := false
		if opts != nil {
			embed = opts.GetEmbed() || opts.GetEmbedWithPrefix()
			enumDispatch = opts.GetEnumDispatched() || opts.GetEnumDispatchedWithPrefix()
		}
		if !embed && !enumDispatch && oneofHasEnums[idx] {
			maxNumber++
			discName := oneof.GetName() + "_disc"
			out = append(out, &FieldPlan{
				OrigField: nil,
				NewField: FieldSpec{
					Name:     discName,
					Number:   maxNumber,
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL,
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING,
					TypeName: "",
				},
				Origin: FieldOrigin{IsVirtual: true},
				Ops: []FieldOp{{
					Kind:   OpOverrideType,
					Reason: "oneof discriminator",
					Data: map[string]string{
						"name":        "EnumDiscriminator",
						"import_path": "github.com/yaroher/protoc-gen-go-plain/oneoff",
					},
				}},
			})
			maxNumber++
			payloadName := oneof.GetName()
			out = append(out, &FieldPlan{
				OrigField: nil,
				NewField: FieldSpec{
					Name:     payloadName,
					Number:   maxNumber,
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL,
					Type:     descriptorpb.FieldDescriptorProto_TYPE_BYTES,
					TypeName: "",
				},
				Origin: FieldOrigin{IsVirtual: true},
				Ops: []FieldOp{{
					Kind:   OpOverrideType,
					Reason: "oneof discriminator payload",
					Data: map[string]string{
						"name":        "any",
						"import_path": "",
					},
				}},
			})
		}

		if !(!embed && !enumDispatch && oneofHasEnums[idx]) {
			for _, f := range group {
				out = append(out, buildFieldPlan(plan, msgIR, f, msgIndex, &maxNumber, prefix, true, oneofIndexToName)...)
			}
		}
	}

	msgIR.FieldPlan = out
}

func buildEnumDispatchPlan(plan *IR, msgIR *MessageIR, oneofName string, opts *goplain.OneofOptions, fields []*descriptorpb.FieldDescriptorProto) *EnumDispatchPlan {
	if opts == nil {
		return nil
	}
	if opts.GetEnumDispatched() || opts.GetEnumDispatchedWithPrefix() {
		enumName := strcase.ToCamel(oneofName) + "Type"
		enumFull := msgIR.NewFullName + "." + enumName
		values := []EnumValueSpec{{Name: strings.ToUpper(strcase.ToSnake(enumName)) + "_UNSPECIFIED", Number: 0}}
		for i, f := range fields {
			values = append(values, EnumValueSpec{Name: strings.ToUpper(strcase.ToSnake(enumName)) + "_" + strings.ToUpper(strcase.ToSnake(f.GetName())), Number: int32(i + 1)})
		}
		msgIR.GeneratedEnums = append(msgIR.GeneratedEnums, &EnumSpec{Name: enumName, Values: values})
		return &EnumDispatchPlan{EnumFullName: enumFull, WithPrefix: opts.GetEnumDispatchedWithPrefix(), Generated: true}
	}
	return nil
}

func buildFieldPlan(plan *IR, msgIR *MessageIR, field *descriptorpb.FieldDescriptorProto, msgIndex map[string]*descriptorpb.DescriptorProto, maxNumber *int32, prefix string, clearOneof bool, oneofIndexToName map[int32]string) []*FieldPlan {
	if field == nil {
		return nil
	}

	opts := getFieldOptions(field)
	embed := opts != nil && (opts.GetEmbed() || opts.GetEmbedWithPrefix())
	if embed {
		return expandEmbeddedField(plan, msgIR, field, msgIndex, maxNumber, prefix)
	}

	cloned := proto.Clone(field).(*descriptorpb.FieldDescriptorProto)
	name := cloned.GetName()
	if prefix != "" {
		name = prefix + name
	}

	fs := FieldSpec{
		Name:           name,
		Number:         cloned.GetNumber(),
		Label:          cloned.GetLabel(),
		Type:           cloned.GetType(),
		TypeName:       cleanTypeName(cloned.GetTypeName()),
		OneofIndex:     cloned.OneofIndex,
		Proto3Optional: cloned.GetProto3Optional(),
		Options:        cloned.GetOptions(),
	}
	if clearOneof {
		fs.OneofIndex = nil
		fs.Proto3Optional = false
	}
	origin := FieldOrigin{}
	var fpOps []FieldOp
	if field.OneofIndex != nil {
		origin.IsOneof = true
		if oneofIndexToName != nil {
			if name, ok := oneofIndexToName[field.GetOneofIndex()]; ok {
				origin.OneofGroup = name
			}
		}
		if opts != nil && len(opts.GetWithEnums()) > 0 {
			origin.OneofEnums = append(origin.OneofEnums, opts.GetWithEnums()...)
		}
	}

	applyTypeAlias(plan, &fs, msgIndex, &origin)
	applySerialize(&fs, opts, &origin)
	applyEnumFormat(&fs, opts, field, &origin)
	applyOverride(&fs, opts, &origin, &fpOps)

	fp := &FieldPlan{
		OrigField: &FieldRef{
			MessageFullName: msgIR.FullName,
			FieldName:       field.GetName(),
			FieldNumber:     field.GetNumber(),
		},
		NewField: fs,
		Origin:   origin,
		Ops:      fpOps,
	}

	return []*FieldPlan{fp}
}

func expandEmbeddedField(plan *IR, msgIR *MessageIR, field *descriptorpb.FieldDescriptorProto, msgIndex map[string]*descriptorpb.DescriptorProto, maxNumber *int32, prefix string) []*FieldPlan {
	embeddedType := cleanTypeName(field.GetTypeName())
	if embeddedType == "" {
		return nil
	}
	msg := msgIndex[embeddedType]
	if msg == nil {
		return nil
	}

	fieldOpts := getFieldOptions(field)
	localPrefix := prefix
	if fieldOpts != nil && fieldOpts.GetEmbedWithPrefix() {
		localPrefix += field.GetName() + "_"
	}
	out := make([]*FieldPlan, 0, len(msg.GetField()))
	for _, ef := range msg.GetField() {
		*maxNumber++
		embeddedPlans := buildFieldPlan(plan, msgIR, ef, msgIndex, maxNumber, localPrefix, true, nil)
		for _, fp := range embeddedPlans {
			fp.NewField.Number = *maxNumber
			fp.Origin.IsEmbedded = true
			fp.Origin.EmbedSource = &FieldRef{MessageFullName: msgIR.FullName, FieldName: field.GetName(), FieldNumber: field.GetNumber()}
			fp.Ops = append(fp.Ops, FieldOp{Kind: OpEmbed, Reason: "embedded field"})
			out = append(out, fp)
		}
	}
	return out
}

func buildVirtualFields(plan *IR, msgIR *MessageIR, msg *descriptorpb.DescriptorProto) {
	opts := getMessageOptions(msg)
	if opts == nil || len(opts.GetVirtualFields()) == 0 {
		return
	}
	maxNumber := int32(0)
	for _, fp := range msgIR.FieldPlan {
		if fp.NewField.Number > maxNumber {
			maxNumber = fp.NewField.Number
		}
	}
	for _, vf := range opts.GetVirtualFields() {
		if vf == nil {
			continue
		}
		fieldNumber := vf.GetNumber()
		if fieldNumber == 0 {
			maxNumber++
			fieldNumber = maxNumber
		}
		fp := &FieldPlan{
			OrigField: nil,
			NewField:  virtualFieldSpec(vf, fieldNumber),
			Origin:    FieldOrigin{IsVirtual: true},
			Ops:       []FieldOp{{Kind: OpEmbed, Reason: "virtual field"}},
		}
		msgIR.FieldPlan = append(msgIR.FieldPlan, fp)
		msgIR.VirtualPlan = append(msgIR.VirtualPlan, fp)
	}
}

func virtualFieldSpec(vf *typepb.Field, number int32) FieldSpec {
	fs := FieldSpec{
		Name:   vf.GetName(),
		Number: number,
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL,
	}
	if vf.GetCardinality() == typepb.Field_CARDINALITY_REPEATED {
		fs.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	}
	switch vf.GetKind() {
	case typepb.Field_TYPE_DOUBLE:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE
	case typepb.Field_TYPE_FLOAT:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_FLOAT
	case typepb.Field_TYPE_INT64:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64
	case typepb.Field_TYPE_UINT64:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT64
	case typepb.Field_TYPE_INT32:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32
	case typepb.Field_TYPE_FIXED64:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_FIXED64
	case typepb.Field_TYPE_FIXED32:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_FIXED32
	case typepb.Field_TYPE_BOOL:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL
	case typepb.Field_TYPE_STRING:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING
	case typepb.Field_TYPE_GROUP:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_GROUP
	case typepb.Field_TYPE_MESSAGE:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
		fs.TypeName = vf.GetTypeUrl()
	case typepb.Field_TYPE_BYTES:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES
	case typepb.Field_TYPE_UINT32:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_UINT32
	case typepb.Field_TYPE_ENUM:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM
		fs.TypeName = vf.GetTypeUrl()
	case typepb.Field_TYPE_SFIXED32:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_SFIXED32
	case typepb.Field_TYPE_SFIXED64:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_SFIXED64
	case typepb.Field_TYPE_SINT32:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_SINT32
	case typepb.Field_TYPE_SINT64:
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_SINT64
	}
	return fs
}

func applySerialize(fs *FieldSpec, opts *goplain.FieldOptions, origin *FieldOrigin) {
	if opts == nil || !opts.GetSerialize() {
		return
	}
	fs.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES
	fs.TypeName = ""
	origin.IsSerialized = true
}

func applyOverride(fs *FieldSpec, opts *goplain.FieldOptions, origin *FieldOrigin, ops *[]FieldOp) {
	if opts == nil || opts.GetOverrideType() == nil {
		return
	}
	override := opts.GetOverrideType()
	if override.GetName() == "" {
		return
	}
	*ops = append(*ops, FieldOp{
		Kind:   OpOverrideType,
		Reason: "field override_type",
		Data: map[string]string{
			"name":        override.GetName(),
			"import_path": override.GetImportPath(),
		},
	})
}

func applyEnumFormat(fs *FieldSpec, opts *goplain.FieldOptions, field *descriptorpb.FieldDescriptorProto, origin *FieldOrigin) {
	if opts == nil || field == nil {
		return
	}
	if field.GetType() != descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		return
	}
	if opts.GetEnumAsString() {
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING
		fs.TypeName = ""
		origin.EnumAsString = true
		return
	}
	if opts.GetEnumAsInt() {
		fs.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32
		fs.TypeName = ""
		origin.EnumAsInt = true
	}
}

func applyTypeAlias(plan *IR, fs *FieldSpec, msgIndex map[string]*descriptorpb.DescriptorProto, origin *FieldOrigin) {
	if fs.Type != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE || fs.TypeName == "" {
		return
	}
	msg := msgIndex[fs.TypeName]
	if msg == nil {
		return
	}
	opts := getMessageOptions(msg)
	if opts == nil || !opts.GetTypeAlias() {
		return
	}
	if len(msg.GetField()) != 1 || msg.GetField()[0].GetName() != "value" {
		return
	}
	value := msg.GetField()[0]
	if value.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE || value.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM || value.GetType() == descriptorpb.FieldDescriptorProto_TYPE_GROUP {
		return
	}
	fs.Type = value.GetType()
	fs.TypeName = ""
	origin.IsTypeAlias = true
	origin.OriginalType = value.GetType().String()
}

func getMessageOptions(msg *descriptorpb.DescriptorProto) *goplain.MessageOptions {
	if msg.GetOptions() == nil {
		return nil
	}
	ext := proto.GetExtension(msg.GetOptions(), goplain.E_Message)
	if ext == nil {
		return nil
	}
	opts, _ := ext.(*goplain.MessageOptions)
	return opts
}

func getFieldOptions(field *descriptorpb.FieldDescriptorProto) *goplain.FieldOptions {
	if field.GetOptions() == nil {
		return nil
	}
	ext := proto.GetExtension(field.GetOptions(), goplain.E_Field)
	if ext == nil {
		return nil
	}
	opts, _ := ext.(*goplain.FieldOptions)
	return opts
}

func getOneofOptions(oneof *descriptorpb.OneofDescriptorProto) *goplain.OneofOptions {
	if oneof.GetOptions() == nil {
		return nil
	}
	ext := proto.GetExtension(oneof.GetOptions(), goplain.E_Oneof)
	if ext == nil {
		return nil
	}
	opts, _ := ext.(*goplain.OneofOptions)
	return opts
}

func fullName(pkg, parent, name string) string {
	parts := make([]string, 0, 3)
	if pkg != "" {
		parts = append(parts, pkg)
	}
	if parent != "" {
		parts = append(parts, parent)
	}
	if name != "" {
		parts = append(parts, name)
	}
	if len(parts) == 0 {
		return ""
	}
	return "." + strings.Join(parts, ".")
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	if name == "" {
		return parent
	}
	return parent + "." + name
}

func cleanTypeName(typeName string) string {
	if typeName == "" {
		return ""
	}
	if strings.HasPrefix(typeName, ".") {
		return typeName
	}
	return "." + typeName
}

// sortDiagnostics keeps output stable.
func sortDiagnostics(diags []Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Level != diags[j].Level {
			return diags[i].Level < diags[j].Level
		}
		return diags[i].Subject < diags[j].Subject
	})
}
