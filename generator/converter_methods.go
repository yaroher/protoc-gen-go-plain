package generator

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"github.com/yaroher/protoc-gen-plain/plain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FieldConversion содержит информацию о конверсии поля
type FieldConversion struct {
	PlainField      *protogen.Field // Поле в plain сообщении (после конверсии)
	PbField         *protogen.Field // Соответствующее поле в pb сообщении (до конверсии), может быть nil
	HasOverride     bool
	OverrideIdent   *goplain.GoIdent
	CasterParamName string // Имя параметра-конвертера для этого поля (дедуплицированное)
	CasterKey       string // Ключ для дедупликации (sourceType->targetType)

	// Дополнительная информация для сложных случаев
	IsTypeAlias  bool   // Поле является type alias
	IsEmbedded   bool   // Поле пришло из embedded сообщения
	EmbedPath    string // Путь к embedded полю (например, "Embed")
	IsSerialized bool   // Поле сериализовано в []byte
	IsVirtual    bool   // Поле виртуальное (не существует в pb)
	IsOneof      bool   // Поле из oneof
}

// CasterParam представляет параметр конвертера
type CasterParam struct {
	Name          string
	SourceType    string
	TargetType    string
	TargetGoIdent *goplain.GoIdent
}

// getFieldConversions возвращает информацию о полях, требующих конверсии
// Анализирует plain сообщение (msg) и оригинальное pb сообщение (pbMsg) для построения связей
func getFieldConversions(msg *protogen.Message, pbMsg *protogen.Message, typeAliasOverrides map[string]*goplain.GoIdent) []*FieldConversion {
	var conversions []*FieldConversion
	casterNames := make(map[string]string) // casterKey -> paramName

	// Строим мапу pb полей и анализируем их структуру
	pbFieldMap := buildPbFieldMap(pbMsg)

	for _, plainField := range msg.Fields {
		conv := &FieldConversion{
			PlainField: plainField,
		}

		// Пытаемся найти соответствующее pb поле
		pbFieldInfo := findPbFieldForPlainField(plainField, pbFieldMap, pbMsg)
		if pbFieldInfo != nil {
			conv.PbField = pbFieldInfo.Field
			conv.IsTypeAlias = pbFieldInfo.IsTypeAlias
			conv.IsEmbedded = pbFieldInfo.IsEmbedded
			conv.EmbedPath = pbFieldInfo.EmbedPath
			conv.IsSerialized = pbFieldInfo.IsSerialized
			conv.IsOneof = pbFieldInfo.IsOneof
		} else {
			// Поле не найдено в pb - возможно виртуальное
			conv.IsVirtual = true
		}

		// Проверяем наличие override
		if override := getFieldOverride(plainField, typeAliasOverrides); override != nil {
			conv.HasOverride = true
			conv.OverrideIdent = override

			// Определяем источник типа (из pb поля или type alias)
			var pbType string
			if conv.IsTypeAlias && pbFieldInfo != nil && pbFieldInfo.TypeAliasInnerType != "" {
				pbType = pbFieldInfo.TypeAliasInnerType
			} else {
				pbType = getFieldGoTypeString(plainField)
			}

			targetType := override.GetName()
			importPath := override.GetImportPath()
			conv.CasterKey = fmt.Sprintf("%s:%s:%s", pbType, targetType, importPath)

			// Проверяем, есть ли уже параметр для этого типа конверсии
			if paramName, exists := casterNames[conv.CasterKey]; exists {
				conv.CasterParamName = paramName
			} else {
				// Создаем новое уникальное имя используя strcase.ToLowerCamel
				baseName := strcase.ToLowerCamel(targetType) + "Caster"
				paramName := baseName
				counter := 1
				for _, existingKey := range casterNames {
					if existingKey == paramName {
						paramName = fmt.Sprintf("%s%d", baseName, counter)
						counter++
					}
				}
				casterNames[conv.CasterKey] = paramName
				conv.CasterParamName = paramName
			}
		}

		conversions = append(conversions, conv)
	}

	return conversions
}

// PbFieldInfo содержит информацию о pb поле
type PbFieldInfo struct {
	Field              *protogen.Field
	IsTypeAlias        bool
	TypeAliasInnerType string // Внутренний тип для type alias (например "string" для IdAlias)
	IsEmbedded         bool
	EmbedPath          string
	IsSerialized       bool // Поле сериализовано в []byte
	IsOneof            bool // Поле из oneof
}

