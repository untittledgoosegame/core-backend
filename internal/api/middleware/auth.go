package middleware

import (
	"context"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/auth"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

type userIDKey struct{}

func RequireAuth(tokens auth.Tokens) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				httpx.Error(w, r, apperr.Unauthorized("missing bearer token"))
				return
			}
			id, err := tokens.Parse(raw)
			if err != nil {
				httpx.Error(w, r, apperr.Unauthorized("invalid or expired token"))
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
		})
	}
}

func UserID(ctx context.Context) bson.ObjectID {
	id, _ := ctx.Value(userIDKey{}).(bson.ObjectID)
	return id
}
