package virtual

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestUserRoundTrip(t *testing.T) {
	plain := &UserPlain{
		Name:       "Jane",
		VirtAddr:   &Address{Street: "Main"},
		VirtStatus: Status_STATUS_ACTIVE,
	}
	pb := plain.IntoPb()
	pb2 := pb.IntoPlain().IntoPb()
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	pb3 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb3))
}
