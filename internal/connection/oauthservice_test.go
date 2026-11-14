package connection

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/provider"
	"github.com/untittledgoosegame/core-backend/pkg/config"
)

func TestConnectURL(t *testing.T) {
	o := newOAuthService(config.OAuthServiceConfig{BaseURL: "https://oauth.example.com/", Secret: "shared"})
	userID := bson.NewObjectID()
	now := time.Now()

	link, expires, err := o.connectURL(userID, provider.Spotify, now)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "https://oauth.example.com/v1/connect/spotify?token=") {
		t.Fatalf("unexpected link %s", link)
	}
	if !expires.Equal(now.Add(connectTokenTTL)) {
		t.Fatalf("expires = %v", expires)
	}

	u, _ := url.Parse(link)
	var claims ConnectClaims
	_, err = jwt.ParseWithClaims(u.Query().Get("token"), &claims, func(*jwt.Token) (any, error) { return []byte("shared"), nil },
		jwt.WithAudience(connectAudience), jwt.WithIssuer(connectIssuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		t.Fatalf("token does not verify with the shared secret: %v", err)
	}
	if claims.Subject != userID.Hex() || claims.Provider != provider.Spotify {
		t.Fatalf("claims = %+v", claims)
	}

	if _, _, err := newOAuthService(config.OAuthServiceConfig{}).connectURL(userID, provider.Spotify, now); err == nil {
		t.Fatal("unconfigured service produced a link")
	}
}
