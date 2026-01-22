package message

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

type AddVirtual struct {
	Field *typepb.Field
}

func (a *AddVirtual) Render(target *protogen.GeneratedFile) error {
	//target.P(a.Field.Name)
	return nil
}
