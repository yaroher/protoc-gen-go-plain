package test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestIntoPlainAndBack(t *testing.T) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}},
		},
		ParentEventId: "parent",
	}

	plain := in.IntoPlain()
	require.NotNil(t, plain, "IntoPlain returned nil")
	require.NotNil(t, plain.Path, "expected Path to be set")
	require.Equal(t, "/tmp/a", *plain.Path)
	require.Equal(t, "/tmp/b", plain.NonPlatformEventPath)
	require.NotEmpty(t, plain.PathCRF)
	require.Contains(t, plain.PathCRF, "file_rename")

	out := plain.IntoPb()
	require.NotNil(t, out, "IntoPb returned nil")
	require.True(t, proto.Equal(in, out), "roundtrip mismatch\ninput:  %v\noutput: %v", in, out)

	data, err := json.Marshal(plain)
	require.NoError(t, err)
	t.Logf("plain: %s", string(data))

	var decoded EventPlain
	require.NoError(t, json.Unmarshal(data, &decoded))

	outDecoded := decoded.IntoPb()
	require.NotNil(t, outDecoded, "IntoPb returned nil after json roundtrip")
	require.True(t, proto.Equal(in, outDecoded), "json roundtrip mismatch\ninput:  %v\noutput: %v", in, outDecoded)
}

func BenchmarkConverters(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}},
		},
		ParentEventId: "parent",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		plain := in.IntoPlain()
		if plain == nil || plain.Path == nil {
			b.Fatal("IntoPlain returned nil")
		}
		out := plain.IntoPb()
		if out == nil {
			b.Fatal("IntoPb returned nil")
		}
		if !proto.Equal(in, out) {
			b.Fatal("roundtrip mismatch")
		}
	}
}

func BenchmarkConvertersJSON(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}},
		},
		ParentEventId: "parent",
	}

	plain := in.IntoPlain()
	if plain == nil {
		b.Fatal("IntoPlain returned nil")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(plain)
		if err != nil {
			b.Fatal(err)
		}

		var decoded EventPlain
		if err := json.Unmarshal(data, &decoded); err != nil {
			b.Fatal(err)
		}

		out := decoded.IntoPb()
		if out == nil {
			b.Fatal("IntoPb returned nil after json roundtrip")
		}
		if !proto.Equal(in, out) {
			b.Fatal("json roundtrip mismatch")
		}
	}
}
