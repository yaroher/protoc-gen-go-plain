package virtual

import "testing"

func TestUserRoundTrip(t *testing.T) {
	plain := &UserPlain{
		Name:       "Jane",
		VirtAddr:   &Address{Street: "Main"},
		VirtStatus: Status_STATUS_ACTIVE,
	}
	pb := plain.IntoPb()
	if pb.GetName() != "Jane" {
		t.Fatalf("pb name mismatch: %q", pb.GetName())
	}
	plain2 := pb.IntoPlain()
	if plain2.Name != "Jane" {
		t.Fatalf("plain name mismatch: %q", plain2.Name)
	}
}
