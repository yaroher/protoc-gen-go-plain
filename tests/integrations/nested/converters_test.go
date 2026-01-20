package embed

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestNestedUserWrapperRoundTrip(t *testing.T) {
	pb := &UserWrapper{
		User: &User{
			Name:    "Jane",
			Contact: &User_Email{Email: "jane@example.com"},
		},
	}
	plain := pb.IntoPlain()
	require.NotNil(t, plain)
	require.True(t, proto.Equal(pb, plain.IntoPb()))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserWrapperPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	pb2 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb2))
}
