package wkt_map

import (
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestUserRoundTrip(t *testing.T) {
	anyMsg, _ := anypb.New(&Address{Street: "Main"})
	pb := &User{
		FAny:      anyMsg,
		FDuration: durationpb.New(5),
		FEmpty:    &emptypb.Empty{},
		FStruct:   structpb.NewStringValue("v").GetStructValue(),
		FTs:       timestamppb.Now(),
		FBool:     wrapperspb.Bool(true),
		FString:   wrapperspb.String("x"),
		FMapInt32: map[string]int32{"k": 1},
		FMapMsg:   map[string]*Address{"a": {Street: "S"}},
	}
	plain := pb.IntoPlain()
	if plain == nil || plain.FAny == nil || plain.FMapMsg["a"].Street != "S" {
		t.Fatalf("plain conversion failed: %+v", plain)
	}
	pb2 := plain.IntoPb()
	if pb2.GetFBool().GetValue() != true {
		t.Fatalf("pb bool roundtrip failed: %#v", pb2.GetFBool())
	}
	if pb2.GetFMapMsg()["a"].GetStreet() != "S" {
		t.Fatalf("pb map msg roundtrip failed: %#v", pb2.GetFMapMsg())
	}
}
