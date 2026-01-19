package generator

import (
	"fmt"
	"strings"

	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-plain/logger"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
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
	logger.Info(fmt.Sprintf("mergeOverrides: existing=%d, new=%d", len(overrides), len(newOverrides)))

	selectorEq := func(a, b *goplain.OverrideSelector) bool {
		if a == nil || b == nil {
			return false
		}

		// Если оба селектора используют TargetFullPath, сравниваем только по нему
		aPath := a.GetTargetFullPath()
		bPath := b.GetTargetFullPath()
		if aPath != "" && bPath != "" {
			result := aPath == bPath
			logger.Info(fmt.Sprintf("    selectorEq (by path): a=%s b=%s result=%v", aPath, bPath, result))
			return result
		}

		// Иначе сравниваем по полям Kind, Cardinality и TypeUrl
		kindMatch := a.GetFieldKind() == b.GetFieldKind()
		cardMatch := a.GetFieldCardinality() == b.GetFieldCardinality()
		typeUrlMatch := a.GetFieldTypeUrl() == b.GetFieldTypeUrl()
		result := kindMatch && cardMatch && typeUrlMatch

		logger.Info(fmt.Sprintf("    selectorEq (by kind/card/url): kindMatch=%v cardMatch=%v typeUrlMatch=%v result=%v",
			kindMatch, cardMatch, typeUrlMatch, result))

		return result
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
			logger.Info(fmt.Sprintf("  Adding override for: %s", newOverride.Selector.GetTargetFullPath()))
			overrides = append(overrides, newOverride)
		} else {
			logger.Info(fmt.Sprintf("  Skipping duplicate override for: %s", newOverride.Selector.GetTargetFullPath()))
		}
	}

	logger.Info(fmt.Sprintf("mergeOverrides: result=%d", len(overrides)))
	return overrides
}

// getPluginOverrides returns all type overrides defined in the plugin.
func getPluginOverrides(plugin *protogen.Plugin) []*goplain.TypeOverride {
	logger.Info("=== getPluginOverrides started ===")
	var overrides []*goplain.TypeOverride

	logger.Info(fmt.Sprintf("Processing %d files", len(plugin.Files)))
	logger.Info(fmt.Sprintf("Plugin has %d files to generate", len(plugin.Files)))
	for _, f := range plugin.Files {
		logger.Info(fmt.Sprintf("File: %s, Generate=%v", f.Desc.Path(), f.Generate))
	}

	for _, f := range plugin.Files {
		logger.Info(fmt.Sprintf("Processing file: %s (Generate=%v)", f.Desc.Path(), f.Generate))

		fileOpts := f.Desc.Options().(*descriptorpb.FileOptions)
		fileOverrides := proto.GetExtension(fileOpts, goplain.E_File).(*goplain.FileOptions)

		logger.Info(fmt.Sprintf("File options: %v", fileOverrides))

		if fileOverrides != nil {
			logger.Info(fmt.Sprintf("Found %d file-level overrides", len(fileOverrides.GetGoTypesOverrides())))
			overrides = append(overrides, fileOverrides.GetGoTypesOverrides()...)
		} else {
			logger.Info("No file-level overrides found")
		}

		logger.Info(fmt.Sprintf("Processing %d messages in file", len(f.Messages)))
		for _, m := range f.Messages {
			processMessageOverrides(m, &overrides)
		}
	}

	logger.Info(fmt.Sprintf("=== getPluginOverrides finished, found %d total overrides ===", len(overrides)))
	return overrides
}

// processMessageOverrides рекурсивно обрабатывает overrides для сообщения и всех его вложенных сообщений
func processMessageOverrides(m *protogen.Message, overrides *[]*goplain.TypeOverride) {
	logger.Info(fmt.Sprintf("  Processing message: %s (fields: %d, nested: %d)", m.Desc.FullName(), len(m.Fields), len(m.Messages)))

	for _, f := range m.Fields {
		fieldOpts := f.Desc.Options().(*descriptorpb.FieldOptions)

		// Проверяем, есть ли расширение
		hasExt := proto.HasExtension(fieldOpts, goplain.E_Field)
		logger.Info(fmt.Sprintf("    Field: %s, hasExtension: %v", f.Desc.FullName(), hasExt))

		if !hasExt {
			continue
		}

		fieldOverride := proto.GetExtension(fieldOpts, goplain.E_Field).(*goplain.FieldOptions)
		logger.Info(fmt.Sprintf("    Field: %s, override value: %v, overrideType: %v",
			f.Desc.FullName(), fieldOverride, fieldOverride.GetOverrideType()))

		if fieldOverride != nil {
			logger.Info(fmt.Sprintf("    ✓ Found field override for: %s -> %s.%s",
				f.Desc.FullName(),
				fieldOverride.GetOverrideType().GetImportPath(),
				fieldOverride.GetOverrideType().GetName()))
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

// buildFieldMap рекурсивно строит мапу полей до конверсии
// Ключ: будущее полное имя после конверсии (с Plain суффиксом)
// Значение: оригинальное полное имя до конверсии
func buildFieldMap(m *protogen.Message, fieldMap map[string]string) {
	// Текущее и будущее имена сообщения
	origMsgFullName := string(m.Desc.FullName())
	plainMsgFullName := origMsgFullName + "Plain"

	// Мапим все поля
	for _, f := range m.Fields {
		origFieldFullName := string(f.Desc.FullName())
		// Заменяем имя сообщения в полном имени поля
		plainFieldFullName := strings.Replace(origFieldFullName, origMsgFullName, plainMsgFullName, 1)
		fieldMap[plainFieldFullName] = origFieldFullName
	}

	// Рекурсивно обрабатываем вложенные сообщения
	for _, nested := range m.Messages {
		buildFieldMap(nested, fieldMap)
	}
}
