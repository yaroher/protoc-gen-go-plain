package generator

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/yaroher/protoc-gen-go-plain/goplain"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/typepb"
)

// IRBuilder строит IR из protogen
type IRBuilder struct {
	// Suffix для plain-структур (по умолчанию "Plain")
	Suffix string
	// GlobalOverrides — глобальные переопределения типов
	GlobalOverrides []*goplain.TypeOverride
	// Collisions — найденные коллизии
	Collisions []Collision
	// Errors — ошибки валидации
	Errors []error

	// nextFieldNumber — счётчик для нумерации полей
	nextFieldNumber int32
	// fieldNames — имена полей текущего сообщения (для проверки коллизий)
	fieldNames map[string]*IRField
}

// NewIRBuilder создаёт новый IRBuilder
func NewIRBuilder(suffix string) *IRBuilder {
	if suffix == "" {
		suffix = "Plain"
	}
	return &IRBuilder{
		Suffix:     suffix,
		fieldNames: make(map[string]*IRField),
	}
}

// BuildFile строит IRFile из protogen.File
func (b *IRBuilder) BuildFile(f *protogen.File) (*IRFile, error) {
	irFile := &IRFile{
		Source:   f,
		Messages: make([]*IRMessage, 0),
		Imports:  make([]GoImport, 0),
	}

	// Получаем file-level опции
	fileOpts := b.getFileOptions(f)
	if fileOpts != nil {
		b.GlobalOverrides = append(b.GlobalOverrides, fileOpts.GoTypesOverrides...)

		// Обрабатываем virtual_types
		for _, vt := range fileOpts.VirtualTypes {
			irMsg := b.BuildVirtualType(vt, f)
			if irMsg != nil {
				irFile.Messages = append(irFile.Messages, irMsg)
			}
		}
	}

	// Обрабатываем все сообщения
	for _, msg := range f.Messages {
		irMsg, err := b.BuildMessage(msg, "")
		if err != nil {
			return nil, err
		}
		if irMsg != nil {
			irFile.Messages = append(irFile.Messages, irMsg)
		}
	}

	return irFile, nil
}

