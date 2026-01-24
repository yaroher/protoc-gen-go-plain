package generator

import (
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FieldOrigin описывает откуда пришло поле в plain-сообщение
type FieldOrigin int

const (
	// OriginDirect — поле из оригинального сообщения без изменений
	OriginDirect FieldOrigin = iota
	// OriginEmbed — поле развёрнуто из вложенного сообщения через embed
	OriginEmbed
	// OriginOneofEmbed — поле из oneof с embed=true
	OriginOneofEmbed
	// OriginVirtual — виртуальное поле, добавленное через virtual_fields
	OriginVirtual
	// OriginSerialized — поле сериализуется в bytes
	OriginSerialized
	// OriginTypeAlias — поле развёрнуто из type_alias
	OriginTypeAlias
)

func (o FieldOrigin) String() string {
	switch o {
	case OriginDirect:
		return "direct"
	case OriginEmbed:
		return "embed"
	case OriginOneofEmbed:
		return "oneof_embed"
	case OriginVirtual:
		return "virtual"
	case OriginSerialized:
		return "serialized"
	case OriginTypeAlias:
		return "type_alias"
	default:
		return fmt.Sprintf("unknown(%d)", o)
	}
}

// FieldKind описывает тип поля в IR
type FieldKind int

const (
	KindScalar FieldKind = iota
	KindMessage
	KindEnum
	KindBytes // для serialized полей
	KindMap
)

func (k FieldKind) String() string {
	switch k {
	case KindScalar:
		return "scalar"
	case KindMessage:
		return "message"
	case KindEnum:
		return "enum"
	case KindBytes:
		return "bytes"
	case KindMap:
		return "map"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// IRFile представляет файл с plain-сообщениями
type IRFile struct {
	// Source — исходный proto-файл
	Source *protogen.File
	// Messages — plain-сообщения
	Messages []*IRMessage
	// Imports — необходимые импорты для Go
	Imports []GoImport
}

// GoImport представляет Go-импорт
type GoImport struct {
	Path  string
	Alias string
}

// IRMessage представляет plain-сообщение
type IRMessage struct {
	// Source — исходное protobuf-сообщение
	Source *protogen.Message
	// Name — имя plain-сообщения (обычно OriginalName + "Plain")
	Name string
	// GoName — имя Go-структуры
	GoName string
	// Fields — поля plain-сообщения (после всех трансформаций)
	Fields []*IRField
	// OriginalFields — оригинальные поля до трансформаций (для отладки)
	OriginalFields []*protogen.Field
	// Nested — вложенные plain-сообщения
	Nested []*IRMessage
	// Comment — комментарий к сообщению
	Comment string
	// EmPath — путь embed (для отладки и CRF)
	EmPath string

	// PathTable — таблица путей (field numbers) для всех полей
	// Используется для навигации по protobuf-структуре
	PathTable []int32

	// EmbeddedOneofs — информация о embedded oneof для генерации Case полей
	EmbeddedOneofs []*EmbeddedOneof

	// IsVirtual — виртуальный тип (не имеет Source protobuf сообщения)
	IsVirtual bool
}

// EmbeddedOneof хранит информацию о oneof с embed=true
type EmbeddedOneof struct {
	// Name — имя oneof в proto
	Name string
	// GoName — имя в Go (для поля Case)
	GoName string
	// CaseFieldName — имя поля для хранения выбранного варианта (e.g., "PlatformEventCase")
	CaseFieldName string
	// JSONName — имя для JSON сериализации
	JSONName string
	// Variants — список вариантов oneof
	Variants []*OneofVariant
	// Source — исходный protogen.Oneof
	Source *protogen.Oneof
}

// OneofVariant представляет один вариант oneof
type OneofVariant struct {
	// Name — имя варианта в proto (e.g., "heartbeat")
	Name string
	// GoName — имя в Go (e.g., "Heartbeat")
	GoName string
	// FieldNumber — номер поля в proto
	FieldNumber int32
}

// IRField представляет поле в plain-сообщении
type IRField struct {
	// Source — исходное protobuf-поле (может быть nil для virtual)
	Source *protogen.Field

	// Name — имя поля в proto
	Name string
	// GoName — имя поля в Go
	GoName string
	// JSONName — имя поля для JSON
	JSONName string

	// Index — индекс поля в plain-структуре (для _src битмаски)
	Index uint16

	// Number — номер поля в plain-сообщении
	Number int32
	// OriginalNumber — оригинальный номер поля (до перенумерации)
	OriginalNumber int32

	// Kind — тип поля (scalar, message, enum, bytes, map)
	Kind FieldKind
	// ScalarKind — конкретный скалярный тип (если Kind == KindScalar)
	ScalarKind protoreflect.Kind

	// GoType — Go-тип поля
	GoType GoType
	// ProtoType — полный путь proto-типа (для message/enum)
	ProtoType string

	// Origin — откуда пришло поле
	Origin FieldOrigin
	// EmPath — путь embed (например "address.street" или "platform_event.heartbeat.timestamp")
	EmPath string
	// PathNumbers — путь как массив номеров полей в protobuf (для навигации)
	PathNumbers []int32

	// OneofName — имя oneof (если поле из embedded oneof)
	OneofName string
	// OneofGoName — Go имя oneof
	OneofGoName string
	// OneofVariant — имя варианта oneof (e.g., "heartbeat")
	OneofVariant string

	// IsRepeated — repeated поле
	IsRepeated bool
	// IsOptional — optional поле (proto3 optional)
	IsOptional bool
	// IsMap — map поле
	IsMap bool

	// MapKey — тип ключа для map
	MapKey *IRField
	// MapValue — тип значения для map
	MapValue *IRField

	// EnumAsString — сериализовать enum как строку
	EnumAsString bool
	// EnumAsInt — сериализовать enum как int
	EnumAsInt bool

	// Type override casters
	// ToPlainCast — функция конвертации из protobuf типа в target Go тип
	// Пример: "time.Duration" или "mypackage.ToMyType"
	ToPlainCast string
	// ToPbCast — функция конвертации из target Go типа в protobuf тип
	// Пример: "int64" или "mypackage.FromMyType"
	ToPbCast string

	// Comment — комментарий к полю
	Comment string

	// FieldIndex — индекс поля в структуре (для метаданных)
	FieldIndex int
}

// GoType представляет Go-тип
type GoType struct {
	// Name — имя типа (например "string", "int64", "MyMessage")
	Name string
	// ImportPath — путь импорта (пустой для builtin типов)
	ImportPath string
	// IsPointer — нужен ли указатель
	IsPointer bool
	// IsSlice — это слайс
	IsSlice bool
}

// String возвращает строковое представление типа
func (t GoType) String() string {
	result := t.Name
	if t.IsPointer {
		result = "*" + result
	}
	if t.IsSlice {
		result = "[]" + result
	}
	return result
}

// QualifiedName возвращает полное имя с пакетом
func (t GoType) QualifiedName(currentPkg string) string {
	if t.ImportPath == "" || t.ImportPath == currentPkg {
		return t.String()
	}
	// Используем последний компонент пути как имя пакета
	name := t.Name
	if t.IsPointer {
		name = "*" + name
	}
	if t.IsSlice {
		name = "[]" + name
	}
	return name
}

// Collision представляет коллизию имён полей
type Collision struct {
	// FieldName — имя поля, вызвавшего коллизию
	FieldName string
	// ExistingField — существующее поле
	ExistingField *IRField
	// NewField — новое поле, которое пытаемся добавить
	NewField *IRField
	// Message — сообщение, в котором произошла коллизия
	Message *IRMessage
}

func (c Collision) Error() string {
	return fmt.Sprintf(
		"field name collision in message %s: field %q (origin: %s, empath: %s) conflicts with existing field (origin: %s, empath: %s)",
		c.Message.Name,
		c.FieldName,
		c.NewField.Origin,
		c.NewField.EmPath,
		c.ExistingField.Origin,
		c.ExistingField.EmPath,
	)
}

// ValidationError представляет ошибку валидации IR
type ValidationError struct {
	Message string
	Field   *IRField
	IRMsg   *IRMessage
}

func (e ValidationError) Error() string {
	if e.Field != nil {
		return fmt.Sprintf("validation error in %s.%s: %s", e.IRMsg.Name, e.Field.Name, e.Message)
	}
	return fmt.Sprintf("validation error in %s: %s", e.IRMsg.Name, e.Message)
}
