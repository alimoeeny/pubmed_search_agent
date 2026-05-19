package tools

import (
	"fmt"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
)

// generateText calls the LLM and returns the first text part from the response.
func generateText(ctx tool.Context, llm model.LLM, req *model.LLMRequest) (string, error) {
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
