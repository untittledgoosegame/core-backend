package handlers

import (
	"net/http"

	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

func reply(w http.ResponseWriter, r *http.Request, body any, err error) {
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, body)
}

func replyUser(w http.ResponseWriter, r *http.Request, u *user.User, err error) {
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, u.View())
}
