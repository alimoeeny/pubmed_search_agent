package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
)

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

		return FetchResult{Articles: articles}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "pubmed_fetch_details",
		Description: "Fetch full metadata and abstracts for a list of PMIDs. Returns title, journal, publication date, abstract, authors, and MeSH terms.",
	}, handler)
}
