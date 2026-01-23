package generator

import (
	"encoding/json"
	"os"

	"github.com/yaroher/protoc-gen-go-plain/crf"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/logger"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/known/typepb"
)

type Generator struct {
	Settings *PluginSettings
	Plugin   *protogen.Plugin
	suffix   string

	overrides []*goplain.TypeOverride
}

type Option func(*Generator) error

func WithPlainSuffix(suffix string) Option {
	return func(g *Generator) error {
		g.suffix = suffix
		return nil
	}
}

func WithTypeOverrides(overrides []*goplain.TypeOverride) Option {
	return func(g *Generator) error {
		g.overrides = overrides
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
		suffix:   "Plain",
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(g); err != nil {
			return nil, err
		}
	}
	return g, nil
}

func (g *Generator) GetOverrides() []*goplain.TypeOverride {
	return g.overrides
}

func (g *Generator) AddOverride(override *goplain.TypeOverride) {
	g.overrides = append(g.overrides, override)
}

func (g *Generator) Generate() error {
	logger.Info("generate start")
	for _, file := range g.Plugin.Files {
		if opts, ok := fileOptions(file.Desc.Options()); ok {
			for _, override := range opts.GetGoTypesOverrides() {
				g.AddOverride(override)
			}
		}
	}

	builder := newBuilder(g)
	builder.collectSymbols()
	if err := builder.collectAliases(); err != nil {
		return err
	}
	builder.markGeneratedMessages()

	var typeIRs []*TypePbIR

	for _, file := range g.Plugin.Files {
		if !file.Generate {
			continue
		}
		typeIR := &TypePbIR{
			File:     file,
			Messages: make(map[string]*TypeWrapper),
		}

		if opts, ok := fileOptions(file.Desc.Options()); ok {
			for _, virtual := range opts.GetVirtualTypes() {
				if virtual == nil {
					continue
				}
				wrapper := &TypeWrapper{Type: virtual}
				typeIR.Messages[virtual.GetName()] = wrapper
			}
		}

		for _, msg := range file.Messages {
			if err := g.buildMessageIR(builder, typeIR, msg); err != nil {
				return err
			}
		}

		if len(typeIR.Messages) == 0 {
			continue
		}

		typeIRs = append(typeIRs, typeIR)
		filename := file.GeneratedFilenamePrefix + "_plain.pb.go"
		genFile := g.Plugin.NewGeneratedFile(filename, file.GoImportPath)
		ctx := &renderContext{
			builder: builder,
			file:    file,
			g:       genFile,
		}
		if err := g.renderFile(ctx, typeIR); err != nil {
			return err
		}
	}

	if len(typeIRs) > 0 {
		g.WriteFile("ir", typeIRs)
	}

	return nil
}

func (g *Generator) buildMessageIR(builder *Builder, typeIR *TypePbIR, msg *protogen.Message) error {
	if msg.Desc.IsMapEntry() {
		return nil
	}
	if wrapper, err := builder.buildMessageType(msg); err != nil {
		return err
	} else if wrapper != nil && wrapper.Type != nil {
		typeIR.Messages[wrapper.Type.GetName()] = wrapper
	}

	for _, child := range msg.Messages {
		if err := g.buildMessageIR(builder, typeIR, child); err != nil {
			return err
		}
	}
	return nil
}

func (g *Generator) WriteFile(name string, typeIRs []*TypePbIR) {
	var A []struct {
		FileName string                  `json:"fileName"`
		Messages map[string]*typepb.Type `json:"messages"`
		CRF      map[string]*crf.CRF     `json:"crf,omitempty"`
	}
	for _, r := range typeIRs {
		messages := make(map[string]*typepb.Type, len(r.Messages))
		crfMeta := make(map[string]*crf.CRF)
		for k, v := range r.Messages {
			if v == nil {
				continue
			}
			messages[k] = v.Type
			if v.CRF != nil {
				crfMeta[k] = v.CRF
			}
		}
		if len(crfMeta) == 0 {
			crfMeta = nil
		}
		A = append(A, struct {
			FileName string                  `json:"fileName"`
			Messages map[string]*typepb.Type `json:"messages"`
			CRF      map[string]*crf.CRF     `json:"crf,omitempty"`
		}{
			FileName: r.File.Desc.Path(),
			Messages: messages,
			CRF:      crfMeta,
		})
	}
	jsonMessages, err := json.MarshalIndent(A, "", "  ")
	if err != nil {
		panic(err)
	}
	os.WriteFile(
		"bin/json/"+name+".json",
		jsonMessages,
		0644,
	)
}
