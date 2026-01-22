package message

import (
	"fmt"

	"github.com/yaroher/protoc-gen-go-plain/generator/ir/transform/field"
	"google.golang.org/protobuf/compiler/protogen"
)

type TransformationType string

const (
	TransformationTypeAddVirtual TransformationType = "add_virtual"
)

type Transformation struct {
	OriginalMessage    *protogen.Message
	TransformationType TransformationType
	AddVirtual         *AddVirtual
}

func (m *Transformation) Render(target *protogen.GeneratedFile) error {
	switch m.TransformationType {
	case TransformationTypeAddVirtual:
		if m.AddVirtual == nil {
			return fmt.Errorf("add virtual message transformation is nil")
		}
		return m.AddVirtual.Render(target)
	default:
		return fmt.Errorf("unknown message transformation type: %s", m.TransformationType)
	}
}

type Message struct {
	OriginalMessage       *protogen.Message
	Transformations       []*Transformation
	FieldsTransformations []*field.Transformation
}

func (m *Message) Render(target *protogen.GeneratedFile) error {
	for _, transformation := range m.Transformations {
		if err := transformation.Render(target); err != nil {
			return err
		}
	}
	for _, transformation := range m.FieldsTransformations {
		if err := transformation.Render(target); err != nil {
			return err
		}
	}
	return nil
}
