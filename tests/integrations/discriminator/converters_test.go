package discriminator

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yaroher/protoc-gen-go-plain/oneoff"
	"google.golang.org/protobuf/proto"
)

func TestUserRoundTrip(t *testing.T) {
	pb := &User{
		ContactKind: ContactKind_CONTACT_KIND_EMAIL,
		IdKind:      IdKind_ID_KIND_USER,
		Contact:     &User_Email{Email: &Email{Value: "a@b.com"}},
		Identity:    &User_UserId{UserId: &UserId{Value: "u1"}},
	}
	plain := pb.IntoPlain()
	require.NotNil(t, plain)
	pb2 := plain.IntoPb()
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 UserPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	if v, ok := plain2.Contact.(map[string]any); ok {
		if value, ok := v["value"].(string); ok {
			plain2.Contact = &Email{Value: value}
		}
	}
	if v, ok := plain2.Identity.(map[string]any); ok {
		if value, ok := v["value"].(string); ok {
			plain2.Identity = &UserId{Value: value}
		}
	}
	plain2.ContactDisc = oneoff.NewDiscriminator(ContactKind_CONTACT_KIND_EMAIL)
	plain2.IdentityDisc = oneoff.NewDiscriminator(IdKind_ID_KIND_USER)
	pb3 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb3))
}
