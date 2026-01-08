package generator

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
)

type Generator struct {
	Settings  *PluginSettings
	Plugin    *protogen.Plugin
	overrides *overrideRegistry

	virtualFields   []*goplain.VirtualField
	virtualMessages []*goplain.VirtualMessage
}

type Option func(*Generator) error

func WithOverrides(overrides ...*goplain.OverwriteType) Option {
	return func(g *Generator) error {
		g.Settings.TypeOverrides = append(g.Settings.TypeOverrides, overrides...)
		return nil
	}
}

func WithPlainSuffix(suffix string) Option {
	return func(g *Generator) error {
		g.Settings.PlainSuffix = suffix
		return nil
	}
}

func WithVirtualFields(fields ...*goplain.VirtualField) Option {
	return func(g *Generator) error {
		g.virtualFields = append(g.virtualFields, fields...)
		return nil
	}
}

func WithVirtualMessages(msgs ...*goplain.VirtualMessage) Option {
	return func(g *Generator) error {
		g.virtualMessages = append(g.virtualMessages, msgs...)
		return nil
	}
}

func NewGenerator(p *protogen.Plugin, opts ...Option) (*Generator, error) {
	settings, err := NewPluginSettingsFromPlugin(p)
	if err != nil {
		return nil, err
	}
	g := &Generator{
		Settings: settings,
		Plugin:   p,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(g); err != nil {
			return nil, err
		}
	}
	g.overrides = newOverrideRegistry(g.Settings.TypeOverrides)
	return g, nil
}

func (g *Generator) Generate() error {
	for _, f := range g.Plugin.Files {
		if !f.Generate {
			continue
		}
		model, err := g.BuildModel(f)
		if err != nil {
			return err
		}
		fg := newFileGen(g, f, model)
		fg.genFile()
	}
	return nil
}
