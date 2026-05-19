package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

// ModelRole identifies which component of the agent a model is serving.
type ModelRole string

const (
	RoleOrchestrator ModelRole = "orchestrator"
	RoleValidator    ModelRole = "validator"
	RolePlanner      ModelRole = "planner"
)

// Provider identifies the LLM provider.
type Provider string

const (
	ProviderGemini    Provider = "gemini"
	ProviderAnthropic Provider = "anthropic" // not wired in v1
)

// ModelSpec is a parsed `provider:model-id` specification.
type ModelSpec struct {
	Provider Provider
	ModelID  string
}

// ErrUnsupportedProvider is returned when a provider has no wired constructor.
var ErrUnsupportedProvider = errors.New("unsupported LLM provider")

// envKeyForRole returns the primary env var name for a given role.
func envKeyForRole(role ModelRole) string {
	return "PUBMED_AGENT_MODEL_" + strings.ToUpper(string(role))
}

// ParseModelSpec parses a "provider:model-id" string.
func ParseModelSpec(s string) (ModelSpec, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ModelSpec{}, fmt.Errorf("invalid model spec %q: expected format provider:model-id", s)
	}
	return ModelSpec{Provider: Provider(parts[0]), ModelID: parts[1]}, nil
}

// resolveSpec resolves the ModelSpec for a role using the env-var priority:
// role-specific → PUBMED_AGENT_MODEL_DEFAULT → hard-coded default.
func resolveSpec(role ModelRole) (ModelSpec, error) {
	candidates := []string{
		os.Getenv(envKeyForRole(role)),
		os.Getenv("PUBMED_AGENT_MODEL_DEFAULT"),
		"gemini:gemini-flash-latest",
	}
	for _, c := range candidates {
		if c != "" {
			return ParseModelSpec(c)
		}
	}
	// Unreachable — the hard-coded fallback is always non-empty.
	return ParseModelSpec("gemini:gemini-flash-latest")
}

// ModelFor returns a model.LLM for the given role, respecting env-var overrides.
func ModelFor(ctx context.Context, role ModelRole) (model.LLM, error) {
	spec, err := resolveSpec(role)
	if err != nil {
		return nil, fmt.Errorf("role %q: %w", role, err)
	}

	switch spec.Provider {
	case ProviderGemini:
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY is required for Gemini provider (role %q)", role)
		}
		m, err := gemini.NewModel(ctx, spec.ModelID, &genai.ClientConfig{APIKey: apiKey})
		if err != nil {
			return nil, fmt.Errorf("role %q: creating Gemini model %q: %w", role, spec.ModelID, err)
		}
		return m, nil

	case ProviderAnthropic:
		return nil, fmt.Errorf("role %q: provider %q: %w (not wired in v1)", role, spec.Provider, ErrUnsupportedProvider)

	default:
		return nil, fmt.Errorf("role %q: provider %q: %w", role, spec.Provider, ErrUnsupportedProvider)
	}
}
