// Package middleware provides HTTP middleware for the PubMed agent server.
package middleware

import (
	"net/http"
	"strings"
)

// CORS returns middleware that sets Access-Control headers on every response
// and short-circuits OPTIONS preflight requests with 200 OK — no auth required.
//
// allowedOrigins is a comma-separated list of allowed origins.
// Pass "*" (or an empty string) to allow all origins (suitable for dev).
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	origins := parseOrigins(allowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if allowed(origin, origins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(origins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
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
