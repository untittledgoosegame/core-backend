package handlers

import (
	"errors"
	"io"
	"net/http"

	"github.com/untittledgoosegame/core-backend/internal/api/middleware"
	"github.com/untittledgoosegame/core-backend/internal/user"
	"github.com/untittledgoosegame/core-backend/pkg/apperr"
	"github.com/untittledgoosegame/core-backend/pkg/httpx"
)

const multipartOverhead = 64 << 10

// UploadAvatar expects multipart/form-data with the image in an "avatar" field.
func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, user.MaxAvatarBytes+multipartOverhead)
	if err := r.ParseMultipartForm(user.MaxAvatarBytes); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			httpx.Error(w, r, user.ErrAvatarTooLarge)
			return
		}
		httpx.Error(w, r, apperr.BadRequest("expected multipart/form-data with an \"avatar\" file"))
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile("avatar")
	if err != nil {
		httpx.Error(w, r, apperr.BadRequest("missing \"avatar\" file"))
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, user.MaxAvatarBytes+1))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if len(raw) > user.MaxAvatarBytes {
		httpx.Error(w, r, user.ErrAvatarTooLarge)
		return
	}

	u, err := h.avatars.Upload(r.Context(), middleware.UserID(r.Context()), raw)
	replyUser(w, r, u, err)
}

func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	u, err := h.avatars.Remove(r.Context(), middleware.UserID(r.Context()))
	replyUser(w, r, u, err)
}
