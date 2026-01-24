package generator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestIRBuilder_NewIRBuilder(t *testing.T) {
	// Тест с дефолтным суффиксом
	b := NewIRBuilder("")
	assert.Equal(t, "Plain", b.Suffix)

	// Тест с кастомным суффиксом
	b2 := NewIRBuilder("DTO")
	assert.Equal(t, "DTO", b2.Suffix)
}

func TestFieldOrigin_String(t *testing.T) {
	tests := []struct {
		origin   FieldOrigin
		expected string
	}{
		{OriginDirect, "direct"},
		{OriginEmbed, "embed"},
		{OriginOneofEmbed, "oneof_embed"},
		{OriginVirtual, "virtual"},
		{OriginSerialized, "serialized"},
		{OriginTypeAlias, "type_alias"},
		{FieldOrigin(999), "unknown(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.origin.String())
		})
	}
}

func TestFieldKind_String(t *testing.T) {
	tests := []struct {
		kind     FieldKind
		expected string
	}{
		{KindScalar, "scalar"},
		{KindMessage, "message"},
		{KindEnum, "enum"},
		{KindBytes, "bytes"},
		{KindMap, "map"},
		{FieldKind(999), "unknown(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.String())
		})
	}
}

func TestGoType_String(t *testing.T) {
	tests := []struct {
		goType   GoType
		expected string
	}{
		{GoType{Name: "string"}, "string"},
		{GoType{Name: "int64"}, "int64"},
		{GoType{Name: "MyMessage", IsPointer: true}, "*MyMessage"},
		{GoType{Name: "string", IsSlice: true}, "[]string"},
		{GoType{Name: "[]byte", IsSlice: false}, "[]byte"},
		{GoType{Name: "MyMessage", IsPointer: true, IsSlice: true}, "[]*MyMessage"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.goType.String())
		})
	}
}

func TestCollision_Error(t *testing.T) {
	msg := &IRMessage{Name: "TestMessage"}
	existing := &IRField{
		Name:   "field1",
		Origin: OriginDirect,
		EmPath: "",
	}
	newField := &IRField{
		Name:   "field1",
		Origin: OriginEmbed,
		EmPath: "address.field1",
	}

	collision := Collision{
		FieldName:     "field1",
		ExistingField: existing,
		NewField:      newField,
		Message:       msg,
	}

	errStr := collision.Error()
	assert.Contains(t, errStr, "TestMessage")
	assert.Contains(t, errStr, "field1")
	assert.Contains(t, errStr, "embed")
	assert.Contains(t, errStr, "direct")
}

func TestIRBuilder_goTypeFromProtoKind(t *testing.T) {
	b := NewIRBuilder("Plain")

	tests := []struct {
		kind     protoreflect.Kind
		expected GoType
	}{
		{protoreflect.BoolKind, GoType{Name: "bool"}},
		{protoreflect.Int32Kind, GoType{Name: "int32"}},
		{protoreflect.Int64Kind, GoType{Name: "int64"}},
		{protoreflect.Uint32Kind, GoType{Name: "uint32"}},
		{protoreflect.Uint64Kind, GoType{Name: "uint64"}},
		{protoreflect.FloatKind, GoType{Name: "float32"}},
		{protoreflect.DoubleKind, GoType{Name: "float64"}},
		{protoreflect.StringKind, GoType{Name: "string"}},
		{protoreflect.BytesKind, GoType{Name: "[]byte", IsSlice: false}},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			result := b.goTypeFromProtoKind(tt.kind)
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.IsSlice, result.IsSlice)
		})
	}
}

