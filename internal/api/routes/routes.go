package routes

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/untittledgoosegame/core-backend/internal/api/handlers"
	"github.com/untittledgoosegame/core-backend/internal/api/middleware"
	"github.com/untittledgoosegame/core-backend/internal/auth"
	"github.com/untittledgoosegame/core-backend/pkg/config"
)

func New(h *handlers.Handler, tokens auth.Tokens, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(
		chimw.RequestID,
		middleware.ClientIP(cfg.Server.TrustedProxies),
		middleware.RequestLogger,
		chimw.Recoverer,
	)

	// Server-to-server and signed, so it is exempt from per-IP limits.
	r.Post("/v1/webhooks/oauth", h.OAuthWebhook)

	globalLimit := middleware.RateLimitPerMinute(cfg.RateLimit.GlobalPerMinute)
	authLimit := middleware.RateLimitPerMinute(cfg.RateLimit.AuthPerMinute)
	requireAuth := middleware.RequireAuth(tokens)

	r.Group(func(r chi.Router) {
		r.Use(globalLimit)

		r.Get("/health", h.Health)
		r.Get("/.well-known/assetlinks.json", h.AndroidAssetLinks)
		r.Get("/.well-known/apple-app-site-association", h.AppleAppSiteAssociation)

		r.Route("/v1/auth", func(r chi.Router) {
			r.With(authLimit).Post("/register", h.Register)
			r.With(authLimit).Post("/login", h.Login)
			r.With(authLimit).Post("/2fa/verify", h.VerifyTwoFactor)

			r.Group(func(r chi.Router) {
				r.Use(requireAuth)
				r.With(authLimit).Post("/link", h.LinkMethod)
				r.Delete("/link", h.UnlinkMethod)
				r.Post("/2fa/setup", h.SetupTwoFactor)
				r.With(authLimit).Post("/2fa/enable", h.EnableTwoFactor)
				r.With(authLimit).Post("/2fa/disable", h.DisableTwoFactor)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(requireAuth)
			r.Get("/v1/users/me", h.Me)
			r.Put("/v1/users/me/profile", h.UpdateProfile)
			r.Route("/v1/users/me/connections", func(r chi.Router) {
				r.Get("/", h.ListConnections)
				r.Get("/{provider}", h.GetConnection)
				r.Post("/{provider}", h.ConnectProvider)
				r.Delete("/{provider}", h.DisconnectProvider)
			})
			r.Get("/v1/usernames/suggestions", h.SuggestUsernames)
			r.Get("/v1/usernames/availability", h.UsernameAvailability)
		})
	})
	return r
}
