package generator

import (
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-plain/plain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func strToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func FoundFieldInPlugin(plugin *protogen.Plugin, field *protogen.Field) *protogen.Field {
	for _, f := range plugin.Files {
		for _, m := range f.Messages {
			if found := foundFieldInMessage(m, field); found != nil {
				return found
			}
		}
	}
	return nil
}

// foundFieldInMessage рекурсивно ищет поле в сообщении и всех его вложенных сообщениях
func foundFieldInMessage(m *protogen.Message, field *protogen.Field) *protogen.Field {
	// Проверяем поля текущего сообщения
	for _, f := range m.Fields {
		if f.Desc.FullName() == field.Desc.FullName() {
			return field
		}
	}

	// Рекурсивно проверяем все вложенные сообщения
	for _, nested := range m.Messages {
		if found := foundFieldInMessage(nested, field); found != nil {
			return found
		}
	}
	return nil
}

// Merge overrides from the plugin and the file-level overrides.
func mergeOverrides(overrides []*goplain.TypeOverride, newOverrides []*goplain.TypeOverride) []*goplain.TypeOverride {
	selectorEq := func(a, b *goplain.OverrideSelector) bool {
		if a == nil || b == nil {
			return false
		}

		// Если оба селектора используют TargetFullPath, сравниваем только по нему
		aPath := a.GetTargetFullPath()
		bPath := b.GetTargetFullPath()
		if aPath != "" && bPath != "" {
			return aPath == bPath
		}

		// Иначе сравниваем по полям Kind, Cardinality и TypeUrl
		kindMatch := a.GetFieldKind() == b.GetFieldKind()
		cardMatch := a.GetFieldCardinality() == b.GetFieldCardinality()
		typeUrlMatch := a.GetFieldTypeUrl() == b.GetFieldTypeUrl()
		return kindMatch && cardMatch && typeUrlMatch
	}

	for _, newOverride := range newOverrides {
		found := false
		for _, override := range overrides {
			if selectorEq(override.Selector, newOverride.Selector) {
				found = true
				break
			}
		}
		if !found {
			overrides = append(overrides, newOverride)
		}
	}

	return overrides
}

// getPluginOverrides returns all type overrides defined in the plugin.
func getPluginOverrides(plugin *protogen.Plugin) []*goplain.TypeOverride {
	var overrides []*goplain.TypeOverride

	for _, f := range plugin.Files {
		fileOpts := f.Desc.Options().(*descriptorpb.FileOptions)
		fileOverrides := proto.GetExtension(fileOpts, goplain.E_File).(*goplain.FileOptions)

		if fileOverrides != nil {
			overrides = append(overrides, fileOverrides.GetGoTypesOverrides()...)
		}

		for _, m := range f.Messages {
			processMessageOverrides(m, &overrides)
		}
	}

	return overrides
}

// processMessageOverrides рекурсивно обрабатывает overrides для сообщения и всех его вложенных сообщений
func processMessageOverrides(m *protogen.Message, overrides *[]*goplain.TypeOverride) {
	for _, f := range m.Fields {
		fieldOpts := f.Desc.Options().(*descriptorpb.FieldOptions)

		if !proto.HasExtension(fieldOpts, goplain.E_Field) {
			continue
		}

		fieldOverride := proto.GetExtension(fieldOpts, goplain.E_Field).(*goplain.FieldOptions)
		if fieldOverride != nil && fieldOverride.GetOverrideType() != nil {
			*overrides = append(*overrides, &goplain.TypeOverride{
				Selector: &goplain.OverrideSelector{
					TargetFullPath: strToPtr(string(f.Desc.FullName())),
				},
				TargetGoType: fieldOverride.GetOverrideType(),
			})
		}
	}

	// Рекурсивно обрабатываем все вложенные сообщения
	for _, nested := range m.Messages {
		processMessageOverrides(nested, overrides)
	}
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
								fieldFullName := string(field.Desc.FullName())
								result[fieldFullName] = overrideType
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
				// Для каждого поля в embedded сообщении
				for _, embeddedField := range field.Message.Fields {
					embeddedFieldFullName := string(embeddedField.Desc.FullName())

					// Проверяем, есть ли override для этого embedded поля
					if override, found := overrides[embeddedFieldFullName]; found {
						// Добавляем override с новым именем после embed
						newFieldFullName := msgFullName + "." + string(embeddedField.Desc.Name())
						overrides[newFieldFullName] = override
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
