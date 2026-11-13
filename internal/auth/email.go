package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/mailer"
)

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || addr.Name != "" {
		return "", apperr.BadRequest("a valid email is required")
	}
	return strings.ToLower(addr.Address), nil
}

func (s *Service) sendCode(ctx context.Context, key, email string, purpose mailer.Purpose) (*Result, error) {
	code, err := s.otp.issue(ctx, key)
	if err != nil {
		return nil, err
	}
	if err := s.mail.SendCode(email, code, purpose, codeTTL); err != nil {
		_ = s.otp.discard(ctx, key)
		var appErr *apperr.Error
		if errors.As(err, &appErr) {
			return nil, err
		}
		log.Error("sending email code failed", "err", err)
		return nil, apperr.Unavailable("could not send the email, try again")
	}
	return codeSent, nil
}

func (s *Service) emailTaken(ctx context.Context, email string) (bool, error) {
	_, err := s.users.ByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) registerEmail(ctx context.Context, req Request) (*Result, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	key := "register:" + email
	if req.Code == "" {
		taken, err := s.emailTaken(ctx, email)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, apperr.Conflict("account already exists, log in instead")
		}
		return s.sendCode(ctx, key, email, mailer.PurposeRegister)
	}

	if err := s.otp.verify(ctx, key, req.Code); err != nil {
		return nil, err
	}
	u := &user.User{Email: email, WebAuthnUserID: newWebAuthnID()}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.session(u)
}

func (s *Service) loginEmail(ctx context.Context, req Request) (*Result, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	u, err := s.users.ByEmail(ctx, email)
	if errors.Is(err, user.ErrNotFound) {
		return nil, apperr.NotFound("no account for this email, register instead")
	}
	if err != nil {
		return nil, err
	}
	key := "login:" + email
	if req.Code == "" {
		return s.sendCode(ctx, key, email, mailer.PurposeLogin)
	}
	if err := s.otp.verify(ctx, key, req.Code); err != nil {
		return nil, err
	}
	return s.complete(ctx, u)
}

func (s *Service) linkEmail(ctx context.Context, userID bson.ObjectID, req Request) (*Result, error) {
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return nil, err
	}
	key := "link:" + userID.Hex() + ":" + email
	if req.Code == "" {
		u, err := s.users.ByID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if u.Email != "" {
			return nil, apperr.Conflict("an email is already linked; unlink it first")
		}
		taken, err := s.emailTaken(ctx, email)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, apperr.Conflict("this email belongs to another account")
		}
		return s.sendCode(ctx, key, email, mailer.PurposeLink)
	}

	if err := s.otp.verify(ctx, key, req.Code); err != nil {
		return nil, err
	}
	u, err := s.users.LinkEmail(ctx, userID, email)
	if err != nil {
		return nil, err
	}
	return withUser(u), nil
}
