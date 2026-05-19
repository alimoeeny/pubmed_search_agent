package guard

import (
	"context"
	"strings"
	"testing"
)

func TestReplacePMIDCitation_Plain(t *testing.T) {
	text := "See [PMID:111] for details."
	got := replacePMIDCitation(text, "111", "[REPLACED]")
	if got != "See [REPLACED] for details." {
		t.Errorf("replacePMIDCitation plain: got %q", got)
	}
}

func TestReplacePMIDCitation_Linked(t *testing.T) {
	text := "See [PMID:111](https://pubmed.ncbi.nlm.nih.gov/111/) for details."
	got := replacePMIDCitation(text, "111", "[REPLACED]")
	if got != "See [REPLACED] for details." {
		t.Errorf("replacePMIDCitation linked: got %q", got)
	}
}

func TestUnknownPMIDs_AllAllowed(t *testing.T) {
	allowed := PMIDSet([]string{"111", "222"})
	text := "See [PMID:111] and [PMID:222]."
	unknown := unknownPMIDs(text, allowed)
	if len(unknown) != 0 {
		t.Errorf("expected no unknown PMIDs, got %v", unknown)
	}
}

func TestUnknownPMIDs_SomeUnknown(t *testing.T) {
	allowed := PMIDSet([]string{"111"})
	text := "See [PMID:111] and [PMID:999]."
	unknown := unknownPMIDs(text, allowed)
	if len(unknown) != 1 || unknown[0] != "999" {
		t.Errorf("expected [999], got %v", unknown)
	}
}

func TestFixCitations_AllAllowed_NoOp(t *testing.T) {
	allowed := PMIDSet([]string{"12345678"})
	text := "Aspirin works [PMID:12345678](https://pubmed.ncbi.nlm.nih.gov/12345678/)."

	// With no unknown PMIDs, FixCitations should return unchanged text immediately
	// without hitting the network or LLM.
	v := &Verifier{client: nil, llm: nil}
	got, err := v.FixCitations(context.Background(), text, allowed)
	if err != nil {
		t.Fatalf("FixCitations: %v", err)
	}
	if got != text {
		t.Errorf("FixCitations no-op: expected unchanged text, got %q", got)
	}
}

func TestFixCitations_FinalFallback(t *testing.T) {
	// Test replacePMIDCitation + warning marker format (the final-pass behaviour).
	text := "Real [PMID:111] and bad [PMID:999999]."

	// Simulate maxFixAttempts exhausted by directly calling the final-pass logic.
	// We test replacePMIDCitation + the marker format.
	result := replacePMIDCitation(text, "999999", "[⚠ hallucination warning: PMID 999999 — could not verify]")
	if strings.Contains(result, "[PMID:999999]") {
		t.Errorf("expected citation replaced, got: %q", result)
	}
	if !strings.Contains(result, "hallucination warning") {
		t.Errorf("expected warning marker in: %q", result)
	}
	if !strings.Contains(result, "[PMID:111]") {
		t.Errorf("real citation should be untouched in: %q", result)
	}
}
