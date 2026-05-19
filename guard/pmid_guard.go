// Package guard provides post-processing utilities for agent output.
package guard

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"google.golang.org/genai"
)

// pmidPattern matches [PMID:12345678] and [PMID:12345678](url) in text.
// The optional (url) group is non-capturing so the PMID is always capture group 1.
var pmidPattern = regexp.MustCompile(`\[PMID:(\d+)\](?:\([^)]*\))?`)

// StripHallucinatedPMIDs removes any [PMID:N] citations from content whose N
// is not in the allowedPMIDs set. Stripped citations are logged as warnings.
// Returns the sanitized content, or the original if no changes are needed.
func StripHallucinatedPMIDs(content *genai.Content, allowedPMIDs map[string]struct{}) *genai.Content {
	if content == nil || len(allowedPMIDs) == 0 {
		return content
	}

	changed := false
	newParts := make([]*genai.Part, 0, len(content.Parts))
	for _, part := range content.Parts {
		if part.Text == "" {
			newParts = append(newParts, part)
			continue
		}
		sanitized := pmidPattern.ReplaceAllStringFunc(part.Text, func(match string) string {
			subs := pmidPattern.FindStringSubmatch(match)
			if len(subs) < 2 {
				return match
			}
			pmid := subs[1]
			if _, ok := allowedPMIDs[pmid]; ok {
				return match
			}
			log.Printf("WARN: pmid_guard: stripped hallucinated citation %s", match)
			changed = true
			return fmt.Sprintf("[citation removed: PMID %s not in fetched set]", pmid)
		})
		if sanitized != part.Text {
			newParts = append(newParts, &genai.Part{Text: sanitized})
		} else {
			newParts = append(newParts, part)
		}
	}

	if !changed {
		return content
	}

	return &genai.Content{
		Role:  content.Role,
		Parts: newParts,
	}
}

// ExtractPMIDs returns the set of PMIDs mentioned in the text as [PMID:N] citations.
func ExtractPMIDs(text string) map[string]struct{} {
	matches := pmidPattern.FindAllStringSubmatch(text, -1)
	result := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			result[m[1]] = struct{}{}
		}
	}
	return result
}

// PMIDSet converts a slice of PMID strings to a lookup set.
func PMIDSet(pmids []string) map[string]struct{} {
	s := make(map[string]struct{}, len(pmids))
	for _, p := range pmids {
		s[strings.TrimSpace(p)] = struct{}{}
	}
	return s
}
