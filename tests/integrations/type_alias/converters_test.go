package type_alias

import "testing"

func TestUserRoundTrip(t *testing.T) {
	pb := &User{Id: &UserId{Value: "u1"}}
	plain := pb.IntoPlain()
	if plain == nil {
		t.Fatal("plain is nil")
	}
	if plain.Id != "u1" {
		t.Fatalf("plain id mismatch: %q", plain.Id)
	}
	pb2 := plain.IntoPb()
	if pb2.GetId() == nil || pb2.GetId().GetValue() != "u1" {
		t.Fatalf("pb id roundtrip failed: %#v", pb2.GetId())
	}
}
