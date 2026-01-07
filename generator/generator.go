package generator

import (
	"google.golang.org/protobuf/compiler/protogen"
)

type Generator struct {
	Settings  *PluginSettings
	Plugin    *protogen.Plugin
	overrides *overrideRegistry
}

func NewGenerator(p *protogen.Plugin) (*Generator, error) {
	settings, err := NewPluginSettingsFromPlugin(p)
	if err != nil {
		return nil, err
	}
	return &Generator{
		Settings:  settings,
		Plugin:    p,
		overrides: newOverrideRegistry(settings.TypeOverrides),
	}, nil
}

func (g *Generator) Generate() error {
	for _, f := range g.Plugin.Files {
		if !f.Generate {
			continue
		}
		fg := newFileGen(g, f)
		fg.genFile()
	}
	return nil
}
