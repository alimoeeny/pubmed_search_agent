# Review/Refine Loop Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a post-summarization review/refine loop that evaluates citation coverage, identifies evidence gaps, and retries with targeted PubMed searches (up to 2 times) before generating the PDF.

**Architecture:** A new `review_summary` ADK tool wraps a `RoleReviewer` LLM call that sees the draft summary plus fetched article metadata from session state. The orchestrator's `agentInstruction` controls the loop — it calls `review_summary` after step 5, then conditionally re-searches and re-summarizes based on the verdict, before proceeding to step 6 (PDF). Loop state (`review_loop_count`, verdict, gaps) lives in `StateDelta` so it survives across ADK turns.

**Tech Stack:** Go 1.26, `google.golang.org/adk` (functiontool, tool.Context, StateDelta), `google.golang.org/genai`

---

### Task 1: Add `RoleReviewer` to model_factory.go

**Files:**
- Modify: `model_factory.go:19-23`

**Step 1: Add the constant**

In `model_factory.go`, add `RoleReviewer` alongside the existing role constants:

```go
const (
	RoleOrchestrator ModelRole = "orchestrator"
	RoleValidator    ModelRole = "validator"
	RolePlanner      ModelRole = "planner"
	RoleReviewer     ModelRole = "reviewer"
)
```

**Step 2: Run quality gates**

```bash
go build ./...
go vet ./...
go test ./...
```

Expected: all pass (no logic change, just a new constant).

---

### Task 2: Add session state keys for review loop

**Files:**
- Modify: `tools/details.go:12-18` (add new constants after existing ones)

**Step 1: Write the failing test**

Create `tools/review_test.go` (file does not exist yet) with:

```go
package tools

import (
	"testing"
)

func TestSessionKeys_ReviewLoop(t *testing.T) {
	if SessionKeyReviewLoopCount == "" {
		t.Fatal("SessionKeyReviewLoopCount must be non-empty")
	}
	if SessionKeyReviewLastVerdict == "" {
		t.Fatal("SessionKeyReviewLastVerdict must be non-empty")
	}
	if SessionKeyReviewLastGaps == "" {
		t.Fatal("SessionKeyReviewLastGaps must be non-empty")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./tools/ -run TestSessionKeys_ReviewLoop -v
```

Expected: compile error — `SessionKeyReviewLoopCount undefined`.

**Step 3: Add the constants to `tools/details.go`**

After the existing `SessionKeyFetchedArticles` constant, add:

```go
// SessionKeyReviewLoopCount tracks how many review/refine iterations have run this session.
const SessionKeyReviewLoopCount = "review_loop_count"

// SessionKeyReviewLastVerdict holds the verdict from the most recent review_summary call.
const SessionKeyReviewLastVerdict = "review_last_verdict"

// SessionKeyReviewLastGaps holds the evidence gap topics from the most recent review_summary call.
const SessionKeyReviewLastGaps = "review_last_gaps"
```

**Step 4: Run test to verify it passes**

```bash
go test ./tools/ -run TestSessionKeys_ReviewLoop -v
```

Expected: PASS.

---

### Task 3: Implement `review_summary` tool

**Files:**
- Create: `tools/review.go`
- Modify: `tools/review_test.go` (extend with functional tests)

**Step 1: Write failing tests**

Add to `tools/review_test.go`:

