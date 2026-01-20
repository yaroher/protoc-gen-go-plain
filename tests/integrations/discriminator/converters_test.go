package discriminator

import (
	"testing"

	"github.com/yaroher/protoc-gen-go-plain/oneoff"
)

func TestUserRoundTrip(t *testing.T) {
	pb := &User{
		ContactKind: ContactKind_CONTACT_KIND_EMAIL,
		IdKind:      IdKind_ID_KIND_USER,
		Contact:     &User_Email{Email: &Email{Value: "a@b.com"}},
		Identity:    &User_UserId{UserId: &UserId{Value: "u1"}},
	}
	plain := pb.IntoPlain()
	if plain == nil {
		t.Fatal("plain is nil")
	}
	if plain.ContactDisc == "" || plain.IdentityDisc == "" {
		t.Fatalf("discriminators not set: contact=%q identity=%q", plain.ContactDisc, plain.IdentityDisc)
	}
	if _, ok := plain.Contact.(*Email); !ok {
		t.Fatalf("contact payload type mismatch: %#v", plain.Contact)
	}
	if _, ok := plain.Identity.(*UserId); !ok {
		t.Fatalf("identity payload type mismatch: %#v", plain.Identity)
	}

	plain.ContactDisc = oneoff.NewDiscriminator(ContactKind_CONTACT_KIND_EMAIL)
	plain.IdentityDisc = oneoff.NewDiscriminator(IdKind_ID_KIND_USER)
	pb2 := plain.IntoPb()
	if pb2.GetEmail() == nil || pb2.GetEmail().Value != "a@b.com" {
		t.Fatalf("pb email roundtrip failed: %#v", pb2.GetEmail())
	}
	if pb2.GetUserId() == nil || pb2.GetUserId().Value != "u1" {
		t.Fatalf("pb user_id roundtrip failed: %#v", pb2.GetUserId())
	}
}
