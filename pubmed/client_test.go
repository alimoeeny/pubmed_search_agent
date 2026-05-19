package pubmed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestESearch_URLConstruction(t *testing.T) {
	var capturedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		resp := esearchResponse{}
		resp.ESearchResult.Count = "5"
		resp.ESearchResult.IDList = []string{"111", "222"}
		resp.ESearchResult.WebEnv = "testwebenv"
		resp.ESearchResult.QueryKey = "1"
		resp.ESearchResult.QueryTranslation = "aspirin[MeSH]"
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	origBase := baseURL
	// Temporarily override the base URL for this test via a patched client.
	_ = origBase // not used — we patch via the http.Client Transport

	client := &Client{
		http:    patchedHTTPClient(srv.URL),
		email:   "test@example.com",
		limiter: rateLimiterForTest(),
	}

	plan := QueryPlan{
		BooleanQuery: "aspirin[MeSH]",
		SortOrder:    SortRelevance,
		Filters: Filters{
			HumansOnly: true,
		},
	}

	result, err := client.ESearch(context.Background(), plan)
	if err != nil {
		t.Fatalf("ESearch: %v", err)
	}
	if result.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", result.TotalCount)
	}
	if len(result.PMIDs) != 2 {
		t.Errorf("PMIDs len = %d, want 2", len(result.PMIDs))
	}

	// Verify required params are present.
	for _, param := range []string{"db=pubmed", "term=", "email=test%40example.com", "tool=pubmed_search_agent", "sort=relevance"} {
		if !contains(capturedQuery, param) {
			t.Errorf("ESearch URL missing param %q in query %q", param, capturedQuery)
		}
	}
}

func TestESearch_CacheHit(t *testing.T) {
	// Point cache at a fresh temp dir so tests are hermetic.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := esearchResponse{}
		resp.ESearchResult.Count = "1"
		resp.ESearchResult.IDList = []string{"999"}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	crt, err := NewCachingRoundTripper(patchedHTTPClient(srv.URL).Transport)
	if err != nil {
		t.Fatalf("NewCachingRoundTripper: %v", err)
	}

	client := &Client{
		http:    &http.Client{Transport: crt},
		email:   "test@example.com",
		limiter: rateLimiterForTest(),
	}

	plan := QueryPlan{BooleanQuery: "unique_test_term_xyz"}

	_, err = client.ESearch(context.Background(), plan)
	if err != nil {
		t.Fatalf("first ESearch: %v", err)
	}
	_, err = client.ESearch(context.Background(), plan)
	if err != nil {
		t.Fatalf("second ESearch: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 HTTP call (second should be cached), got %d", callCount)
	}
}

// patchedHTTPClient returns an *http.Client whose requests are redirected to testServerURL.
func patchedHTTPClient(testServerURL string) *http.Client {
	return &http.Client{
		Transport: &redirectingTransport{base: testServerURL},
	}
}

type redirectingTransport struct {
	base string
}

func (r *redirectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite scheme+host to the test server; preserve path+query.
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = r.base[len("http://"):]
	return http.DefaultTransport.RoundTrip(req2)
}

// rateLimiterForTest returns a fast ticker to avoid slowing down unit tests.
func rateLimiterForTest() *time.Ticker {
	return time.NewTicker(1) // fires immediately
}
