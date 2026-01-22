package full

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestIntoPlainJSONRoundtrip(t *testing.T) {
	note := "note"
	in := &Complex{
		Base:      &Base{Source: "api"},
		Extra:     &Extra{Id: "extra-id", Tag: "tag"},
		Name:      "complex",
		Labels:    []string{"a", "b"},
		Note:      &note,
		Counters:  map[string]int32{"a": 1, "b": 2},
		CreatedAt: timestamppb.New(time.Unix(10, 0)),
		Comment:   wrapperspb.String("comment"),
		Contact:   &Complex_Email{Email: "a@example.com"},
		CustomId:  "11111111-1111-1111-1111-111111111111",
	}

	plain := in.IntoPlain(newTestCasterToPlain())
	require.NotNil(t, plain)

	data, err := json.Marshal(plain)
	require.NoError(t, err)

	var decoded ComplexPlain
	require.NoError(t, json.Unmarshal(data, &decoded))

	out := decoded.IntoPb(newTestCasterToPb())
	require.NotNil(t, out)
	require.True(t, proto.Equal(in, out))
}

func BenchmarkMarshalPlainJX(b *testing.B) {
	note := "note"
	in := &Complex{
		Base:      &Base{Source: "api"},
		Extra:     &Extra{Id: "extra-id", Tag: "tag"},
		Name:      "complex",
		Labels:    []string{"a", "b"},
		Note:      &note,
		Counters:  map[string]int32{"a": 1, "b": 2},
		CreatedAt: timestamppb.New(time.Unix(10, 0)),
		Comment:   wrapperspb.String("comment"),
		Contact:   &Complex_Email{Email: "a@example.com"},
		CustomId:  "11111111-1111-1111-1111-111111111111",
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

func BenchmarkMarshalProtoJSON(b *testing.B) {
	note := "note"
	in := &Complex{
		Base:      &Base{Source: "api"},
		Extra:     &Extra{Id: "extra-id", Tag: "tag"},
		Name:      "complex",
		Labels:    []string{"a", "b"},
		Note:      &note,
		Counters:  map[string]int32{"a": 1, "b": 2},
		CreatedAt: timestamppb.New(time.Unix(10, 0)),
		Comment:   wrapperspb.String("comment"),
		Contact:   &Complex_Email{Email: "a@example.com"},
		CustomId:  "11111111-1111-1111-1111-111111111111",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := protojson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}
