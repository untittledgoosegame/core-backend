package handlers

import (
	"net/http"

	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Client().Ping(r.Context(), nil); err != nil {
		httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "mongo unavailable"})
		return
	}
	if err := h.redis.Ping(r.Context()).Err(); err != nil {
		httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "redis unavailable"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
