package field

import "google.golang.org/protobuf/compiler/protogen"

type EnumInterpreter struct {
	OriginalField *protogen.Field
	OriginalEnum  *protogen.Enum
	AsString      bool
	AsInt         bool
}

func (e *EnumInterpreter) Render(target *protogen.GeneratedFile) error {
	//target.P(e.OriginalField.GoName)
	return nil
}
