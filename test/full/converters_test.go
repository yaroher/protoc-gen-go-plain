package full

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/yaroher/protoc-gen-go-plain/cast"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

func TestIntoPlainAndBack(t *testing.T) {
	note := "note"
	in := &Complex{
		Base:            &Base{Source: "api"},
		Extra:           &Extra{Id: "extra-id", Tag: "tag"},
		Name:            "complex",
		Labels:          []string{"a", "b"},
		Note:            &note,
		Counters:        map[string]int32{"a": 1, "b": 2},
		CreatedAt:       timestamppb.New(time.Unix(10, 0)),
		Comment:         wrapperspb.String("comment"),
		Contact:         &Complex_Email{Email: "a@example.com"},
		CustomId:        "11111111-1111-1111-1111-111111111111",
		AliasId:         &StringAlias{Value: "alias"},
		AliasList:       []*StringAlias{{Value: "a1"}, {Value: "a2"}},
		Int32Alias:      &Int32Alias{Value: 42},
		Int64Alias:      &Int64Alias{Value: 100},
		BoolAliasList:   []*BoolAlias{{Value: true}, {Value: false}},
		BytesAlias:      &BytesAlias{Value: []byte("bytes")},
		FloatAlias:      &FloatAlias{Value: 3.14},
		DoubleAlias:     &DoubleAlias{Value: 2.71},
		CustomNameAlias: &CustomNameAlias{Data: "custom"},
	}

	plain := in.IntoPlain(newTestCasterToPlain())
	require.NotNil(t, plain)
	require.NotNil(t, plain.Id)
	require.Equal(t, "extra-id", *plain.Id)
	require.Equal(t, "extra/id", plain.IdCRF)
	require.Equal(t, "api", plain.Source)
	require.Equal(t, "tag", plain.Tag)
	require.Equal(t, "complex", plain.Name)
	require.Equal(t, []string{"a", "b"}, plain.Labels)
	require.NotNil(t, plain.Note)
	require.Equal(t, "note", *plain.Note)
	require.Equal(t, map[string]int32{"a": 1, "b": 2}, plain.Counters)
	require.NotNil(t, plain.CreatedAt)
	require.True(t, in.CreatedAt.AsTime().Equal(plain.CreatedAt.AsTime()))
	require.NotNil(t, plain.Comment)
	require.Equal(t, "comment", plain.Comment.GetValue())
	require.NotNil(t, plain.ContactEmail)
	require.Equal(t, "a@example.com", *plain.ContactEmail)
	require.Equal(t, "alias", plain.AliasId)
	require.Equal(t, []string{"a1", "a2"}, plain.AliasList)
	require.Equal(t, uuid.MustParse("11111111-1111-1111-1111-111111111111"), plain.CustomId)
	require.Equal(t, int32(42), plain.Int32Alias)
	require.Equal(t, int64(100), plain.Int64Alias)
	require.Equal(t, []bool{true, false}, plain.BoolAliasList)
	require.Equal(t, []byte("bytes"), plain.BytesAlias)
	require.Equal(t, float32(3.14), plain.FloatAlias)
	require.Equal(t, float64(2.71), plain.DoubleAlias)
	require.Equal(t, "custom", plain.CustomNameAlias)

	out := plain.IntoPb(newTestCasterToPb())
	require.NotNil(t, out)
	require.True(t, proto.Equal(in, out))
}
