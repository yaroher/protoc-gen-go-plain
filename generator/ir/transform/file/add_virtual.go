package file

import (
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

type AddVirtualMessage struct {
	OriginalFile *protogen.File
	NewMessage   *typepb.Type
}

func (a *AddVirtualMessage) Render(target *protogen.GeneratedFile) error {
	return nil
}
