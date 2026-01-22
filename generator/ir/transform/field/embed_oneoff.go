package field

import "google.golang.org/protobuf/compiler/protogen"

type EmbedOneOff struct {
	OriginalField   *protogen.Field
	OriginalOneOff  *protogen.Oneof
	OriginalMessage *protogen.Message
	WithPrefix      bool
}

func (e *EmbedOneOff) Render(target *protogen.GeneratedFile) error {
	//target.P(e.OriginalField.GoName)
	return nil
}
