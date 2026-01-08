package generator

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (g *Generator) BuildIR(f *protogen.File) (*ir.File, error) {
	params := getFileParams(f)

	virtualFields := append([]*goplain.VirtualField(nil), params.GetVirtualField()...)
	virtualFields = append(virtualFields, g.virtualFields...)
	virtualMessages := append([]*goplain.VirtualMessage(nil), params.GetVirtualMessage()...)
	virtualMessages = append(virtualMessages, g.virtualMessages...)

	model := &ir.File{
		GoPackage:       string(f.GoPackageName),
		GoImportPath:    string(f.GoImportPath),
		PlainSuffix:     g.Settings.PlainSuffix,
		VirtualMessages: append([]*goplain.VirtualMessage(nil), virtualMessages...),
	}

	byFull, byGo := collectMessages(f.Messages)
	ordered := orderedMessages(f.Messages)
	virtualByMessage := make(map[protoreflect.FullName][]*goplain.VirtualFieldSpec)
	for _, vf := range virtualFields {
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

		if mm.TypeAlias {
			mm.AliasValueField = aliasValueFieldIR(mm)
		} else {
			appendVirtualFields(mm, msgParams.GetVirtualFields())
			appendVirtualFields(mm, virtualByMessage[msg.Desc.FullName()])
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

func appendVirtualFields(msg *ir.Message, specs []*goplain.VirtualFieldSpec) {
	if msg == nil || len(specs) == 0 {
		return
	}
	for _, spec := range specs {
		if spec == nil || spec.GetGoType() == nil {
			continue
		}
		name := goSanitized(spec.GetName())
		if name == "" {
			continue
		}
		msg.Fields = append(msg.Fields, &ir.Field{
			ProtoName: name,
			GoName:    name,
			IsVirtual: true,
			GoType:    irGoIdentFromSpec(spec.GetGoType()),
		})
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
		if ov := fileOverrides.byProto[protoTypeKey(field)]; ov != nil {
			return ov
		}
	}
	if globalOverrides != nil {
		return globalOverrides.byProto[protoTypeKey(field)]
	}
	return nil
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
