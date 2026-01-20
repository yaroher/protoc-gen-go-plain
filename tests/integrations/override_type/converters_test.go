package override_type

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type uuidFromString struct{}

type uuidToString struct{}

type uuidFromStringErr struct{}

type uuidToStringErr struct{}

func (uuidFromString) Cast(v string) uuid.UUID {
	id, _ := uuid.Parse(v)
	return id
}

func (uuidToString) Cast(v uuid.UUID) string {
	return v.String()
}

func (uuidFromStringErr) CastErr(v string) (uuid.UUID, error) {
	return uuid.Parse(v)
}

func (uuidToStringErr) CastErr(v uuid.UUID) (string, error) {
	return v.String(), nil
}

type tsToTime struct{}

type timeToTs struct{}

type tsToTimeErr struct{}

type timeToTsErr struct{}

func (tsToTime) Cast(v *timestamppb.Timestamp) time.Time {
	if v == nil {
		return time.Time{}
	}
	return v.AsTime()
}

func (timeToTs) Cast(v time.Time) *timestamppb.Timestamp {
	return timestamppb.New(v)
}

func (tsToTimeErr) CastErr(v *timestamppb.Timestamp) (time.Time, error) {
	if v == nil {
		return time.Time{}, nil
	}
	return v.AsTime(), nil
}

func (timeToTsErr) CastErr(v time.Time) (*timestamppb.Timestamp, error) {
	return timestamppb.New(v), nil
}

func TestUserRoundTrip(t *testing.T) {
	id := uuid.New()
	ts := timestamppb.New(time.Unix(1, 0))
	pb := &User{RawId: id.String(), CreatedAt: ts}
	plain := pb.IntoPlain(uuidFromString{}, tsToTime{})
	require.NotNil(t, plain)
	pb2 := plain.IntoPb(uuidToString{}, timeToTs{})
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	pb3 := plain2.IntoPb(uuidToString{}, timeToTs{})
	require.True(t, proto.Equal(pb, pb3))

	plainErr, err := pb.IntoPlainErr(uuidFromStringErr{}, tsToTimeErr{})
	if err != nil || plainErr == nil {
		require.NoError(t, err)
	}
	_, err = plainErr.IntoPbErr(uuidToStringErr{}, timeToTsErr{})
	require.NoError(t, err)
}
