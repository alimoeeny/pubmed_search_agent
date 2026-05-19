package pubmed

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	cacheTTL     = 7 * 24 * time.Hour
	cacheSubdir  = "pubmed_search_agent"
	cacheVersion = "v1"
)

// cacheEntry is the on-disk format for a cached response.
type cacheEntry struct {
	FetchedAt  time.Time         `json:"fetched_at"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
}

// CachingRoundTripper wraps an http.RoundTripper and caches responses on disk.
// It is disabled when the PUBMED_CACHE_DISABLE env var is set to "1".
// Concurrent requests for the same cache key are deduplicated via singleflight.
type CachingRoundTripper struct {
	Base     http.RoundTripper
	cacheDir string
	group    singleflight.Group
}

// NewCachingRoundTripper creates a CachingRoundTripper backed by Base.
// Cache files are stored under ${XDG_CACHE_HOME:-$HOME/.cache}/pubmed_search_agent/.
func NewCachingRoundTripper(base http.RoundTripper) (*CachingRoundTripper, error) {
	dir, err := cacheDirectory()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating cache directory %q: %w", dir, err)
	}
	return &CachingRoundTripper{Base: base, cacheDir: dir}, nil
}

// RoundTrip implements http.RoundTripper.
func (c *CachingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if os.Getenv("PUBMED_CACHE_DISABLE") == "1" {
		return c.Base.RoundTrip(req)
	}

	key, err := cacheKey(req)
	if err != nil {
		return c.Base.RoundTrip(req)
	}

	path := filepath.Join(c.cacheDir, key+".json")

	if entry, ok := readCacheEntry(path); ok {
		return entryToResponse(req, entry), nil
	}

	// Deduplicate concurrent cache misses for the same key.
	type result struct {
		resp *http.Response
		err  error
	}
	v, err, _ := c.group.Do(key, func() (any, error) {
		// Re-check cache inside the singleflight to handle a race where
		// another goroutine already wrote the file while we were waiting.
		if entry, ok := readCacheEntry(path); ok {
			return &result{resp: entryToResponse(req, entry)}, nil
		}
		resp, err := c.Base.RoundTrip(req)
		if err != nil {
			return &result{err: err}, nil
		}
		if resp.StatusCode == http.StatusOK {
			if saveErr := saveCacheEntry(path, resp); saveErr != nil {
				fmt.Fprintf(os.Stderr, "pubmed cache: failed to save %q: %v\n", path, saveErr)
			}
		}
		return &result{resp: resp}, nil
	})
	if err != nil {
		return nil, err
	}
	r := v.(*result)
	return r.resp, r.err
}

func cacheDirectory() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining home directory: %w", err)
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, cacheSubdir, cacheVersion), nil
}

func cacheKey(req *http.Request) (string, error) {
	q := req.URL.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	parts = append(parts, req.Method)
	parts = append(parts, req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
	for _, k := range keys {
		for _, v := range q[k] {
			parts = append(parts, k+"="+v)
		}
	}

	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}

func readCacheEntry(path string) (*cacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}
	if time.Since(entry.FetchedAt) > cacheTTL {
		_ = os.Remove(path)
		return nil, false
	}
	return &entry, true
}

func saveCacheEntry(path string, resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	headers := make(map[string]string, len(resp.Header))
	for k, vs := range resp.Header {
		headers[k] = strings.Join(vs, ", ")
	}

	entry := cacheEntry{
		FetchedAt:  time.Now().UTC(),
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshalling cache entry: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func entryToResponse(req *http.Request, entry *cacheEntry) *http.Response {
	headers := make(http.Header, len(entry.Headers))
	for k, v := range entry.Headers {
		headers.Set(k, v)
	}
	headers.Set("X-Pubmed-Cache", "hit")
	return &http.Response{
		StatusCode: entry.StatusCode,
		Status:     fmt.Sprintf("%d %s", entry.StatusCode, http.StatusText(entry.StatusCode)),
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(entry.Body)),
		Request:    req,
	}
}