// buildPbFieldMap строит мапу pb полей с анализом их структуры
func buildPbFieldMap(pbMsg *protogen.Message) map[string]*PbFieldInfo {
	result := make(map[string]*PbFieldInfo)

	for _, pbField := range pbMsg.Fields {
		info := &PbFieldInfo{
			Field: pbField,
		}

		// Проверяем является ли поле type alias
		if pbField.Desc.Kind() == protoreflect.MessageKind && pbField.Message != nil {
			msgOpts := pbField.Message.Desc.Options().(*descriptorpb.MessageOptions)
			if proto.HasExtension(msgOpts, plain.E_Message) {
				plainMsgOpts := proto.GetExtension(msgOpts, plain.E_Message).(*plain.MessageOptions)
				if plainMsgOpts.GetTypeAlias() && len(pbField.Message.Fields) >= 1 {
					info.IsTypeAlias = true
					info.TypeAliasInnerType = getFieldGoTypeString(pbField.Message.Fields[0])
				}
			}
		}

		// Проверяем опции поля (embedded, serialized, oneof)
		fieldOpts := pbField.Desc.Options().(*descriptorpb.FieldOptions)
		if proto.HasExtension(fieldOpts, plain.E_Field) {
			plainFieldOpts := proto.GetExtension(fieldOpts, plain.E_Field).(*plain.FieldOptions)

			// Проверяем serialized
			if plainFieldOpts.GetSerialize() {
				info.IsSerialized = true
			}

			// Проверяем embedded
			if plainFieldOpts.GetEmbed() {
				info.IsEmbedded = true
				info.EmbedPath = pbField.GoName

				// Добавляем все поля из embedded сообщения
				if pbField.Message != nil {
					for _, embeddedField := range pbField.Message.Fields {
						embeddedInfo := &PbFieldInfo{
							Field:      embeddedField,
							IsEmbedded: true,
							EmbedPath:  pbField.GoName,
						}

						// Проверяем является ли embedded поле type alias
						if embeddedField.Desc.Kind() == protoreflect.MessageKind && embeddedField.Message != nil {
							embMsgOpts := embeddedField.Message.Desc.Options().(*descriptorpb.MessageOptions)
							if proto.HasExtension(embMsgOpts, plain.E_Message) {
								plainEmbMsgOpts := proto.GetExtension(embMsgOpts, plain.E_Message).(*plain.MessageOptions)
								if plainEmbMsgOpts.GetTypeAlias() && len(embeddedField.Message.Fields) >= 1 {
									embeddedInfo.IsTypeAlias = true
									embeddedInfo.TypeAliasInnerType = getFieldGoTypeString(embeddedField.Message.Fields[0])
								}
							}
						}

						result[embeddedField.GoName] = embeddedInfo
					}
				}
			}
		}

		// Проверяем является ли поле частью oneof
		if pbField.Oneof != nil && !pbField.Oneof.Desc.IsSynthetic() {
			info.IsOneof = true
		}

		result[pbField.GoName] = info
	}

	return result
}

// findPbFieldForPlainField находит соответствующее pb поле для plain поля
func findPbFieldForPlainField(plainField *protogen.Field, pbFieldMap map[string]*PbFieldInfo, pbMsg *protogen.Message) *PbFieldInfo {
	// Сначала пробуем прямое совпадение по имени
	if info, ok := pbFieldMap[plainField.GoName]; ok {
		return info
	}

	// Если не нашли, возможно это embedded поле с префиксом
	// Например EmbedId -> найти Embed.EmbedId
	for _, info := range pbFieldMap {
		if info.IsEmbedded && strings.HasPrefix(plainField.GoName, info.EmbedPath) {
			// Убираем префикс и пробуем найти
			fieldNameWithoutPrefix := strings.TrimPrefix(plainField.GoName, info.EmbedPath)
			if embInfo, ok := pbFieldMap[fieldNameWithoutPrefix]; ok && embInfo.IsEmbedded {
				return embInfo
			}
		}
	}

	return nil
}