// BuildMessage строит IRMessage из protogen.Message
func (b *IRBuilder) BuildMessage(msg *protogen.Message, parentEmPath string) (*IRMessage, error) {
	// Проверяем, нужно ли генерировать это сообщение
	// Генерируем ТОЛЬКО если явно указано generate = true
	msgOpts := b.getMessageOptions(msg)
	if msgOpts == nil || !msgOpts.Generate {
		return nil, nil
	}

	// Сбрасываем состояние для нового сообщения
	b.nextFieldNumber = 1
	b.fieldNames = make(map[string]*IRField)

	irMsg := &IRMessage{
		Source:         msg,
		Name:           string(msg.Desc.Name()) + b.Suffix,
		GoName:         msg.GoIdent.GoName + b.Suffix,
		Fields:         make([]*IRField, 0),
		OriginalFields: msg.Fields,
		Nested:         make([]*IRMessage, 0),
		Comment:        string(msg.Comments.Leading),
		EmPath:         parentEmPath,
	}

	// Проверяем type_alias
	if msgOpts != nil && msgOpts.TypeAlias {
		// type_alias сообщение не генерируем как отдельную структуру
		// оно будет развёрнуто при использовании
		return nil, nil
	}

	// Обрабатываем обычные поля (не в oneof)
	for _, field := range msg.Fields {
		if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
			// Пропускаем поля из oneof — они обрабатываются отдельно
			continue
		}

		irFields, err := b.processField(field, irMsg, "", nil)
		if err != nil {
			return nil, err
		}
		for _, f := range irFields {
			b.addField(irMsg, f)
		}
	}

	// Обрабатываем oneof
	for _, oneof := range msg.Oneofs {
		if oneof.Desc.IsSynthetic() {
			// Synthetic oneof — это proto3 optional, обрабатывается как обычное поле
			continue
		}

		oneofOpts := b.getOneofOptions(oneof)
		if oneofOpts != nil && oneofOpts.Embed {
			// Создаём информацию о embedded oneof
			embeddedOneof := &EmbeddedOneof{
				Name:          string(oneof.Desc.Name()),
				GoName:        oneof.GoName,
				CaseFieldName: oneof.GoName + "Case",
				JSONName:      string(oneof.Desc.Name()) + "_case",
				Variants:      make([]*OneofVariant, 0, len(oneof.Fields)),
				Source:        oneof,
			}

			// Для oneof embed используем prefix: oneof_name + variant_name
			// чтобы избежать коллизий (многие варианты могут иметь одинаковые поля)
			basePrefix := string(oneof.Desc.Name())

			// Embed oneof — разворачиваем все варианты
			for _, field := range oneof.Fields {
				// Добавляем вариант в список
				embeddedOneof.Variants = append(embeddedOneof.Variants, &OneofVariant{
					Name:        string(field.Desc.Name()),
					GoName:      field.GoName,
					FieldNumber: int32(field.Desc.Number()),
				})

				// Prefix включает имя oneof и имя варианта для уникальности
				variantPrefix := basePrefix + "_" + string(field.Desc.Name())
				irFields, err := b.processField(field, irMsg, variantPrefix, nil)
				if err != nil {
					return nil, err
				}
				for _, f := range irFields {
					// Добавляем префикс oneof если нужно
					if f.Origin == OriginEmbed {
						f.Origin = OriginOneofEmbed
					}
					// Сохраняем имя oneof для использования в IntoPb
					f.OneofName = embeddedOneof.Name
					f.OneofGoName = embeddedOneof.GoName
					f.OneofVariant = string(field.Desc.Name())
					b.addField(irMsg, f)
				}
			}

			irMsg.EmbeddedOneofs = append(irMsg.EmbeddedOneofs, embeddedOneof)
		} else {
			// Обычный oneof — оставляем как есть (пока не поддерживаем)
			// TODO: добавить поддержку обычных oneof
			b.Errors = append(b.Errors, fmt.Errorf(
				"oneof %s in message %s: non-embedded oneofs are not yet supported",
				oneof.Desc.Name(), msg.Desc.Name(),
			))
		}
	}

	// Обрабатываем virtual_fields
	if msgOpts != nil {
		for _, vf := range msgOpts.VirtualFields {
			irField := b.buildVirtualField(vf, irMsg)
			b.addField(irMsg, irField)
		}
	}

	// Обрабатываем вложенные сообщения
	for _, nested := range msg.Messages {
		nestedIR, err := b.BuildMessage(nested, irMsg.EmPath)
		if err != nil {
			return nil, err
		}
		if nestedIR != nil {
			irMsg.Nested = append(irMsg.Nested, nestedIR)
		}
	}

	return irMsg, nil
}

// BuildVirtualType строит IRMessage из google.protobuf.Type (virtual type)
func (b *IRBuilder) BuildVirtualType(vt *typepb.Type, f *protogen.File) *IRMessage {
	if vt == nil || vt.Name == "" {
		return nil
	}

	// Сбрасываем состояние для нового сообщения
	b.nextFieldNumber = 1
	b.fieldNames = make(map[string]*IRField)

	// Имя типа: если содержит точку, берём последнюю часть
	typeName := vt.Name
	if idx := strings.LastIndex(typeName, "."); idx != -1 {
		typeName = typeName[idx+1:]
	}

	irMsg := &IRMessage{
		Source:         nil, // нет исходного protobuf сообщения
		Name:           typeName + b.Suffix,
		GoName:         strcase.ToCamel(typeName) + b.Suffix,
		Fields:         make([]*IRField, 0),
		OriginalFields: nil,
		Nested:         make([]*IRMessage, 0),
		Comment:        "// Virtual type: " + vt.Name,
		EmPath:         "",
		IsVirtual:      true,
	}

	// Обрабатываем поля virtual type
	for _, field := range vt.Fields {
		irField := b.buildVirtualField(field, irMsg)
		b.addField(irMsg, irField)
	}

	// Строим PathTable и присваиваем FieldIndex
	b.buildPathTable(irMsg)

	return irMsg
}

// buildPathTable строит таблицу путей и присваивает индексы полям
func (b *IRBuilder) buildPathTable(irMsg *IRMessage) {
	pathTable := make([]int32, 0)

	for i, field := range irMsg.Fields {
		field.FieldIndex = i

		if len(field.PathNumbers) > 0 {
			// Записываем индекс начала пути в PathTable
			pathIndex := len(pathTable)
			pathTable = append(pathTable, field.PathNumbers...)

			// Обновляем PathNumbers чтобы хранить только индекс и длину
			// (оригинальные номера теперь в PathTable)
			_ = pathIndex // PathIndex будет использоваться при генерации метаданных
		}
	}

	irMsg.PathTable = pathTable
}

