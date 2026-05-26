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
			name:        "sufficient with no gaps",
			llmResponse: `{"verdict":"SUFFICIENT","coverage_score":0.9,"evidence_gaps":[],"suggested_query_refinement":""}`,
			wantVerdict: "SUFFICIENT",
			wantGaps:    0,
		},
		{
			name:        "needs more evidence with gaps",
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
