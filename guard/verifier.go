package guard

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
)

const (
	maxFixAttempts      = 3
	hallucinationMarker = "[⚠ hallucination warning: PMID %s — could not verify]"
)

// Verifier checks PMID citations in a summary against a known-good set, fetches
// any unknown PMIDs from NCBI to verify them, and uses an LLM correction loop to
// fix or remove bad citations. After maxFixAttempts failures the citation is
// replaced with a visible warning marker.
type Verifier struct {
	client *pubmed.Client
	llm    model.LLM
}

// NewVerifier creates a Verifier.
func NewVerifier(client *pubmed.Client, llm model.LLM) *Verifier {
	return &Verifier{client: client, llm: llm}
}

// FixCitations validates all [PMID:N] citations in summaryText against allowedPMIDs.
// Unknown PMIDs are fetched from NCBI; if they exist their real metadata is given
// to the LLM to correct; if unreachable or uncorrectable after maxFixAttempts they
// become hallucination warning markers.
// Returns the corrected summary text.
func (v *Verifier) FixCitations(ctx context.Context, summaryText string, allowedPMIDs map[string]struct{}) (string, error) {
	text := summaryText
	for attempt := 0; attempt < maxFixAttempts; attempt++ {
		unknown := unknownPMIDs(text, allowedPMIDs)
		if len(unknown) == 0 {
			return text, nil
		}

		// Fetch real metadata for each unknown PMID.
		realMeta := make(map[string]*pubmed.Article, len(unknown))
		notFound := make(map[string]bool)
		for _, pmid := range unknown {
			articles, err := v.fetchOne(ctx, pmid)
			if err != nil || len(articles) == 0 {
				notFound[pmid] = true
				continue
			}
			realMeta[pmid] = &articles[0]
		}

		// Immediately replace PMIDs that don't exist on PubMed.
		for pmid := range notFound {
			text = replacePMIDCitation(text, pmid, fmt.Sprintf("[⚠ hallucination warning: PMID %s — article not found on PubMed]", pmid))
			log.Printf("WARN: verifier: PMID %s does not exist on PubMed — replaced with warning", pmid)
		}

		// If everything was not-found, re-check for remaining unknowns.
		remaining := unknownPMIDsFromMap(text, allowedPMIDs, realMeta)
		if len(remaining) == 0 {
			continue
		}

		// Build correction prompt with real metadata.
		corrected, err := v.askLLMToCorrect(ctx, text, realMeta)
		if err != nil {
			log.Printf("WARN: verifier: LLM correction attempt %d failed: %v", attempt+1, err)
			continue
		}
		text = corrected
	}

	// Final pass: replace any still-unknown citations with hard warning markers.
	final := unknownPMIDs(text, allowedPMIDs)
	for _, pmid := range final {
		text = replacePMIDCitation(text, pmid, fmt.Sprintf(hallucinationMarker, pmid))
		log.Printf("WARN: verifier: PMID %s could not be verified after %d attempts — replaced with warning", pmid, maxFixAttempts)
	}
	return text, nil
}

// unknownPMIDs returns PMIDs cited in text that are not in allowed.
func unknownPMIDs(text string, allowed map[string]struct{}) []string {
	cited := ExtractPMIDs(text)
	var unknown []string
	for pmid := range cited {
		if _, ok := allowed[pmid]; !ok {
			unknown = append(unknown, pmid)
		}
	}
	return unknown
}

// unknownPMIDsFromMap returns PMIDs in text not in allowed AND not already fetched.
func unknownPMIDsFromMap(text string, allowed map[string]struct{}, fetched map[string]*pubmed.Article) []string {
	cited := ExtractPMIDs(text)
	var unknown []string
	for pmid := range cited {
		if _, ok := allowed[pmid]; ok {
			continue
		}
		if _, ok := fetched[pmid]; ok {
			continue
		}
		unknown = append(unknown, pmid)
	}
	return unknown
}

// fetchOne fetches a single PMID from NCBI and returns the parsed article slice.
func (v *Verifier) fetchOne(ctx context.Context, pmid string) ([]pubmed.Article, error) {
	xmlData, err := v.client.EFetch(ctx, []string{pmid})
	if err != nil {
		return nil, err
	}
	return pubmed.ParseEFetchXML(xmlData)
}

// askLLMToCorrect sends the summary + real metadata to the LLM and asks it to
// correct or remove any mismatched citations. Returns the corrected text.
func (v *Verifier) askLLMToCorrect(ctx context.Context, summaryText string, realMeta map[string]*pubmed.Article) (string, error) {
	var metaBlock strings.Builder
	for pmid, a := range realMeta {
		metaBlock.WriteString(fmt.Sprintf(
			"  [PMID:%s] real title: %q, journal: %q, date: %q\n",
			pmid, a.Title, a.Journal, a.PublicationDate,
		))
	}

	prompt := fmt.Sprintf(`You are a biomedical citation editor.
The following summary contains citation(s) whose metadata may not match the actual PubMed record.
The real PubMed metadata for each questionable PMID is listed below.

REAL METADATA:
%s

INSTRUCTIONS:
- If a citation's context in the summary is consistent with the real title and journal, keep it unchanged.
- If a citation's context is inconsistent (wrong topic, wrong finding), remove the citation tag entirely from that sentence.
- Do NOT add new citations or change any other part of the summary.
- Return ONLY the corrected summary text, no explanation.

SUMMARY:
%s`, metaBlock.String(), summaryText)

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText(prompt, "user"),
		},
	}

	var lastErr error
	for resp, err := range v.llm.GenerateContent(ctx, req, false) {
		if err != nil {
			lastErr = err
			continue
		}
		if resp.Content == nil {
			continue
		}
		for _, part := range resp.Content.Parts {
			if part.Text != "" {
				return strings.TrimSpace(part.Text), nil
			}
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("verifier: LLM returned no text")
}

// replacePMIDCitation replaces all occurrences of [PMID:N](url) and [PMID:N] in text
// with the given replacement string. The linked form is handled first to avoid a
// partial match that leaves the trailing (url) intact.
func replacePMIDCitation(text, pmid, replacement string) string {
	// Handle linked form [PMID:N](url) first.
	linked := fmt.Sprintf("[PMID:%s](", pmid)
	for {
		i := strings.Index(text, linked)
		if i < 0 {
			break
		}
		j := strings.Index(text[i:], ")")
		if j < 0 {
			break
		}
		text = text[:i] + replacement + text[i+j+1:]
	}
	// Handle plain form [PMID:N].
	text = strings.ReplaceAll(text, fmt.Sprintf("[PMID:%s]", pmid), replacement)
	return text
}
