package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/untittledgoosegame/core-backend/internal/api/middleware"
	"github.com/untittledgoosegame/core-backend/internal/connection"
	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
	"github.com/untittledgoosegame/core-backend/pkg/webhook"
)

const (
	maxWebhookBody     = 256 << 10
	signatureTolerance = 5 * time.Minute
	eventDedupTTL      = 7 * 24 * time.Hour
)

func (h *Handler) MyConnections(w http.ResponseWriter, r *http.Request) {
	views, err := h.connections.ListForUser(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"connections": views})
}

// OAuthWebhook receives connection events from the OAuth service. Any 2xx means
// "don't retry", so duplicates, stale events and unknown users answer 200 with a status.
func (h *Handler) OAuthWebhook(w http.ResponseWriter, r *http.Request) {
	if h.webhooks.OAuthSecret == "" {
		httpx.Error(w, r, apperr.Unavailable("webhook is not configured"))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		httpx.Error(w, r, apperr.New(http.StatusRequestEntityTooLarge, "body too large"))
		return
	}
	if err := webhook.Verify(h.webhooks.OAuthSecret, r.Header.Get(webhook.SignatureHeader), body, time.Now(), signatureTolerance); err != nil {
		httpx.Error(w, r, apperr.Unauthorized(err.Error()))
		return
	}

	var event connection.Event
	if err := json.Unmarshal(body, &event); err != nil {
		httpx.Error(w, r, apperr.BadRequest("invalid JSON body: "+err.Error()))
		return
	}
	if err := event.Validate(time.Now()); err != nil {
		httpx.Error(w, r, err)
		return
	}
	userID, err := bson.ObjectIDFromHex(event.UserID)
	if err != nil {
		httpx.Error(w, r, apperr.BadRequest("user_id is not a valid id"))
		return
	}
	if _, err := h.users.ByID(r.Context(), userID); errors.Is(err, user.ErrNotFound) {
		log.Warn("webhook for unknown user", "event_id", event.ID, "user_id", event.UserID)
		h.webhookResult(w, event, "ignored_unknown_user")
		return
	} else if err != nil {
		httpx.Error(w, r, err)
		return
	}

	dedupKey := "webhook:oauth:" + event.ID
	fresh, err := h.redis.SetNX(r.Context(), dedupKey, 1, eventDedupTTL).Result()
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if !fresh {
		h.webhookResult(w, event, "duplicate")
		return
	}

	applied, err := h.connections.Apply(r.Context(), userID, event)
	if err != nil {
		h.redis.Del(r.Context(), dedupKey)
		httpx.Error(w, r, err)
		return
	}
	if !applied {
		h.webhookResult(w, event, "stale")
		return
	}
	h.webhookResult(w, event, "applied")
}

func (h *Handler) webhookResult(w http.ResponseWriter, event connection.Event, status string) {
	httpx.JSON(w, http.StatusOK, map[string]string{"id": event.ID, "status": status})
}
