package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/untittledgoosegame/core-backend/internal/api/middleware"
	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.users.ByID(r.Context(), middleware.UserID(r.Context()))
	replyUser(w, r, u, err)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var p user.Profile
	if err := httpx.Decode(w, r, &p); err != nil {
		httpx.Error(w, r, err)
		return
	}
	p, err := user.NormalizeProfile(p, time.Now().UTC())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.users.SetProfile(r.Context(), middleware.UserID(r.Context()), p)
	replyUser(w, r, u, err)
}

func (h *Handler) SuggestUsernames(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		httpx.Error(w, r, apperr.BadRequest("name query parameter is required"))
		return
	}
	suggestions, err := h.users.SuggestUsernames(r.Context(), name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string][]string{"suggestions": suggestions})
}

func (h *Handler) UsernameAvailability(w http.ResponseWriter, r *http.Request) {
	username := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("username")))
	available, reason, err := h.users.UsernameAvailable(r.Context(), username)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"username": username, "available": available, "reason": reason})
}
