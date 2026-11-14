package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/provider"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/config"
	"github.com/untittledgoosegame/core-backend/pkg/webhook"
)

const (
	connectTokenTTL = 10 * time.Minute
	connectAudience = "oauth-service"
	connectIssuer   = "core-backend"
)

var errNotConfigured = apperr.Unavailable("connections are not configured")

// oauthService talks to the separate OAuth service that owns provider tokens.
type oauthService struct {
	baseURL string
	secret  string
	client  *http.Client
}

func newOAuthService(cfg config.OAuthServiceConfig) *oauthService {
	return &oauthService{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		secret:  cfg.Secret,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (o *oauthService) configured() bool { return o.baseURL != "" && o.secret != "" }

type ConnectClaims struct {
	Provider provider.Provider `json:"provider"`
	jwt.RegisteredClaims
}

// connectURL returns a link to the OAuth service's connect page. The signed token
// tells the OAuth service which user is connecting; it echoes that user_id back in webhooks.
func (o *oauthService) connectURL(userID bson.ObjectID, p provider.Provider, now time.Time) (string, time.Time, error) {
	if !o.configured() {
		return "", time.Time{}, errNotConfigured
	}
	expires := now.Add(connectTokenTTL)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, ConnectClaims{
		Provider: p,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    connectIssuer,
			Audience:  jwt.ClaimStrings{connectAudience},
			Subject:   userID.Hex(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
			ID:        bson.NewObjectID().Hex(),
		},
	}).SignedString([]byte(o.secret))
	if err != nil {
		return "", time.Time{}, err
	}
	link := fmt.Sprintf("%s/v1/connect/%s?token=%s", o.baseURL, url.PathEscape(string(p)), url.QueryEscape(token))
	return link, expires, nil
}

type revokeRequest struct {
	UserID      string            `json:"user_id"`
	Provider    provider.Provider `json:"provider"`
	RequestedAt time.Time         `json:"requested_at"`
}

// revoke asks the OAuth service to drop its tokens. 404 means it holds nothing, which is fine.
func (o *oauthService) revoke(ctx context.Context, userID bson.ObjectID, p provider.Provider) error {
	if !o.configured() {
		return errNotConfigured
	}
	now := time.Now().UTC()
	body, err := json.Marshal(revokeRequest{UserID: userID.Hex(), Provider: p, RequestedAt: now})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/v1/connections/revoke", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(webhook.SignatureHeader, webhook.Sign(o.secret, body, now))

	resp, err := o.client.Do(req)
	if err != nil {
		return apperr.New(http.StatusBadGateway, "could not reach the connections service, try again")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return apperr.New(http.StatusBadGateway, fmt.Sprintf("connections service refused to disconnect (status %d), try again", resp.StatusCode))
}
