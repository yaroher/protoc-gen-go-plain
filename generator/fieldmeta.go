package generator

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

// Origin constants (3 bits, values 0-7)
const (
	MetaOriginDirect     uint16 = 0 // Поле без изменений
	MetaOriginEmbed      uint16 = 1 // Развёрнуто из вложенного message
	MetaOriginOneofEmbed uint16 = 2 // Из oneof с embed
	MetaOriginVirtual    uint16 = 3 // Виртуальное поле
	MetaOriginSerialized uint16 = 4 // Сериализовано в bytes
	MetaOriginTypeAlias  uint16 = 5 // Развёрнуто из type_alias
)

// Bit positions
const (
	metaOriginShift   = 13
	metaOriginMask    = 0x7 << metaOriginShift // 0xE000
	metaDepthShift    = 11
	metaDepthMask     = 0x3 << metaDepthShift // 0x1800
	metaPathIndexMask = 0x7FF                 // 0x07FF (11 bits = 2048 values)
	metaMaxPathIndex  = 2047
	metaMaxDepth      = 3
)

// NewFieldMeta creates a new FieldMeta from components
func NewFieldMeta(origin uint16, depth uint16, pathIndex uint16) FieldMeta {
	if depth > metaMaxDepth {
		depth = metaMaxDepth
	}
	if pathIndex > metaMaxPathIndex {
		pathIndex = metaMaxPathIndex
	}
	return FieldMeta((origin << metaOriginShift) | (depth << metaDepthShift) | pathIndex)
}

// Origin returns the origin type (0-7)
func (m FieldMeta) Origin() uint16 {
	return (uint16(m) & metaOriginMask) >> metaOriginShift
}

// Depth returns the nesting depth (0-3)
func (m FieldMeta) Depth() uint16 {
	return (uint16(m) & metaDepthMask) >> metaDepthShift
}

// PathIndex returns the index into the path table
func (m FieldMeta) PathIndex() uint16 {
	return uint16(m) & metaPathIndexMask
}

// OriginToMeta converts FieldOrigin to meta origin constant
func OriginToMeta(origin FieldOrigin) uint16 {
	switch origin {
	case OriginDirect:
		return MetaOriginDirect
	case OriginEmbed:
		return MetaOriginEmbed
	case OriginOneofEmbed:
		return MetaOriginOneofEmbed
	case OriginVirtual:
		return MetaOriginVirtual
	case OriginSerialized:
		return MetaOriginSerialized
	case OriginTypeAlias:
		return MetaOriginTypeAlias
	default:
		return MetaOriginDirect
	}
}

// PathEntry — элемент пути (номер поля в protobuf)
type PathEntry uint16

// PlainMeta — метаданные для plain-структуры
// Генерируется как переменная для каждого типа
type PlainMeta struct {
	// TypeName — имя Go-типа
	TypeName string
	// Fields — метаданные полей (индекс = порядок поля в структуре)
	Fields []FieldMeta
	// FieldNames — имена полей для JSON
	FieldNames []string
	// Paths — таблица путей (field numbers)
	Paths []PathEntry
}

// FieldPath returns the path for a field as slice of field numbers
func (pm *PlainMeta) FieldPath(fieldIdx int) []PathEntry {
	if fieldIdx < 0 || fieldIdx >= len(pm.Fields) {
		return nil
	}
	meta := pm.Fields[fieldIdx]
	depth := int(meta.Depth())
	if depth == 0 {
		return nil // Direct field, no path
	}
	pathIdx := int(meta.PathIndex())
	if pathIdx+depth > len(pm.Paths) {
		return nil
	}
	return pm.Paths[pathIdx : pathIdx+depth]
}
