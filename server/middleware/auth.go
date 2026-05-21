package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// UserIdentity holds the identity extracted from a validated Supabase JWT.
type UserIdentity struct {
	ID    string // JWT "sub" claim — Supabase user UUID
	Email string // JWT "email" claim
}

type contextKey string

const userIdentityCtxKey contextKey = "user_identity"

// UserIdentityFromContext retrieves the UserIdentity injected by Auth middleware.
// Returns the zero value and false if not present.
func UserIdentityFromContext(ctx context.Context) (UserIdentity, bool) {
	v, ok := ctx.Value(userIdentityCtxKey).(UserIdentity)
	return v, ok
}

// Auth returns middleware that validates a Supabase-issued HS256 JWT in the
// Authorization header and attaches a UserIdentity to the request context.
//
// Requests without a valid JWT receive a 401 JSON response.
// jwtSecret is the "JWT Secret" value from your Supabase project settings.
func Auth(jwtSecret string) func(http.Handler) http.Handler {
	secret := []byte(jwtSecret)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				writeAuthError(w, "missing or malformed Authorization header")
				return
			}

			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(raw, claims, func(_ *jwt.Token) (any, error) {
				return secret, nil
			}, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil {
				writeAuthError(w, "invalid or expired token")
				return
			}

			sub, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)
			if sub == "" {
				writeAuthError(w, "token missing sub claim")
				return
			}

			ctx := context.WithValue(r.Context(), userIdentityCtxKey, UserIdentity{
				ID:    sub,
				Email: email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	token := strings.TrimPrefix(h, "Bearer ")
	if token == "" {
		return "", false
	}
	return token, true
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
