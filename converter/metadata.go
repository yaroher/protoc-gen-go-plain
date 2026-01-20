package converter

import (
	"google.golang.org/protobuf/compiler/protogen"
)

// FieldMetadata содержит полную информацию о трансформации поля
type FieldMetadata struct {
	// Оригинальное поле (до конверсии)
	OriginalField *protogen.Field

	// Трансформированное поле (после конверсии)
	PlainField *protogen.Field

	// Флаги трансформации
	IsTypeAlias  bool // Поле было type alias и развернуто в скаляр
	IsEmbedded   bool // Поле пришло из embedded сообщения
	IsSerialized bool // Поле сериализовано в []byte
	IsVirtual    bool // Поле виртуальное (не существует в оригинале)
	IsOneof      bool // Поле является частью oneof

	// Дополнительная информация
	EmbedPath      string // Путь к embedded полю (например, "Embed")
	OneofGroupName string // Имя oneof группы
	OriginalType   string // Оригинальный тип для type alias (например, "string" для IdAlias)

	// Для embedded полей - оригинальное поле из которого было развернуто
	EmbedSourceField *protogen.Field
}

// MessageMetadata содержит полную информацию о трансформации сообщения
type MessageMetadata struct {
	// Оригинальное сообщение (до конверсии)
	OriginalMessage *protogen.Message

	// Трансформированное сообщение (после конверсии)
	PlainMessage *protogen.Message

	// Метаинформация по всем полям
	Fields []*FieldMetadata

	// Флаги сообщения
	IsGenerated bool // Было ли сообщение трансформировано (generate=true)
}

// ConversionMetadata содержит полную интроспекцию конверсии
type ConversionMetadata struct {
	// Метаинформация по всем сообщениям
	// Ключ - полное имя plain сообщения (например, ".test.TestMessagePlain")
	Messages map[string]*MessageMetadata
}

// NewConversionMetadata создает новый экземпляр метаданных конверсии
func NewConversionMetadata() *ConversionMetadata {
	return &ConversionMetadata{
		Messages: make(map[string]*MessageMetadata),
	}
}

// GetMessageMetadata возвращает метаданные для сообщения по его plain имени
func (m *ConversionMetadata) GetMessageMetadata(plainMessageFullName string) *MessageMetadata {
	return m.Messages[plainMessageFullName]
}

// GetFieldMetadata возвращает метаданные для поля по имени сообщения и имени поля
func (m *ConversionMetadata) GetFieldMetadata(plainMessageFullName, plainFieldName string) *FieldMetadata {
	msgMeta := m.GetMessageMetadata(plainMessageFullName)
	if msgMeta == nil {
		return nil
	}

	for _, fieldMeta := range msgMeta.Fields {
		if fieldMeta.PlainField != nil && fieldMeta.PlainField.GoName == plainFieldName {
			return fieldMeta
		}
	}

	return nil
}
