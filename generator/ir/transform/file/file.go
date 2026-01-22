package file

import (
	"github.com/yaroher/protoc-gen-go-plain/generator/ir/globals"
	"github.com/yaroher/protoc-gen-go-plain/generator/ir/transform/message"
	"google.golang.org/protobuf/compiler/protogen"
)

type TransformationType string

const (
	TransformationTypeAddVirtualMessage TransformationType = "add_virtual_message"
)

type Transformation struct {
	TransformationType TransformationType
	AddVirtualMessage  *AddVirtualMessage
}

type File struct {
	OriginalFile    *protogen.File
	Messages        []*message.Message
	Transformations []*Transformation
}

//func NewFileFromProtogenFile(
//	plugin *protogen.Plugin,
//	file *protogen.File,
//	suffix string,
//) (*File, error) {
//	return &File{
//		Plugin:          plugin,
//		OriginalFile:    file,
//		Messages:        make([]*Message, 0),
//		VirtualMessages: make([]*typepb.Type, 0),
//	}, nil
//}

func (f *File) Render() error {
	targetFile := globals.Plugin().NewGeneratedFile(
		f.OriginalFile.GeneratedFilenamePrefix+".pb.go_plain.go",
		f.OriginalFile.GoImportPath,
	)
	for _, msg := range f.Messages {
		if err := msg.Render(targetFile); err != nil {
			return err
		}
	}
	return nil
}