// generateConverterMethods генерирует методы конверсии для сообщения
func generateConverterMethods(g *protogen.GeneratedFile, msg *protogen.Message, pbMsg *protogen.Message, typeAliasOverrides map[string]*goplain.GoIdent) {
	conversions := getFieldConversions(msg, pbMsg, typeAliasOverrides)

	// Генерируем IntoPlain и IntoPlainErr
	generateIntoPlain(g, msg, pbMsg, conversions)
	generateIntoPlainErr(g, msg, pbMsg, conversions)

	// Генерируем IntoPb и IntoPbErr
	generateIntoPb(g, msg, pbMsg, conversions)
	generateIntoPbErr(g, msg, pbMsg, conversions)
}

// generateIntoPlain генерирует метод IntoPlain (из Pb в Plain)
func generateIntoPlain(g *protogen.GeneratedFile, msg *protogen.Message, pbMsg *protogen.Message, conversions []*FieldConversion) {
	castIdent := g.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Caster",
		GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
	})

	// Генерируем параметры для конвертеров
	casterParams := generateCasterParams(g, conversions, castIdent)
	paramsStr := strings.Join(casterParams, ", ")

	// Генерируем сигнатуру метода
	g.P("func (m *", pbMsg.GoIdent.GoName, ") IntoPlain(", paramsStr, ") *", msg.GoIdent.GoName, " {")
	g.P("if m == nil {")
	g.P("return nil")
	g.P("}")
	g.P("return &", msg.GoIdent.GoName, "{")

	// Генерируем конверсию полей
	for _, conv := range conversions {
		generateFieldConversionPbToPlain(g, conv)
	}

	g.P("}")
	g.P("}")
	g.P()
}

// generateIntoPlainErr генерирует метод IntoPlainErr
func generateIntoPlainErr(g *protogen.GeneratedFile, msg *protogen.Message, pbMsg *protogen.Message, conversions []*FieldConversion) {
	castIdent := g.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Caster",
		GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
	})

	casterParams := generateCasterParamsErr(g, conversions, castIdent)
	paramsStr := strings.Join(casterParams, ", ")

	g.P("func (m *", pbMsg.GoIdent.GoName, ") IntoPlainErr(", paramsStr, ") (*", msg.GoIdent.GoName, ", error) {")
	g.P("if m == nil {")
	g.P("return nil, nil")
	g.P("}")

	// Генерируем переменные для полей с caster'ами (которые могут возвращать ошибки)
	for _, conv := range conversions {
		if conv.HasOverride {
			pbValueExpr := generatePbFieldAccessExpr(g, conv)
			g.P(conv.PlainField.GoName, "Val, err := ", conv.CasterParamName, "(", pbValueExpr, ")")
			g.P("if err != nil {")
			g.P("return nil, err")
			g.P("}")
		}
	}

	g.P("return &", msg.GoIdent.GoName, "{")

	for _, conv := range conversions {
		if conv.HasOverride {
			// Используем уже сконвертированную переменную
			g.P(conv.PlainField.GoName, ": ", conv.PlainField.GoName, "Val,")
		} else {
			// Генерируем прямое присваивание
			pbValueExpr := generatePbFieldAccessExpr(g, conv)
			g.P(conv.PlainField.GoName, ": ", pbValueExpr, ",")
		}
	}

	g.P("}, nil")
	g.P("}")
	g.P()
}

// generateIntoPb генерирует метод IntoPb (из Plain в Pb)
func generateIntoPb(g *protogen.GeneratedFile, msg *protogen.Message, pbMsg *protogen.Message, conversions []*FieldConversion) {
	castIdent := g.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Caster",
		GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
	})

	casterParams := generateCasterParamsReverse(g, conversions, castIdent)
	paramsStr := strings.Join(casterParams, ", ")

	g.P("func (m *", msg.GoIdent.GoName, ") IntoPb(", paramsStr, ") *", pbMsg.GoIdent.GoName, " {")
	g.P("if m == nil {")
	g.P("return nil")
	g.P("}")

	// Генерируем embedded структуры
	embeddedStructs := generateEmbeddedStructs(g, pbMsg, conversions, false)
	for embedPath, embedStruct := range embeddedStructs {
		g.P(embedPath, " := ", embedStruct)
	}

	// Генерируем oneof переменные
	oneofVars := generateOneofVariables(g, pbMsg, conversions, false)

	g.P("return &", pbMsg.GoIdent.GoName, "{")

	// Генерируем обычные поля (не oneof, не embedded)
	for _, conv := range conversions {
		if !conv.IsEmbedded && !conv.IsOneof {
			generateFieldConversionPlainToPb(g, conv)
		}
	}

	// Добавляем embedded структуры
	for embedPath := range embeddedStructs {
		g.P(embedPath, ": ", embedPath, ",")
	}

	// Добавляем oneof переменные
	for oneofName, varName := range oneofVars {
		g.P(oneofName, ": ", varName, ",")
	}

	g.P("}")
	g.P("}")
	g.P()
}

