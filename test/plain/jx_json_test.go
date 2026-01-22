package plain

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestIntoPlainJSONRoundtrip(t *testing.T) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-2",
				Source: "cli",
			},
			Name: "Bob",
			Contact: &User_Phone{
				Phone: "+123",
			},
		},
	}

	plain := in.IntoPlain()
	require.NotNil(t, plain, "IntoPlain returned nil")

	data, err := json.Marshal(plain)
	require.NoError(t, err)

	var decoded UserEventPlain
	require.NoError(t, json.Unmarshal(data, &decoded))

	out := decoded.IntoPb()
	require.NotNil(t, out, "IntoPb returned nil after json roundtrip")
	require.True(t, proto.Equal(in, out), "json roundtrip mismatch\ninput:  %v\noutput: %v", in, out)
}

func TestJXUnmarshalNullOptional(t *testing.T) {
	payload := []byte(`{"contactEmail":null,"id":"u-3","source":"api","eventType":"updated","name":"Jane","unknown":123}`)
	var out UserEventPlain
	require.NoError(t, out.UnmarshalJSON(payload))
	require.Nil(t, out.ContactEmail)
	require.Equal(t, "u-3", out.Id)
	require.Equal(t, "Jane", out.Name)
}

func TestWritePlainJSONSample(t *testing.T) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-10",
				Source: "ui",
			},
			Name: "Eve",
			Contact: &User_Email{
				Email: "eve@example.com",
			},
		},
	}
	plain := in.IntoPlain()
	require.NotNil(t, plain, "IntoPlain returned nil")

	data, err := json.MarshalIndent(plain, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("plain_sample.json", data, 0o644))
}

func BenchmarkMarshalPlainJX(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}
	plain := in.IntoPlain()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plain.MarshalJSON(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalPlainStdJSON(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}
	plain := in.IntoPlain()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(plain); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalProtoJSON(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := protojson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalPlainJX(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}
	plain := in.IntoPlain()
	raw, err := plain.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out UserEventPlain
		if err := out.UnmarshalJSON(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalPlainStdJSON(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}
	plain := in.IntoPlain()
	raw, err := json.Marshal(plain)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out UserEventPlain
		if err := json.Unmarshal(raw, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalProtoJSON(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}
	raw, err := protojson.Marshal(in)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out UserEvent
		if err := protojson.Unmarshal(raw, &out); err != nil {
			b.Fatal(err)
		}
	}
}
