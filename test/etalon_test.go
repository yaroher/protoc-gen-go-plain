package test

import (
	"reflect"
	"testing"
	"time"

	uuid "github.com/google/uuid"
	cast "github.com/yaroher/protoc-gen-go-plain/cast"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fieldSig struct {
	Name      string
	Type      string
	Anonymous bool
}

func TestPlainStructMatchesEtalon(t *testing.T) {
	got := reflect.TypeOf(TestMessagePlain{})
	want := reflect.TypeOf(TestMessagePlainEtalon{})

	if got.Kind() != reflect.Struct || want.Kind() != reflect.Struct {
		t.Fatalf("expected struct types, got %v and %v", got.Kind(), want.Kind())
	}

	gotFields := collectFieldSigs(got)
	wantFields := collectFieldSigs(want)

	if len(gotFields) != len(wantFields) {
		t.Fatalf("field count mismatch: got %d, want %d", len(gotFields), len(wantFields))
	}

	for i := range wantFields {
		if gotFields[i] != wantFields[i] {
			t.Fatalf("field %d mismatch: got %+v, want %+v", i, gotFields[i], wantFields[i])
		}
	}
}

func TestIntoPlainConversion(t *testing.T) {
	pb, want := sampleMessage()
	got := pb.IntoPlain(
		WithTestMessageMeta(42),
		WithTestMessageTraceId(want.TraceId),
		WithTestMessageDebug("debug"),
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IntoPlain mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestIntoPlainDeepConversion(t *testing.T) {
	pb, want := sampleMessage()
	got := pb.IntoPlainDeep(
		WithTestMessageMeta(42),
		WithTestMessageTraceId(want.TraceId),
		WithTestMessageDebug("debug"),
	)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IntoPlainDeep mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestIntoPbConversion(t *testing.T) {
	want, plain := sampleMessage()
	got := plain.IntoPb()
	if !proto.Equal(got, want) {
		t.Fatalf("IntoPb mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestIntoPbDeepConversion(t *testing.T) {
	want, plain := sampleMessage()
	got := plain.IntoPbDeep()
	if !proto.Equal(got, want) {
		t.Fatalf("IntoPbDeep mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func collectFieldSigs(t reflect.Type) []fieldSig {
	fields := make([]fieldSig, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fields = append(fields, fieldSig{
			Name:      f.Name,
			Type:      f.Type.String(),
			Anonymous: f.Anonymous,
		})
	}
	return fields
}

func sampleMessage() (*TestMessage, *TestMessagePlain) {
	uuidStr := "550e8400-e29b-41d4-a716-446655440000"
	uuidVal := uuid.MustParse(uuidStr)
	traceID := uuid.MustParse("880e8400-e29b-41d4-a716-446655440000")

	oidcID := "oidc-id"
	id := "id"
	embedOidcID := "embed-oidc-id"
	embedID := "embed-id"

	optInt32 := int32(123)
	optString := "optional"
	optEnum := TestEnum_TEST_ENUM_TWO

	repMsg1 := &NestedMessage{Name: "rep-1"}
	repMsg2 := &NestedMessage{Name: "rep-2"}
	repSer1 := &NestedMessage{Name: "rep-ser-1"}
	repSer2 := &NestedMessage{Name: "rep-ser-2"}

	nested := &NestedMessage{Name: "nested"}
	optNested := &NestedMessage{Name: "opt-nested"}
	embedded := &NestedMessage{
		Name: "embedded",
		Inner: &NestedMessage_InnerMessage{
			InnerInner: &NestedMessage_InnerMessage_InnerInnerMessage{
				Depth: 1,
				Note:  "inner",
			},
		},
	}
	nestedSerialized := &NestedMessage{Name: "serialized"}

	anyMsg, err := anypb.New(&NestedMessage{Name: "any"})
	if err != nil {
		panic(err)
	}

	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	dur := 5*time.Second + 250*time.Millisecond
	tsPB := timestamppb.New(ts)
	durPB := durationpb.New(dur)

	structMap := map[string]any{"name": "bob", "count": float64(3), "active": true}
	structPB, err := structpb.NewStruct(structMap)
	if err != nil {
		panic(err)
	}
	valPB, err := structpb.NewValue("value")
	if err != nil {
		panic(err)
	}
	listPB, err := structpb.NewList([]any{"a", float64(1), true})
	if err != nil {
		panic(err)
	}

	wktDouble := wrapperspb.Double(1.5)
	wktFloat := wrapperspb.Float(2.5)
	wktInt64 := wrapperspb.Int64(64)
	wktUint64 := wrapperspb.UInt64(128)
	wktInt32 := wrapperspb.Int32(32)
	wktUint32 := wrapperspb.UInt32(64)
	wktBool := wrapperspb.Bool(true)
	wktString := wrapperspb.String("wrap")
	wktBytes := wrapperspb.Bytes([]byte{0x0a, 0x0b})

	oneofBytes := []byte{0x0c, 0x0d}
	doubleNested := &NestedMessage_InnerMessage_InnerInnerMessage{Depth: 7, Note: "deep"}

	pb := &TestMessage{
		OidcId: &OidcIdAlias{Value: oidcID},
		Id:     &IdAlias{Value: id},
		Embed: &EmbedWithAlias{
			EmbedOidcId: &OidcIdAlias{Value: embedOidcID},
			EmbedId:     &IdAlias{Value: embedID},
		},
		FDouble:     1.1,
		FFloat:      2.2,
		FInt32:      -3,
		FInt64:      -4,
		FUint32:     5,
		FUint64:     6,
		FSint32:     -7,
		FSint64:     -8,
		FFixed32:    9,
		FFixed64:    10,
		FSfixed32:   -11,
		FSfixed64:   -12,
		FBool:       true,
		FString:     "hello",
		FUuid:       uuidStr,
		FBytes:      []byte{0x01, 0x02},
		FOptInt32:   &optInt32,
		FOptString:  &optString,
		FOptMessage: optNested,
		FOptEnum:    &optEnum,
		FRepInt32:   []int32{1, 2, 3},
		FRepString:  []string{"a", "b"},
		FRepMessage: []*NestedMessage{
			repMsg1,
			repMsg2,
		},
		FRepMessageSerialized: []*NestedMessage{
			repSer1,
			repSer2,
		},
		FRepEnum: []TestEnum{TestEnum_TEST_ENUM_ONE, TestEnum_TEST_ENUM_TWO},
		FMapInt32String: map[int32]string{
			1: "one",
		},
		FMapInt64Int32: map[int64]int32{
			2: 3,
		},
		FMapUint32Uint64: map[uint32]uint64{
			4: 5,
		},
		FMapUint64Bool: map[uint64]bool{
			6: true,
		},
		FMapSint32Bytes: map[int32][]byte{
			7: []byte{0x07},
		},
		FMapSint64Float: map[int64]float32{
			8: 9.5,
		},
		FMapFixed32Double: map[uint32]float64{
			10: 11.5,
		},
		FMapFixed64Message: map[uint64]*NestedMessage{
			12: {Name: "map-msg"},
		},
		FMapSfixed32Enum: map[int32]TestEnum{
			13: TestEnum_TEST_ENUM_ONE,
		},
		FMapSfixed64String: map[int64]string{
			14: "fourteen",
		},
		FMapBoolInt32: map[bool]int32{
			true: 15,
		},
		FMapStringString: map[string]string{
			"key": "value",
		},
		FOneof:                   &TestMessage_FOneofBytes{FOneofBytes: oneofBytes},
		FNestedMessage:           nested,
		FNestedMessageEmbedded:   embedded,
		FNestedMessageSerialized: nestedSerialized,
		FEnum:                    TestEnum_TEST_ENUM_ONE,
		FAny:                     anyMsg,
		FTimestamp:               tsPB,
		FDuration:                durPB,
		FStruct:                  structPB,
		FValue:                   valPB,
		FListValue:               listPB,
		FWktDouble:               wktDouble,
		FWktFloat:                wktFloat,
		FWktInt64:                wktInt64,
		FWktUint64:               wktUint64,
		FWktInt32:                wktInt32,
		FWktUint32:               wktUint32,
		FWktBool:                 wktBool,
		FWktString:               wktString,
		FWktBytes:                wktBytes,
		FDoubleNested:            doubleNested,
	}

	plain := &TestMessagePlain{
		Meta:                     42,
		TraceId:                  traceID,
		Debug:                    "debug",
		OidcId:                   oidcID,
		Id:                       id,
		EmbedOidcId:              embedOidcID,
		EmbedId:                  embedID,
		FDouble:                  pb.FDouble,
		FFloat:                   pb.FFloat,
		FInt32:                   pb.FInt32,
		FInt64:                   pb.FInt64,
		FUint32:                  pb.FUint32,
		FUint64:                  pb.FUint64,
		FSint32:                  pb.FSint32,
		FSint64:                  pb.FSint64,
		FFixed32:                 pb.FFixed32,
		FFixed64:                 pb.FFixed64,
		FSfixed32:                pb.FSfixed32,
		FSfixed64:                pb.FSfixed64,
		FBool:                    pb.FBool,
		FString:                  pb.FString,
		FUuid:                    uuidVal,
		FBytes:                   pb.FBytes,
		FOptInt32:                &optInt32,
		FOptString:               &optString,
		FOptMessage:              optNested,
		FOptEnum:                 &optEnum,
		FRepInt32:                pb.FRepInt32,
		FRepString:               pb.FRepString,
		FRepMessage:              pb.FRepMessage,
		FRepMessageSerialized:    cast.MessageToSliceByteSlice(pb.FRepMessageSerialized),
		FRepEnum:                 pb.FRepEnum,
		FMapInt32String:          pb.FMapInt32String,
		FMapInt64Int32:           pb.FMapInt64Int32,
		FMapUint32Uint64:         pb.FMapUint32Uint64,
		FMapUint64Bool:           pb.FMapUint64Bool,
		FMapSint32Bytes:          pb.FMapSint32Bytes,
		FMapSint64Float:          pb.FMapSint64Float,
		FMapFixed32Double:        pb.FMapFixed32Double,
		FMapFixed64Message:       pb.FMapFixed64Message,
		FMapSfixed32Enum:         pb.FMapSfixed32Enum,
		FMapSfixed64String:       pb.FMapSfixed64String,
		FMapBoolInt32:            pb.FMapBoolInt32,
		FMapStringString:         pb.FMapStringString,
		FOneofBytes:              &oneofBytes,
		FNestedMessage:           nested,
		Name:                     embedded.Name,
		Inner:                    embedded.Inner,
		FNestedMessageSerialized: cast.MessageToSliceByte(nestedSerialized),
		FEnum:                    pb.FEnum,
		FAny:                     cast.MessageToSliceByte(anyMsg),
		FTimestamp:               cast.TimestampToPtrTime(tsPB),
		FDuration:                cast.DurationToPtrTime(durPB),
		FStruct:                  cast.StructToMap(structPB),
		FValue:                   cast.MessageToSliceByte(valPB),
		FListValue:               cast.MessageToSliceByte(listPB),
		FWktDouble:               cast.DoubleValueToPtrFloat64(wktDouble),
		FWktFloat:                cast.FloatValueToPtrFloat32(wktFloat),
		FWktInt64:                cast.Int64ValueToPtrInt64(wktInt64),
		FWktUint64:               cast.UInt64ValueToPtrUint64(wktUint64),
		FWktInt32:                cast.Int32ValueToPtrInt32(wktInt32),
		FWktUint32:               cast.UInt32ValueToPtrUint32(wktUint32),
		FWktBool:                 cast.BoolValueToPtrBool(wktBool),
		FWktString:               cast.StringValueToPtrString(wktString),
		FWktBytes:                cast.BytesValueToPtrBytes(wktBytes),
		FDoubleNested:            doubleNested,
	}

	return pb, plain
}