// generateIntoPbErr генерирует метод IntoPbErr
func generateIntoPbErr(g *protogen.GeneratedFile, msg *protogen.Message, pbMsg *protogen.Message, conversions []*FieldConversion) {
	castIdent := g.QualifiedGoIdent(protogen.GoIdent{
		GoName:       "Caster",
		GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
	})

	casterParams := generateCasterParamsReverseErr(g, conversions, castIdent)
	paramsStr := strings.Join(casterParams, ", ")

	g.P("func (m *", msg.GoIdent.GoName, ") IntoPbErr(", paramsStr, ") (*", pbMsg.GoIdent.GoName, ", error) {")
	g.P("if m == nil {")
	g.P("return nil, nil")
	g.P("}")

	// Генерируем переменные для всех полей с caster'ами (которые могут возвращать ошибки)
	for _, conv := range conversions {
		if conv.HasOverride {
			plainValue := "m." + conv.PlainField.GoName
			g.P(conv.PlainField.GoName, "Val, err := ", conv.CasterParamName, "(", plainValue, ")")
			g.P("if err != nil {")
			g.P("return nil, err")
			g.P("}")
		}
	}

	// Генерируем embedded структуры (с обработкой ошибок для caster'ов)
	embeddedStructs := generateEmbeddedStructsWithErr(g, pbMsg, conversions)
	for embedPath, embedStruct := range embeddedStructs {
		g.P(embedPath, " := ", embedStruct)
	}

	// Генерируем oneof переменные
	oneofVars := generateOneofVariables(g, pbMsg, conversions, true)

	g.P("return &", pbMsg.GoIdent.GoName, "{")

	// Генерируем обычные поля (не oneof, не embedded)
	for _, conv := range conversions {
		if !conv.IsEmbedded && !conv.IsOneof {
			generateFieldConversionPlainToPbWithErr(g, conv)
		}
	}

	// Добавляем embedded структуры
	for embedPath := range embeddedStructs {
		g.P(embedPath, ": ", embedPath, ",")
	}

	// Добавляем oneof переменные
	for oneofName, varName := range oneofVars {
		g.P(oneofName, ": ", varName, ",")
	}

	g.P("}, nil")
	g.P("}")
	g.P()
}

// generateCasterParams генерирует параметры для конвертеров (Pb -> Plain) без ошибок
func generateCasterParams(g *protogen.GeneratedFile, conversions []*FieldConversion, castIdent string) []string {
	return generateCasterParamsInternal(g, conversions, castIdent, false, false)
}

// generateCasterParamsErr генерирует параметры для конвертеров с ошибками (Pb -> Plain)
func generateCasterParamsErr(g *protogen.GeneratedFile, conversions []*FieldConversion, castIdent string) []string {
	return generateCasterParamsInternal(g, conversions, castIdent, false, true)
}

// generateCasterParamsReverse генерирует параметры для конвертеров (Plain -> Pb) без ошибок
func generateCasterParamsReverse(g *protogen.GeneratedFile, conversions []*FieldConversion, castIdent string) []string {
	return generateCasterParamsInternal(g, conversions, castIdent, true, false)
}

// generateCasterParamsReverseErr генерирует параметры для конвертеров с ошибками (Plain -> Pb)
func generateCasterParamsReverseErr(g *protogen.GeneratedFile, conversions []*FieldConversion, castIdent string) []string {
	return generateCasterParamsInternal(g, conversions, castIdent, true, true)
}