// processField обрабатывает одно поле и возвращает список IR-полей
// (может вернуть несколько полей при embed)
// pathNumbers — путь номеров полей от корня до текущего уровня
func (b *IRBuilder) processField(field *protogen.Field, irMsg *IRMessage, oneofPrefix string, pathNumbers []int32) ([]*IRField, error) {
	fieldOpts := b.getFieldOptions(field)

	// Добавляем номер текущего поля к пути
	currentPath := append(pathNumbers, int32(field.Desc.Number()))

	// Проверяем взаимоисключающие опции
	if fieldOpts != nil {
		if fieldOpts.Embed && fieldOpts.Serialize {
			return nil, fmt.Errorf(
				"field %s.%s: embed and serialize are mutually exclusive",
				irMsg.Source.Desc.Name(), field.Desc.Name(),
			)
		}
	}

	// Проверяем type_alias
	if field.Message != nil {
		msgOpts := b.getMessageOptions(field.Message)
		if msgOpts != nil && msgOpts.TypeAlias {
			return b.processTypeAliasField(field, irMsg, oneofPrefix, msgOpts, currentPath)
		}
	}

	// serialize — сериализуем в bytes
	if fieldOpts != nil && fieldOpts.Serialize {
		return b.processSerializedField(field, irMsg, oneofPrefix, currentPath)
	}

	// embed — разворачиваем вложенное сообщение
	if fieldOpts != nil && fieldOpts.Embed {
		withPrefix := fieldOpts.EmbedWithPrefix
		return b.processEmbedField(field, irMsg, oneofPrefix, currentPath, withPrefix)
	}

	// Обычное поле
	return []*IRField{b.buildDirectField(field, oneofPrefix, currentPath)}, nil
}

// processTypeAliasField обрабатывает поле с type_alias сообщением
func (b *IRBuilder) processTypeAliasField(
	field *protogen.Field,
	irMsg *IRMessage,
	oneofPrefix string,
	msgOpts *goplain.MessageOptions,
	pathNumbers []int32,
) ([]*IRField, error) {
	// Находим поле-алиас в сообщении
	aliasFieldName := msgOpts.TypeAliasField
	if aliasFieldName == "" {
		aliasFieldName = "value"
	}

	var aliasField *protogen.Field
	for _, f := range field.Message.Fields {
		if string(f.Desc.Name()) == aliasFieldName {
			aliasField = f
			break
		}
	}

	if aliasField == nil {
		return nil, fmt.Errorf(
			"type_alias message %s does not have field %q",
			field.Message.Desc.Name(), aliasFieldName,
		)
	}

	// Добавляем номер поля алиаса к пути
	fullPath := append(pathNumbers, int32(aliasField.Desc.Number()))

	// Создаём поле с типом алиаса
	irField := &IRField{
		Source:         field,
		Name:           b.buildFieldName(field, oneofPrefix),
		GoName:         b.buildGoFieldName(field, oneofPrefix),
		JSONName:       b.buildJSONName(field, oneofPrefix),
		Number:         b.nextFieldNumber,
		OriginalNumber: int32(field.Desc.Number()),
		Kind:           b.kindFromProtoKind(aliasField.Desc.Kind()),
		ScalarKind:     aliasField.Desc.Kind(),
		GoType:         b.goTypeFromField(aliasField),
		Origin:         OriginTypeAlias,
		EmPath:         b.buildEmPath(irMsg.EmPath, field, oneofPrefix),
		PathNumbers:    copyPath(fullPath),
		IsRepeated:     field.Desc.IsList(),
		IsOptional:     field.Desc.HasOptionalKeyword(),
		Comment:        string(field.Comments.Leading),
	}

	b.nextFieldNumber++
	return []*IRField{irField}, nil
}

