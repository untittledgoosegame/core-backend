package middleware

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ClientIP resolves the caller's IP. X-Forwarded-For is only trusted when the
// request came through one of the given proxy CIDRs; otherwise the TCP peer is used.
func ClientIP(trustedProxies []string) func(http.Handler) http.Handler {
	if len(trustedProxies) == 0 {
		return chimw.ClientIPFromRemoteAddr
	}
	return chimw.ClientIPFromXFF(trustedProxies...)
}
