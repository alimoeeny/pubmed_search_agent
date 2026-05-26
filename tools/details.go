package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
)

// SessionKeyFetchedPMIDs is the session-state key under which pubmed_fetch_details
// accumulates the set of fetched PMIDs. Used by the PMID guard in main.go.
const SessionKeyFetchedPMIDs = "fetched_pmids"

// SessionKeyFetchedArticles is the session-state key under which pubmed_fetch_details
// accumulates a map[string]pubmed.Article (keyed by PMID). Used by the PDF generator.
const SessionKeyFetchedArticles = "fetched_articles"

// SessionKeyReviewLoopCount tracks how many review/refine iterations have run this session.
const SessionKeyReviewLoopCount = "review_loop_count"

// SessionKeyReviewLastVerdict holds the verdict from the most recent review_summary call.
const SessionKeyReviewLastVerdict = "review_last_verdict"

// SessionKeyReviewLastGaps holds the evidence gap topics from the most recent review_summary call.
const SessionKeyReviewLastGaps = "review_last_gaps"

const (
	defaultFetchMax = 20
	hardFetchCap    = 50
)

// FetchArgs is the input for the pubmed_fetch_details tool.
type FetchArgs struct {
	PMIDs []string `json:"pmids"`
	Max   int      `json:"max,omitempty"`
}

// FetchResult is the output for the pubmed_fetch_details tool.
type FetchResult struct {
	Articles []pubmed.Article `json:"articles"`
}

// NewPubmedFetchDetailsTool creates the pubmed_fetch_details ADK tool.
func NewPubmedFetchDetailsTool(client *pubmed.Client) (tool.Tool, error) {
	handler := func(ctx tool.Context, args FetchArgs) (FetchResult, error) {
		max := args.Max
		if max <= 0 {
			max = defaultFetchMax
		}
		if max > hardFetchCap {
			max = hardFetchCap
		}

		pmids := args.PMIDs
		if len(pmids) > max {
			pmids = pmids[:max]
		}
		if len(pmids) == 0 {
			return FetchResult{}, nil
		}

		xmlData, err := client.EFetch(ctx, pmids)
		if err != nil {
			return FetchResult{}, fmt.Errorf("pubmed_fetch_details: %w", err)
		}

		articles, err := pubmed.ParseEFetchXML(xmlData)
		if err != nil {
			return FetchResult{}, fmt.Errorf("pubmed_fetch_details: parsing XML: %w", err)
		}

		// Persist fetched PMIDs into session state so the PMID guard
		// knows which PMIDs are legitimate citations.
		// Read from committed session state (not StateDelta) so that a second
		// pubmed_fetch_details call during the review loop accumulates rather
		// than overwrites the existing set.
		existingRaw, _ := ctx.State().Get(SessionKeyFetchedPMIDs)
		prev := toStringSliceAny(existingRaw)
		for _, a := range articles {
			if a.PMID != "" {
				prev = append(prev, a.PMID)
			}
		}
		ctx.Actions().StateDelta[SessionKeyFetchedPMIDs] = deduplicateStrings(prev)

		// Persist full article metadata map for use by the PDF generator.
		// Same pattern: read from committed state so gap-fill rounds accumulate.
		existingArticlesRaw, _ := ctx.State().Get(SessionKeyFetchedArticles)
		articleMap := toArticleMapAny(existingArticlesRaw)
		for _, a := range articles {
			if a.PMID != "" {
				articleMap[a.PMID] = a
			}
		}
		ctx.Actions().StateDelta[SessionKeyFetchedArticles] = articleMap

		return FetchResult{Articles: articles}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "pubmed_fetch_details",
		Description: "Fetch full metadata and abstracts for a list of PMIDs. Returns title, journal, publication date, abstract, authors, and MeSH terms.",
	}, handler)
}
