package generator

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-plain/converter"
	"github.com/yaroher/protoc-gen-plain/logger"
	"github.com/yaroher/protoc-gen-plain/plain"
	"go.uber.org/zap"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
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
	g.overrides = mergeOverrides(g.overrides, getPluginOverrides(g.Plugin))
	return g, nil
}

func (g *Generator) GetOverrides() []*goplain.TypeOverride {
	return g.overrides
}

func (g *Generator) AddOverride(override *goplain.TypeOverride) {
	g.overrides = append(g.overrides, override)
}

// buildTypeAliasOverrides собирает overrides из type alias полей ДО конверсии
func buildTypeAliasOverrides(plugin *protogen.Plugin) map[string]*goplain.GoIdent {
	result := make(map[string]*goplain.GoIdent)

	for _, file := range plugin.Files {
		for _, msg := range file.Messages {
			collectTypeAliasOverridesFromMessage(msg, result)
		}
	}

	// Добавляем overrides для embedded полей
	for _, file := range plugin.Files {
		for _, msg := range file.Messages {
			collectEmbeddedFieldOverrides(msg, result)
		}
	}

	return result
}

// collectTypeAliasOverridesFromMessage рекурсивно собирает overrides из полей сообщения
func collectTypeAliasOverridesFromMessage(msg *protogen.Message, result map[string]*goplain.GoIdent) {
	for _, field := range msg.Fields {
		// Проверяем, является ли тип поля MESSAGE (потенциальный type alias)
		if field.Desc.Kind() == protoreflect.MessageKind && field.Message != nil {
			// Проверяем, является ли это type alias
			msgOpts := field.Message.Desc.Options().(*descriptorpb.MessageOptions)

			if proto.HasExtension(msgOpts, plain.E_Message) {
				plainMsgOpts := proto.GetExtension(msgOpts, plain.E_Message).(*plain.MessageOptions)
				if plainMsgOpts.GetTypeAlias() && len(field.Message.Fields) >= 1 {
					valueField := field.Message.Fields[0]
					if valueField.GoName == "Value" {
						valueOpts := valueField.Desc.Options().(*descriptorpb.FieldOptions)

						if proto.HasExtension(valueOpts, goplain.E_Field) {
							valueExtOpts := proto.GetExtension(valueOpts, goplain.E_Field).(*goplain.FieldOptions)
							if overrideType := valueExtOpts.GetOverrideType(); overrideType != nil {
								// Ключ: test.TestMessage.id
								fieldFullName := string(field.Desc.FullName())
								result[fieldFullName] = overrideType

								logger.Debug("Collected type alias override",
									zap.String("field", fieldFullName),
									zap.String("alias", string(field.Message.Desc.FullName())),
									zap.String("type", overrideType.GetName()))
							}
						}
					}
				}
			}
		}
	}

	// Рекурсивно обрабатываем вложенные сообщения
	for _, nested := range msg.Messages {
		collectTypeAliasOverridesFromMessage(nested, result)
	}
}

// collectEmbeddedFieldOverrides добавляет overrides для embedded полей
func collectEmbeddedFieldOverrides(msg *protogen.Message, overrides map[string]*goplain.GoIdent) {
	msgFullName := string(msg.Desc.FullName())

	for _, field := range msg.Fields {
		// Проверяем, есть ли у поля опция embed
		fieldOpts := field.Desc.Options().(*descriptorpb.FieldOptions)
		if proto.HasExtension(fieldOpts, plain.E_Field) {
			plainFieldOpts := proto.GetExtension(fieldOpts, plain.E_Field).(*plain.FieldOptions)
			if plainFieldOpts.GetEmbed() && field.Message != nil {
				logger.Debug("Found embed field",
					zap.String("parent", msgFullName),
					zap.String("field", field.GoName),
					zap.String("embedType", string(field.Message.Desc.FullName())))

				// Для каждого поля в embedded сообщении
				for _, embeddedField := range field.Message.Fields {
					embeddedFieldFullName := string(embeddedField.Desc.FullName())

					// Проверяем, есть ли override для этого embedded поля
					if override, found := overrides[embeddedFieldFullName]; found {
						// Добавляем override с новым именем после embed
						// test.TestMessage.embed_id (вместо test.EmbedWithAlias.embed_id)
						newFieldFullName := msgFullName + "." + string(embeddedField.Desc.Name())
						overrides[newFieldFullName] = override

						logger.Debug("Added override for embedded field",
							zap.String("original", embeddedFieldFullName),
							zap.String("embedded_as", newFieldFullName),
							zap.String("type", override.GetName()))
					}
				}
			}
		}
	}

	// Рекурсивно обрабатываем вложенные сообщения
	for _, nested := range msg.Messages {
		collectEmbeddedFieldOverrides(nested, overrides)
	}
}

func (g *Generator) Generate() error {
	// Собираем type alias overrides ДО конверсии
	typeAliasOverrides := buildTypeAliasOverrides(g.Plugin)
	logger.Debug("Built type alias overrides", zap.Int("count", len(typeAliasOverrides)))

	newPlugin, err := converter.Convert(g.Plugin, converter.WithPlainSuffix(g.suffix))
	if err != nil {
		return err
	}
	logger.Debug("Transformed plugin", zap.Any("plugin", g.Plugin.Request.GetParameter()))

	for _, g := range g.overrides {
		logger.Debug(
			"Override type alias",
			zap.String("target_type", g.GetTargetGoType().GetName()),
			zap.String("selector", g.GetSelector().String()),
		)
	}

	logger.Debug("Starting file generation", zap.Int("files_count", len(newPlugin.Files)))
	for _, fd := range newPlugin.Files {
		logger.Debug("Processing file", zap.String("name", fd.Desc.Path()))
		plainFile := g.Plugin.NewGeneratedFile(fd.GeneratedFilenamePrefix+".pb.go_plain.go", fd.GoImportPath)
		plainFile.P("// Code generated by protoc-gen-go-plain. DO NOT EDIT.\n\n")
		plainFile.P("package " + fd.GoPackageName + "\n\n")
		genCounter := 0

		for _, m := range fd.Messages {
			msgOpts := m.Desc.Options().(*descriptorpb.MessageOptions)
			msgGenerate := proto.GetExtension(msgOpts, goplain.E_Message).(*goplain.MessageOptions)
			if !msgGenerate.GetGenerate() {
				continue
			}
			genCounter++
			logger.Debug("Processing message", zap.String("name", m.GoIdent.GoName))

			plainFile.P("type " + m.GoIdent.GoName + " struct {\n")
			for _, f := range m.Fields {
				fieldType := getFieldGoTypeWithFile(plainFile, f, typeAliasOverrides)
				plainFile.P("\t" + f.GoName + " " + fieldType + "\n")
			}
			plainFile.P("}\n\n")
		}
		if genCounter == 0 {
			plainFile.Skip()
		}
	}
	return nil
}
