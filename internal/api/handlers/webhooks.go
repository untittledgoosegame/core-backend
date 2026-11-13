package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/untittledgoosegame/core-backend/internal/connection"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
	"github.com/untittledgoosegame/core-backend/pkg/webhook"
)

const (
	maxWebhookBody     = 256 << 10
	signatureTolerance = 5 * time.Minute
)

// OAuthWebhook receives connection events from the OAuth service. Any 2xx means
// "don't retry", so duplicates, stale events and unknown users answer 200 with a status.
func (h *Handler) OAuthWebhook(w http.ResponseWriter, r *http.Request) {
	secret := h.connections.WebhookSecret()
	if secret == "" {
		httpx.Error(w, r, apperr.Unavailable("webhook is not configured"))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		httpx.Error(w, r, apperr.New(http.StatusRequestEntityTooLarge, "body too large"))
		return
	}
	if err := webhook.Verify(secret, r.Header.Get(webhook.SignatureHeader), body, time.Now(), signatureTolerance); err != nil {
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
	status, err := h.connections.HandleEvent(r.Context(), event)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"id": event.ID, "status": string(status)})
}
