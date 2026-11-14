package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/charmbracelet/log"

	"github.com/untittledgoosegame/core-backend/pkg/apperr"
)

const maxBodyBytes = 64 << 10

func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.BadRequest("invalid JSON body: " + err.Error())
	}
	return nil
}

func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		JSON(w, appErr.Status, map[string]string{"error": appErr.Message})
		return
	}
	log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}
