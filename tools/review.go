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

// ReviewArgs is the input for the review_summary tool.
type ReviewArgs struct {
	Question string `json:"question"`
	Summary  string `json:"summary"`
}

// ReviewResult is the output for the review_summary tool.
type ReviewResult struct {
	Verdict                  string   `json:"verdict"` // "SUFFICIENT" | "NEEDS_MORE_EVIDENCE"
	CoverageScore            float64  `json:"coverage_score"`
	EvidenceGaps             []string `json:"evidence_gaps"`
	SuggestedQueryRefinement string   `json:"suggested_query_refinement"`
	LoopCount                int      `json:"loop_count"`
}

const reviewPrompt = `You are an independent scientific editor reviewing a draft literature summary.
You will be given:
1. The original research question
2. The draft summary (with inline PMID citations)
3. The titles and abstracts of articles that were fetched to write the summary

Your job is to evaluate whether the summary is well-supported by the fetched literature.

Evaluate:
- CITATION COVERAGE: Does every major factual claim have at least one supporting PMID from the fetched articles?
- ANSWER COMPLETENESS: Does the summary address all sub-questions implied by the research question?
- EVIDENCE GAPS: Are there topics mentioned in the summary (or important to the question) that have no supporting abstract?

Respond ONLY with valid JSON matching this schema:
{
  "verdict": "SUFFICIENT" | "NEEDS_MORE_EVIDENCE",
  "coverage_score": <float 0.0-1.0, fraction of claims supported by fetched abstracts>,
  "evidence_gaps": ["<topic not supported by any fetched abstract>", ...],
  "suggested_query_refinement": "<natural-language hint for a follow-up PubMed search targeting the gaps, or empty string if SUFFICIENT>"
}

Set verdict to SUFFICIENT if coverage_score >= 0.8 or if there are no meaningful evidence gaps.
Set verdict to NEEDS_MORE_EVIDENCE only if specific topics important to the question are missing from the fetched literature.

Research question: %s

Fetched article titles and abstracts:
%s

Draft summary:
%s`

// NewReviewSummaryTool creates the review_summary ADK tool.
// The tool reads fetched article metadata from session state so the LLM does not need to pass it.
func NewReviewSummaryTool(llm model.LLM) (tool.Tool, error) {
	handler := func(ctx tool.Context, args ReviewArgs) (ReviewResult, error) {
		rawArticles, _ := ctx.State().Get(SessionKeyFetchedArticles)
		articleMap := toArticleMapAny(rawArticles)

		rawCount, _ := ctx.State().Get(SessionKeyReviewLoopCount)
		loopCount := toInt(rawCount)

		articleContext := buildArticleContext(articleMap)

		prompt := fmt.Sprintf(reviewPrompt, args.Question, articleContext, args.Summary)
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
			return ReviewResult{}, fmt.Errorf("review_summary: %w", err)
		}

		var result ReviewResult
		if err := parseReviewResult(text, &result); err != nil {
			return ReviewResult{}, err
		}

		loopCount++
		result.LoopCount = loopCount
		ctx.Actions().StateDelta[SessionKeyReviewLoopCount] = loopCount
		ctx.Actions().StateDelta[SessionKeyReviewLastVerdict] = result.Verdict
		ctx.Actions().StateDelta[SessionKeyReviewLastGaps] = result.EvidenceGaps

		return result, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "review_summary",
		Description: "Review the draft summary for citation coverage and evidence gaps. Returns verdict (SUFFICIENT or NEEDS_MORE_EVIDENCE), a coverage score, and a list of gap topics to search if more evidence is needed.",
	}, handler)
}

// parseReviewResult unmarshals the LLM JSON response into a ReviewResult.
func parseReviewResult(text string, result *ReviewResult) error {
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), result); err != nil {
		return fmt.Errorf("review_summary: parsing model response: %w (raw: %s)", err, text)
	}
	if result.Verdict != "SUFFICIENT" && result.Verdict != "NEEDS_MORE_EVIDENCE" {
		return fmt.Errorf("review_summary: unexpected verdict %q", result.Verdict)
	}
	return nil
}

// buildArticleContext formats fetched articles for the review prompt.
func buildArticleContext(articles map[string]pubmed.Article) string {
	if len(articles) == 0 {
		return "(no articles fetched)"
	}
	var sb strings.Builder
	for pmid, a := range articles {
		fmt.Fprintf(&sb, "[PMID:%s] %s — %s\nAbstract: %s\n\n",
			pmid, a.Title, a.Journal, truncate(a.Abstract, 400))
	}
	return sb.String()
}

// truncate shortens s to at most n runes, appending "..." if truncated.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// toInt safely converts any session-state value to int.
// Handles float64 because JSON round-trips through Postgres deserialize numbers as float64.
func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	}
	return 0
}
