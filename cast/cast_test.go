package cast

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestStructCasts(t *testing.T) {
	if got := StructToMap(nil); got != nil {
		t.Fatalf("StructToMap(nil) = %v, want nil", got)
	}
	if got := StructToPtrMap(nil); got != nil {
		t.Fatalf("StructToPtrMap(nil) = %v, want nil", got)
	}
	if got := StructFromMap(nil); got != nil {
		t.Fatalf("StructFromMap(nil) = %v, want nil", got)
	}
	if got := StructFromPtrMap(nil); got != nil {
		t.Fatalf("StructFromPtrMap(nil) = %v, want nil", got)
	}

	m := map[string]any{
		"name":   "alice",
		"age":    float64(30),
		"active": true,
		"tags":   []any{"a", "b"},
		"meta": map[string]any{
			"k": "v",
		},
	}
	s := StructFromMap(m)
	if s == nil {
		t.Fatalf("StructFromMap(%v) = nil, want non-nil", m)
	}
	if got := StructToMap(s); !reflect.DeepEqual(got, m) {
		t.Fatalf("StructToMap(%v) = %v, want %v", s, got, m)
	}
	if got := StructToPtrMap(s); got == nil || !reflect.DeepEqual(*got, m) {
		t.Fatalf("StructToPtrMap(%v) = %v, want %v", s, got, m)
	}
	if got := StructFromPtrMap(&m); got == nil || !reflect.DeepEqual(StructToMap(got), m) {
		t.Fatalf("StructFromPtrMap(%v) = %v, want %v", m, got, m)
	}
}

func TestValueCasts(t *testing.T) {
	if got := ValueToInterface(nil); got != nil {
		t.Fatalf("ValueToInterface(nil) = %v, want nil", got)
	}
	if got := ValueToPtrInterface(nil); got != nil {
		t.Fatalf("ValueToPtrInterface(nil) = %v, want nil", got)
	}
	if got := ValueFromInterface(nil); got != nil {
		t.Fatalf("ValueFromInterface(nil) = %v, want nil", got)
	}
	if got := ValueFromPtrInterface(nil); got != nil {
		t.Fatalf("ValueFromPtrInterface(nil) = %v, want nil", got)
	}

	m := map[string]any{"key": "value"}
	val := ValueFromInterface(m)
	if val == nil {
		t.Fatalf("ValueFromInterface(%v) = nil, want non-nil", m)
	}
	if got := ValueToInterface(val); !reflect.DeepEqual(got, m) {
		t.Fatalf("ValueToInterface(%v) = %v, want %v", val, got, m)
	}
	if got := ValueToPtrInterface(val); got == nil || !reflect.DeepEqual(*got, m) {
		t.Fatalf("ValueToPtrInterface(%v) = %v, want %v", val, got, m)
	}

	var s any = "hello"
	if got := ValueFromPtrInterface(&s); got == nil || got.GetStringValue() != "hello" {
		t.Fatalf("ValueFromPtrInterface(%v) = %v, want string value", s, got)
	}
}

func TestAnyCasts(t *testing.T) {
	if got := AnyToMessage[*wrapperspb.StringValue](nil); got != nil {
		t.Fatalf("AnyToMessage(nil) = %v, want nil", got)
	}

	msg := &wrapperspb.StringValue{Value: "hi"}
	anyMsg := AnyFromMessage(msg)
	if anyMsg == nil {
		t.Fatalf("AnyFromMessage(%v) = nil, want non-nil", msg)
	}
	if got := AnyToMessage[*wrapperspb.StringValue](anyMsg); got == nil || got.Value != "hi" {
		t.Fatalf("AnyToMessage(%v) = %v, want value %q", anyMsg, got, "hi")
	}

	var nilMsg *wrapperspb.StringValue
	if got := AnyFromMessage(nilMsg); got != nil {
		t.Fatalf("AnyFromMessage(nil) = %v, want nil", got)
	}
}

func TestEmptyCasts(t *testing.T) {
	if got := EmptyToStruct(nil); got != (struct{}{}) {
		t.Fatalf("EmptyToStruct(nil) = %v, want zero struct", got)
	}
	if got := EmptyToPtrStruct(nil); got != nil {
		t.Fatalf("EmptyToPtrStruct(nil) = %v, want nil", got)
	}
	if got := EmptyFromPtrStruct(nil); got != nil {
		t.Fatalf("EmptyFromPtrStruct(nil) = %v, want nil", got)
	}

	v := struct{}{}
	if got := EmptyFromStruct(v); got == nil {
		t.Fatalf("EmptyFromStruct(%v) = nil, want non-nil", v)
	}
	if got := EmptyFromPtrStruct(&v); got == nil {
		t.Fatalf("EmptyFromPtrStruct(%v) = nil, want non-nil", v)
	}
	if got := EmptyToPtrStruct(&emptypb.Empty{}); got == nil {
		t.Fatalf("EmptyToPtrStruct(non-nil) = nil, want non-nil")
	}
}

