package middleware

import (
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		status := ww.Status()
		if status == 0 {
			status = http.StatusOK
		}
		log.Info("request completed",
			"request_id", chimw.GetReqID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"latency", time.Since(start),
			"ip", chimw.GetClientIP(r.Context()),
		)
	})
}
