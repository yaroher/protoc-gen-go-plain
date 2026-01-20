package override_type

import (
	"testing"
	"time"

	"github.com/go-faster/jx"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type uuidCodec struct{}

type uuidCodecErr struct{}

type timeCodec struct{}

type timeCodecErr struct{}
type uuidToStringBench struct{}
type uuidToStringErrBench struct{}
type timeToTsBench struct{}
type timeToTsErrBench struct{}

func (uuidCodec) Cast(v string) uuid.UUID {
	id, _ := uuid.Parse(v)
	return id
}

func (uuidCodec) EncodeJx(e *jx.Encoder, v uuid.UUID) {
	e.Str(v.String())
}

func (uuidCodec) DecodeJx(d *jx.Decoder) (uuid.UUID, error) {
	s, err := d.Str()
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(s)
}

func (uuidCodecErr) CastErr(v string) (uuid.UUID, error) {
	return uuid.Parse(v)
}

func (uuidCodecErr) EncodeJx(e *jx.Encoder, v uuid.UUID) error {
	e.Str(v.String())
	return nil
}

func (uuidCodecErr) DecodeJx(d *jx.Decoder) (uuid.UUID, error) {
	s, err := d.Str()
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(s)
}

func (timeCodec) Cast(v *timestamppb.Timestamp) time.Time {
	if v == nil {
		return time.Time{}
	}
	return v.AsTime()
}

func (timeCodec) EncodeJx(e *jx.Encoder, v time.Time) {
	b, _ := protojson.Marshal(timestamppb.New(v))
	e.Raw(b)
}

func (timeCodec) DecodeJx(d *jx.Decoder) (time.Time, error) {
	raw, err := d.Raw()
	if err != nil {
		return time.Time{}, err
	}
	if string(raw) == "null" {
		return time.Time{}, nil
	}
	var ts timestamppb.Timestamp
	if err := protojson.Unmarshal(raw, &ts); err != nil {
		return time.Time{}, err
	}
	return ts.AsTime(), nil
}

func (timeCodecErr) CastErr(v *timestamppb.Timestamp) (time.Time, error) {
	if v == nil {
		return time.Time{}, nil
	}
	return v.AsTime(), nil
}

func (timeCodecErr) EncodeJx(e *jx.Encoder, v time.Time) error {
	b, err := protojson.Marshal(timestamppb.New(v))
	if err != nil {
		return err
	}
	e.Raw(b)
	return nil
}

func (timeCodecErr) DecodeJx(d *jx.Decoder) (time.Time, error) {
	raw, err := d.Raw()
	if err != nil {
		return time.Time{}, err
	}
	if string(raw) == "null" {
		return time.Time{}, nil
	}
	var ts timestamppb.Timestamp
	if err := protojson.Unmarshal(raw, &ts); err != nil {
		return time.Time{}, err
	}
	return ts.AsTime(), nil
}

func (uuidToStringBench) Cast(v uuid.UUID) string {
	return v.String()
}

func (uuidToStringErrBench) CastErr(v uuid.UUID) (string, error) {
	return v.String(), nil
}

func (timeToTsBench) Cast(v time.Time) *timestamppb.Timestamp {
	return timestamppb.New(v)
}

func (timeToTsErrBench) CastErr(v time.Time) (*timestamppb.Timestamp, error) {
	return timestamppb.New(v), nil
}

func sampleUser() *User {
	id := uuid.New()
	return &User{
		RawId:     id.String(),
		CreatedAt: timestamppb.New(time.Unix(1, 0)),
	}
}

func sampleUserPlain() *UserPlain {
	id := uuid.New()
	return &UserPlain{
		RawId:     id,
		CreatedAt: time.Unix(1, 0),
	}
}

func BenchmarkProtojsonMarshal(b *testing.B) {
	msg := sampleUser()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = protojson.Marshal(msg)
	}
}

func BenchmarkProtojsonUnmarshal(b *testing.B) {
	msg := sampleUser()
	data, _ := protojson.Marshal(msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst User
		_ = protojson.Unmarshal(data, &dst)
	}
}

func BenchmarkJXMarshal(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.MarshalJSON()
	}
}

func BenchmarkJXUnmarshal(b *testing.B) {
	msg := sampleUserPlain()
	data, _ := msg.MarshalJSON()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst UserPlain
		_ = dst.UnmarshalJSON(data)
	}
}

func BenchmarkJXWithMarshal(b *testing.B) {
	msg := sampleUserPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.MarshalJSONWith(uuidCodec{}, timeCodec{})
	}
}

func BenchmarkJXWithUnmarshal(b *testing.B) {
	msg := sampleUserPlain()
	data, _ := msg.MarshalJSONWith(uuidCodec{}, timeCodec{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst UserPlain
		_ = dst.UnmarshalJSONWith(data, uuidCodecErr{}, timeCodecErr{})
	}
}
