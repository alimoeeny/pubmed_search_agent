package main

import (
	"context"
	"strings"
	"testing"
)

func TestParseModelSpec(t *testing.T) {
	tests := []struct {
		input    string
		wantProv Provider
		wantID   string
		wantErr  bool
	}{
		{"gemini:gemini-flash-latest", ProviderGemini, "gemini-flash-latest", false},
		{"gemini:gemini-2.5-pro", ProviderGemini, "gemini-2.5-pro", false},
		{"anthropic:claude-sonnet-4", ProviderAnthropic, "claude-sonnet-4", false},
		{"", "", "", true},
		{"nocolon", "", "", true},
		{":model", "", "", true},
		{"provider:", "", "", true},
	}

	for _, tc := range tests {
		spec, err := ParseModelSpec(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseModelSpec(%q): expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseModelSpec(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if spec.Provider != tc.wantProv {
			t.Errorf("ParseModelSpec(%q).Provider = %q, want %q", tc.input, spec.Provider, tc.wantProv)
		}
		if spec.ModelID != tc.wantID {
			t.Errorf("ParseModelSpec(%q).ModelID = %q, want %q", tc.input, spec.ModelID, tc.wantID)
		}
	}
}

func TestResolveSpecDefaults(t *testing.T) {
	t.Setenv("PUBMED_AGENT_MODEL_ORCHESTRATOR", "")
	t.Setenv("PUBMED_AGENT_MODEL_DEFAULT", "")

	spec, err := resolveSpec(RoleOrchestrator)
	if err != nil {
		t.Fatalf("resolveSpec: unexpected error: %v", err)
	}
	if spec.Provider != ProviderGemini {
		t.Errorf("default provider = %q, want %q", spec.Provider, ProviderGemini)
	}
	if spec.ModelID != "gemini-flash-latest" {
		t.Errorf("default model = %q, want %q", spec.ModelID, "gemini-flash-latest")
	}
}

func TestResolveSpecRoleSpecificOverride(t *testing.T) {
	t.Setenv("PUBMED_AGENT_MODEL_PLANNER", "gemini:gemini-2.5-pro")
	t.Setenv("PUBMED_AGENT_MODEL_DEFAULT", "gemini:gemini-flash-latest")

	spec, err := resolveSpec(RolePlanner)
	if err != nil {
		t.Fatalf("resolveSpec: unexpected error: %v", err)
	}
	if spec.ModelID != "gemini-2.5-pro" {
		t.Errorf("planner model = %q, want gemini-2.5-pro", spec.ModelID)
	}
}

func TestResolveSpecDefaultFallback(t *testing.T) {
	t.Setenv("PUBMED_AGENT_MODEL_VALIDATOR", "")
	t.Setenv("PUBMED_AGENT_MODEL_DEFAULT", "gemini:gemini-2.5-pro")

	spec, err := resolveSpec(RoleValidator)
	if err != nil {
		t.Fatalf("resolveSpec: unexpected error: %v", err)
	}
	if spec.ModelID != "gemini-2.5-pro" {
		t.Errorf("validator fallback model = %q, want gemini-2.5-pro", spec.ModelID)
	}
}

func TestModelForUnsupportedProvider(t *testing.T) {
	t.Setenv("PUBMED_AGENT_MODEL_ORCHESTRATOR", "anthropic:claude-sonnet-4")

	_, err := ModelFor(context.Background(), RoleOrchestrator)
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	if !isUnsupportedProviderErr(err) {
		t.Errorf("expected ErrUnsupportedProvider in error chain, got: %v", err)
	}
}

func isUnsupportedProviderErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unsupported LLM provider")
}
