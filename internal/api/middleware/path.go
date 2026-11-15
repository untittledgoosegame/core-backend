package middleware

import "net/http"

// DecodedPath makes routing use the decoded path, so clients that encode "@" as "%40"
// still reach /users/@me. No route takes a parameter that may contain an encoded "/".
func DecodedPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.RawPath = ""
		next.ServeHTTP(w, r)
	})
}
