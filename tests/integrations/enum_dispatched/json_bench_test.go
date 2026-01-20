package enum_dispatched

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
)

func samplePayment() *Payment {
	return &Payment{
		Method:       &Payment_Card{Card: &PaymentCard{Number: "4111"}},
		BackupMethod: &Payment_BackupCrypto{BackupCrypto: &PaymentCrypto{Address: "0xabc"}},
	}
}

func samplePaymentPlain() *PaymentPlain {
	return samplePayment().IntoPlain()
}

func marshalJxWithPayment(m *PaymentPlain) ([]byte, error) {
	return m.MarshalJSON()
}

func unmarshalJxWithPayment(data []byte, dst *PaymentPlain) error {
	return dst.UnmarshalJSON(data)
}

func BenchmarkProtojsonMarshal(b *testing.B) {
	msg := samplePayment()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = protojson.Marshal(msg)
	}
}

func BenchmarkProtojsonUnmarshal(b *testing.B) {
	msg := samplePayment()
	data, _ := protojson.Marshal(msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst Payment
		_ = protojson.Unmarshal(data, &dst)
	}
}

func BenchmarkJXMarshal(b *testing.B) {
	msg := samplePaymentPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = msg.MarshalJSON()
	}
}

func BenchmarkJXUnmarshal(b *testing.B) {
	msg := samplePaymentPlain()
	data, _ := msg.MarshalJSON()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst PaymentPlain
		_ = dst.UnmarshalJSON(data)
	}
}

func BenchmarkJXWithMarshal(b *testing.B) {
	msg := samplePaymentPlain()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = marshalJxWithPayment(msg)
	}
}

func BenchmarkJXWithUnmarshal(b *testing.B) {
	msg := samplePaymentPlain()
	data, _ := marshalJxWithPayment(msg)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var dst PaymentPlain
		_ = unmarshalJxWithPayment(data, &dst)
	}
}
