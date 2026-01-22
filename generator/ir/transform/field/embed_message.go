package field

import "google.golang.org/protobuf/compiler/protogen"

type EmbedMessage struct {
	OriginalField   *protogen.Field
	OriginalMessage *protogen.Message
	WithPrefix      bool
}

func (e *EmbedMessage) Render(target *protogen.GeneratedFile) error {
	//target.P(e.OriginalField.GoName)
	return nil
}
