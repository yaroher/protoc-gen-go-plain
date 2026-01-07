package test

import "time"

// TestMessagePlainEtalon — “плоская” структура для TestMessage.
//
// Правила учтены:
// - имена полей как в protoc-gen-go
// - optional => *T
// - oneof => отдельные поля, все nullable (*T; для bytes — *[]byte)
// - nested message поля допускаем как *NestedMessage (как в p       rotoc-gen-go для message)
// - repeated message => []*NestedMessage
// - map как в protoc-gen-go (message value => *NestedMessage)
// - embedded => поля NestedMessage “поднимаются” наверх плоско (Name/Inner)
// - serialized => []byte
// - WKT нормализуем в native Go типы с presence (в основном через указатели)
// - теги не добавляем
type TestMessagePlainEtalon struct {
	OidcId string // alias for oidc_id_alias.value
	Id     string // alias for id_alias.value
	// Scalar numeric types.
	FDouble   float64
	FFloat    float32
	FInt32    int32
	FInt64    int64
	FUint32   uint32
	FUint64   uint64
	FSint32   int32
	FSint64   int64
	FFixed32  uint32
	FFixed64  uint64
	FSfixed32 int32
	FSfixed64 int64

	// Scalar non-numeric types.
	FBool   bool
	FString string
	FBytes  []byte

	// Optional fields (proto3 optional).
	FOptInt32   *int32
	FOptString  *string
	FOptMessage *NestedMessage

	// Repeated fields.
	FRepInt32   []int32
	FRepString  []string
	FRepMessage []*NestedMessage
	FRepEnum    []TestEnum

	// Map fields (как в protoc-gen-go).
	FMapInt32String    map[int32]string
	FMapInt64Int32     map[int64]int32
	FMapUint32Uint64   map[uint32]uint64
	FMapUint64Bool     map[uint64]bool
	FMapSint32Bytes    map[int32][]byte
	FMapSint64Float    map[int64]float32
	FMapFixed32Double  map[uint32]float64
	FMapFixed64Message map[uint64]*NestedMessage
	FMapSfixed32Enum   map[int32]TestEnum
	FMapSfixed64String map[int64]string
	FMapBoolInt32      map[bool]int32
	FMapStringString   map[string]string

	// Oneof fields (все nullable).
	// Замечание: presence для bytes делаем через *[]byte, чтобы отличать "не задано" от "задано пустое".
	FOneofInt32   *int32
	FOneofString  *string
	FOneofBytes   *[]byte
	FOneofMessage *NestedMessage
	FOneofEnum    *TestEnum

	// Nested message and enum fields.
	FNestedMessage *NestedMessage

	// f_nested_message_embedded (embedded=true) => поля NestedMessage поднимаем наверх плоско.
	// NestedMessage:
	//   string name
	//   InnerMessage inner
	Name  string
	Inner *NestedMessage_InnerMessage

	// f_nested_message_serialized (serialized=true) => []byte.
	FNestedMessageSerialized []byte

	FEnum TestEnum

	// Well-known types (WKT) — нормализация.
	FAny []byte

	// Timestamp/Duration: native типы с presence.
	FTimestamp *time.Time
	FDuration  *time.Duration

	// Struct/Value/ListValue: нормализуем.
	// - Struct: map (nil = отсутствует, empty map = присутствует но пустой)
	// - Value: *any (pointer даёт presence, а *v==nil можно использовать для proto NULL)
	// - ListValue: *[]any (pointer даёт presence)
	FStruct    map[string]any
	FValue     []byte
	FListValue []byte

	// Wrappers: нормализуем в *T (presence через указатель).
	FWktDouble *float64
	FWktFloat  *float32
	FWktInt64  *int64
	FWktUint64 *uint64
	FWktInt32  *int32
	FWktUint32 *uint32
	FWktBool   *bool
	FWktString *string
	FWktBytes  *[]byte

	// NestedMessage.InnerMessage.InnerInnerMessage
	FDoubleNested *NestedMessage_InnerMessage_InnerInnerMessage
}
