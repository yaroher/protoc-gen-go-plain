package cast

import (
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func protoNew[T proto.Message]() (model T) {
	return model.ProtoReflect().Type().New().Interface().(T)
}

func EnumToInt32[T protoreflect.Enum](v T) int32 {
	return int32(v.Number())
}
func EnumToSliceInt32[T protoreflect.Enum](v []T) []int32 {
	result := make([]int32, len(v))
	for i, el := range v {
		result[i] = EnumToInt32[T](el)
	}
	return result
}
func EnumFromInt32[T protoreflect.Enum](v int32) (ret T) {
	return ret.Type().New(protoreflect.EnumNumber(v)).(T)
}
func EnumFromSliceInt32[T protoreflect.Enum](v []int32) []T {
	result := make([]T, len(v))
	for i, el := range v {
		result[i] = EnumFromInt32[T](el)
	}
	return result
}

func TimestampToTime(t *timestamppb.Timestamp) time.Time {
	return t.AsTime()
}
func TimestampToPtrTime(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}
func TimestampFromTime(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}
func TimestampFromPtrTime(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return TimestampFromTime(*t)
}

func DurationToTime(d *durationpb.Duration) time.Duration {
	if d == nil {
		return 0
	}
	return time.Duration(d.Seconds)*time.Second + time.Duration(d.Nanos)*time.Nanosecond
}
func DurationToPtrTime(d *durationpb.Duration) *time.Duration {
	if d == nil {
		return nil
	}
	v := DurationToTime(d)
	return &v
}
func DurationFromTime(d time.Duration) *durationpb.Duration {
	return &durationpb.Duration{
		Seconds: int64(d / time.Second),
		Nanos:   int32(d % time.Second / time.Nanosecond),
	}
}
func DurationFromPtrTime(d *time.Duration) *durationpb.Duration {
	if d == nil {
		return nil
	}
	return DurationFromTime(*d)
}

func StructToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}
func StructToPtrMap(s *structpb.Struct) *map[string]any {
	if s == nil {
		return nil
	}
	m := s.AsMap()
	return &m
}
func StructFromMap(m map[string]any) *structpb.Struct {
	if m == nil {
		return nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}
func StructFromPtrMap(m *map[string]any) *structpb.Struct {
	if m == nil {
		return nil
	}
	return StructFromMap(*m)
}

func ValueToInterface(v *structpb.Value) any {
	if v == nil {
		return nil
	}
	return v.AsInterface()
}
func ValueToPtrInterface(v *structpb.Value) *any {
	if v == nil {
		return nil
	}
	i := v.AsInterface()
	return &i
}
func ValueFromInterface(v any) *structpb.Value {
	if v == nil {
		return nil
	}
	val, err := structpb.NewValue(v)
	if err != nil {
		panic(err)
	}
	return val
}
func ValueFromPtrInterface(v *any) *structpb.Value {
	if v == nil {
		return nil
	}
	return ValueFromInterface(*v)
}

func AnyToMessage[T proto.Message](v *anypb.Any) T {
	var zero T
	if v == nil {
		return zero
	}
	ret := protoNew[T]()
	if err := anypb.UnmarshalTo(v, ret, proto.UnmarshalOptions{}); err != nil {
		panic(err)
	}
	return ret
}
func AnyFromMessage[T proto.Message](v T) *anypb.Any {
	if !v.ProtoReflect().IsValid() {
		return nil
	}
	ret, err := anypb.New(v)
	if err != nil {
		panic(err)
	}
	return ret
}

func EmptyToStruct(v *emptypb.Empty) struct{} {
	_ = v
	return struct{}{}
}
func EmptyToPtrStruct(v *emptypb.Empty) *struct{} {
	if v == nil {
		return nil
	}
	ret := struct{}{}
	return &ret
}
func EmptyFromStruct(v struct{}) *emptypb.Empty {
	_ = v
	return &emptypb.Empty{}
}
func EmptyFromPtrStruct(v *struct{}) *emptypb.Empty {
	if v == nil {
		return nil
	}
	return EmptyFromStruct(*v)
}

func DoubleValueToFloat64(v *wrapperspb.DoubleValue) float64 {
	if v == nil {
		return 0
	}
	return v.Value
}
func DoubleValueToPtrFloat64(v *wrapperspb.DoubleValue) *float64 {
	if v == nil {
		return nil
	}
	return &v.Value
}
func DoubleValueFromFloat64(v float64) *wrapperspb.DoubleValue {
	return &wrapperspb.DoubleValue{Value: v}
}
func DoubleValueFromPtrFloat64(v *float64) *wrapperspb.DoubleValue {
	if v == nil {
		return nil
	}
	return DoubleValueFromFloat64(*v)
}

func FloatValueToFloat32(v *wrapperspb.FloatValue) float32 {
	if v == nil {
		return 0
	}
	return v.Value
}
func FloatValueToPtrFloat32(v *wrapperspb.FloatValue) *float32 {
	if v == nil {
		return nil
	}
	return &v.Value
}
func FloatValueFromFloat32(v float32) *wrapperspb.FloatValue {
	return &wrapperspb.FloatValue{Value: v}
}
func FloatValueFromPtrFloat32(v *float32) *wrapperspb.FloatValue {
	if v == nil {
		return nil
	}
	return FloatValueFromFloat32(*v)
}

func Int32ValueToInt32(v *wrapperspb.Int32Value) int32 {
	if v == nil {
		return 0
	}
	return v.Value
}
func Int32ValueToPtrInt32(v *wrapperspb.Int32Value) *int32 {
	if v == nil {
		return nil
	}
	return &v.Value
}
func Int32ValueFromInt32(v int32) *wrapperspb.Int32Value {
	return &wrapperspb.Int32Value{Value: v}
}
func Int32ValueFromPtrInt32(v *int32) *wrapperspb.Int32Value {
	if v == nil {
		return nil
	}
	return Int32ValueFromInt32(*v)
}

func UInt32ValueToUint32(v *wrapperspb.UInt32Value) uint32 {
	if v == nil {
		return 0
	}
	return v.Value
}
func UInt32ValueToPtrUint32(v *wrapperspb.UInt32Value) *uint32 {
	if v == nil {
		return nil
	}
	return &v.Value
}
func UInt32ValueFromUint32(v uint32) *wrapperspb.UInt32Value {
	return &wrapperspb.UInt32Value{Value: v}
}
func UInt32ValueFromPtrUint32(v *uint32) *wrapperspb.UInt32Value {
	if v == nil {
		return nil
	}
	return UInt32ValueFromUint32(*v)
}

func Int64ValueToInt64(v *wrapperspb.Int64Value) int64 {
	if v == nil {
		return 0
	}
	return v.Value
}
func Int64ValueToPtrInt64(v *wrapperspb.Int64Value) *int64 {
	if v == nil {
		return nil
	}
	return &v.Value
}
func Int64ValueFromInt64(v int64) *wrapperspb.Int64Value {
	return &wrapperspb.Int64Value{Value: v}
}
func Int64ValueFromPtrInt64(v *int64) *wrapperspb.Int64Value {
	if v == nil {
		return nil
	}
	return Int64ValueFromInt64(*v)
}

func UInt64ValueToUint64(v *wrapperspb.UInt64Value) uint64 {
	if v == nil {
		return 0
	}
	return v.Value
}
func UInt64ValueToPtrUint64(v *wrapperspb.UInt64Value) *uint64 {
	if v == nil {
		return nil
	}
	return &v.Value
}
func UInt64ValueFromUint64(v uint64) *wrapperspb.UInt64Value {
	return &wrapperspb.UInt64Value{Value: v}
}
func UInt64ValueFromPtrUint64(v *uint64) *wrapperspb.UInt64Value {
	if v == nil {
		return nil
	}
	return UInt64ValueFromUint64(*v)
}

func BoolValueToBool(v *wrapperspb.BoolValue) bool {
	if v == nil {
		return false
	}
	return v.Value
}
func BoolValueToPtrBool(v *wrapperspb.BoolValue) *bool {
	if v == nil {
		return nil
	}
	return &v.Value
}
func BoolValueFromBool(v bool) *wrapperspb.BoolValue {
	return &wrapperspb.BoolValue{Value: v}
}
func BoolValueFromPtrBool(v *bool) *wrapperspb.BoolValue {
	if v == nil {
		return nil
	}
	return BoolValueFromBool(*v)
}

func StringValueToString(v *wrapperspb.StringValue) string {
	if v == nil {
		return ""
	}
	return v.Value
}
func StringValueToPtrString(v *wrapperspb.StringValue) *string {
	if v == nil {
		return nil
	}
	return &v.Value
}
func StringValueFromString(v string) *wrapperspb.StringValue {
	return &wrapperspb.StringValue{Value: v}
}
func StringValueFromPtrString(v *string) *wrapperspb.StringValue {
	if v == nil {
		return nil
	}
	return StringValueFromString(*v)
}

func BytesValueToBytes(v *wrapperspb.BytesValue) []byte {
	if v == nil {
		return nil
	}
	return v.Value
}
func BytesValueToPtrBytes(v *wrapperspb.BytesValue) *[]byte {
	if v == nil {
		return nil
	}
	return &v.Value
}
func BytesValueFromBytes(v []byte) *wrapperspb.BytesValue {
	return &wrapperspb.BytesValue{Value: v}
}
func BytesValueFromPtrBytes(v *[]byte) *wrapperspb.BytesValue {
	if v == nil {
		return nil
	}
	return BytesValueFromBytes(*v)
}

func MessageToSliceByteSlice[T proto.Message](v []T) [][]byte {
	result := make([][]byte, len(v))
	for i, el := range v {
		result[i] = MessageToSliceByte[T](el)
	}
	return result
}
func MessageFromSliceByteSlice[T proto.Message](v [][]byte) []T {
	result := make([]T, len(v))
	for i, el := range v {
		result[i] = MessageFromSliceByte[T](el)
	}
	return result
}
func MessageToSliceByte[T proto.Message](v T) []byte {
	if !v.ProtoReflect().IsValid() {
		return nil
	}
	b, err := protojson.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func MessageToSliceByteErr[T proto.Message](v T) ([]byte, error) {
	if !v.ProtoReflect().IsValid() {
		return nil, nil
	}
	return protojson.Marshal(v)
}
func MessageFromSliceByte[T proto.Message](v []byte) T {
	ret := protoNew[T]()
	err := protojson.Unmarshal(v, ret)
	if err != nil {
		panic(err)
	}
	return ret
}
func MessageFromSliceByteErr[T proto.Message](v []byte) (T, error) {
	ret := protoNew[T]()
	if err := protojson.Unmarshal(v, ret); err != nil {
		var zero T
		return zero, err
	}
	return ret, nil
}
