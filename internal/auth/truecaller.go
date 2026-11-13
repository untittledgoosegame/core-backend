package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/user"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/config"
)

type truecallerProfile struct {
	Sub         string `json:"sub"`
	GivenName   string `json:"given_name"`
	FamilyName  string `json:"family_name"`
	PhoneNumber string `json:"phone_number"`
	Picture     string `json:"picture"`
}

func (p truecallerProfile) name() string {
	return strings.TrimSpace(p.GivenName + " " + p.FamilyName)
}

type truecaller struct {
	cfg    config.TruecallerConfig
	client *http.Client
}

func newTruecaller(cfg config.TruecallerConfig) truecaller {
	return truecaller{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}}
}

func (t truecaller) profile(ctx context.Context, code, verifier string) (*truecallerProfile, error) {
	if t.cfg.ClientID == "" {
		return nil, apperr.Unavailable("truecaller sign-in is not configured")
	}
	if code == "" || verifier == "" {
		return nil, apperr.BadRequest("authorization_code and code_verifier are required")
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {t.cfg.ClientID},
		"code":          {code},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.BaseURL+"/v1/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := t.do(req, &token); err != nil {
		return nil, err
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, t.cfg.BaseURL+"/v1/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	var profile truecallerProfile
	if err := t.do(req, &profile); err != nil {
		return nil, err
	}
	if profile.Sub == "" {
		return nil, apperr.Unauthorized("truecaller did not return a user")
	}
	return &profile, nil
}

func (t truecaller) do(req *http.Request, out any) error {
	resp, err := t.client.Do(req)
	if err != nil {
		return apperr.Unavailable("truecaller is unreachable")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return apperr.Unauthorized("truecaller rejected the authorization code")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("truecaller %s: status %d", req.URL.Path, resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

func (s *Service) registerTruecaller(ctx context.Context, req Request) (*Result, error) {
	p, err := s.truecaller.profile(ctx, req.AuthorizationCode, req.CodeVerifier)
	if err != nil {
		return nil, err
	}
	u := &user.User{
		TruecallerID:   p.Sub,
		Phone:          p.PhoneNumber,
		WebAuthnUserID: newWebAuthnID(),
		Profile:        user.Profile{Name: p.name()},
	}
	if pic, err := url.Parse(p.Picture); err == nil && pic.Scheme == "https" {
		u.Profile.AvatarURL = p.Picture
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return s.session(u)
}

func (s *Service) loginTruecaller(ctx context.Context, req Request) (*Result, error) {
	p, err := s.truecaller.profile(ctx, req.AuthorizationCode, req.CodeVerifier)
	if err != nil {
		return nil, err
	}
	u, err := s.users.ByTruecallerID(ctx, p.Sub)
	if errors.Is(err, user.ErrNotFound) {
		return nil, apperr.NotFound("no account for this truecaller profile, register instead")
	}
	if err != nil {
		return nil, err
	}
	return s.complete(ctx, u)
}

func (s *Service) linkTruecaller(ctx context.Context, userID bson.ObjectID, req Request) (*Result, error) {
	p, err := s.truecaller.profile(ctx, req.AuthorizationCode, req.CodeVerifier)
	if err != nil {
		return nil, err
	}
	u, err := s.users.LinkTruecaller(ctx, userID, p.Sub, p.PhoneNumber)
	if err != nil {
		return nil, err
	}
	return withUser(u), nil
}
