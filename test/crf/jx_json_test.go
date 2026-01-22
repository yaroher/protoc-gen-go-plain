package crf

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestIntoPlainJSONRoundtrip(t *testing.T) {
	in := &Event{
		EventId: 2,
		Process: &Process{File: &File{Path: "/proc2"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/c"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/d"}}},
		ParentEventId: "parent2",
	}

	casterToPlain := newTestCasterToPlain()
	casterToPb := newTestCasterToPb()

	plain := in.IntoPlain(casterToPlain)
	require.NotNil(t, plain, "IntoPlain returned nil")

	data, err := json.Marshal(plain)
	require.NoError(t, err)

	var decoded EventPlain
	require.NoError(t, json.Unmarshal(data, &decoded))

	out := decoded.IntoPb(casterToPb)
	require.NotNil(t, out, "IntoPb returned nil after json roundtrip")
	require.True(t, proto.Equal(in, out), "json roundtrip mismatch\ninput:  %v\noutput: %v", in, out)
}

func TestJXUnmarshalNullPath(t *testing.T) {
	payload := []byte(`{"path":null,"pathCRF":"","nonPlatformEventPath":"/tmp/x","eventVirtualType":"","parentEventId":"parent","eventId":3}`)
	var out EventPlain
	require.NoError(t, out.UnmarshalJSON(payload))
	require.Nil(t, out.Path)
	require.Equal(t, "/tmp/x", out.NonPlatformEventPath)
}

func TestJXUnmarshalUnknownField(t *testing.T) {
	payload := []byte(`{"pathCRF":"file_rename/file/path","unknown":123}`)
	var out EventPlain
	require.NoError(t, out.UnmarshalJSON(payload))
	require.Equal(t, "file_rename/file/path", out.PathCRF)
}

func TestWritePlainJSONSample(t *testing.T) {
	in := &Event{
		EventId: 4,
		Process: &Process{File: &File{Path: "/proc4"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/e"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/f"}}},
		ParentEventId: "parent4",
	}
	plain := in.IntoPlain(newTestCasterToPlain())
	require.NotNil(t, plain, "IntoPlain returned nil")

	data, err := json.MarshalIndent(plain, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("crf_sample.json", data, 0o644))
}

func BenchmarkMarshalPlainJX(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}}},
		ParentEventId: "parent",
	}
	plain := in.IntoPlain(newTestCasterToPlain())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plain.MarshalJSON(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalPlainStdJSON(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}}},
		ParentEventId: "parent",
	}
	plain := in.IntoPlain(newTestCasterToPlain())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(plain); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalProtoJSON(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}}},
		ParentEventId: "parent",
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
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}}},
		ParentEventId: "parent",
	}
	plain := in.IntoPlain(newTestCasterToPlain())
	raw, err := plain.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out EventPlain
		if err := out.UnmarshalJSON(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalPlainStdJSON(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}}},
		ParentEventId: "parent",
	}
	plain := in.IntoPlain(newTestCasterToPlain())
	raw, err := json.Marshal(plain)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out EventPlain
		if err := json.Unmarshal(raw, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalProtoJSON(b *testing.B) {
	in := &Event{
		EventId: 1,
		Process: &Process{File: &File{Path: "/proc"}},
		Data: &EventData{
			PlatformEvent:    &EventData_FileRename{FileRename: &FileRename{File: &File{Path: "/tmp/a"}}},
			NonPlatformEvent: &EventData_File{File: &File{Path: "/tmp/b"}}},
		ParentEventId: "parent",
	}
	raw, err := protojson.Marshal(in)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out Event
		if err := protojson.Unmarshal(raw, &out); err != nil {
			b.Fatal(err)
		}
	}
}
