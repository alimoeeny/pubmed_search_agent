package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// ValidateArgs is the input for the validate_question tool.
type ValidateArgs struct {
	Question string `json:"question"`
}

// ValidateResult is the output for the validate_question tool.
type ValidateResult struct {
	IsResearchable       bool     `json:"is_researchable"`
	Reason               string   `json:"reason"`
	SuggestedRefinements []string `json:"suggested_refinements,omitempty"`
}

const validatePrompt = `You are a biomedical research expert.
Determine whether the following question is suitable for a PubMed literature search.
A question is suitable if it relates to biomedical research, clinical medicine, public health,
pharmacology, or related life sciences that would produce peer-reviewed articles in PubMed.
A question is NOT suitable if it is too vague (e.g. "tell me about cancer"),
non-biomedical (e.g. "what is the weather"), or unanswerable by literature review.

Respond ONLY with valid JSON matching this schema:
{
  "is_researchable": <bool>,
  "reason": "<one sentence explanation>",
  "suggested_refinements": ["<refinement 1>", "<refinement 2>"]  // only if is_researchable is false
}

Question: %s`

// NewValidateQuestionTool creates the validate_question ADK tool.
func NewValidateQuestionTool(llm model.LLM) (tool.Tool, error) {
	handler := func(ctx tool.Context, args ValidateArgs) (ValidateResult, error) {
		prompt := fmt.Sprintf(validatePrompt, args.Question)
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
			return ValidateResult{}, fmt.Errorf("validate_question: %w", err)
		}

		var result ValidateResult
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &result); err != nil {
			return ValidateResult{}, fmt.Errorf("validate_question: parsing model response: %w (raw: %s)", err, text)
		}
		return result, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "validate_question",
		Description: "Validate whether a research question is suitable for a PubMed search. Returns is_researchable and suggested refinements if not.",
	}, handler)
}
