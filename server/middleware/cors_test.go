package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alimoeeny/pubmed_search_agent/server/middleware"
)

func TestCORS_PreflightShortCircuits(t *testing.T) {
	handler := middleware.CORS("https://app.example.com")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("inner handler should not be called for OPTIONS preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/sessions", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rr.Code)
	}
}

func TestCORS_AllowedOriginPassesThrough(t *testing.T) {
	reached := false
	handler := middleware.CORS("https://app.example.com")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !reached {
		t.Error("inner handler should have been called")
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("ACAO header = %q, want %q", got, "https://app.example.com")
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("ACAC header = %q, want %q", got, "true")
	}
	if got := rr.Header().Get("Vary"); got == "" {
		t.Error("Vary header must be set")
	}
}

// TestCORS_PermissiveReflectsOrigin verifies that empty CORS_ALLOWED_ORIGINS
// reflects the exact requesting origin (not "*") so credentialed requests work.
func TestCORS_PermissiveReflectsOrigin(t *testing.T) {
	handler := middleware.CORS("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("ACAO header = %q, want %q", got, "http://localhost:5173")
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("ACAC header = %q, want %q", got, "true")
	}
}

// TestCORS_NoOriginGetsWildcard verifies that requests without an Origin header
// (e.g. curl, server-to-server) still receive the wildcard header.
func TestCORS_NoOriginGetsWildcard(t *testing.T) {
	handler := middleware.CORS("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	// no Origin header
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO header = %q, want %q", got, "*")
	}
}

// TestCORS_UnrecognisedOriginBlocked verifies that an unrecognised origin receives
// no Access-Control-Allow-Origin header so the browser blocks the request.
func TestCORS_UnrecognisedOriginBlocked(t *testing.T) {
	handler := middleware.CORS("https://app.example.com")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unrecognised origin should get no ACAO header, got %q", got)
	}
}
