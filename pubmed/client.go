package pubmed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
)

const (
	baseURL       = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils/"
	defaultRetMax = 100
	httpTimeout   = 20 * time.Second
	maxRetries    = 3
	toolName      = "pubmed_search_agent"
	agentVersion  = "0.1"
)

// Client is a rate-limited, retrying NCBI E-utilities client.
// All methods are safe for concurrent use.
type Client struct {
	http    *http.Client
	email   string
	limiter *time.Ticker
}

// ClientConfig holds constructor options.
type ClientConfig struct {
	Email      string
	HTTPClient *http.Client
}

// NewClient creates a Client.
// email must be non-empty (NCBI policy); caller is responsible for validation.
func NewClient(cfg ClientConfig) *Client {
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: httpTimeout}
	}
	return &Client{
		http:    hc,
		email:   cfg.Email,
		limiter: time.NewTicker(time.Second / 3), // 3 req/s
	}
}

// Close releases the internal rate-limit ticker.
func (c *Client) Close() {
	c.limiter.Stop()
}

// Email returns the email address currently associated with this client.
func (c *Client) Email() string {
	return c.email
}

// WithEmail returns a shallow copy of the client with email replaced.
// The HTTP client and rate limiter are shared with the original — safe for
// concurrent use and avoids duplicate tickers.
func (c *Client) WithEmail(email string) *Client {
	cp := *c
	cp.email = email
	return &cp
}

// esearchResponse mirrors the relevant fields of the NCBI esearch JSON response.
type esearchResponse struct {
	ESearchResult struct {
		Count            string   `json:"count"`
		RetMax           string   `json:"retmax"`
		RetStart         string   `json:"retstart"`
		IDList           []string `json:"idlist"`
		WebEnv           string   `json:"webenv"`
		QueryKey         string   `json:"querykey"`
		QueryTranslation string   `json:"querytranslation"`
	} `json:"esearchresult"`
}

// ESearch calls esearch.fcgi and returns the matching PMIDs plus metadata.
func (c *Client) ESearch(ctx context.Context, plan QueryPlan) (*SearchResult, error) {
	term := buildSearchTerm(plan)

	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("term", term)
	retMax := defaultRetMax
	if plan.MaxResults > 0 {
		retMax = plan.MaxResults
	}
	q.Set("retmax", fmt.Sprintf("%d", retMax))
	q.Set("usehistory", "y")
	q.Set("retmode", "json")
	if plan.SortOrder != "" {
		q.Set("sort", ncbiSortParam(plan.SortOrder))
	}
	c.addPoliteParams(q)

	body, err := c.get(ctx, "esearch.fcgi", q)
	if err != nil {
		return nil, fmt.Errorf("esearch: %w", err)
	}

	var parsed esearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("esearch: parsing response: %w", err)
	}

	count := 0
	fmt.Sscanf(parsed.ESearchResult.Count, "%d", &count)

	return &SearchResult{
		PMIDs:       parsed.ESearchResult.IDList,
		TotalCount:  count,
		QueryEchoed: parsed.ESearchResult.QueryTranslation,
		WebEnv:      parsed.ESearchResult.WebEnv,
		QueryKey:    parsed.ESearchResult.QueryKey,
	}, nil
}

// EFetch calls efetch.fcgi and returns the raw PubMed XML for the given PMIDs.
func (c *Client) EFetch(ctx context.Context, pmids []string) ([]byte, error) {
	if len(pmids) == 0 {
		return nil, nil
	}
	q := url.Values{}
	q.Set("db", "pubmed")
	q.Set("id", strings.Join(pmids, ","))
	q.Set("retmode", "xml")
	q.Set("rettype", "abstract")
	c.addPoliteParams(q)

	body, err := c.get(ctx, "efetch.fcgi", q)
	if err != nil {
		return nil, fmt.Errorf("efetch: %w", err)
	}
	return body, nil
}

func (c *Client) addPoliteParams(q url.Values) {
	q.Set("tool", toolName)
	q.Set("email", c.email)
}

func (c *Client) get(ctx context.Context, endpoint string, q url.Values) ([]byte, error) {
	rawURL := baseURL + endpoint + "?" + q.Encode()

	op := func() ([]byte, error) {
		select {
		case <-c.limiter.C:
		case <-ctx.Done():
			return nil, backoff.Permanent(ctx.Err())
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, backoff.Permanent(fmt.Errorf("building request: %w", err))
		}
		req.Header.Set("User-Agent", fmt.Sprintf("%s/%s (%s)", toolName, agentVersion, c.email))

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, fmt.Errorf("NCBI HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, backoff.Permanent(fmt.Errorf("NCBI HTTP %d", resp.StatusCode))
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading body: %w", err)
		}
		return data, nil
	}

	body, err := backoff.Retry(ctx, op,
		backoff.WithMaxTries(uint(maxRetries)),
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
	)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// buildSearchTerm composes the PubMed boolean query from a QueryPlan.
func buildSearchTerm(plan QueryPlan) string {
	parts := []string{plan.BooleanQuery}

	// Publication-type filters
	var ptFilters []string
	for _, st := range plan.Filters.StudyTypes {
		if term := PubTypeFilterTerm(st); term != "" {
			ptFilters = append(ptFilters, term)
		}
	}
	if len(ptFilters) == 1 {
		parts = append(parts, ptFilters[0])
	} else if len(ptFilters) > 1 {
		parts = append(parts, "("+strings.Join(ptFilters, " OR ")+")")
	}

	// Date range
	if plan.Filters.DateFrom != "" || plan.Filters.DateTo != "" {
		from := plan.Filters.DateFrom
		if from == "" {
			from = "1900/01/01"
		}
		to := plan.Filters.DateTo
		if to == "" {
			to = "3000/12/31"
		}
		parts = append(parts, fmt.Sprintf(`("%s"[PDAT] : "%s"[PDAT])`, from, to))
	}

	// Languages
	for _, lang := range plan.Filters.Languages {
		parts = append(parts, lang+"[lang]")
	}

	// Humans only
	if plan.Filters.HumansOnly {
		parts = append(parts, "humans[MeSH Terms]")
	}

	joined := strings.Join(parts, " AND ")
	return joined
}

// ncbiSortParam maps our SortOrder enum to the NCBI sort parameter value.
func ncbiSortParam(s SortOrder) string {
	switch s {
	case SortRelevance:
		return "relevance"
	case SortMostRecent:
		return "pub_date"
	case SortFirstAuthor:
		return "first_author"
	case SortJournal:
		return "journal"
	default:
		return "relevance"
	}
}