// generateCasterParamsInternal генерирует параметры с дедупликацией
func generateCasterParamsInternal(g *protogen.GeneratedFile, conversions []*FieldConversion, castIdent string, reverse bool, useErr bool) []string {
	var params []string
	seen := make(map[string]bool)

	casterType := castIdent
	if useErr {
		// Заменяем "Caster" на "CasterErr"
		casterType = strings.Replace(castIdent, "Caster", "CasterErr", 1)
	}

	for _, conv := range conversions {
		if conv.HasOverride {
			// Проверяем дедупликацию по ключу
			if seen[conv.CasterKey] {
				continue
			}
			seen[conv.CasterKey] = true

			pbType := getFieldGoTypeString(conv.PlainField)
			plainType := g.QualifiedGoIdent(protogen.GoIdent{
				GoName:       conv.OverrideIdent.GetName(),
				GoImportPath: protogen.GoImportPath(conv.OverrideIdent.GetImportPath()),
			})

			if reverse {
				params = append(params, fmt.Sprintf("%s %s[%s, %s]",
					conv.CasterParamName, casterType, plainType, pbType))
			} else {
				params = append(params, fmt.Sprintf("%s %s[%s, %s]",
					conv.CasterParamName, casterType, pbType, plainType))
			}
		}
	}

	return params
}

// generateFieldConversionPbToPlain генерирует конверсию поля из Pb в Plain
func generateFieldConversionPbToPlain(g *protogen.GeneratedFile, conv *FieldConversion) {
	plainFieldName := conv.PlainField.GoName

	// Генерируем выражение для получения значения из pb
	pbValueExpr := generatePbFieldAccessExpr(g, conv)

	// Если есть override, применяем caster
	if conv.HasOverride {
		g.P(plainFieldName, ": ", conv.CasterParamName, "(", pbValueExpr, "),")
	} else {
		g.P(plainFieldName, ": ", pbValueExpr, ",")
	}
}

// generateFieldConversionPlainToPb генерирует конверсию поля из Plain в Pb
func generateFieldConversionPlainToPb(g *protogen.GeneratedFile, conv *FieldConversion) {
	pbField := conv.PbField

	// Пропускаем виртуальные поля
	if conv.IsVirtual {
		return
	}

	// Пропускаем embedded поля (они обрабатываются отдельно)
	if conv.IsEmbedded {
		return
	}

	// Генерируем выражение для конверсии значения
	plainValueExpr := generatePlainValueToPbExpr(g, conv)

	// Для oneof полей нужна специальная обработка
	if conv.IsOneof && pbField != nil {
		oneofWrapperType := fmt.Sprintf("&%s_%s{%s: %s}",
			pbField.Parent.GoIdent.GoName,
			pbField.GoName,
			pbField.GoName,
			plainValueExpr)
		g.P(pbField.GoName, ": ", oneofWrapperType, ",")
		return
	}

	// Обычное присваивание
	if pbField != nil {
		g.P(pbField.GoName, ": ", plainValueExpr, ",")
	}
}

// generateFieldConversionPlainToPbWithErr генерирует конверсию поля из Plain в Pb с обработкой ошибок
func generateFieldConversionPlainToPbWithErr(g *protogen.GeneratedFile, conv *FieldConversion) {
	plainFieldName := conv.PlainField.GoName
	pbField := conv.PbField

	// Пропускаем виртуальные поля
	if conv.IsVirtual {
		return
	}

	// Пропускаем embedded поля (они обрабатываются отдельно)
	if conv.IsEmbedded {
		return
	}

	var plainValueExpr string

	// Для полей с override используем уже преобразованное значение
	if conv.HasOverride {
		plainValueExpr = plainFieldName + "Val"

		// Но нужно применить type alias wrapper если это type alias
		if conv.IsTypeAlias && pbField != nil && pbField.Message != nil {
			typeName := g.QualifiedGoIdent(pbField.Message.GoIdent)
			plainValueExpr = fmt.Sprintf("&%s{Value: %s}", typeName, plainValueExpr)
		}
	} else {
		plainValueExpr = generatePlainValueToPbExpr(g, conv)
	}

	// Для oneof полей нужна специальная обработка
	if conv.IsOneof && pbField != nil {
		oneofWrapperType := fmt.Sprintf("&%s_%s{%s: %s}",
			pbField.Parent.GoIdent.GoName,
			pbField.GoName,
			pbField.GoName,
			plainValueExpr)
		g.P(pbField.GoName, ": ", oneofWrapperType, ",")
		return
	}

	// Обычное присваивание
	if pbField != nil {
		g.P(pbField.GoName, ": ", plainValueExpr, ",")
	}
}

