package field

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
)

type TransformationType string

const (
	TransformationTypeCopy            TransformationType = "copy"
	TransformationTypeEmbedMessage    TransformationType = "embed_message"
	TransformationTypeEmbedOneOff     TransformationType = "embed_one_off"
	TransformationTypeEnumInterpreter TransformationType = "enum_interpreter"
	TransformationOverwriteType       TransformationType = "overwrite_type"
)

type Transformation struct {
	TransformationType TransformationType
	Copy               *Copy
	EmbedMessage       *EmbedMessage
	EmbedOneOff        *EmbedOneOff
	EnumInterpreter    *EnumInterpreter
	OverwriteType      *OverwriteType
}

func (f *Transformation) Render(target *protogen.GeneratedFile) error {
	switch f.TransformationType {
	case TransformationTypeCopy:
		if f.Copy == nil {
			return fmt.Errorf("copy field transformation is nil")
		}
		return f.Copy.Render(target)
	case TransformationTypeEmbedMessage:
		if f.EmbedMessage == nil {
			return fmt.Errorf("embed message field transformation is nil")
		}
		return f.EmbedMessage.Render(target)
	case TransformationTypeEmbedOneOff:
		if f.EmbedOneOff == nil {
			return fmt.Errorf("embed one off field transformation is nil")
		}
		return f.EmbedOneOff.Render(target)
	case TransformationTypeEnumInterpreter:
		if f.EnumInterpreter == nil {
			return fmt.Errorf("enum interpreter field transformation is nil")
		}
		return f.EnumInterpreter.Render(target)
	case TransformationOverwriteType:
		if f.OverwriteType == nil {
			return fmt.Errorf("overwrite type field transformation is nil")
		}
		return f.OverwriteType.Render(target)
	default:
		return fmt.Errorf("unknown field transformation type: %s", f.TransformationType)
	}
}
