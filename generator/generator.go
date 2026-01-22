package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/generator/empath"
	"github.com/yaroher/protoc-gen-go-plain/generator/marker"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-go-plain/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/sourcecontextpb"
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

type ResultMessage struct {
}
type TypePbIR struct {
	File     *protogen.File
	Messages map[string]*typepb.Type
}

const (
	// ---------------------------------
	embedMarker  = "embed"
	prefixMarker = "prefix"
	trueVal      = "true"
	//----------------------------------
)

func (g *Generator) Collect() []*TypePbIR {
	result := make([]*TypePbIR, 0)
	l := logger.Logger.Named("Collect")
	for _, file := range g.Plugin.Files {
		if strings.Contains(string(file.Desc.FullName()), "goplain") ||
			strings.Contains(string(file.Desc.FullName()), "google.protobuf") {
			continue
		}

		l.Debug("file", zap.String("full_path", string(file.Desc.FullName())))
		newResult := &TypePbIR{
			File:     file,
			Messages: make(map[string]*typepb.Type),
		}
		fOpt := file.Desc.Options().(*descriptorpb.FileOptions)
		fGen := proto.GetExtension(fOpt, goplain.E_File).(*goplain.FileOptions)
		for _, vm := range fGen.GetVirtualTypes() {
			// TODO: VALIDATE
			mvApply := &typepb.Type{
				Name:    fmt.Sprintf("%s.%s", string(file.Desc.Package()), string(vm.Name)),
				Fields:  vm.Fields,
				Oneofs:  vm.Oneofs,
				Options: vm.Options,
				Syntax:  typepb.Syntax_SYNTAX_PROTO3,
				SourceContext: &sourcecontextpb.SourceContext{
					FileName: string(file.Desc.Package()) + "." + string(file.Desc.FullName()),
				},
			}
			newResult.Messages[vm.Name] = mvApply
			l.Debug("virtual_type", zap.String("full_path", vm.Name))
		}
		for _, message := range file.Messages {
			msgOpt := message.Desc.Options().(*descriptorpb.MessageOptions)
			msgGen := proto.GetExtension(msgOpt, goplain.E_Message).(*goplain.MessageOptions)
			//if !msgGen.GetGenerate() {
			//	continue
			//}

			resMessage := &typepb.Type{
				Name:    string(message.Desc.FullName()),
				Fields:  []*typepb.Field{},
				Oneofs:  []string{},
				Syntax:  typepb.Syntax_SYNTAX_PROTO3,
				Options: []*typepb.Option{},
				SourceContext: &sourcecontextpb.SourceContext{
					FileName: string(file.Desc.Package()) + "." + string(file.Desc.FullName()),
				},
			}
			for cnt, vf := range msgGen.GetVirtualFields() {
				// TODO: VALIDATE
				vfApply := &typepb.Field{
					Kind:         vf.Kind,
					Cardinality:  vf.Cardinality,
					Number:       int32(-(cnt + 1)), // virtual fields are negative for alignment
					Name:         vf.Name,
					JsonName:     stringOrDefault(vf.JsonName, strcase.ToLowerCamel(vf.Name)),
					DefaultValue: vf.DefaultValue,
					TypeUrl:      vf.TypeUrl,
					OneofIndex:   0,
					//Options: []*typepb.Option{
					//	{Name: fmt.Sprintf("generate=%t", msgGen.GetGenerate())},
					//},
					Packed: vf.Packed,
				}
				l.Debug(
					"virtual_field",
					zap.String("full_path", vfApply.Name),
					zap.Int32("number", vfApply.Number),
					zap.String("kind", vfApply.Kind.String()),
					zap.String("cardinality", vfApply.Cardinality.String()),
					zap.String("json_name", vfApply.JsonName),
				)
				resMessage.Fields = append(resMessage.Fields, vfApply)
			}
			l.Debug("message", zap.String("full_path", string(message.Desc.FullName())))
			for _, oneof := range message.Oneofs {
				oneOffOpts := oneof.Desc.Options().(*descriptorpb.OneofOptions)
				oneOffGen := proto.GetExtension(oneOffOpts, goplain.E_Oneof).(*goplain.OneofOptions)
				l.Debug(
					"oneof",
					zap.String("full_path", string(oneof.Desc.FullName())),
					zap.String("go_name", oneof.GoName),
					zap.Bool("generate", oneOffGen.GetEmbed()),
					zap.Bool("generate_prefix", oneOffGen.GetEmbedWithPrefix()),
				)
				oneoffName := string(oneof.Desc.FullName())
				if oneOffGen.GetEmbed() {
					oneoffName = marker.New(oneoffName, map[string]string{embedMarker: trueVal}).String()
				} else if oneOffGen.GetEmbedWithPrefix() {
					oneoffName = marker.New(oneoffName, map[string]string{embedMarker: trueVal, prefixMarker: trueVal}).String()
				}
				resMessage.Oneofs = append(resMessage.Oneofs, oneoffName)
			}
			for _, field := range message.Fields {
				fieldOpt := field.Desc.Options().(*descriptorpb.FieldOptions)
				fieldGen := proto.GetExtension(fieldOpt, goplain.E_Field).(*goplain.FieldOptions)
				var fieldOptions []*typepb.Option
				//for _, opt := range field.Desc.Options() {
				//	fieldOptions = &typepb.Option{
				//		Name:  opt.Get(),
				//		path: opt.GetValue(),
				//	}
				//}
				newField := &typepb.Field{
					Kind:         typepb.Field_Kind(field.Desc.Kind()),
					Cardinality:  typepb.Field_Cardinality(field.Desc.Cardinality()),
					Number:       int32(field.Desc.Number()),
					Name:         string(field.Desc.FullName()),
					Options:      fieldOptions,
					Packed:       field.Desc.IsPacked(),
					JsonName:     field.Desc.JSONName(),
					DefaultValue: field.Desc.Default().String(),
				}
				logOpts := []zap.Field{
					zap.String("full_path", string(field.Desc.FullName())),
					zap.String("go_name", field.GoName),
				}
				if field.Oneof != nil {
					logOpts = append(
						logOpts,
						zap.String("from_oneoff", string(field.Oneof.Desc.FullName())),
						zap.Int32("oneof_index", int32(field.Desc.ContainingOneof().Index())),
					)
					for idx, oneof := range message.Oneofs {
						if oneof.Desc.FullName() == field.Oneof.Desc.FullName() {
							newField.OneofIndex = int32(idx + 1)
							break
						}
					}
					if newField.OneofIndex == 0 {
						panic("oneof not found")
					}
				}
				if field.Message != nil {
					logOpts = append(logOpts, zap.String("is_message", string(field.Message.Desc.FullName())))
					fieldName := string(field.Message.Desc.FullName())
					if fieldGen.GetEmbed() {
						fieldName = marker.New(fieldName, map[string]string{embedMarker: trueVal}).String()
					} else if fieldGen.GetEmbedWithPrefix() {
						fieldName = marker.New(fieldName, map[string]string{embedMarker: trueVal, prefixMarker: trueVal}).String()
					}
					newField.TypeUrl = fieldName
				}
				l.Debug("field", logOpts...)
				resMessage.Fields = append(resMessage.Fields, newField)
			}
			newResult.Messages[string(message.Desc.FullName())] = resMessage
		}
		result = append(result, newResult)
	}
	return result
}

