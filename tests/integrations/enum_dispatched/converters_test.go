package enum_dispatched

import "testing"

func TestPaymentRoundTrip(t *testing.T) {
	pb := &Payment{
		Method:       &Payment_Card{Card: &PaymentCard{Number: "4111"}},
		BackupMethod: &Payment_BackupCrypto{BackupCrypto: &PaymentCrypto{Address: "0xabc"}},
	}
	plain := pb.IntoPlain()
	if plain == nil {
		t.Fatal("plain is nil")
	}
	if plain.Card == nil || plain.Card.Number != "4111" {
		t.Fatalf("card not copied: %#v", plain.Card)
	}
	if plain.BackupMethodBackupCrypto == nil || plain.BackupMethodBackupCrypto.Address != "0xabc" {
		t.Fatalf("backup crypto not copied: %#v", plain.BackupMethodBackupCrypto)
	}
	pb2 := plain.IntoPb()
	if pb2.GetCard() == nil || pb2.GetCard().Number != "4111" {
		t.Fatalf("pb card roundtrip failed: %#v", pb2.GetCard())
	}
	if pb2.GetBackupCrypto() == nil || pb2.GetBackupCrypto().Address != "0xabc" {
		t.Fatalf("pb backup crypto roundtrip failed: %#v", pb2.GetBackupCrypto())
	}
}
