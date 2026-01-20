package serialize

import "testing"

func TestUserRoundTrip(t *testing.T) {
	pb := &User{Settings: &Settings{Locale: "ru"}}
	plain := pb.IntoPlain()
	if plain == nil {
		t.Fatal("plain is nil")
	}
	if len(plain.Settings) == 0 {
		t.Fatal("settings not serialized")
	}
	pb2 := plain.IntoPb()
	if pb2.GetSettings() == nil || pb2.GetSettings().GetLocale() != "ru" {
		t.Fatalf("pb settings roundtrip failed: %#v", pb2.GetSettings())
	}
}
