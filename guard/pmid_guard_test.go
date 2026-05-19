package guard

import (
	"strings"
	"testing"

	"google.golang.org/genai"
)

func TestStripHallucinatedPMIDs_NoHallucinations(t *testing.T) {
	allowed := PMIDSet([]string{"12345678", "87654321"})
	content := &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{Text: "Aspirin reduces mortality [PMID:12345678] and also [PMID:87654321]."},
		},
	}

	result := StripHallucinatedPMIDs(content, allowed)

	if result != content {
		t.Error("expected same pointer when no hallucinations present")
	}
}

func TestStripHallucinatedPMIDs_WithHallucinations(t *testing.T) {
	allowed := PMIDSet([]string{"12345678"})
	content := &genai.Content{
		Role: "model",
		Parts: []*genai.Part{
			{Text: "Real citation [PMID:12345678] and fake [PMID:99999999]."},
		},
	}

	result := StripHallucinatedPMIDs(content, allowed)

	if result == content {
		t.Error("expected new content when hallucination stripped")
	}
	text := result.Parts[0].Text
	if strings.Contains(text, "[PMID:99999999]") {
		t.Errorf("hallucinated PMID still present in: %q", text)
	}
	if !strings.Contains(text, "[PMID:12345678]") {
		t.Errorf("real PMID was incorrectly stripped from: %q", text)
	}
	if !strings.Contains(text, "citation removed") {
		t.Errorf("expected 'citation removed' placeholder in: %q", text)
	}
}

func TestStripHallucinatedPMIDs_NilContent(t *testing.T) {
	result := StripHallucinatedPMIDs(nil, PMIDSet([]string{"123"}))
	if result != nil {
		t.Error("expected nil for nil input")
	}
}

func TestStripHallucinatedPMIDs_EmptyAllowed(t *testing.T) {
	content := &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{{Text: "Some text [PMID:12345678]."}},
	}
	result := StripHallucinatedPMIDs(content, map[string]struct{}{})
	if result != content {
		t.Error("expected same pointer when allowed set is empty (passthrough)")
	}
}

func TestExtractPMIDs(t *testing.T) {
	text := "See [PMID:111] and [PMID:222]. Also [PMID:111] again."
	pmids := ExtractPMIDs(text)
	if len(pmids) != 2 {
		t.Errorf("ExtractPMIDs: got %d unique PMIDs, want 2", len(pmids))
	}
	for _, id := range []string{"111", "222"} {
		if _, ok := pmids[id]; !ok {
			t.Errorf("ExtractPMIDs: missing PMID %q", id)
		}
	}
}

func TestPMIDSet(t *testing.T) {
	s := PMIDSet([]string{"  111 ", "222", "111"})
	if len(s) != 2 {
		t.Errorf("PMIDSet: got %d entries, want 2 (dedup)", len(s))
	}
	if _, ok := s["111"]; !ok {
		t.Error("PMIDSet: missing trimmed entry '111'")
	}
}
