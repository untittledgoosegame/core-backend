package handlers

import (
	"net/http"

	"github.com/untittledgoosegame/core-backend/internal/api/middleware"
	"github.com/untittledgoosegame/core-backend/internal/auth"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req auth.Request
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.auth.Register(r.Context(), req)
	reply(w, r, res, err)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req auth.Request
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.auth.Login(r.Context(), req)
	reply(w, r, res, err)
}

func (h *Handler) VerifyTwoFactor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAToken string `json:"mfa_token"`
		Code     string `json:"code"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.auth.VerifyMFA(r.Context(), req.MFAToken, req.Code)
	reply(w, r, res, err)
}

func (h *Handler) SetupTwoFactor(w http.ResponseWriter, r *http.Request) {
	res, err := h.auth.SetupTOTP(r.Context(), middleware.UserID(r.Context()))
	reply(w, r, res, err)
}

func (h *Handler) EnableTwoFactor(w http.ResponseWriter, r *http.Request) {
	var req codeRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.auth.EnableTOTP(r.Context(), middleware.UserID(r.Context()), req.Code)
	replyUser(w, r, u, err)
}

func (h *Handler) DisableTwoFactor(w http.ResponseWriter, r *http.Request) {
	var req codeRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.auth.DisableTOTP(r.Context(), middleware.UserID(r.Context()), req.Code)
	replyUser(w, r, u, err)
}

func (h *Handler) LinkMethod(w http.ResponseWriter, r *http.Request) {
	var req auth.Request
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.auth.Link(r.Context(), middleware.UserID(r.Context()), req)
	reply(w, r, res, err)
}

func (h *Handler) UnlinkMethod(w http.ResponseWriter, r *http.Request) {
	var req auth.UnlinkRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	u, err := h.auth.Unlink(r.Context(), middleware.UserID(r.Context()), req)
	replyUser(w, r, u, err)
}

type codeRequest struct {
	Code string `json:"code"`
}
