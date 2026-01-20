package wkt_map

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func sampleUser() *User {
	anyMsg, _ := anypb.New(&Address{Street: "Main"})
	return &User{
		FAny:      anyMsg,
		FDuration: durationpb.New(5),
		FEmpty:    &emptypb.Empty{},
		FStruct:   structpb.NewStringValue("v").GetStructValue(),
		FTs:       timestamppb.Now(),
		FBool:     wrapperspb.Bool(true),
		FString:   wrapperspb.String("x"),
		FMapInt32: map[string]int32{"k": 1},
		FMapMsg:   map[string]*Address{"a": {Street: "S"}},
	}
}

func sampleUserPlain() *UserPlain {
	return sampleUser().IntoPlain()
}

func marshalJxWithUser(m *UserPlain) ([]byte, error) {
	return m.MarshalJSON()
}

func unmarshalJxWithUser(data []byte, dst *UserPlain) error {
	return dst.UnmarshalJSON(data)
}

func BenchmarkProtojsonMarshal(b *testing.B) {
	msg := sampleUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = protojson.Marshal(msg)
	}
}

func BenchmarkProtojsonUnmarshal(b *testing.B) {
	msg := sampleUser()
	data, _ := protojson.Marshal(msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst User
		_ = protojson.Unmarshal(data, &dst)
	}
}

func BenchmarkJXMarshal(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.MarshalJSON()
	}
}

func BenchmarkJXUnmarshal(b *testing.B) {
	msg := sampleUserPlain()
	data, _ := msg.MarshalJSON()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst UserPlain
		_ = dst.UnmarshalJSON(data)
	}
}

func BenchmarkJXWithMarshal(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = marshalJxWithUser(msg)
	}
}

func BenchmarkJXWithUnmarshal(b *testing.B) {
	msg := sampleUserPlain()
	data, _ := marshalJxWithUser(msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst UserPlain
		_ = unmarshalJxWithUser(data, &dst)
	}
}
