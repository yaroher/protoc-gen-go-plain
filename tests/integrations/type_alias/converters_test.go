package type_alias

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestUserRoundTrip(t *testing.T) {
	pb := &User{Id: &UserId{Value: "u1"}}
	plain := pb.IntoPlain()
	require.NotNil(t, plain)
	pb2 := plain.IntoPb()
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	pb3 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb3))
}
