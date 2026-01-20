package override_type

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
	if plain == nil {
		t.Fatal("plain is nil")
	}
	if plain.RawId != id {
		t.Fatalf("raw_id mismatch: %v", plain.RawId)
	}
	pb2 := plain.IntoPb(uuidToString{}, timeToTs{})
	if pb2.GetRawId() != id.String() {
		t.Fatalf("pb raw_id roundtrip failed: %q", pb2.GetRawId())
	}
	if !pb2.GetCreatedAt().AsTime().Equal(ts.AsTime()) {
		t.Fatalf("pb created_at roundtrip failed: %v", pb2.GetCreatedAt())
	}

	plain2, err := pb.IntoPlainErr(uuidFromStringErr{}, tsToTimeErr{})
	if err != nil || plain2 == nil {
		t.Fatalf("IntoPlainErr failed: %v", err)
	}
	_, err = plain2.IntoPbErr(uuidToStringErr{}, timeToTsErr{})
	if err != nil {
		t.Fatalf("IntoPbErr failed: %v", err)
	}
}
