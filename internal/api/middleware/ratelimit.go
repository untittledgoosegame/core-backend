package middleware

import (
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

// ponytail: in-memory counters per process; switch to httprate-redis when running several instances.
func RateLimitPerMinute(limit int) func(http.Handler) http.Handler {
	return httprate.LimitBy(limit, time.Minute,
		func(r *http.Request) (string, error) {
			return httprate.CanonicalizeIP(chimw.GetClientIP(r.Context())), nil
		},
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			httpx.Error(w, r, apperr.TooMany("too many requests, slow down"))
		}),
	)
}
