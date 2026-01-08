package generator

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (g *Generator) BuildModel(f *protogen.File) (*goplain.FileModel, error) {
	params := getFileParams(f)

	virtualFields := append([]*goplain.VirtualField(nil), params.GetVirtualField()...)
	virtualFields = append(virtualFields, g.virtualFields...)
	virtualMessages := append([]*goplain.VirtualMessage(nil), params.GetVirtualMessage()...)
	virtualMessages = append(virtualMessages, g.virtualMessages...)

	model := &goplain.FileModel{
		GoPackage:      string(f.GoPackageName),
		PlainSuffix:    g.Settings.PlainSuffix,
		Overwrite:      params.GetOverwrite(),
		VirtualMessage: virtualMessages,
	}

	byFull, byGo := collectMessages(f.Messages)
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

	for _, msg := range byFull {
		msgParams := getMessageParams(msg)
		mm := &goplain.MessageModel{
			FullName:  string(msg.Desc.FullName()),
			GoName:    msg.GoIdent.GoName,
			PlainName: msg.GoIdent.GoName + g.Settings.PlainSuffix,
			Generate:  shouldGenerateMessage(msg),
		}
		for _, field := range msg.Fields {
			fm := &goplain.FieldModel{
				ProtoName:    string(field.Desc.Name()),
				GoName:       field.GoName,
				Kind:         field.Desc.Kind().String(),
				IsList:       field.Desc.IsList(),
				IsMap:        field.Desc.IsMap(),
				IsOptional:   field.Desc.HasOptionalKeyword(),
				IsEmbedded:   isEmbeddedMessage(field),
				IsSerialized: isSerializedMessage(field),
			}
			if field.Desc.Kind() == protoreflect.MessageKind {
				fm.MessageFullName = string(field.Message.Desc.FullName())
			}
			if field.Desc.Kind() == protoreflect.EnumKind {
				fm.EnumFullName = string(field.Enum.Desc.FullName())
			}
			mm.Fields = append(mm.Fields, fm)
		}
		if extras := msgParams.GetVirtualFields(); len(extras) > 0 {
			for _, field := range extras {
				if field == nil {
					continue
				}
				field.Name = goSanitized(field.GetName())
				mm.VirtualFields = append(mm.VirtualFields, field)
			}
		}
		if extras := virtualByMessage[msg.Desc.FullName()]; len(extras) > 0 {
			mm.VirtualFields = append(mm.VirtualFields, extras...)
		}
		model.Messages = append(model.Messages, mm)
	}

	return model, nil
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
