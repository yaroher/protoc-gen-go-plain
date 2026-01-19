package test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/yaroher/protoc-gen-go-plain/cast"
	"google.golang.org/protobuf/proto"
)

func TestGeneratedConverters(t *testing.T) {
	const uuidValue = "11111111-1111-1111-1111-111111111111"

	stringToUUID := cast.Caster[string, uuid.UUID](func(s string) uuid.UUID {
		if s == "" {
			return uuid.Nil
		}
		u, err := uuid.Parse(s)
		if err != nil {
			t.Fatalf("failed to parse uuid: %v", err)
		}
		return u
	})

	uuidToString := cast.Caster[uuid.UUID, string](func(u uuid.UUID) string {
		return u.String()
	})

	uuidErr := cast.CasterErr[string, uuid.UUID](func(s string) (uuid.UUID, error) {
		if s == "" {
			return uuid.Nil, nil
		}
		return uuid.Parse(s)
	})

	pb := &TestMessage{
		Id: &IdAlias{Value: uuidValue},
		Embed: &EmbedWithAlias{
			EmbedId: &IdAlias{Value: uuidValue},
		},
		FString:   "hello",
		FDouble:   1.5,
		FOptInt32: proto.Int32(42),
		FOneof: &TestMessage_FOneofString{
			FOneofString: "oneof",
		},
		FNestedMessageSerialized: &NestedMessage{},
	}

	plain := pb.IntoPlain(stringToUUID)
	if plain == nil {
		t.Fatalf("IntoPlain returned nil")
	}

	if plain.Id != uuid.MustParse(uuidValue) {
		t.Fatalf("unexpected plain Id: %v", plain.Id)
	}

	if plain.EmbedId != uuid.MustParse(uuidValue) {
		t.Fatalf("unexpected plain embedded Id: %v", plain.EmbedId)
	}

	if plain.FOptInt32 == nil || *plain.FOptInt32 != 42 {
		t.Fatalf("optional int32 not preserved")
	}

	if _, err := pb.IntoPlainErr(uuidErr); err != nil {
		t.Fatalf("IntoPlainErr failed: %v", err)
	}

	back := plain.IntoPb(uuidToString)
	if back == nil {
		t.Fatalf("IntoPb returned nil")
	}

	if back.GetId().GetValue() != uuidValue {
		t.Fatalf("id mismatch after roundtrip")
	}

	if back.GetEmbed().GetEmbedId().GetValue() != uuidValue {
		t.Fatalf("embedded id mismatch after roundtrip")
	}

	if back.GetFOptInt32() != 42 {
		t.Fatalf("optional int32 lost after roundtrip")
	}
}
