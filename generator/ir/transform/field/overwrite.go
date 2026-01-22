package field

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

type OverwriteType struct {
	OriginalField *protogen.Field
	NewType       *typepb.Field
}

func (o *OverwriteType) Render(target *protogen.GeneratedFile) error {
	//target.P(o.OriginalField.GoName)
	return nil
}
