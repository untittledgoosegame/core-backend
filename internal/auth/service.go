package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/config"
	"github.com/untittledgoosegame/core-backend/pkg/mailer"
	"github.com/untittledgoosegame/core-backend/pkg/secret"
)

const (
	MethodEmail      = "email"
	MethodPasskey    = "passkey"
	MethodTruecaller = "truecaller"
)

var errUnknownMethod = apperr.BadRequest("method must be one of: email, passkey, truecaller")

type Request struct {
	Method            string          `json:"method"`
	Email             string          `json:"email,omitempty"`
	Code              string          `json:"code,omitempty"`
	ChallengeID       string          `json:"challenge_id,omitempty"`
	Credential        json.RawMessage `json:"credential,omitempty"`
	AuthorizationCode string          `json:"authorization_code,omitempty"`
	CodeVerifier      string          `json:"code_verifier,omitempty"`
}

type Result struct {
	Status      string     `json:"status,omitempty"`
	ChallengeID string     `json:"challenge_id,omitempty"`
	Options     any        `json:"options,omitempty"`
	MFARequired bool       `json:"mfa_required,omitempty"`
	MFAToken    string     `json:"mfa_token,omitempty"`
	Token       string     `json:"token,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	User        *user.View `json:"user,omitempty"`
}

var codeSent = &Result{Status: "code_sent"}

type Service struct {
	users      *user.Store
	challenges *challengeStore
	otp        *otpStore
	webauthn   *webauthn.WebAuthn
	tokens     Tokens
	mail       *mailer.Mailer
	truecaller truecaller
	secrets    *secret.Box
	appName    string
}

func NewService(ctx context.Context, db *mongo.Database, rdb *redis.Client, users *user.Store, mail *mailer.Mailer, cfg config.Config) (*Service, error) {
	challenges, err := newChallengeStore(ctx, db)
	if err != nil {
		return nil, err
	}
	wa, err := newWebAuthn(cfg)
	if err != nil {
		return nil, err
	}
	box, err := newSecretBox(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{
		users:      users,
		challenges: challenges,
		otp:        newOTPStore(rdb, cfg.JWT.Secret),
		webauthn:   wa,
		tokens:     NewTokens(cfg.JWT.Secret, cfg.JWT.TTL),
		mail:       mail,
		truecaller: newTruecaller(cfg.Truecaller),
		secrets:    box,
		appName:    cfg.App.Name,
	}, nil
}

func newSecretBox(cfg config.Config) (*secret.Box, error) {
	if cfg.Security.EncryptionKey != "" {
		return secret.NewFromBase64(cfg.Security.EncryptionKey)
	}
	key := sha256.Sum256([]byte("dev-encryption-key:" + cfg.JWT.Secret))
	return secret.New(key[:])
}

func (s *Service) Tokens() Tokens { return s.tokens }

func (s *Service) Register(ctx context.Context, req Request) (*Result, error) {
	switch req.Method {
	case MethodEmail:
		return s.registerEmail(ctx, req)
	case MethodPasskey:
		return s.registerPasskey(ctx, req)
	case MethodTruecaller:
		return s.registerTruecaller(ctx, req)
	}
	return nil, errUnknownMethod
}

func (s *Service) Login(ctx context.Context, req Request) (*Result, error) {
	switch req.Method {
	case MethodEmail:
		return s.loginEmail(ctx, req)
	case MethodPasskey:
		return s.loginPasskey(ctx, req)
	case MethodTruecaller:
		return s.loginTruecaller(ctx, req)
	}
	return nil, errUnknownMethod
}

func (s *Service) complete(ctx context.Context, u *user.User) (*Result, error) {
	if !u.TOTP.Enabled {
		return s.session(u)
	}
	id, err := s.challenges.createChallenge(ctx, challengeDoc{Kind: kindMFA, UserID: u.ID})
	if err != nil {
		return nil, err
	}
	return &Result{MFARequired: true, MFAToken: id}, nil
}

func (s *Service) session(u *user.User) (*Result, error) {
	token, expires, err := s.tokens.Issue(u.ID)
	if err != nil {
		return nil, err
	}
	view := u.View()
	return &Result{Token: token, ExpiresAt: &expires, User: &view}, nil
}

func withUser(u *user.User) *Result {
	view := u.View()
	return &Result{User: &view}
}

func (s *Service) alert(email, title, message string) {
	if email == "" {
		return
	}
	go func() {
		if err := s.mail.SendSecurityAlert(email, title, message); err != nil {
			log.Warn("security alert not sent", "title", title, "err", err)
		}
	}()
}