```go
func TestReviewSummary_SufficientVerdict(t *testing.T) {
	llm := &mockLLM{text: `{
		"verdict": "SUFFICIENT",
		"coverage_score": 0.9,
		"evidence_gaps": [],
		"suggested_query_refinement": ""
	}`}

	tool, err := NewReviewSummaryTool(llm)
	if err != nil {
		t.Fatalf("NewReviewSummaryTool: %v", err)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestReviewSummary_NeedsMoreEvidence(t *testing.T) {
	llm := &mockLLM{text: `{
		"verdict": "NEEDS_MORE_EVIDENCE",
		"coverage_score": 0.4,
		"evidence_gaps": ["long-term outcomes", "pediatric dosing"],
		"suggested_query_refinement": "focus on long-term follow-up and pediatric populations"
	}`}

	tool, err := NewReviewSummaryTool(llm)
	if err != nil {
		t.Fatalf("NewReviewSummaryTool: %v", err)
	}
	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestReviewSummaryPromptParsing(t *testing.T) {
	tests := []struct {
		name        string
		llmResponse string
		wantVerdict string
		wantGaps    int
	}{
		{
			name: "sufficient with no gaps",
			llmResponse: `{"verdict":"SUFFICIENT","coverage_score":0.9,"evidence_gaps":[],"suggested_query_refinement":""}`,
			wantVerdict: "SUFFICIENT",
			wantGaps:    0,
		},
		{
			name: "needs more evidence with gaps",
			llmResponse: `{"verdict":"NEEDS_MORE_EVIDENCE","coverage_score":0.4,"evidence_gaps":["gap A","gap B"],"suggested_query_refinement":"search gap topics"}`,
			wantVerdict: "NEEDS_MORE_EVIDENCE",
			wantGaps:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ReviewResult
			if err := parseReviewResult(tt.llmResponse, &result); err != nil {
				t.Fatalf("parseReviewResult: %v", err)
			}
			if result.Verdict != tt.wantVerdict {
				t.Errorf("verdict = %q, want %q", result.Verdict, tt.wantVerdict)
			}
			if len(result.EvidenceGaps) != tt.wantGaps {
				t.Errorf("len(EvidenceGaps) = %d, want %d", len(result.EvidenceGaps), tt.wantGaps)
			}
		})
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./tools/ -run "TestReviewSummary|TestReviewSummaryPromptParsing" -v
```

Expected: compile error — `NewReviewSummaryTool` and `ReviewResult` undefined.

**Step 3: Implement `tools/review.go`**

```go
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
// The tool reads fetched article metadata from session state (no need for the LLM to pass it).
func NewReviewSummaryTool(llm model.LLM) (tool.Tool, error) {
	handler := func(ctx tool.Context, args ReviewArgs) (ReviewResult, error) {
		// Read fetched articles from session state.
		rawArticles, _ := ctx.Actions().StateDelta[SessionKeyFetchedArticles]
		articleMap := toArticleMapAny(rawArticles)

		// Read current loop count.
		rawCount, _ := ctx.Actions().StateDelta[SessionKeyReviewLoopCount]
		loopCount := toInt(rawCount)

		// Build article context for the prompt.
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

		// Increment and persist loop count.
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

// parseReviewResult unmarshals the LLM's JSON response into a ReviewResult.
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
		sb.WriteString(fmt.Sprintf("[PMID:%s] %s — %s\nAbstract: %s\n\n",
			pmid, a.Title, a.Journal, truncate(a.Abstract, 400)))
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
func toInt(v any) int {
	if v == nil {
		return 0
	}
	if i, ok := v.(int); ok {
		return i
	}
	return 0
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./tools/ -run "TestReviewSummary|TestReviewSummaryPromptParsing" -v
```

Expected: all PASS.

**Step 5: Run full test suite**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: all pass.

---

### Task 4: Wire `reviewerModel` and `reviewTool` into `main.go`

**Files:**
- Modify: `main.go:66-77` (model instantiation block)
- Modify: `main.go:217-234` (agent build block)

**Step 1: Add reviewer model instantiation**

After the `plannerModel` instantiation (around line 76), add:

```go
reviewerModel, err := ModelFor(ctx, RoleReviewer, userCfg)
if err != nil {
    log.Fatalf("Failed to create reviewer model: %v", err)
}
```

**Step 2: Build `reviewTool`**

After `askUserTool` is created (around line 110), add:

```go
reviewTool, err := agenttools.NewReviewSummaryTool(reviewerModel)
if err != nil {
    log.Fatalf("Failed to create review_summary tool: %v", err)
}
```

**Step 3: Register `reviewTool` in the agent**

In the `llmagent.Config.Tools` slice (around line 222), add `reviewTool`:

```go
Tools: []tool.Tool{
    validateTool,
    planTool,
    searchTool,
    detailsTool,
    askUserTool,
    reviewTool,
    pdfTool,
},
```

**Step 4: Run quality gates**

```bash
go build ./... && go vet ./...
```

Expected: clean compile, no vet warnings.

---

### Task 5: Update `agentInstruction` to add the review/refine loop (Step 5a)