// generateEmbeddedStructsWithErr генерирует создание embedded структур с обработкой ошибок
func generateEmbeddedStructsWithErr(g *protogen.GeneratedFile, pbMsg *protogen.Message, conversions []*FieldConversion) map[string]string {
	// Группируем поля по EmbedPath
	embedGroups := make(map[string][]*FieldConversion)
	embedTypes := make(map[string]*protogen.Message)

	for _, conv := range conversions {
		if conv.IsEmbedded && conv.EmbedPath != "" {
			embedGroups[conv.EmbedPath] = append(embedGroups[conv.EmbedPath], conv)
			// Находим тип embedded сообщения в pbMsg
			if embedTypes[conv.EmbedPath] == nil {
				for _, f := range pbMsg.Fields {
					if f.GoName == conv.EmbedPath && f.Message != nil {
						embedTypes[conv.EmbedPath] = f.Message
						break
					}
				}
			}
		}
	}

	result := make(map[string]string)

	for embedPath, fields := range embedGroups {
		embedType := embedTypes[embedPath]
		if embedType == nil {
			continue
		}

		typeName := g.QualifiedGoIdent(embedType.GoIdent)

		// Начинаем формировать структуру
		structExpr := fmt.Sprintf("&%s{", typeName)

		for _, conv := range fields {
			var plainValue string
			if conv.HasOverride {
				// Для полей с override используем уже преобразованное значение
				plainValue = conv.PlainField.GoName + "Val"

				// Но нужно применить type alias wrapper если это type alias
				if conv.IsTypeAlias && conv.PbField.Message != nil {
					typeName := g.QualifiedGoIdent(conv.PbField.Message.GoIdent)
					plainValue = fmt.Sprintf("&%s{Value: %s}", typeName, plainValue)
				}
			} else {
				plainValue = generatePlainValueToPbExpr(g, conv)
			}
			structExpr += fmt.Sprintf("%s: %s, ", conv.PbField.GoName, plainValue)
		}

		structExpr += "}"
		result[embedPath] = structExpr
	}

	return result
}

// generateEmbeddedStructs генерирует создание embedded структур
func generateEmbeddedStructs(g *protogen.GeneratedFile, pbMsg *protogen.Message, conversions []*FieldConversion, withError bool) map[string]string {
	// Группируем поля по EmbedPath
	embedGroups := make(map[string][]*FieldConversion)
	embedTypes := make(map[string]*protogen.Message)

	for _, conv := range conversions {
		if conv.IsEmbedded && conv.EmbedPath != "" {
			embedGroups[conv.EmbedPath] = append(embedGroups[conv.EmbedPath], conv)
			// Находим тип embedded сообщения в pbMsg
			if embedTypes[conv.EmbedPath] == nil {
				for _, f := range pbMsg.Fields {
					if f.GoName == conv.EmbedPath && f.Message != nil {
						embedTypes[conv.EmbedPath] = f.Message
						break
					}
				}
			}
		}
	}

	result := make(map[string]string)

	for embedPath, fields := range embedGroups {
		embedType := embedTypes[embedPath]
		if embedType == nil {
			continue
		}

		typeName := g.QualifiedGoIdent(embedType.GoIdent)

		// Начинаем формировать структуру
		structExpr := fmt.Sprintf("&%s{", typeName)

		for _, conv := range fields {
			plainValue := generatePlainValueToPbExpr(g, conv)
			structExpr += fmt.Sprintf("%s: %s, ", conv.PbField.GoName, plainValue)
		}

		structExpr += "}"
		result[embedPath] = structExpr
	}

	return result
}