func TestDoubleValueCasts(t *testing.T) {
	if got := DoubleValueToFloat64(nil); got != 0 {
		t.Fatalf("DoubleValueToFloat64(nil) = %v, want 0", got)
	}
	if got := DoubleValueToPtrFloat64(nil); got != nil {
		t.Fatalf("DoubleValueToPtrFloat64(nil) = %v, want nil", got)
	}
	if got := DoubleValueFromPtrFloat64(nil); got != nil {
		t.Fatalf("DoubleValueFromPtrFloat64(nil) = %v, want nil", got)
	}

	v := 12.5
	pb := DoubleValueFromFloat64(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("DoubleValueFromFloat64(%v) = %+v, want value %v", v, pb, v)
	}
	if got := DoubleValueToFloat64(pb); got != v {
		t.Fatalf("DoubleValueToFloat64(%v) = %v, want %v", pb, got, v)
	}
	if got := DoubleValueToPtrFloat64(pb); got == nil || *got != v {
		t.Fatalf("DoubleValueToPtrFloat64(%v) = %v, want %v", pb, got, v)
	}
	if got := DoubleValueFromPtrFloat64(&v); got == nil || got.Value != v {
		t.Fatalf("DoubleValueFromPtrFloat64(%v) = %v, want %v", v, got, v)
	}
}

func TestFloatValueCasts(t *testing.T) {
	if got := FloatValueToFloat32(nil); got != 0 {
		t.Fatalf("FloatValueToFloat32(nil) = %v, want 0", got)
	}
	if got := FloatValueToPtrFloat32(nil); got != nil {
		t.Fatalf("FloatValueToPtrFloat32(nil) = %v, want nil", got)
	}
	if got := FloatValueFromPtrFloat32(nil); got != nil {
		t.Fatalf("FloatValueFromPtrFloat32(nil) = %v, want nil", got)
	}

	v := float32(3.25)
	pb := FloatValueFromFloat32(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("FloatValueFromFloat32(%v) = %+v, want value %v", v, pb, v)
	}
	if got := FloatValueToFloat32(pb); got != v {
		t.Fatalf("FloatValueToFloat32(%v) = %v, want %v", pb, got, v)
	}
	if got := FloatValueToPtrFloat32(pb); got == nil || *got != v {
		t.Fatalf("FloatValueToPtrFloat32(%v) = %v, want %v", pb, got, v)
	}
	if got := FloatValueFromPtrFloat32(&v); got == nil || got.Value != v {
		t.Fatalf("FloatValueFromPtrFloat32(%v) = %v, want %v", v, got, v)
	}
}

func TestInt32ValueCasts(t *testing.T) {
	if got := Int32ValueToInt32(nil); got != 0 {
		t.Fatalf("Int32ValueToInt32(nil) = %v, want 0", got)
	}
	if got := Int32ValueToPtrInt32(nil); got != nil {
		t.Fatalf("Int32ValueToPtrInt32(nil) = %v, want nil", got)
	}
	if got := Int32ValueFromPtrInt32(nil); got != nil {
		t.Fatalf("Int32ValueFromPtrInt32(nil) = %v, want nil", got)
	}

	v := int32(-42)
	pb := Int32ValueFromInt32(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("Int32ValueFromInt32(%v) = %+v, want value %v", v, pb, v)
	}
	if got := Int32ValueToInt32(pb); got != v {
		t.Fatalf("Int32ValueToInt32(%v) = %v, want %v", pb, got, v)
	}
	if got := Int32ValueToPtrInt32(pb); got == nil || *got != v {
		t.Fatalf("Int32ValueToPtrInt32(%v) = %v, want %v", pb, got, v)
	}
	if got := Int32ValueFromPtrInt32(&v); got == nil || got.Value != v {
		t.Fatalf("Int32ValueFromPtrInt32(%v) = %v, want %v", v, got, v)
	}
}

