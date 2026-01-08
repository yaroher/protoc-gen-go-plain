package ir

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func getMessageParams(msg *protogen.Message) *goplain.PlainMessageParams {
	opts := msg.Desc.Options().(*descriptorpb.MessageOptions)
	params, _ := proto.GetExtension(opts, goplain.E_Message).(*goplain.PlainMessageParams)
	if params == nil {
		return &goplain.PlainMessageParams{}
	}
	return params
}