// processSerializedField обрабатывает поле с serialize=true
func (b *IRBuilder) processSerializedField(
	field *protogen.Field,
	irMsg *IRMessage,
	oneofPrefix string,
	pathNumbers []int32,
) ([]*IRField, error) {
	irField := &IRField{
		Source:         field,
		Name:           b.buildFieldName(field, oneofPrefix),
		GoName:         b.buildGoFieldName(field, oneofPrefix),
		JSONName:       b.buildJSONName(field, oneofPrefix),
		Number:         b.nextFieldNumber,
		OriginalNumber: int32(field.Desc.Number()),
		Kind:           KindBytes,
		GoType:         GoType{Name: "byte", IsSlice: true},
		ProtoType:      string(field.Message.Desc.FullName()),
		Origin:         OriginSerialized,
		EmPath:         b.buildEmPath(irMsg.EmPath, field, oneofPrefix),
		PathNumbers:    copyPath(pathNumbers),
		IsRepeated:     field.Desc.IsList(),
		IsOptional:     field.Desc.HasOptionalKeyword(),
		Comment:        string(field.Comments.Leading),
	}

	b.nextFieldNumber++
	return []*IRField{irField}, nil
}

// processEmbedField разворачивает вложенное сообщение
// withPrefix=true добавляет имя поля как префикс к именам вложенных полей
func (b *IRBuilder) processEmbedField(
	field *protogen.Field,
	irMsg *IRMessage,
	oneofPrefix string,
	pathNumbers []int32,
	withPrefix bool,
) ([]*IRField, error) {
	if field.Message == nil {
		return nil, fmt.Errorf(
			"field %s.%s: embed is only valid for message fields",
			irMsg.Source.Desc.Name(), field.Desc.Name(),
		)
	}

	if field.Desc.IsList() {
		return nil, fmt.Errorf(
			"field %s.%s: embed is not supported for repeated fields",
			irMsg.Source.Desc.Name(), field.Desc.Name(),
		)
	}

	var result []*IRField

	// Формируем префикс для полей только если withPrefix=true
	var prefix string
	if withPrefix {
		prefix = string(field.Desc.Name())
		if oneofPrefix != "" {
			prefix = oneofPrefix + "_" + prefix
		}
	} else if oneofPrefix != "" {
		// Если embed без префикса, но есть oneofPrefix — используем его
		prefix = oneofPrefix
	}

	// Разворачиваем все поля вложенного сообщения
	for _, nestedField := range field.Message.Fields {
		// Рекурсивно обрабатываем поля (они тоже могут иметь embed)
		// Передаём текущий путь — он уже содержит номер этого поля
		nestedFields, err := b.processField(nestedField, irMsg, prefix, pathNumbers)
		if err != nil {
			return nil, err
		}

		for _, nf := range nestedFields {
			nf.Origin = OriginEmbed
			result = append(result, nf)
		}
	}

	// TODO: обработать oneof внутри вложенного сообщения

	return result, nil
}

// buildDirectField создаёт IRField из обычного protogen.Field
func (b *IRBuilder) buildDirectField(field *protogen.Field, prefix string, pathNumbers []int32) *IRField {
	fieldOpts := b.getFieldOptions(field)

	irField := &IRField{
		Source:         field,
		Name:           b.buildFieldName(field, prefix),
		GoName:         b.buildGoFieldName(field, prefix),
		JSONName:       b.buildJSONName(field, prefix),
		Number:         b.nextFieldNumber,
		OriginalNumber: int32(field.Desc.Number()),
		Kind:           b.kindFromField(field),
		ScalarKind:     field.Desc.Kind(),
		GoType:         b.goTypeFromField(field),
		Origin:         OriginDirect,
		EmPath:         "", // Direct поля не имеют EmPath
		PathNumbers:    copyPath(pathNumbers),
		IsRepeated:     field.Desc.IsList(),
		IsOptional:     field.Desc.HasOptionalKeyword(),
		IsMap:          field.Desc.IsMap(),
		Comment:        string(field.Comments.Leading),
	}

	if prefix != "" {
		irField.Origin = OriginEmbed
		irField.EmPath = prefix + "." + string(field.Desc.Name())
	}

	// Обрабатываем enum опции
	if fieldOpts != nil {
		irField.EnumAsString = fieldOpts.EnumAsString
		irField.EnumAsInt = fieldOpts.EnumAsInt
	}

	// Обрабатываем override_type (field-level)
	if fieldOpts != nil && fieldOpts.OverrideType != nil {
		irField.GoType = GoType{
			Name:       fieldOpts.OverrideType.Name,
			ImportPath: fieldOpts.OverrideType.ImportPath,
		}
	}

	// Применяем GlobalOverrides (file-level)
	b.applyGlobalOverrides(field, irField)

	// Map поля
	if field.Desc.IsMap() {
		irField.Kind = KindMap
		irField.MapKey = &IRField{
			Kind:       b.kindFromProtoKind(field.Message.Fields[0].Desc.Kind()),
			ScalarKind: field.Message.Fields[0].Desc.Kind(),
			GoType:     b.goTypeFromField(field.Message.Fields[0]),
		}
		irField.MapValue = &IRField{
			Kind:       b.kindFromProtoKind(field.Message.Fields[1].Desc.Kind()),
			ScalarKind: field.Message.Fields[1].Desc.Kind(),
			GoType:     b.goTypeFromField(field.Message.Fields[1]),
		}
	}

	// Message поля
	if field.Message != nil && !field.Desc.IsMap() {
		irField.ProtoType = string(field.Message.Desc.FullName())
	}

	// Enum поля
	if field.Enum != nil {
		irField.ProtoType = string(field.Enum.Desc.FullName())
		// Меняем Go-тип если нужно
		if irField.EnumAsString {
			irField.GoType = GoType{Name: "string"}
			irField.Kind = KindScalar
		} else if irField.EnumAsInt {
			irField.GoType = GoType{Name: "int32"}
			irField.Kind = KindScalar
		}
	}

	b.nextFieldNumber++
	return irField
}

