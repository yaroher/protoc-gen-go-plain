package generator

import (
	"slices"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func isWrapper(field *protogen.Field) bool {
	return field.Message.Desc.FullName() == "google.protobuf.StringValue" ||
		field.Message.Desc.FullName() == "google.protobuf.Int32Value" ||
		field.Message.Desc.FullName() == "google.protobuf.Int64Value" ||
		field.Message.Desc.FullName() == "google.protobuf.UInt32Value" ||
		field.Message.Desc.FullName() == "google.protobuf.UInt64Value" ||
		field.Message.Desc.FullName() == "google.protobuf.BoolValue" ||
		field.Message.Desc.FullName() == "google.protobuf.FloatValue" ||
		field.Message.Desc.FullName() == "google.protobuf.DoubleValue" ||
		field.Message.Desc.FullName() == "google.protobuf.BytesValue"
}

func isKnownType(field *protogen.Field) bool {
	if field.Desc.Kind() == protoreflect.MessageKind {
		return slices.Contains([]protoreflect.FullName{
			"google.protobuf.Timestamp",
			"google.protobuf.Duration",
			"google.protobuf.Empty",
			"google.protobuf.Struct",
			"google.protobuf.Value",
			"google.protobuf.ListValue",
			"google.protobuf.Any",
		}, field.Message.Desc.FullName()) || isWrapper(field)
	}
	if field.Desc.Kind() == protoreflect.EnumKind {
		return true
	}
	return false
}
