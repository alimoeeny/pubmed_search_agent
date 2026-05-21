package tools_test

import (
	"testing"

	"github.com/alimoeeny/pubmed_search_agent/storage"
	agenttools "github.com/alimoeeny/pubmed_search_agent/tools"
)

func TestNewGeneratePDFTool_Construction(t *testing.T) {
	tool, err := agenttools.NewGeneratePDFTool(agenttools.PDFToolConfig{
		Backend: &storage.LocalBackend{Dir: t.TempDir(), BaseURL: "http://localhost:8081"},
	})
	if err != nil {
		t.Fatalf("NewGeneratePDFTool failed: %v", err)
	}
	if tool.Name() != "generate_pdf" {
		t.Errorf("unexpected tool name: %s", tool.Name())
	}
}

func TestGeneratePDFArgs_EmptyQuestionRejected(t *testing.T) {
	// Argument validation (empty question/summary) happens inside the tool handler at
	// invocation time and is exercised by integration tests. This is a placeholder.
	t.Skip("argument validation is exercised in integration tests")
}
