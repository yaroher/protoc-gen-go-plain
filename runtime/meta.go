// Package runtime provides types used by generated plain structs.
package runtime

// FieldMeta — компактные метаданные поля (uint16)
//
// Формат:
//
//	┌─────────────┬──────────────┬─────────────────┐
//	│ Origin (3b) │ Depth (2b)   │ PathIndex (11b) │
//	└─────────────┴──────────────┴─────────────────┘
//
// Origin (биты 13-15): тип происхождения поля
// Depth (биты 11-12): глубина вложенности (0-3)
// PathIndex (биты 0-10): индекс начала пути в таблице путей
type FieldMeta uint16

// Origin constants
const (
	OriginDirect     uint16 = 0 // Поле без изменений
	OriginEmbed      uint16 = 1 // Развёрнуто из вложенного message
	OriginOneofEmbed uint16 = 2 // Из oneof с embed
	OriginVirtual    uint16 = 3 // Виртуальное поле
	OriginSerialized uint16 = 4 // Сериализовано в bytes
	OriginTypeAlias  uint16 = 5 // Развёрнуто из type_alias
)

// Bit layout
const (
	OriginShift   = 13
	OriginMask    = 0x7 << OriginShift // 0xE000
	DepthShift    = 11
	DepthMask     = 0x3 << DepthShift // 0x1800
	PathIndexMask = 0x7FF             // 0x07FF
)

// NewFieldMeta creates a FieldMeta from components
func NewFieldMeta(origin, depth, pathIndex uint16) FieldMeta {
	return FieldMeta((origin << OriginShift) | (depth << DepthShift) | (pathIndex & PathIndexMask))
}

// Origin returns the origin type (0-7)
func (m FieldMeta) Origin() uint16 {
	return (uint16(m) & OriginMask) >> OriginShift
}

// Depth returns the nesting depth (0-3)
func (m FieldMeta) Depth() uint16 {
	return (uint16(m) & DepthMask) >> DepthShift
}

// PathIndex returns the index into the path table
func (m FieldMeta) PathIndex() uint16 {
	return uint16(m) & PathIndexMask
}

// IsDirect returns true if field is direct (not embedded)
func (m FieldMeta) IsDirect() bool {
	return m.Origin() == OriginDirect
}

// IsEmbed returns true if field came from embed
func (m FieldMeta) IsEmbed() bool {
	o := m.Origin()
	return o == OriginEmbed || o == OriginOneofEmbed
}

// PlainTypeInfo — информация о plain-типе
type PlainTypeInfo struct {
	// Fields — метаданные полей (индекс = порядок поля в структуре)
	Fields []FieldMeta
	// Paths — таблица путей (field numbers в protobuf)
	Paths []uint16
	// JSONNames — имена полей для JSON (для sparse сериализации)
	JSONNames []string
}

// FieldPath возвращает путь для поля как slice field numbers
func (info *PlainTypeInfo) FieldPath(fieldIdx int) []uint16 {
	if fieldIdx < 0 || fieldIdx >= len(info.Fields) {
		return nil
	}
	meta := info.Fields[fieldIdx]
	depth := int(meta.Depth())
	if depth == 0 {
		return nil
	}
	pathIdx := int(meta.PathIndex())
	if pathIdx+depth > len(info.Paths) {
		return nil
	}
	return info.Paths[pathIdx : pathIdx+depth]
}

// PlainValue — обёртка для JSON сериализации plain-структур
// Хранит только заполненные поля и их индексы
type PlainValue struct {
	// Src — индексы заполненных полей (для восстановления в protobuf)
	Src []uint16 `json:"_src,omitempty"`
	// Data — данные полей (map для sparse хранения)
	Data map[string]any `json:"-"`
}
