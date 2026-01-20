package serialize

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
)

func sampleUser() *User {
	return &User{Settings: &Settings{Locale: "ru"}}
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