// buildVirtualField создаёт IRField из виртуального поля
func (b *IRBuilder) buildVirtualField(vf *typepb.Field, irMsg *IRMessage) *IRField {
	goType := b.goTypeFromProtoKind(protoreflect.Kind(vf.Kind))

	irField := &IRField{
		Source:         nil,
		Name:           vf.Name,
		GoName:         strcase.ToCamel(vf.Name),
		JSONName:       strcase.ToLowerCamel(vf.Name),
		Number:         b.nextFieldNumber,
		OriginalNumber: 0, // Виртуальные поля не имеют оригинального номера
		Kind:           b.kindFromProtoKind(protoreflect.Kind(vf.Kind)),
		ScalarKind:     protoreflect.Kind(vf.Kind),
		GoType:         goType,
		Origin:         OriginVirtual,
		EmPath:         "virtual",
		IsRepeated:     vf.Cardinality == typepb.Field_CARDINALITY_REPEATED,
		Comment:        "",
	}

	b.nextFieldNumber++
	return irField
}

// addField добавляет поле в сообщение с проверкой коллизий
// При коллизии поле не добавляется, коллизия записывается в b.Collisions
func (b *IRBuilder) addField(irMsg *IRMessage, field *IRField) {
	if existing, ok := b.fieldNames[field.Name]; ok {
		collision := Collision{
			FieldName:     field.Name,
			ExistingField: existing,
			NewField:      field,
			Message:       irMsg,
		}
		b.Collisions = append(b.Collisions, collision)
		return
	}

	// Назначаем индекс поля для _src
	field.Index = uint16(len(irMsg.Fields))

	b.fieldNames[field.Name] = field
	irMsg.Fields = append(irMsg.Fields, field)
}

// Вспомогательные методы

// copyPath создаёт копию slice путей
func copyPath(path []int32) []int32 {
	if len(path) == 0 {
		return nil
	}
	result := make([]int32, len(path))
	copy(result, path)
	return result
}

func (b *IRBuilder) buildFieldName(field *protogen.Field, prefix string) string {
	name := string(field.Desc.Name())
	if prefix != "" {
		return prefix + "_" + name
	}
	return name
}

func (b *IRBuilder) buildGoFieldName(field *protogen.Field, prefix string) string {
	name := field.GoName
	if prefix != "" {
		return strcase.ToCamel(prefix) + name
	}
	return name
}

func (b *IRBuilder) buildJSONName(field *protogen.Field, prefix string) string {
	name := string(field.Desc.JSONName())
	if prefix != "" {
		return strcase.ToLowerCamel(prefix + "_" + string(field.Desc.Name()))
	}
	return name
}

func (b *IRBuilder) buildEmPath(parentPath string, field *protogen.Field, prefix string) string {
	name := string(field.Desc.Name())
	if prefix != "" {
		name = prefix + "." + name
	}
	if parentPath != "" {
		return parentPath + "." + name
	}
	return name
}

func (b *IRBuilder) kindFromField(field *protogen.Field) FieldKind {
	if field.Desc.IsMap() {
		return KindMap
	}
	if field.Message != nil {
		return KindMessage
	}
	if field.Enum != nil {
		return KindEnum
	}
	return KindScalar
}

