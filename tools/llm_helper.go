package tools

import (
	"context"
	"fmt"

	"github.com/alimoeeny/pubmed_search_agent/pubmed"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// toStringSliceAny safely converts any session-state value to []string.
func toStringSliceAny(v any) []string {
	if v == nil {
		return nil
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	return nil
}

// deduplicateStrings returns a new slice with duplicate entries removed,
// preserving order of first occurrence.
func deduplicateStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// toArticleMapAny safely converts any session-state value to map[string]pubmed.Article.
// Returns an empty (non-nil) map on nil or wrong type so callers can always write into it.
func toArticleMapAny(v any) map[string]pubmed.Article {
	if v == nil {
		return make(map[string]pubmed.Article)
	}
	if m, ok := v.(map[string]pubmed.Article); ok {
		return m
	}
	return make(map[string]pubmed.Article)
}

// ncbiEmailFromState reads the authenticated user's email from session state.
// Falls back to fallback (the pubmed.Client's built-in email) when the key is
// absent — e.g. in ADK web UI mode where no session state is seeded by the server.
func ncbiEmailFromState(ctx tool.Context, fallback string) string {
	v, _ := ctx.State().Get(SessionKeyNCBIEmail)
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

// generateText calls the LLM and returns the first text part from the response.
// ctx may be any context.Context — tool.Context satisfies this via embedding.
func generateText(ctx context.Context, llm model.LLM, req *model.LLMRequest) (string, error) {
	var lastErr error
	for resp, err := range llm.GenerateContent(ctx, req, false) {
		if err != nil {
			lastErr = err
			continue
		}
		if resp.Content == nil {
			continue
		}
		for _, part := range resp.Content.Parts {
			if part.Text != "" {
				return part.Text, nil
			}
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("model returned no text content")
}
