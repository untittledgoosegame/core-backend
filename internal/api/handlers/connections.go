package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/untittledgoosegame/core-backend/internal/api/middleware"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

func (h *Handler) ListConnections(w http.ResponseWriter, r *http.Request) {
	views, err := h.connections.List(r.Context(), middleware.UserID(r.Context()))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"connections": views})
}

func (h *Handler) GetConnection(w http.ResponseWriter, r *http.Request) {
	view, err := h.connections.Get(r.Context(), middleware.UserID(r.Context()), chi.URLParam(r, "provider"))
	reply(w, r, view, err)
}

func (h *Handler) ConnectProvider(w http.ResponseWriter, r *http.Request) {
	link, err := h.connections.Connect(r.Context(), middleware.UserID(r.Context()), chi.URLParam(r, "provider"))
	reply(w, r, link, err)
}

func (h *Handler) DisconnectProvider(w http.ResponseWriter, r *http.Request) {
	view, err := h.connections.Disconnect(r.Context(), middleware.UserID(r.Context()), chi.URLParam(r, "provider"))
	reply(w, r, view, err)
}
