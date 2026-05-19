package tools

import (
	"testing"
)

func TestNewAskUserTool_Construction(t *testing.T) {
	tool, err := NewAskUserTool()
	if err != nil {
		t.Fatalf("NewAskUserTool: %v", err)
	}
	if tool == nil {
		t.Fatal("NewAskUserTool returned nil tool")
	}
}

func TestAskUserArgs_Serialization(t *testing.T) {
	args := AskUserArgs{
		Question: "What population?",
		Options:  []string{"adults", "children", "both"},
	}
	if args.Question == "" {
		t.Error("Question should not be empty")
	}
	if len(args.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(args.Options))
	}
}

func TestAskUserPending_StatusField(t *testing.T) {
	pending := AskUserPending{
		Status:   "pending",
		Question: "Are you sure?",
	}
	if pending.Status != "pending" {
		t.Errorf("expected status=pending, got %q", pending.Status)
	}
}
