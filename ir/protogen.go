package ir

import (
	"google.golang.org/protobuf/compiler/protogen"
)

type IRFile = protogen.File

type IRMessage = protogen.Message

type IRField = protogen.Field

func ConvertField(field *protogen.Field) *IRField {
	return &IRField{
		Desc:     field.Desc,
		GoName:   field.GoName,
		GoIdent:  field.GoIdent,
		Parent:   field.Parent,
		Oneof:    field.Oneof,
		Extendee: field.Extendee,
		Enum:     field.Enum,
		Message:  field.Message,
		Location: field.Location,
		Comments: field.Comments,
	}
}

func ConvertMessage(msg *protogen.Message) *IRMessage {
	fields := make([]*IRField, 0)
	for _, field := range msg.Fields {
		fields = append(fields, ConvertField(field))
	}
	return &IRMessage{
		Desc:       msg.Desc,
		GoIdent:    msg.GoIdent,
		Fields:     msg.Fields,
		Oneofs:     msg.Oneofs,
		Enums:      msg.Enums,
		Messages:   msg.Messages,
		Extensions: msg.Extensions,
		Location:   msg.Location,
		Comments:   msg.Comments,
		APILevel:   msg.APILevel,
	}
}

func ConvertFile(f *protogen.File) *IRFile {
	genMessages := make([]*IRMessage, 0)
	for _, msg := range f.Messages {
		params := getMessageParams(msg)
		if !params.GetGenerate() {
			continue
		}
		genMessages = append(genMessages, ConvertMessage(msg))
	}
	return &IRFile{
		Desc:                    f.Desc,
		Proto:                   f.Proto,
		GoDescriptorIdent:       f.GoDescriptorIdent,
		GoPackageName:           f.GoPackageName,
		GoImportPath:            f.GoImportPath,
		Enums:                   f.Enums,
		Messages:                genMessages,
		Extensions:              f.Extensions,
		Services:                f.Services,
		Generate:                f.Generate,
		GeneratedFilenamePrefix: f.GeneratedFilenamePrefix,
		APILevel:                f.APILevel,
	}
}