// generateOneofVariables генерирует переменные для oneof полей и возвращает map (oneofName -> varName)
func generateOneofVariables(g *protogen.GeneratedFile, pbMsg *protogen.Message, conversions []*FieldConversion, withErr bool) map[string]string {
	// Группируем oneof поля по их oneof группе
	oneofGroups := make(map[string][]*FieldConversion)

	for _, conv := range conversions {
		if conv.IsOneof && conv.PbField != nil && conv.PbField.Oneof != nil {
			oneofName := conv.PbField.Oneof.GoName
			oneofGroups[oneofName] = append(oneofGroups[oneofName], conv)
		}
	}

	result := make(map[string]string)

	// Для каждой oneof группы генерируем переменную
	for oneofName, fields := range oneofGroups {
		varName := "oneof" + oneofName
		result[oneofName] = varName

		// Генерируем имя интерфейса oneof
		oneofInterfaceName := fmt.Sprintf("is%s_%s", pbMsg.GoIdent.GoName, oneofName)
		g.P("var ", varName, " ", oneofInterfaceName)

		for i, conv := range fields {
			plainFieldName := conv.PlainField.GoName
			pbFieldName := conv.PbField.GoName

			// Генерируем условие проверки (if для первого, else if для остальных)
			if i == 0 {
				g.P("if m.", plainFieldName, " != nil {")
			} else {
				g.P("} else if m.", plainFieldName, " != nil {")
			}

			// Генерируем значение
			var valueExpr string
			// Разыменовываем только для скалярных типов и enum с указателями
			if conv.PlainField.Desc.HasOptionalKeyword() && isScalarOrEnumType(conv.PlainField) {
				// Разыменовываем указатель для скалярных типов
				valueExpr = "*m." + plainFieldName
			} else {
				valueExpr = "m." + plainFieldName
			}

			// Генерируем wrapper
			wrapperType := fmt.Sprintf("%s_%s", pbMsg.GoIdent.GoName, pbFieldName)
			g.P(varName, " = &", wrapperType, "{", pbFieldName, ": ", valueExpr, "}")
		}
		g.P("}")
	}

	return result
}

// isPointerType проверяет, является ли тип поля указателем
func isPointerType(field *protogen.Field) bool {
	switch field.Desc.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return true
	default:
		return false
	}
}

// isScalarOrEnumType проверяет, является ли поле скалярным типом или enum
func isScalarOrEnumType(field *protogen.Field) bool {
	switch field.Desc.Kind() {
	case protoreflect.EnumKind:
		return true
	case protoreflect.BoolKind,
		protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind,
		protoreflect.StringKind:
		return true
	default:
		return false
	}
}

// generatePlainValueToPbExpr генерирует выражение для конверсии значения из Plain в Pb
func generatePlainValueToPbExpr(g *protogen.GeneratedFile, conv *FieldConversion) string {
	plainFieldName := conv.PlainField.GoName
	pbField := conv.PbField

	if pbField == nil {
		return "m." + plainFieldName
	}

	plainValue := "m." + plainFieldName

	// Применяем caster если есть override
	if conv.HasOverride {
		plainValue = fmt.Sprintf("%s(%s)", conv.CasterParamName, plainValue)
	}

	// Обрабатываем type alias - нужно создать структуру
	if conv.IsTypeAlias && pbField.Message != nil {
		typeName := g.QualifiedGoIdent(pbField.Message.GoIdent)
		return fmt.Sprintf("&%s{Value: %s}", typeName, plainValue)
	}

	// Обрабатываем serialized поля
	if conv.IsSerialized {
		castPkg := g.QualifiedGoIdent(protogen.GoIdent{
			GoName:       "",
			GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
		})

		msgTypeName := g.QualifiedGoIdent(pbField.Message.GoIdent)

		if pbField.Desc.IsList() {
			// repeated serialized: cast.MessageFromSliceByteSlice[*Message](plainValue)
			return fmt.Sprintf("%sMessageFromSliceByteSlice[*%s](%s)", castPkg, msgTypeName, plainValue)
		} else {
			// single serialized: cast.MessageFromSliceByte[*Message](plainValue)
			return fmt.Sprintf("%sMessageFromSliceByte[*%s](%s)", castPkg, msgTypeName, plainValue)
		}
	}

	// Обрабатываем optional поля - нужно развернуть указатель
	if conv.PlainField.Desc.HasOptionalKeyword() {
		// Для optional полей в pb используется Get паттерн, но при установке просто передаем значение
		return plainValue
	}

	return plainValue
}

