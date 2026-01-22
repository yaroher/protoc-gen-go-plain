package ir

import (
	"github.com/yaroher/protoc-gen-go-plain/generator/ir/transform/file"
)

type IR struct {
	Files []*file.File
}

//func NewIrFromPlugin(plugin *protogen.Plugin, suffix string) (*IR, error) {
//	globals.SetPlugin(plugin)
//	globals.SetSuffix(suffix)
//	return &IR{
//		Plugin: plugin,
//		Files:  make([]*File, 0),
//	}, nil
//
//}

func (i *IR) Render() error {
	for _, f := range i.Files {
		if err := f.Render(); err != nil {
			return err
		}
	}
	return nil
}
