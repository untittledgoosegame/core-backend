package auth

import (
	"context"

	"github.com/pquerna/otp/totp"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

const kindMFA = "mfa"

var errInvalidCode = apperr.Unauthorized("invalid code")

type TOTPSetup struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

func (s *Service) validTOTP(u *user.User, sealed, code string) (bool, error) {
	secret, err := s.secrets.Open(sealed, u.ID.Hex())
	if err != nil {
		return false, err
	}
	return totp.Validate(code, secret), nil
}

func (s *Service) VerifyMFA(ctx context.Context, mfaToken, code string) (*Result, error) {
	if mfaToken == "" || code == "" {
		return nil, apperr.BadRequest("mfa_token and code are required")
	}
	doc, err := s.challenges.attemptChallenge(ctx, mfaToken, kindMFA)
	if err != nil {
		return nil, err
	}
	u, err := s.users.ByID(ctx, doc.UserID)
	if err != nil {
		return nil, err
	}
	if !u.TOTP.Enabled {
		return nil, errInvalidCode
	}
	ok, err := s.validTOTP(u, u.TOTP.Secret, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errInvalidCode
	}
	if err := s.challenges.deleteChallenge(ctx, mfaToken); err != nil {
		return nil, err
	}
	return s.session(u)
}

func (s *Service) SetupTOTP(ctx context.Context, userID bson.ObjectID) (*TOTPSetup, error) {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.TOTP.Enabled {
		return nil, apperr.Conflict("two-factor authentication is already enabled")
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: s.appName, AccountName: u.WebAuthnName()})
	if err != nil {
		return nil, err
	}
	sealed, err := s.secrets.Seal(key.Secret(), u.ID.Hex())
	if err != nil {
		return nil, err
	}
	if _, err := s.users.SetTOTP(ctx, u.ID, user.TOTP{PendingSecret: sealed}); err != nil {
		return nil, err
	}
	return &TOTPSetup{Secret: key.Secret(), OTPAuthURL: key.URL()}, nil
}

func (s *Service) EnableTOTP(ctx context.Context, userID bson.ObjectID, code string) (*user.User, error) {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if u.TOTP.Enabled {
		return nil, apperr.Conflict("two-factor authentication is already enabled")
	}
	if u.TOTP.PendingSecret == "" {
		return nil, apperr.BadRequest("start setup first")
	}
	ok, err := s.validTOTP(u, u.TOTP.PendingSecret, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errInvalidCode
	}
	updated, err := s.users.SetTOTP(ctx, u.ID, user.TOTP{Enabled: true, Secret: u.TOTP.PendingSecret})
	if err != nil {
		return nil, err
	}
	s.alert(u.Email, "Two-factor authentication turned on", "An authenticator app is now required when you sign in with email or Truecaller.")
	return updated, nil
}

func (s *Service) DisableTOTP(ctx context.Context, userID bson.ObjectID, code string) (*user.User, error) {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !u.TOTP.Enabled {
		return nil, apperr.BadRequest("two-factor authentication is not enabled")
	}
	ok, err := s.validTOTP(u, u.TOTP.Secret, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errInvalidCode
	}
	updated, err := s.users.SetTOTP(ctx, u.ID, user.TOTP{})
	if err != nil {
		return nil, err
	}
	s.alert(u.Email, "Two-factor authentication turned off", "Your account no longer asks for an authenticator code when you sign in.")
	return updated, nil
}
