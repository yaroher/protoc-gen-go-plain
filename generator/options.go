package generator

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/proto"
)

func fileOptions(fileDesc proto.Message) (*goplain.FileOptions, bool) {
	if !proto.HasExtension(fileDesc, goplain.E_File) {
		return nil, false
	}
	ext := proto.GetExtension(fileDesc, goplain.E_File)
	opts, ok := ext.(*goplain.FileOptions)
	if !ok || opts == nil {
		return nil, false
	}
	return opts, true
}

func messageOptions(msgDesc proto.Message) (*goplain.MessageOptions, bool) {
	if !proto.HasExtension(msgDesc, goplain.E_Message) {
		return nil, false
	}
	ext := proto.GetExtension(msgDesc, goplain.E_Message)
	opts, ok := ext.(*goplain.MessageOptions)
	if !ok || opts == nil {
		return nil, false
	}
	return opts, true
}

func fieldOptions(fieldDesc proto.Message) (*goplain.FieldOptions, bool) {
	if !proto.HasExtension(fieldDesc, goplain.E_Field) {
		return nil, false
	}
	ext := proto.GetExtension(fieldDesc, goplain.E_Field)
	opts, ok := ext.(*goplain.FieldOptions)
	if !ok || opts == nil {
		return nil, false
	}
	return opts, true
}

func oneofOptions(oneofDesc proto.Message) (*goplain.OneofOptions, bool) {
	if !proto.HasExtension(oneofDesc, goplain.E_Oneof) {
		return nil, false
	}
	ext := proto.GetExtension(oneofDesc, goplain.E_Oneof)
	opts, ok := ext.(*goplain.OneofOptions)
	if !ok || opts == nil {
		return nil, false
	}
	return opts, true
}
