// Package middleware provides HTTP middleware for the PubMed agent server.
package middleware

import (
	"net/http"
	"strings"
)

// CORS returns middleware that sets Access-Control headers on every response
// and short-circuits OPTIONS preflight requests with 204 No Content.
//
// allowedOrigins is a comma-separated list of allowed origins.
// Pass "*" or an empty string to reflect any requesting origin back (dev mode).
//
// For credentialed requests (Authorization header), the spec forbids wildcard "*".
// This middleware always echoes the exact origin and sets Allow-Credentials: true
// so that JWT-authenticated cross-origin requests succeed.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	origins := parseOrigins(allowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Vary must always be set so caches don't serve the wrong origin.
			w.Header().Add("Vary", "Origin")

			if origin != "" {
				if len(origins) == 0 || allowed(origin, origins) {
					// Reflect the exact origin — required for credentialed requests.
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				// Unrecognised origin: no ACAO header → browser will block the request.
			} else {
				// No Origin header (e.g. curl / server-to-server) — wildcard is fine.
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Type, X-Session-ID")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseOrigins(raw string) []string {
	if raw == "" || raw == "*" {
		return nil // nil means allow all
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func allowed(origin string, origins []string) bool {
	if origin == "" {
		return false
	}
	for _, o := range origins {
		if o == origin {
			return true
		}
	}
	return false
}
