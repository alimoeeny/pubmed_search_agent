package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
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

// JWKSAuth returns middleware that validates Supabase-issued JWTs using the
// project's JWKS endpoint. Supports RS256, RS384, RS512, ES256, ES384, ES512.
//
// Keys are fetched lazily from jwksURL on the first auth request and cached for
// 15 minutes. Requests without a valid JWT receive a 401 JSON response.
//
// jwksURL: e.g. https://<project>.supabase.co/auth/v1/.well-known/jwks.json
func JWKSAuth(jwksURL string) func(http.Handler) http.Handler {
	cache := &jwksCache{url: jwksURL, ttl: 15 * time.Minute}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				writeAuthError(w, "missing or malformed Authorization header")
				return
			}

			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
				kid, _ := t.Header["kid"].(string)
				return cache.key(r.Context(), kid)
			}, jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}))
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

// jwksCache fetches and caches the JWKS for a given URL.
type jwksCache struct {
	url string
	ttl time.Duration

	mu   sync.RWMutex
	keys *jose.JSONWebKeySet
	exp  time.Time
}

func (c *jwksCache) key(ctx context.Context, kid string) (any, error) {
	ks, err := c.get(ctx)
	if err != nil {
		return nil, err
	}
	if kid != "" {
		if matches := ks.Key(kid); len(matches) > 0 {
			return publicKey(matches[0])
		}
	}
	if len(ks.Keys) > 0 {
		return publicKey(ks.Keys[0])
	}
	return nil, fmt.Errorf("jwks: no key found (kid=%q)", kid)
}

func (c *jwksCache) get(ctx context.Context) (*jose.JSONWebKeySet, error) {
	c.mu.RLock()
	if c.keys != nil && time.Now().Before(c.exp) {
		ks := c.keys
		c.mu.RUnlock()
		return ks, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check after acquiring write lock.
	if c.keys != nil && time.Now().Before(c.exp) {
		return c.keys, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("jwks: building request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jwks: fetching %s: %w", c.url, err)
	}
	defer resp.Body.Close()

	var ks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&ks); err != nil {
		return nil, fmt.Errorf("jwks: decoding key set: %w", err)
	}
	c.keys = &ks
	c.exp = time.Now().Add(c.ttl)
	return c.keys, nil
}

func publicKey(k jose.JSONWebKey) (any, error) {
	switch pub := k.Key.(type) {
	case *rsa.PublicKey:
		return pub, nil
	case *ecdsa.PublicKey:
		return pub, nil
	default:
		return nil, fmt.Errorf("jwks: unsupported key type %T", k.Key)
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
