package embed

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestUserRoundTrip(t *testing.T) {
	pb := &User{
		Name: "Jane",
		Address: &Address{
			Street: "Main",
			City:   "NY",
		},
		WorkAddress: &Address{
			Street: "Work",
			City:   "SF",
		},
		Contact:       &User_Email{Email: "a@b.com"},
		BackupContact: &User_BackupEmail{BackupEmail: "999"},
		ContactType:   ContactType_CONTACT_TYPE_EMAIL,
	}
	plain := pb.IntoPlain()
	require.NotNil(t, plain)
	plain.ContactType = ContactType_CONTACT_TYPE_EMAIL
	pb2 := plain.IntoPb()
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	plain2.ContactType = ContactType_CONTACT_TYPE_EMAIL
	pb3 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb3))
}
