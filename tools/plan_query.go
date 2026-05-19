package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
)

// PlanQueryArgs is the input for the plan_pubmed_query tool.
type PlanQueryArgs struct {
	Question          string            `json:"question"`
	PriorAttempts     []pubmed.QueryPlan `json:"prior_attempts,omitempty"`
	PriorResultCounts []int             `json:"prior_result_counts,omitempty"`
}

const planQueryPrompt = `You are a PubMed search expert. Generate an optimal PubMed boolean query for the following research question.
%s
Use MeSH terms where appropriate. Include filters for study type, date range, or language only when clearly implied by the question.
For the sort order, choose one of: relevance, most_recent, first_author, journal.

Respond ONLY with valid JSON matching this schema:
{
  "boolean_query": "<full PubMed boolean query string>",
  "mesh_terms": ["<term1>", "<term2>"],
  "filters": {
    "study_types": [],
    "date_from": "",
    "date_to": "",
    "languages": [],
    "humans_only": false
  },
  "sort_order": "relevance",
  "rationale": "<brief explanation of the query strategy>"
}

Research question: %s`

// NewPlanPubmedQueryTool creates the plan_pubmed_query ADK tool.
func NewPlanPubmedQueryTool(llm model.LLM) (tool.Tool, error) {
	handler := func(ctx tool.Context, args PlanQueryArgs) (pubmed.QueryPlan, error) {
		priorContext := buildPriorContext(args.PriorAttempts, args.PriorResultCounts)
		prompt := fmt.Sprintf(planQueryPrompt, priorContext, args.Question)

		req := &model.LLMRequest{
			Contents: []*genai.Content{
				genai.NewContentFromText(prompt, "user"),
			},
			Config: &genai.GenerateContentConfig{
				ResponseMIMEType: "application/json",
			},
		}

		text, err := generateText(ctx, llm, req)
		if err != nil {
			return pubmed.QueryPlan{}, fmt.Errorf("plan_pubmed_query: %w", err)
		}

		var plan pubmed.QueryPlan
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &plan); err != nil {
			return pubmed.QueryPlan{}, fmt.Errorf("plan_pubmed_query: parsing model response: %w (raw: %s)", err, text)
		}
		if plan.BooleanQuery == "" {
			return pubmed.QueryPlan{}, fmt.Errorf("plan_pubmed_query: model returned empty boolean_query")
		}
		return plan, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "plan_pubmed_query",
		Description: "Generate a structured PubMed query plan including boolean query, MeSH terms, filters, and sort order.",
	}, handler)
}

func buildPriorContext(attempts []pubmed.QueryPlan, counts []int) string {
	if len(attempts) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Previous attempts that did not yield good results:\n")
	for i, a := range attempts {
		count := 0
		if i < len(counts) {
			count = counts[i]
		}
		sb.WriteString(fmt.Sprintf("  Attempt %d: query=%q returned %d results. Rationale: %s\n",
			i+1, a.BooleanQuery, count, a.Rationale))
	}
	sb.WriteString("Generate a different, improved query.\n")
	return sb.String()
}
