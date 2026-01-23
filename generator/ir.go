package generator

import (
	"github.com/yaroher/protoc-gen-go-plain/crf"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

type TypePbIR struct {
	File     *protogen.File
	Messages map[string]*TypeWrapper
}

func (t *TypePbIR) Render() string {
	// Note: maybe not needed or in another file
	// TODO: Implement
	panic("not implemented")
}

type IR struct {
	Plugin   *protogen.Plugin
	TypesIrs []*TypePbIR
}

func (t *IR) Render() string {
	// Note: maybe not needed or in another file
	// TODO: Implement
	panic("not implemented")
}

type TypeWrapper struct {
	Type   *typepb.Type
	Fields []*FieldWrapper
	CRF    *crf.CRF
}

type FieldWrapper struct {
	Field   *typepb.Field
	Source  *protogen.Field
	Path    []*protogen.Field
	Oneof   *protogen.Oneof
	Origins []FieldOrigin
}

type FieldOrigin struct {
	Source   *protogen.Field
	Path     []*protogen.Field
	Oneof    *protogen.Oneof
	FullName string
}