func (b *IRBuilder) kindFromProtoKind(kind protoreflect.Kind) FieldKind {
	switch kind {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return KindMessage
	case protoreflect.EnumKind:
		return KindEnum
	case protoreflect.BytesKind:
		return KindBytes
	default:
		return KindScalar
	}
}

func (b *IRBuilder) goTypeFromField(field *protogen.Field) GoType {
	if field.Message != nil && !field.Desc.IsMap() {
		// Проверяем, есть ли у вложенного сообщения generate=true
		// Если нет — используем оригинальный тип
		msgOpts := b.getMessageOptions(field.Message)
		suffix := ""
		if msgOpts != nil && msgOpts.Generate {
			suffix = b.Suffix
		}
		return GoType{
			Name:       field.Message.GoIdent.GoName + suffix,
			ImportPath: string(field.Message.GoIdent.GoImportPath),
			IsPointer:  !field.Desc.IsList(),
		}
	}
	if field.Enum != nil {
		return GoType{
			Name:       field.Enum.GoIdent.GoName,
			ImportPath: string(field.Enum.GoIdent.GoImportPath),
		}
	}
	return b.goTypeFromProtoKind(field.Desc.Kind())
}

func (b *IRBuilder) goTypeFromProtoKind(kind protoreflect.Kind) GoType {
	switch kind {
	case protoreflect.BoolKind:
		return GoType{Name: "bool"}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return GoType{Name: "int32"}
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return GoType{Name: "int64"}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return GoType{Name: "uint32"}
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return GoType{Name: "uint64"}
	case protoreflect.FloatKind:
		return GoType{Name: "float32"}
	case protoreflect.DoubleKind:
		return GoType{Name: "float64"}
	case protoreflect.StringKind:
		return GoType{Name: "string"}
	case protoreflect.BytesKind:
		return GoType{Name: "byte", IsSlice: true}
	default:
		return GoType{Name: "interface{}"}
	}
}

// Методы получения опций

func (b *IRBuilder) getFileOptions(f *protogen.File) *goplain.FileOptions {
	opts := f.Desc.Options()
	if opts == nil {
		return nil
	}
	ext := proto.GetExtension(opts, goplain.E_File)
	if ext == nil {
		return nil
	}
	return ext.(*goplain.FileOptions)
}

func (b *IRBuilder) getMessageOptions(msg *protogen.Message) *goplain.MessageOptions {
	opts := msg.Desc.Options()
	if opts == nil {
		return nil
	}
	ext := proto.GetExtension(opts, goplain.E_Message)
	if ext == nil {
		return nil
	}
	return ext.(*goplain.MessageOptions)
}

func (b *IRBuilder) getFieldOptions(field *protogen.Field) *goplain.FieldOptions {
	opts := field.Desc.Options()
	if opts == nil {
		return nil
	}
	ext := proto.GetExtension(opts, goplain.E_Field)
	if ext == nil {
		return nil
	}
	return ext.(*goplain.FieldOptions)
}

func (b *IRBuilder) getOneofOptions(oneof *protogen.Oneof) *goplain.OneofOptions {
	opts := oneof.Desc.Options()
	if opts == nil {
		return nil
	}
	ext := proto.GetExtension(opts, goplain.E_Oneof)
	if ext == nil {
		return nil
	}
	return ext.(*goplain.OneofOptions)
}

// applyGlobalOverrides применяет глобальные переопределения типов к полю
func (b *IRBuilder) applyGlobalOverrides(field *protogen.Field, irField *IRField) {
	for _, override := range b.GlobalOverrides {
		if override == nil || override.Selector == nil || override.TargetGoType == nil {
			continue
		}

		if b.matchesOverride(field, override.Selector) {
			irField.GoType = GoType{
				Name:       override.TargetGoType.Name,
				ImportPath: override.TargetGoType.ImportPath,
			}
			// Сохраняем кастеры если они указаны
			if override.ToPlainCast != nil {
				irField.ToPlainCast = *override.ToPlainCast
			}
			if override.ToPbCast != nil {
				irField.ToPbCast = *override.ToPbCast
			}
			return // применяем только первый совпадающий override
		}
	}
}