**Files:**
- Modify: `main.go:297-333` (the `agentInstruction` const)

**Step 1: Replace the instruction**

Replace the current `agentInstruction` const with:

```go
const agentInstruction = `You are a PubMed research assistant. Follow these steps for every research question:

1. VALIDATE: Call validate_question with the user's question.
   - If is_researchable is false, call ask_user with the suggested_refinements as options.
   - Do NOT proceed to searching until the question is researchable.

2. PLAN: Call plan_pubmed_query with the (refined) question.
   - On retry, pass prior_attempts and prior_result_counts so the query can be improved.

3. SEARCH: Call pubmed_search with the query plan.
   - If total_count == 0 or < 3: broaden the query — call plan_pubmed_query again with
     the failed attempt and retry. Maximum 3 attempts.
   - If total_count > 200: narrow the query — add MeSH terms, study-type filters, or
     date constraints, then retry.
   - After 3 failed attempts, call ask_user to get clarification before trying again.

4. FETCH: Call pubmed_fetch_details on the top PMIDs (up to 20).

5. SUMMARIZE: Write a concise human-readable answer grounded in the fetched abstracts.
   - Cite every factual claim inline as [PMID:XXXXXXXX](https://pubmed.ncbi.nlm.nih.gov/XXXXXXXX/)
     (replace XXXXXXXX with the actual PMID number in both places).
   - End with a "**References**" section listing each cited paper as:
     - [[XXXXXXXX](https://pubmed.ncbi.nlm.nih.gov/XXXXXXXX/)] Title — *Journal*, YYYY-MM-DD
   - Do NOT invent PMIDs, titles, or findings. Only cite PMIDs returned by pubmed_fetch_details.
   - If the abstracts do not contain enough information to answer the question, say so clearly.

5a. REVIEW: Call review_summary with:
    - question: the original research question (verbatim)
    - summary: the full markdown summary text you just wrote (including the References section)

    Interpret the result:
    - If verdict == "SUFFICIENT" or loop_count >= 2: proceed directly to step 6.
    - If verdict == "NEEDS_MORE_EVIDENCE" and loop_count < 2:
        a. Call plan_pubmed_query. Set question to: the original question plus
           " Focus on these gaps: " followed by the evidence_gaps joined with commas.
           Also pass the suggested_query_refinement as additional context in the question.
        b. Call pubmed_search with the new query plan.
        c. Call pubmed_fetch_details on the new PMIDs.
           - If pubmed_fetch_details returns no articles (all PMIDs already fetched this session),
             PubMed has no more coverage — proceed directly to step 6 without re-summarizing.
        d. Return to step 5: re-synthesize the summary using the now-larger article pool.
           The PMID guard will validate citations on the new summary automatically.

6. REPORT: After delivering the summary, ALWAYS call generate_pdf with:
   - question: the original research question (verbatim)
   - summary: your full markdown summary text (including the References section)
   - articles: the list of articles returned by pubmed_fetch_details, each with pmid, title, journal, and date
   After generate_pdf returns, include the download link in your response as:
   📄 [Download PDF report](<download_url>)

Format rules:
- Use **bold** for the summary header.
- Keep the summary concise (300–500 words unless the question requires more depth).
- The References section must list every PMID cited in the summary.`
```

**Step 2: Run full quality gates**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: all pass.

---

## Testing the complete loop manually

Start the agent locally (ADK web UI mode):

```bash
go run . 
```

Ask a question likely to trigger a refine loop — one with multiple sub-topics:

> "What are the long-term cardiovascular effects of GLP-1 receptor agonists and what is known about their use in pediatric populations?"

Watch for `review_summary` being called in the tool trace. A `NEEDS_MORE_EVIDENCE` verdict on loop 1 should trigger a second `plan_pubmed_query` → `pubmed_search` → `pubmed_fetch_details` → re-summarize. The second `review_summary` call should return `SUFFICIENT` or be skipped because `loop_count >= 2`.

Verify:
- [ ] `review_summary` appears in the ADK tool call trace
- [ ] Loop count increments correctly (check session state in ADK debug panel)
- [ ] PDF is generated with the final (reviewed) summary
- [ ] No hallucinated PMIDs in the final summary
