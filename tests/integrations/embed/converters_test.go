package embed

import "testing"

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
	if plain == nil {
		t.Fatal("plain is nil")
	}
	plain.ContactType = ContactType_CONTACT_TYPE_EMAIL
	if plain.Street != "Main" || plain.WorkAddressStreet != "Work" {
		t.Fatalf("embedded fields not copied: %+v", plain)
	}
	pb2 := plain.IntoPb()
	if pb2.GetAddress() == nil || pb2.GetAddress().Street != "Main" {
		t.Fatalf("pb address roundtrip failed: %#v", pb2.GetAddress())
	}
	if pb2.GetWorkAddress() == nil || pb2.GetWorkAddress().Street != "Work" {
		t.Fatalf("pb work address roundtrip failed: %#v", pb2.GetWorkAddress())
	}
	if pb2.GetEmail() != "a@b.com" {
		t.Fatalf("pb email roundtrip failed: %#v", pb2.GetEmail())
	}
	if pb2.GetBackupEmail() != "999" {
		t.Fatalf("pb backup email roundtrip failed: %#v", pb2.GetBackupEmail())
	}
}