func TestUInt32ValueCasts(t *testing.T) {
	if got := UInt32ValueToUint32(nil); got != 0 {
		t.Fatalf("UInt32ValueToUint32(nil) = %v, want 0", got)
	}
	if got := UInt32ValueToPtrUint32(nil); got != nil {
		t.Fatalf("UInt32ValueToPtrUint32(nil) = %v, want nil", got)
	}
	if got := UInt32ValueFromPtrUint32(nil); got != nil {
		t.Fatalf("UInt32ValueFromPtrUint32(nil) = %v, want nil", got)
	}

	v := uint32(42)
	pb := UInt32ValueFromUint32(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("UInt32ValueFromUint32(%v) = %+v, want value %v", v, pb, v)
	}
	if got := UInt32ValueToUint32(pb); got != v {
		t.Fatalf("UInt32ValueToUint32(%v) = %v, want %v", pb, got, v)
	}
	if got := UInt32ValueToPtrUint32(pb); got == nil || *got != v {
		t.Fatalf("UInt32ValueToPtrUint32(%v) = %v, want %v", pb, got, v)
	}
	if got := UInt32ValueFromPtrUint32(&v); got == nil || got.Value != v {
		t.Fatalf("UInt32ValueFromPtrUint32(%v) = %v, want %v", v, got, v)
	}
}

func TestInt64ValueCasts(t *testing.T) {
	if got := Int64ValueToInt64(nil); got != 0 {
		t.Fatalf("Int64ValueToInt64(nil) = %v, want 0", got)
	}
	if got := Int64ValueToPtrInt64(nil); got != nil {
		t.Fatalf("Int64ValueToPtrInt64(nil) = %v, want nil", got)
	}
	if got := Int64ValueFromPtrInt64(nil); got != nil {
		t.Fatalf("Int64ValueFromPtrInt64(nil) = %v, want nil", got)
	}

	v := int64(-123456789)
	pb := Int64ValueFromInt64(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("Int64ValueFromInt64(%v) = %+v, want value %v", v, pb, v)
	}
	if got := Int64ValueToInt64(pb); got != v {
		t.Fatalf("Int64ValueToInt64(%v) = %v, want %v", pb, got, v)
	}
	if got := Int64ValueToPtrInt64(pb); got == nil || *got != v {
		t.Fatalf("Int64ValueToPtrInt64(%v) = %v, want %v", pb, got, v)
	}
	if got := Int64ValueFromPtrInt64(&v); got == nil || got.Value != v {
		t.Fatalf("Int64ValueFromPtrInt64(%v) = %v, want %v", v, got, v)
	}
}

func TestUInt64ValueCasts(t *testing.T) {
	if got := UInt64ValueToUint64(nil); got != 0 {
		t.Fatalf("UInt64ValueToUint64(nil) = %v, want 0", got)
	}
	if got := UInt64ValueToPtrUint64(nil); got != nil {
		t.Fatalf("UInt64ValueToPtrUint64(nil) = %v, want nil", got)
	}
	if got := UInt64ValueFromPtrUint64(nil); got != nil {
		t.Fatalf("UInt64ValueFromPtrUint64(nil) = %v, want nil", got)
	}

	v := uint64(123456789)
	pb := UInt64ValueFromUint64(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("UInt64ValueFromUint64(%v) = %+v, want value %v", v, pb, v)
	}
	if got := UInt64ValueToUint64(pb); got != v {
		t.Fatalf("UInt64ValueToUint64(%v) = %v, want %v", pb, got, v)
	}
	if got := UInt64ValueToPtrUint64(pb); got == nil || *got != v {
		t.Fatalf("UInt64ValueToPtrUint64(%v) = %v, want %v", pb, got, v)
	}
	if got := UInt64ValueFromPtrUint64(&v); got == nil || got.Value != v {
		t.Fatalf("UInt64ValueFromPtrUint64(%v) = %v, want %v", v, got, v)
	}
}

func TestBoolValueCasts(t *testing.T) {
	if got := BoolValueToBool(nil); got != false {
		t.Fatalf("BoolValueToBool(nil) = %v, want false", got)
	}
	if got := BoolValueToPtrBool(nil); got != nil {
		t.Fatalf("BoolValueToPtrBool(nil) = %v, want nil", got)
	}
	if got := BoolValueFromPtrBool(nil); got != nil {
		t.Fatalf("BoolValueFromPtrBool(nil) = %v, want nil", got)
	}

	v := true
	pb := BoolValueFromBool(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("BoolValueFromBool(%v) = %+v, want value %v", v, pb, v)
	}
	if got := BoolValueToBool(pb); got != v {
		t.Fatalf("BoolValueToBool(%v) = %v, want %v", pb, got, v)
	}
	if got := BoolValueToPtrBool(pb); got == nil || *got != v {
		t.Fatalf("BoolValueToPtrBool(%v) = %v, want %v", pb, got, v)
	}
	if got := BoolValueFromPtrBool(&v); got == nil || got.Value != v {
		t.Fatalf("BoolValueFromPtrBool(%v) = %v, want %v", v, got, v)
	}
}