// matchesOverride проверяет соответствие поля селектору
func (b *IRBuilder) matchesOverride(field *protogen.Field, selector *goplain.OverrideSelector) bool {
	// Проверяем target_full_path
	if selector.TargetFullPath != nil {
		fullPath := string(field.Parent.Desc.FullName()) + "." + string(field.Desc.Name())
		if fullPath != *selector.TargetFullPath {
			return false
		}
	}

	// Проверяем field_kind
	if selector.FieldKind != nil {
		// Конвертируем protoreflect.Kind в typepb.Field_Kind
		protoKind := convertKindToTypePb(field.Desc.Kind())
		if protoKind != *selector.FieldKind {
			return false
		}
	}

	// Проверяем field_cardinality
	if selector.FieldCardinality != nil {
		protoCardinality := convertCardinalityToTypePb(field)
		if protoCardinality != *selector.FieldCardinality {
			return false
		}
	}

	// Проверяем field_type_url
	if selector.FieldTypeUrl != nil {
		if field.Message == nil {
			return false
		}
		typeUrl := string(field.Message.Desc.FullName())
		if typeUrl != *selector.FieldTypeUrl {
			return false
		}
	}

	return true
}

// convertKindToTypePb конвертирует protoreflect.Kind в typepb.Field_Kind
func convertKindToTypePb(kind protoreflect.Kind) typepb.Field_Kind {
	switch kind {
	case protoreflect.BoolKind:
		return typepb.Field_TYPE_BOOL
	case protoreflect.Int32Kind:
		return typepb.Field_TYPE_INT32
	case protoreflect.Sint32Kind:
		return typepb.Field_TYPE_SINT32
	case protoreflect.Uint32Kind:
		return typepb.Field_TYPE_UINT32
	case protoreflect.Int64Kind:
		return typepb.Field_TYPE_INT64
	case protoreflect.Sint64Kind:
		return typepb.Field_TYPE_SINT64
	case protoreflect.Uint64Kind:
		return typepb.Field_TYPE_UINT64
	case protoreflect.Sfixed32Kind:
		return typepb.Field_TYPE_SFIXED32
	case protoreflect.Fixed32Kind:
		return typepb.Field_TYPE_FIXED32
	case protoreflect.FloatKind:
		return typepb.Field_TYPE_FLOAT
	case protoreflect.Sfixed64Kind:
		return typepb.Field_TYPE_SFIXED64
	case protoreflect.Fixed64Kind:
		return typepb.Field_TYPE_FIXED64
	case protoreflect.DoubleKind:
		return typepb.Field_TYPE_DOUBLE
	case protoreflect.StringKind:
		return typepb.Field_TYPE_STRING
	case protoreflect.BytesKind:
		return typepb.Field_TYPE_BYTES
	case protoreflect.MessageKind:
		return typepb.Field_TYPE_MESSAGE
	case protoreflect.EnumKind:
		return typepb.Field_TYPE_ENUM
	default:
		return typepb.Field_TYPE_UNKNOWN
	}
}

// convertCardinalityToTypePb конвертирует поле в typepb.Field_Cardinality
func convertCardinalityToTypePb(field *protogen.Field) typepb.Field_Cardinality {
	if field.Desc.IsList() {
		return typepb.Field_CARDINALITY_REPEATED
	}
	if field.Desc.HasOptionalKeyword() {
		return typepb.Field_CARDINALITY_OPTIONAL
	}
	return typepb.Field_CARDINALITY_REQUIRED
}

// Методы для отладки

// DumpIR возвращает текстовое представление IR для отладки
func (irFile *IRFile) Dump() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\n", irFile.Source.Desc.Path()))
	sb.WriteString(fmt.Sprintf("Messages: %d\n", len(irFile.Messages)))

	for _, msg := range irFile.Messages {
		sb.WriteString(msg.Dump("  "))
	}

	return sb.String()
}

func (irMsg *IRMessage) Dump(indent string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%sMessage: %s (Go: %s)\n", indent, irMsg.Name, irMsg.GoName))
	sb.WriteString(fmt.Sprintf("%s  Fields: %d\n", indent, len(irMsg.Fields)))

	for _, f := range irMsg.Fields {
		sb.WriteString(f.Dump(indent + "    "))
	}

	for _, nested := range irMsg.Nested {
		sb.WriteString(nested.Dump(indent + "  "))
	}

	return sb.String()
}

func (f *IRField) Dump(indent string) string {
	return fmt.Sprintf(
		"%s- %s (%s): %s [origin=%s, empath=%q, number=%d]\n",
		indent,
		f.Name,
		f.GoName,
		f.GoType.String(),
		f.Origin,
		f.EmPath,
		f.Number,
	)
}
