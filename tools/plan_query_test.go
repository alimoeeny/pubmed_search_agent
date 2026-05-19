package tools

import (
	"testing"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
)

func TestBuildPriorContext_Empty(t *testing.T) {
	got := buildPriorContext(nil, nil)
	if got != "" {
		t.Errorf("buildPriorContext(nil, nil) = %q, want empty", got)
	}
}

func TestBuildPriorContext_WithAttempts(t *testing.T) {
	attempts := []pubmed.QueryPlan{
		{BooleanQuery: "aspirin[MeSH]", Rationale: "initial try"},
	}
	counts := []int{0}

	got := buildPriorContext(attempts, counts)
	for _, want := range []string{"aspirin[MeSH]", "returned 0 results", "Attempt 1"} {
		if !containsStr(got, want) {
			t.Errorf("buildPriorContext: missing %q in %q", want, got)
		}
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