const (
	isOneoffedMarker = "is_oneoff"
	isMessageMarker  = "is_message"
)

func (g *Generator) processEmbedOneof(msg *typepb.Type) {
	//l := logger.Logger.Named("processEmbedOneof")
	newOneOffs := make([]string, 0)
	for oldIdx, oneoff := range msg.Oneofs {
		if !marker.Parse(oneoff).HasMarker(embedMarker) {
			for _, field := range msg.Fields {
				if field.OneofIndex != 0 && field.OneofIndex == int32(oldIdx+1) {
					field.OneofIndex = int32(len(newOneOffs) + 1)
				}
			}
			newOneOffs = append(newOneOffs, oneoff)
		}
		if marker.Parse(oneoff).HasMarker(embedMarker) {
			for _, field := range msg.Fields {
				if field.OneofIndex == int32(oldIdx+1) {
					field.OneofIndex = 0
					oneoffPath := empath.New(marker.Parse(oneoff).AddMarker(isOneoffedMarker, trueVal))
					field.TypeUrl = oneoffPath.Append(marker.Parse(field.TypeUrl)).String()
				}
			}
		}
	}
	msg.Oneofs = newOneOffs
}

func (g *Generator) processEmbeddedMessages(ir *TypePbIR, msg *typepb.Type) {
	l := logger.Logger.Named("processEmbeddedMessages")
	foundMessage := func(typeUrl string) (*typepb.Type, bool) {
		for _, m := range ir.Messages {
			if empath.Parse(m.Name).Last().Value() == empath.Parse(typeUrl).Last().Value() {
				return m, true
			}
		}
		return nil, false
	}
	for _, field := range msg.Fields {
		if empath.Parse(field.TypeUrl).Last().HasMarker(embedMarker) {
			msgType, ok := foundMessage(field.TypeUrl)
			if !ok {
				panic("message not found")
			}
			l.Debug(
				"found_message",
				zap.String("full_path", msgType.Name),
				zap.String("for_field", field.Name),
			)
			for _, f := range msgType.Fields {
				msg.Fields = append(msg.Fields, f)
			}
			//field.TypeUrl = msgType.Name
		}
	}
}

func (g *Generator) ProcessOneoffs(typeIRs []*TypePbIR) []*TypePbIR {
	for _, ir := range typeIRs {
		for _, msg := range ir.Messages {
			g.processEmbedOneof(msg)
			g.processEmbeddedMessages(ir, msg)
		}
	}
	return typeIRs
}

func (g *Generator) Generate() error {
	collected := g.Collect()
	g.writeFile("collected", collected)
	oneoffs := g.ProcessOneoffs(collected)
	g.writeFile("oneoffs", oneoffs)
	return nil
}

func (g *Generator) writeFile(name string, typeIRs []*TypePbIR) {
	for _, r := range typeIRs {
		jsonMessages, err := json.MarshalIndent(struct {
			Messages map[string]*typepb.Type `json:"messages"`
		}{Messages: r.Messages}, "", "  ")
		if err != nil {
			panic(err)
		}
		os.WriteFile(
			"bin/json/"+name+".json",
			jsonMessages,
			0644,
		)
	}
}