func TestIRBuilder_kindFromProtoKind(t *testing.T) {
	b := NewIRBuilder("Plain")

	tests := []struct {
		kind     protoreflect.Kind
		expected FieldKind
	}{
		{protoreflect.BoolKind, KindScalar},
		{protoreflect.Int32Kind, KindScalar},
		{protoreflect.StringKind, KindScalar},
		{protoreflect.BytesKind, KindBytes},
		{protoreflect.MessageKind, KindMessage},
		{protoreflect.EnumKind, KindEnum},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			result := b.kindFromProtoKind(tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIRMessage_Dump(t *testing.T) {
	msg := &IRMessage{
		Name:   "TestPlain",
		GoName: "TestPlain",
		Fields: []*IRField{
			{
				Name:   "name",
				GoName: "Name",
				GoType: GoType{Name: "string"},
				Origin: OriginDirect,
				EmPath: "",
				Number: 1,
			},
			{
				Name:   "address_street",
				GoName: "AddressStreet",
				GoType: GoType{Name: "string"},
				Origin: OriginEmbed,
				EmPath: "address.street",
				Number: 2,
			},
		},
	}

	dump := msg.Dump("")
	assert.Contains(t, dump, "TestPlain")
	assert.Contains(t, dump, "name")
	assert.Contains(t, dump, "address_street")
	assert.Contains(t, dump, "embed")
	assert.Contains(t, dump, "direct")
}

func TestIRField_Dump(t *testing.T) {
	field := &IRField{
		Name:   "test_field",
		GoName: "TestField",
		GoType: GoType{Name: "string"},
		Origin: OriginEmbed,
		EmPath: "parent.test_field",
		Number: 5,
	}

	dump := field.Dump("  ")
	assert.Contains(t, dump, "test_field")
	assert.Contains(t, dump, "TestField")
	assert.Contains(t, dump, "string")
	assert.Contains(t, dump, "embed")
	assert.Contains(t, dump, "parent.test_field")
	assert.Contains(t, dump, "5")
}

// Test addField collision detection
func TestIRBuilder_addField_Collision(t *testing.T) {
	b := NewIRBuilder("Plain")
	msg := &IRMessage{
		Name:   "TestPlain",
		Fields: make([]*IRField, 0),
	}

	// Add first field
	field1 := &IRField{
		Name:   "test",
		Origin: OriginDirect,
	}
	b.addField(msg, field1)
	assert.Len(t, msg.Fields, 1)
	assert.Empty(t, b.Collisions)

	// Try to add field with same name - should record collision but not add field
	field2 := &IRField{
		Name:   "test",
		Origin: OriginEmbed,
		EmPath: "address.test",
	}
	b.addField(msg, field2)
	assert.Len(t, msg.Fields, 1) // Still 1 field - duplicate not added
	assert.Len(t, b.Collisions, 1)

	collision := b.Collisions[0]
	assert.Equal(t, "test", collision.FieldName)
	assert.Equal(t, OriginDirect, collision.ExistingField.Origin)
	assert.Equal(t, OriginEmbed, collision.NewField.Origin)
}

func TestIRBuilder_addField_NoCollision(t *testing.T) {
	b := NewIRBuilder("Plain")
	msg := &IRMessage{
		Name:   "TestPlain",
		Fields: make([]*IRField, 0),
	}

	// Add multiple fields with different names
	fields := []*IRField{
		{Name: "field1", Origin: OriginDirect},
		{Name: "field2", Origin: OriginDirect},
		{Name: "field3", Origin: OriginEmbed, EmPath: "nested.field3"},
	}

	for _, f := range fields {
		b.addField(msg, f)
	}

	assert.Len(t, msg.Fields, 3)
	assert.Empty(t, b.Collisions)
}

func TestValidationError_Error(t *testing.T) {
	// With field
	msg := &IRMessage{Name: "TestMessage"}
	field := &IRField{Name: "test_field"}
	err := ValidationError{
		Message: "invalid type",
		Field:   field,
		IRMsg:   msg,
	}
	assert.Contains(t, err.Error(), "TestMessage")
	assert.Contains(t, err.Error(), "test_field")
	assert.Contains(t, err.Error(), "invalid type")

	// Without field
	err2 := ValidationError{
		Message: "missing required option",
		IRMsg:   msg,
	}
	assert.Contains(t, err2.Error(), "TestMessage")
	assert.Contains(t, err2.Error(), "missing required option")
	assert.NotContains(t, err2.Error(), "test_field")
}
