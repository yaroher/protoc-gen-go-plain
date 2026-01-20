package enum_dispatched

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestPaymentRoundTrip(t *testing.T) {
	pb := &Payment{
		Method:       &Payment_Card{Card: &PaymentCard{Number: "4111"}},
		BackupMethod: &Payment_BackupCrypto{BackupCrypto: &PaymentCrypto{Address: "0xabc"}},
	}
	plain := pb.IntoPlain()
	require.NotNil(t, plain)
	pb2 := plain.IntoPb()
	require.True(t, proto.Equal(pb, pb2))

	data, err := plain.MarshalJSON()
	require.NoError(t, err)
	var plain2 PaymentPlain
	require.NoError(t, plain2.UnmarshalJSON(data))
	pb3 := plain2.IntoPb()
	require.True(t, proto.Equal(pb, pb3))
}
