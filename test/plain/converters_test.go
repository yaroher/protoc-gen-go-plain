package plain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestIntoPlainAndBack(t *testing.T) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}

	plain := in.IntoPlain()
	require.NotNil(t, plain, "IntoPlain returned nil")
	require.Equal(t, "u-1", plain.Id)
	require.Equal(t, "api", plain.Source)
	require.Equal(t, "Alice", plain.Name)
	require.NotNil(t, plain.ContactEmail)
	require.Equal(t, "alice@example.com", *plain.ContactEmail)
	require.Nil(t, plain.ContactPhone)

	out := plain.IntoPb()
	require.NotNil(t, out, "IntoPb returned nil")
	require.True(t, proto.Equal(in, out), "roundtrip mismatch\ninput:  %v\noutput: %v", in, out)
}

func BenchmarkConverters(b *testing.B) {
	in := &UserEvent{
		EventType: "created",
		User: &User{
			Base: &BaseInfo{
				Id:     "u-1",
				Source: "api",
			},
			Name: "Alice",
			Contact: &User_Email{
				Email: "alice@example.com",
			},
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plain := in.IntoPlain()
		out := plain.IntoPb()
		if out == nil {
			b.Fatal("IntoPb returned nil")
		}
	}
}
