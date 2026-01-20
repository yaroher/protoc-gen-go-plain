package wkt_map

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
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
		require.NotNil(t, plain)
	}
	pb2 := plain.IntoPb()
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	pb3 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb3))
}
