package generator

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func isFieldOneOf(field *protogen.Field) bool {
	return field.Oneof != nil && !field.Oneof.Desc.IsSynthetic()
}

func isFieldNullable(field *protogen.Field) bool {
	oneOf := isFieldOneOf(field)
	return field.Desc.HasOptionalKeyword() || oneOf
}

func isFieldArray(field *protogen.Field) bool {
	array := field.Desc.IsList()
	return array
	//opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	//sqlField, _ := proto.GetExtension(opts, protopgx.E_SqlField).(*protopgx.SqlField)
	//if sqlField.GetSqlType().GetForceNotArray() {
	//	if array {
	//		help.Logger.Warn(
	//			"use not array type for field cause force not array",
	//			zap.String("name", string(field.Desc.FullName())),
	//		)
	//	}
	//	array = false
	//}
	//return array
}

func isSerializedMessage(field *protogen.Field) bool {
	opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	return plainField.GetSerialized()
}

func isEmbeddedMessage(field *protogen.Field) bool {
	opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	return plainField.GetEmbedded()
}

func getFieldOverwrite(field *protogen.Field) *goplain.OverwriteType {
	opts := field.Desc.Options().(*descriptorpb.FieldOptions)
	plainField, _ := proto.GetExtension(opts, goplain.E_Field).(*goplain.PlainFieldParams)
	if plainField == nil {
		return nil
	}
	return plainField.GetOverwrite()
}

func getFileParams(file *protogen.File) *goplain.PlainFileParams {
	opts := file.Desc.Options().(*descriptorpb.FileOptions)
	params, _ := proto.GetExtension(opts, goplain.E_File).(*goplain.PlainFileParams)
	if params == nil {
		return &goplain.PlainFileParams{}
	}
	return params
}

func shouldGenerateMessage(msg *protogen.Message) bool {
	params := getMessageParams(msg)
	return params.GetGenerate() && !params.GetTypeAlias()
}

func isTypeAliasMessage(msg *protogen.Message) bool {
	return getMessageParams(msg).GetTypeAlias()
}

func getMessageParams(msg *protogen.Message) *goplain.PlainMessageParams {
	opts := msg.Desc.Options().(*descriptorpb.MessageOptions)
	params, _ := proto.GetExtension(opts, goplain.E_Message).(*goplain.PlainMessageParams)
	if params == nil {
		return &goplain.PlainMessageParams{}
	}
	return params
}
