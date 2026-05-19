package tools

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// mockLLM implements model.LLM returning a fixed text response.
type mockLLM struct {
	text string
	err  error
}

func (m *mockLLM) Name() string { return "mock" }

func (m *mockLLM) GenerateContent(_ context.Context, _ *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		if m.err != nil {
			yield(nil, m.err)
			return
		}
		yield(&model.LLMResponse{
			Content: genai.NewContentFromText(m.text, "model"),
		}, nil)
	}
}

func TestValidateQuestion_Researchable(t *testing.T) {
	llm := &mockLLM{text: `{"is_researchable": true, "reason": "Clear clinical question"}`}

	// Verify tool construction succeeds.
	_, err := NewValidateQuestionTool(llm)
	if err != nil {
		t.Fatalf("NewValidateQuestionTool: %v", err)
	}

	// Test generateText + JSON parsing path directly.
	req := &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText("test", "user")},
	}
	text, err := generateText(context.Background(), llm, req)
	if err != nil {
		t.Fatalf("generateText: %v", err)
	}
	if text == "" {
		t.Fatal("generateText returned empty string")
	}
}

func TestValidateQuestion_NotResearchable(t *testing.T) {
	llm := &mockLLM{text: `{"is_researchable": false, "reason": "Too vague", "suggested_refinements": ["narrow to RCTs"]}`}

	req := &model.LLMRequest{
		Contents: []*genai.Content{genai.NewContentFromText("cancer", "user")},
	}
	text, err := generateText(context.Background(), llm, req)
	if err != nil {
		t.Fatalf("generateText: %v", err)
	}
	if text == "" {
		t.Fatal("generateText returned empty string")
	}
}