// generatePbFieldAccessExpr генерирует выражение для доступа к pb полю
func generatePbFieldAccessExpr(g *protogen.GeneratedFile, conv *FieldConversion) string {
	plainField := conv.PlainField
	pbField := conv.PbField

	// Обрабатываем виртуальные поля
	if conv.IsVirtual || pbField == nil {
		return getZeroValue(plainField)
	}

	var expr string

	if conv.IsEmbedded {
		// Для embedded полей: m.GetEmbed().GetEmbedId()
		expr = fmt.Sprintf("m.Get%s().Get%s()", conv.EmbedPath, pbField.GoName)
	} else {
		// Обычный доступ: m.GetFieldName()
		expr = fmt.Sprintf("m.Get%s()", pbField.GoName)
	}

	// Если это type alias, добавляем .GetValue()
	if conv.IsTypeAlias {
		expr = fmt.Sprintf("%s.GetValue()", expr)
	}

	// Обрабатываем serialized поля
	if conv.IsSerialized {
		// Используем хелпер из cast для deserialization
		castPkg := g.QualifiedGoIdent(protogen.GoIdent{
			GoName:       "",
			GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
		})

		msgTypeName := g.QualifiedGoIdent(pbField.Message.GoIdent)

		if pbField.Desc.IsList() {
			// repeated serialized: cast.MessageToSliceByteSlice[*Message](m.GetField())
			expr = fmt.Sprintf("%sMessageToSliceByteSlice[*%s](%s)", castPkg, msgTypeName, expr)
		} else {
			// single serialized: cast.MessageToSliceByte[*Message](m.GetField())
			expr = fmt.Sprintf("%sMessageToSliceByte[*%s](%s)", castPkg, msgTypeName, expr)
		}
		return expr
	}

	// Обрабатываем optional/pointer поля
	if plainField.Desc.HasOptionalKeyword() {
		// Для optional скалярных полей и enum нужно создать указатель
		if isScalarType(plainField) || plainField.Desc.Kind() == protoreflect.EnumKind {
			castPkg := g.QualifiedGoIdent(protogen.GoIdent{
				GoName:       "",
				GoImportPath: "github.com/yaroher/protoc-gen-go-plain/cast",
			})
			expr = fmt.Sprintf("%sIntoPtr(%s)", castPkg, expr)
		}
	}

	return expr
}

// getZeroValue возвращает zero value для поля
func getZeroValue(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "false"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "0"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "0"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "0"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "0"
	case protoreflect.FloatKind:
		return "0.0"
	case protoreflect.DoubleKind:
		return "0.0"
	case protoreflect.StringKind:
		return `""`
	case protoreflect.BytesKind:
		return "nil"
	default:
		return "nil"
	}
}

// isScalarType проверяет является ли поле скалярным типом
func isScalarType(field *protogen.Field) bool {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind,
		protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind,
		protoreflect.StringKind:
		return true
	default:
		return false
	}
}

// getScalarTypeName возвращает имя скалярного типа
func getScalarTypeName(field *protogen.Field) string {
	switch field.Desc.Kind() {
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.FloatKind:
		return "float32"
	case protoreflect.DoubleKind:
		return "float64"
	case protoreflect.StringKind:
		return "string"
	default:
		return ""
	}
}

// getFieldGoTypeString возвращает Go тип поля как строку
func getFieldGoTypeString(field *protogen.Field) string {
	return getFieldGoType(field)
}

// getFieldOverride возвращает override для поля, если он есть
func getFieldOverride(field *protogen.Field, typeAliasOverrides map[string]*goplain.GoIdent) *goplain.GoIdent {
	// 1. Проверяем прямой override
	fieldOpts := field.Desc.Options().(*descriptorpb.FieldOptions)
	if proto.HasExtension(fieldOpts, goplain.E_Field) {
		fieldExtOpts := proto.GetExtension(fieldOpts, goplain.E_Field).(*goplain.FieldOptions)
		if overrideType := fieldExtOpts.GetOverrideType(); overrideType != nil {
			return overrideType
		}
	}

	// 2. Проверяем type alias override
	plainFieldName := string(field.Desc.FullName())
	origFieldName := strings.Replace(plainFieldName, "Plain.", ".", 1)

	if origOverride, found := typeAliasOverrides[origFieldName]; found {
		return origOverride
	}

	return nil
}
