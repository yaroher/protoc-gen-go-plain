package crf

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yaroher/protoc-gen-go-plain/cast"
	"google.golang.org/protobuf/proto"
)

func newTestCasterToPb() cast.Caster[uuid.UUID, string] {
	return cast.CasterFn(
		func(v uuid.UUID) string {
			if v == uuid.Nil {
				return ""
			}
			return v.String()
		},
	)
}

func newTestCasterToPlain() cast.Caster[string, uuid.UUID] {
	return cast.CasterFn(
		func(v string) uuid.UUID {
			if v == "" {
				return uuid.Nil
			}
			id, err := uuid.Parse(v)
			if err != nil {
				return uuid.Nil
			}
			return id
		},
	)
}

func newTestCasterErrToPb() cast.CasterErr[uuid.UUID, string] {
	return cast.CasterErrFn(
		func(v uuid.UUID) (string, error) {
			if v == uuid.Nil {
				return "", nil
			}
			return v.String(), nil
		},
	)
}

func newTestCasterErrToPlain() cast.CasterErr[string, uuid.UUID] {
	return cast.CasterErrFn(
		func(v string) (uuid.UUID, error) {
			if v == "" {
				return uuid.Nil, nil
			}
			return uuid.Parse(v)
		},
	)
}

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

	casterToPlain := newTestCasterToPlain()
	casterToPb := newTestCasterToPb()
	plain := in.IntoPlain(casterToPlain)
	require.NotNil(t, plain, "IntoPlain returned nil")
	require.NotNil(t, plain.Path, "expected Path to be set")
	require.Equal(t, "/tmp/a", *plain.Path)
	require.Equal(t, "/tmp/b", plain.NonPlatformEventPath)
	require.NotEmpty(t, plain.PathCRF)
	require.Contains(t, plain.PathCRF, "file_rename")

	out := plain.IntoPb(casterToPb)
	require.NotNil(t, out, "IntoPb returned nil")
	require.True(t, proto.Equal(in, out), "roundtrip mismatch\ninput:  %v\noutput: %v", in, out)

	data, err := json.Marshal(plain)
	require.NoError(t, err)
	t.Logf("plain: %s", string(data))

	var decoded EventPlain
	require.NoError(t, json.Unmarshal(data, &decoded))

	outDecoded := decoded.IntoPb(casterToPb)
	require.NotNil(t, outDecoded, "IntoPb returned nil after json roundtrip")
	require.True(t, proto.Equal(in, outDecoded), "json roundtrip mismatch\ninput:  %v\noutput: %v", in, outDecoded)

	casterErrToPlain := newTestCasterErrToPlain()
	casterErrToPb := newTestCasterErrToPb()
	plainErr, err := in.IntoPlainErr(casterErrToPlain)
	require.NoError(t, err)
	require.NotNil(t, plainErr)

	outErr, err := plainErr.IntoPbErr(casterErrToPb)
	require.NoError(t, err)
	require.NotNil(t, outErr)
	require.True(t, proto.Equal(in, outErr), "err roundtrip mismatch\ninput:  %v\noutput: %v", in, outErr)
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

	casterToPlain := newTestCasterToPlain()
	casterToPb := newTestCasterToPb()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		plain := in.IntoPlain(casterToPlain)
		if plain == nil || plain.Path == nil {
			b.Fatal("IntoPlain returned nil")
		}
		out := plain.IntoPb(casterToPb)
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

	casterToPlain := newTestCasterToPlain()
	casterToPb := newTestCasterToPb()
	plain := in.IntoPlain(casterToPlain)
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

		out := decoded.IntoPb(casterToPb)
		if out == nil {
			b.Fatal("IntoPb returned nil after json roundtrip")
		}
		if !proto.Equal(in, out) {
			b.Fatal("json roundtrip mismatch")
		}
	}
}
