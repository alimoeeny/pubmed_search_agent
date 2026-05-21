package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"github.com/alimoeeny/pubmed_search_agent/config"
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

// ParseModelSpec parses a "provider:model-id" string.
func ParseModelSpec(s string) (ModelSpec, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ModelSpec{}, fmt.Errorf("invalid model spec %q: expected format provider:model-id", s)
	}
	return ModelSpec{Provider: Provider(parts[0]), ModelID: parts[1]}, nil
}

// resolveSpec returns the ModelSpec for role using cfg, falling back to built-in default.
// Priority: role-specific model → ModelDefault → "gemini:gemini-2.0-flash-latest".
func resolveSpec(role ModelRole, cfg config.UserConfig) (ModelSpec, error) {
	candidates := []string{
		cfg.ModelForRole(string(role)),
		cfg.ModelDefault,
		"gemini:gemini-2.0-flash-latest",
	}
	for _, c := range candidates {
		if c != "" {
			return ParseModelSpec(c)
		}
	}
	// Unreachable — the built-in fallback is always non-empty.
	return ParseModelSpec("gemini:gemini-2.0-flash-latest")
}

// ModelFor returns a model.LLM for the given role using the provided UserConfig.
// All configuration is read from cfg — no os.Getenv calls.
func ModelFor(ctx context.Context, role ModelRole, cfg config.UserConfig) (model.LLM, error) {
	spec, err := resolveSpec(role, cfg)
	if err != nil {
		return nil, fmt.Errorf("role %q: %w", role, err)
	}

	switch spec.Provider {
	case ProviderGemini:
		if cfg.GoogleAPIKey == "" {
			return nil, fmt.Errorf("role %q: GoogleAPIKey is empty in UserConfig", role)
		}
		m, err := gemini.NewModel(ctx, spec.ModelID, &genai.ClientConfig{APIKey: cfg.GoogleAPIKey})
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