func TestStringValueCasts(t *testing.T) {
	if got := StringValueToString(nil); got != "" {
		t.Fatalf("StringValueToString(nil) = %q, want empty string", got)
	}
	if got := StringValueToPtrString(nil); got != nil {
		t.Fatalf("StringValueToPtrString(nil) = %v, want nil", got)
	}
	if got := StringValueFromPtrString(nil); got != nil {
		t.Fatalf("StringValueFromPtrString(nil) = %v, want nil", got)
	}

	v := "hello"
	pb := StringValueFromString(v)
	if pb == nil || pb.Value != v {
		t.Fatalf("StringValueFromString(%q) = %+v, want value %q", v, pb, v)
	}
	if got := StringValueToString(pb); got != v {
		t.Fatalf("StringValueToString(%v) = %q, want %q", pb, got, v)
	}
	if got := StringValueToPtrString(pb); got == nil || *got != v {
		t.Fatalf("StringValueToPtrString(%v) = %v, want %q", pb, got, v)
	}
	if got := StringValueFromPtrString(&v); got == nil || got.Value != v {
		t.Fatalf("StringValueFromPtrString(%q) = %v, want %q", v, got, v)
	}
}

func TestBytesValueCasts(t *testing.T) {
	if got := BytesValueToBytes(nil); got != nil {
		t.Fatalf("BytesValueToBytes(nil) = %v, want nil", got)
	}
	if got := BytesValueToPtrBytes(nil); got != nil {
		t.Fatalf("BytesValueToPtrBytes(nil) = %v, want nil", got)
	}
	if got := BytesValueFromPtrBytes(nil); got != nil {
		t.Fatalf("BytesValueFromPtrBytes(nil) = %v, want nil", got)
	}

	v := []byte{1, 2, 3}
	pb := BytesValueFromBytes(v)
	if pb == nil || !reflect.DeepEqual(pb.Value, v) {
		t.Fatalf("BytesValueFromBytes(%v) = %+v, want value %v", v, pb, v)
	}
	if got := BytesValueToBytes(pb); !reflect.DeepEqual(got, v) {
		t.Fatalf("BytesValueToBytes(%v) = %v, want %v", pb, got, v)
	}
	if got := BytesValueToPtrBytes(pb); got == nil || !reflect.DeepEqual(*got, v) {
		t.Fatalf("BytesValueToPtrBytes(%v) = %v, want %v", pb, got, v)
	}
	if got := BytesValueFromPtrBytes(&v); got == nil || !reflect.DeepEqual(got.Value, v) {
		t.Fatalf("BytesValueFromPtrBytes(%v) = %v, want %v", v, got, v)
	}
}

func TestWrapperCastsRoundTrip(t *testing.T) {
	// A quick sanity check that all wrappers can round-trip through their cast helpers.
	if got := DoubleValueToFloat64(DoubleValueFromFloat64(9.75)); got != 9.75 {
		t.Fatalf("Double round-trip got %v", got)
	}
	if got := FloatValueToFloat32(FloatValueFromFloat32(1.5)); got != 1.5 {
		t.Fatalf("Float round-trip got %v", got)
	}
	if got := Int32ValueToInt32(Int32ValueFromInt32(-7)); got != -7 {
		t.Fatalf("Int32 round-trip got %v", got)
	}
	if got := UInt32ValueToUint32(UInt32ValueFromUint32(7)); got != 7 {
		t.Fatalf("UInt32 round-trip got %v", got)
	}
	if got := Int64ValueToInt64(Int64ValueFromInt64(-8)); got != -8 {
		t.Fatalf("Int64 round-trip got %v", got)
	}
	if got := UInt64ValueToUint64(UInt64ValueFromUint64(8)); got != 8 {
		t.Fatalf("UInt64 round-trip got %v", got)
	}
	if got := BoolValueToBool(BoolValueFromBool(true)); got != true {
		t.Fatalf("Bool round-trip got %v", got)
	}
	if got := StringValueToString(StringValueFromString("ok")); got != "ok" {
		t.Fatalf("String round-trip got %q", got)
	}
	if got := BytesValueToBytes(BytesValueFromBytes([]byte{4, 5})); !reflect.DeepEqual(got, []byte{4, 5}) {
		t.Fatalf("Bytes round-trip got %v", got)
	}
}
