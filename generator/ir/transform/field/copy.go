package field

import "google.golang.org/protobuf/compiler/protogen"

type Copy struct {
	OriginalField protogen.Field
}

func (c *Copy) Render(target *protogen.GeneratedFile) error {
	//target.P(c.OriginalField.GoName)
	return nil
}
