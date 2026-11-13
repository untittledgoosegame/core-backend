package user

import (
	"reflect"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestPasskeyBSONRoundTrip(t *testing.T) {
	in := User{
		ID:             bson.NewObjectID(),
		WebAuthnUserID: []byte{1, 2, 3},
		Passkeys: []webauthn.Credential{{
			ID:              []byte("cred-id"),
			PublicKey:       []byte("public-key"),
			AttestationType: "none",
			Transport:       []protocol.AuthenticatorTransport{protocol.Internal, protocol.Hybrid},
			Flags:           webauthn.CredentialFlags{UserPresent: true, UserVerified: true, BackupEligible: true},
			Authenticator:   webauthn.Authenticator{AAGUID: []byte("aaguid-0123456789"), SignCount: 7},
		}},
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	in.UpdatedAt = in.CreatedAt

	raw, err := bson.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out User
	if err := bson.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Passkeys, out.Passkeys) {
		t.Fatalf("passkeys changed in round trip:\n in=%+v\nout=%+v", in.Passkeys, out.Passkeys)
	}
}
