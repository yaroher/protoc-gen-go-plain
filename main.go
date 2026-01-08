package main

import (
	"github.com/yaroher/protoc-gen-go-plain/generator"
	"github.com/yaroher/protoc-gen-go-plain/ir"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

func Generate(p *protogen.Plugin) error {
	g, err := generator.NewGenerator(p)
	if err != nil {
		return err
	}
	return g.Generate()
}

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		converted, err := ir.ConvertPluginDescriptor(plugin)
		if err != nil {
			return err
		}
		if converted != nil {
			converted.SupportedFeatures = plugin.SupportedFeatures
			return Generate(converted)
		}
		return Generate(plugin)
	})
}
