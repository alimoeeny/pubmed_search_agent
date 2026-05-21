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

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
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
		t.Errorf("unexpected ACAO header: %q", got)
	}
}

func TestCORS_WildcardWhenNoOriginsConfigured(t *testing.T) {
	handler := middleware.CORS("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("expected wildcard ACAO, got %q", got)
	}
}
