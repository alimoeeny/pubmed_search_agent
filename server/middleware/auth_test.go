package middleware_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"

	"github.com/alimoeeny/pubmed_search_agent/server/middleware"
)

const testKID = "test-kid"

func newTestKey(t *testing.T) (*ecdsa.PrivateKey, jose.JSONWebKeySet) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	jwk := jose.JSONWebKey{Key: &priv.PublicKey, KeyID: testKID, Algorithm: "ES256", Use: "sig"}
	return priv, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
}

func jwksServer(t *testing.T, ks jose.JSONWebKeySet) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ks)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/.well-known/jwks.json"
}

func makeES256JWT(t *testing.T, sub, email string, key *ecdsa.PrivateKey, kid string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": sub, "email": email, "exp": exp.Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = kid
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("makeES256JWT: %v", err)
	}
	return raw
}

func TestJWKSAuth_ValidToken_PassesThrough(t *testing.T) {
	priv, ks := newTestKey(t)
	var captured middleware.UserIdentity
	handler := middleware.JWKSAuth(jwksServer(t, ks))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = middleware.UserIdentityFromContext(r.Context())
	}))

	raw := makeES256JWT(t, "user-uuid-123", "alice@example.com", priv, testKID, time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if captured.ID != "user-uuid-123" {
		t.Errorf("ID = %q, want user-uuid-123", captured.ID)
	}
	if captured.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", captured.Email)
	}
}

func TestJWKSAuth_MissingHeader_Returns401(t *testing.T) {
	_, ks := newTestKey(t)
	handler := middleware.JWKSAuth(jwksServer(t, ks))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestJWKSAuth_ExpiredToken_Returns401(t *testing.T) {
	priv, ks := newTestKey(t)
	handler := middleware.JWKSAuth(jwksServer(t, ks))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	}))

	raw := makeES256JWT(t, "user-uuid-123", "alice@example.com", priv, testKID, time.Now().Add(-time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestJWKSAuth_WrongKey_Returns401(t *testing.T) {
	_, ks := newTestKey(t)
	wrongKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating wrong key: %v", err)
	}
	handler := middleware.JWKSAuth(jwksServer(t, ks))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	}))

	raw := makeES256JWT(t, "user-uuid-123", "alice@example.com", wrongKey, testKID, time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
