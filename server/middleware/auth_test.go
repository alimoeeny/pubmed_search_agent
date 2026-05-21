package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/alimoeeny/pubmed_search_agent/server/middleware"
)

const testSecret = "super-secret-test-key"

func makeJWT(t *testing.T, sub, email string, secret string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"sub":   sub,
		"email": email,
		"exp":   exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("makeJWT: %v", err)
	}
	return raw
}

func TestAuth_ValidToken_PassesThrough(t *testing.T) {
	var captured middleware.UserIdentity
	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = middleware.UserIdentityFromContext(r.Context())
	}))

	raw := makeJWT(t, "user-uuid-123", "alice@example.com", testSecret, time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if captured.ID != "user-uuid-123" {
		t.Errorf("expected user ID user-uuid-123, got %q", captured.ID)
	}
	if captured.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", captured.Email)
	}
}

func TestAuth_MissingHeader_Returns401(t *testing.T) {
	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_ExpiredToken_Returns401(t *testing.T) {
	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	}))

	raw := makeJWT(t, "user-uuid-123", "alice@example.com", testSecret, time.Now().Add(-time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuth_WrongSecret_Returns401(t *testing.T) {
	handler := middleware.Auth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called")
	}))

	raw := makeJWT(t, "user-uuid-123", "alice@example.com", "wrong-secret", time.Now().Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
