package test

import (
	"reflect"
	"testing"
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
