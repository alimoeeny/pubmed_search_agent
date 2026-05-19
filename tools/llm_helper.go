package tools

import (
	"context"
	"fmt"

	"google.golang.org/adk/model"
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
