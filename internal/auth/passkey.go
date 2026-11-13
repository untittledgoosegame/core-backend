package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/config"
)

const (
	kindPasskeyRegister = "passkey_register"
	kindPasskeyLogin    = "passkey_login"
	kindPasskeyLink     = "passkey_link"
)

func newWebAuthn(cfg config.Config) (*webauthn.WebAuthn, error) {
	androidOrigins, err := AndroidOrigins(cfg.Mobile.AndroidCertFingerprints)
	if err != nil {
		return nil, err
	}
	return webauthn.New(&webauthn.Config{
		RPID:            cfg.WebAuthn.RPID,
		RPDisplayName:   cfg.App.Name,
		RPOrigins:       cfg.WebAuthn.RPOrigins,
		RPOpaqueOrigins: androidOrigins,
	})
}

// AndroidOrigins turns SHA-256 signing cert fingerprints (AB:CD:...) into the
// android:apk-key-hash origins Android apps present during passkey ceremonies.
func AndroidOrigins(fingerprints []string) ([]string, error) {
	origins := make([]string, 0, len(fingerprints))
	for _, fp := range fingerprints {
		raw, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(fp), ":", ""))
		if err != nil || len(raw) != sha256.Size {
			return nil, fmt.Errorf("invalid android cert fingerprint %q: want 32 hex bytes", fp)
		}
		origins = append(origins, "android:apk-key-hash:"+base64.RawURLEncoding.EncodeToString(raw))
	}
	return origins, nil
}

type ceremonyUser struct {
	id   []byte
	name string
}

func (c ceremonyUser) WebAuthnID() []byte                         { return c.id }
func (c ceremonyUser) WebAuthnName() string                       { return c.name }
func (c ceremonyUser) WebAuthnDisplayName() string                { return c.name }
func (c ceremonyUser) WebAuthnCredentials() []webauthn.Credential { return nil }

func newWebAuthnID() []byte {
	id := make([]byte, 64)
	_, _ = rand.Read(id)
	return id
}

var requireResidentKey = webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired)

func (s *Service) registerPasskey(ctx context.Context, req Request) (*Result, error) {
	if len(req.Credential) == 0 {
		pending := ceremonyUser{id: newWebAuthnID(), name: s.appName + " account"}
		creation, session, err := s.webauthn.BeginRegistration(pending, requireResidentKey)
		if err != nil {
			return nil, err
		}
		return s.beginCeremony(ctx, kindPasskeyRegister, bson.ObjectID{}, session, creation)
	}

	session, _, err := s.finishCeremony(ctx, req.ChallengeID, kindPasskeyRegister)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(req.Credential))
	if err != nil {
		return nil, apperr.BadRequest("invalid passkey credential")
	}
	cred, err := s.webauthn.CreateCredential(ceremonyUser{id: session.UserID}, *session, parsed)
	if err != nil {
		return nil, apperr.Unauthorized("passkey verification failed")
	}
	u := &user.User{WebAuthnUserID: session.UserID, Passkeys: []webauthn.Credential{*cred}}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.session(u)
}

func (s *Service) loginPasskey(ctx context.Context, req Request) (*Result, error) {
	if len(req.Credential) == 0 {
		assertion, session, err := s.webauthn.BeginDiscoverableLogin()
		if err != nil {
			return nil, err
		}
		return s.beginCeremony(ctx, kindPasskeyLogin, bson.ObjectID{}, session, assertion)
	}

	session, _, err := s.finishCeremony(ctx, req.ChallengeID, kindPasskeyLogin)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(req.Credential))
	if err != nil {
		return nil, apperr.BadRequest("invalid passkey credential")
	}
	var owner *user.User
	_, cred, err := s.webauthn.ValidatePasskeyLogin(func(_, userHandle []byte) (webauthn.User, error) {
		found, lookupErr := s.users.ByWebAuthnID(ctx, userHandle)
		if lookupErr != nil {
			return nil, lookupErr
		}
		owner = found
		return found, nil
	}, *session, parsed)
	if err != nil {
		return nil, apperr.Unauthorized("passkey verification failed")
	}
	if err := s.users.UpdatePasskey(ctx, owner.ID, *cred); err != nil {
		return nil, err
	}
	// Passkeys already combine possession and biometrics, so they skip the TOTP step.
	return s.session(owner)
}

func (s *Service) linkPasskey(ctx context.Context, userID bson.ObjectID, req Request) (*Result, error) {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(req.Credential) == 0 {
		exclude := make([]protocol.CredentialDescriptor, 0, len(u.Passkeys))
		for i := range u.Passkeys {
			exclude = append(exclude, u.Passkeys[i].Descriptor())
		}
		creation, session, err := s.webauthn.BeginRegistration(u, requireResidentKey, webauthn.WithExclusions(exclude))
		if err != nil {
			return nil, err
		}
		return s.beginCeremony(ctx, kindPasskeyLink, u.ID, session, creation)
	}

	session, owner, err := s.finishCeremony(ctx, req.ChallengeID, kindPasskeyLink)
	if err != nil {
		return nil, err
	}
	if owner != u.ID {
		return nil, apperr.Unauthorized("challenge belongs to another account")
	}
	parsed, err := protocol.ParseCredentialCreationResponseBody(bytes.NewReader(req.Credential))
	if err != nil {
		return nil, apperr.BadRequest("invalid passkey credential")
	}
	cred, err := s.webauthn.CreateCredential(u, *session, parsed)
	if err != nil {
		return nil, apperr.Unauthorized("passkey verification failed")
	}
	updated, err := s.users.AddPasskey(ctx, u.ID, *cred)
	if err != nil {
		return nil, err
	}
	return withUser(updated), nil
}

func (s *Service) beginCeremony(ctx context.Context, kind string, userID bson.ObjectID, session *webauthn.SessionData, options any) (*Result, error) {
	raw, err := json.Marshal(session)
	if err != nil {
		return nil, err
	}
	id, err := s.challenges.createChallenge(ctx, challengeDoc{Kind: kind, UserID: userID, Session: string(raw)})
	if err != nil {
		return nil, err
	}
	return &Result{ChallengeID: id, Options: options}, nil
}

func (s *Service) finishCeremony(ctx context.Context, challengeID, kind string) (*webauthn.SessionData, bson.ObjectID, error) {
	if challengeID == "" {
		return nil, bson.ObjectID{}, apperr.BadRequest("challenge_id is required with a credential")
	}
	doc, err := s.challenges.takeChallenge(ctx, challengeID, kind)
	if err != nil {
		return nil, bson.ObjectID{}, err
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(doc.Session), &session); err != nil {
		return nil, bson.ObjectID{}, err
	}
	return &session, doc.UserID, nil
}
