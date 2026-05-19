// Package tools provides ADK tool implementations for the PubMed research agent.
package tools

import (
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
)

// SearchArgs is the input for the pubmed_search tool.
type SearchArgs struct {
	BooleanQuery string           `json:"boolean_query"`
	MeshTerms    []string         `json:"mesh_terms,omitempty"`
	Filters      pubmed.Filters   `json:"filters,omitempty"`
	SortOrder    pubmed.SortOrder `json:"sort_order,omitempty"`
	Rationale    string           `json:"rationale,omitempty"`
	MaxResults   int              `json:"max_results,omitempty"` // 0 = default 100
}

// SearchResult is the output for the pubmed_search tool.
type SearchResult struct {
	PMIDs      []string `json:"pmids"`
	TotalCount int      `json:"total_count"`
	Query      string   `json:"query,omitempty"`
}

// NewPubmedSearchTool creates the pubmed_search ADK tool.
func NewPubmedSearchTool(client *pubmed.Client) (tool.Tool, error) {
	handler := func(ctx tool.Context, args SearchArgs) (SearchResult, error) {
		plan := pubmed.QueryPlan{
			BooleanQuery: args.BooleanQuery,
			MeshTerms:    args.MeshTerms,
			Filters:      args.Filters,
			SortOrder:    args.SortOrder,
			Rationale:    args.Rationale,
			MaxResults:   args.MaxResults,
		}
		result, err := client.ESearch(ctx, plan)
		if err != nil {
			return SearchResult{}, fmt.Errorf("pubmed_search: %w", err)
		}
		return SearchResult{
			PMIDs:      result.PMIDs,
			TotalCount: result.TotalCount,
			Query:      result.QueryEchoed,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "pubmed_search",
		Description: "Search PubMed using a boolean query. Returns a list of PMIDs and the total result count. Use this after plan_pubmed_query.",
	}, handler)
}
