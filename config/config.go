// Package config provides centralized, layered configuration for the PubMed agent.
//
// Two layers:
//   - AppConfig  — global infrastructure settings (NCBI email, PDF dirs, ports).
//     Loaded once at startup from a JSON file, with env vars taking precedence.
//   - UserConfig — per-user overridable settings (BYOK API key, model choices, PDF style).
//     Currently served by StaticUserConfigProvider (same config for everyone).
//     Replace with a database-backed UserConfigProvider for per-user customization.
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ─── Structs ───────────────────────────────────────────────────────────────────

// UserConfig holds settings that can differ per user.
// Empty string means "use the app default / built-in fallback".
// All fields are exported and JSON-tagged so they can be stored/loaded from a
// secrets manager, database, or config file.
type UserConfig struct {
	GoogleAPIKey      string `json:"google_api_key,omitempty"`
	ModelOrchestrator string `json:"model_orchestrator,omitempty"` // format: "provider:model-id"
	ModelValidator    string `json:"model_validator,omitempty"`
	ModelPlanner      string `json:"model_planner,omitempty"`
	ModelDefault      string `json:"model_default,omitempty"` // fallback when role-specific is empty
	PDFStylePrompt    string `json:"pdf_style_prompt,omitempty"`
}

// AppConfig holds global infrastructure settings and the default UserConfig
// (applied when no per-user override exists).
type AppConfig struct {
	NCBIEmail          string     `json:"ncbi_email"`
	PDFOutputDir       string     `json:"pdf_output_dir"`
	PDFDownloadBaseURL string     `json:"pdf_download_base_url"`
	PDFPort            string     `json:"pdf_port"`
	DefaultUser        UserConfig `json:"default_user"`
}

// ─── Loading ──────────────────────────────────────────────────────────────────

// hardcodedDefaults returns an AppConfig with sensible built-in defaults.
func hardcodedDefaults() AppConfig {
	return AppConfig{
		PDFOutputDir:       "./reports",
		PDFDownloadBaseURL: "http://localhost:8081",
		PDFPort:            "8081",
		DefaultUser: UserConfig{
			ModelDefault: "gemini:gemini-2.0-flash-latest",
		},
	}
}

// LoadAppConfig loads configuration in priority order (lowest → highest):
//  1. Hardcoded defaults
//  2. JSON file at path (skipped silently if the file does not exist)
//  3. Environment variables (always win)
//
// Returns a validated AppConfig or a descriptive error.
func LoadAppConfig(path string) (AppConfig, error) {
	cfg := hardcodedDefaults()

	if err := loadJSONFile(path, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("config: reading %q: %w", path, err)
	}

	overlayEnvVars(&cfg)

	if err := validate(cfg); err != nil {
		return AppConfig{}, fmt.Errorf("config: validation failed: %w", err)
	}

	return cfg, nil
}

// loadJSONFile unmarshals path into cfg. Missing file is silently ignored.
func loadJSONFile(path string, cfg *AppConfig) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, cfg)
}

// overlayEnvVars replaces any cfg field that has a corresponding non-empty env var.
func overlayEnvVars(cfg *AppConfig) {
	if v := os.Getenv("NCBI_EMAIL"); v != "" {
		cfg.NCBIEmail = v
	}
	if v := os.Getenv("PDF_OUTPUT_DIR"); v != "" {
		cfg.PDFOutputDir = v
	}
	if v := os.Getenv("PDF_DOWNLOAD_BASE_URL"); v != "" {
		cfg.PDFDownloadBaseURL = v
	}
	if v := os.Getenv("PDF_PORT"); v != "" {
		cfg.PDFPort = v
	}
	if v := os.Getenv("GOOGLE_API_KEY"); v != "" {
		cfg.DefaultUser.GoogleAPIKey = v
	}
	if v := os.Getenv("PUBMED_AGENT_MODEL_DEFAULT"); v != "" {
		cfg.DefaultUser.ModelDefault = v
	}
	if v := os.Getenv("PUBMED_AGENT_MODEL_ORCHESTRATOR"); v != "" {
		cfg.DefaultUser.ModelOrchestrator = v
	}
	if v := os.Getenv("PUBMED_AGENT_MODEL_VALIDATOR"); v != "" {
		cfg.DefaultUser.ModelValidator = v
	}
	if v := os.Getenv("PUBMED_AGENT_MODEL_PLANNER"); v != "" {
		cfg.DefaultUser.ModelPlanner = v
	}
	if v := os.Getenv("PDF_STYLE_PROMPT"); v != "" {
		cfg.DefaultUser.PDFStylePrompt = v
	}
}

// validate returns an error if required fields are missing or malformed.
func validate(cfg AppConfig) error {
	if cfg.NCBIEmail == "" {
		return errors.New("ncbi_email is required (set via config file or NCBI_EMAIL env var)")
	}
	if !strings.Contains(cfg.NCBIEmail, "@") {
		return fmt.Errorf("ncbi_email %q does not look like a valid email address", cfg.NCBIEmail)
	}
	if cfg.DefaultUser.GoogleAPIKey == "" {
		return errors.New("google_api_key is required (set via config file default_user.google_api_key or GOOGLE_API_KEY env var)")
	}
	return nil
}

// ─── Merging ──────────────────────────────────────────────────────────────────

// ModelForRole returns the model spec string for the given role name.
// Returns "" if the role has no explicit model set (caller falls back to ModelDefault).
func (u UserConfig) ModelForRole(role string) string {
	switch role {
	case "orchestrator":
		return u.ModelOrchestrator
	case "validator":
		return u.ModelValidator
	case "planner":
		return u.ModelPlanner
	}
	return ""
}

// MergeUserConfig returns a new UserConfig where non-empty fields in override
// take precedence over the corresponding fields in base.
// Pure function — neither argument is modified.
func MergeUserConfig(base, override UserConfig) UserConfig {
	out := base
	if override.GoogleAPIKey != "" {
		out.GoogleAPIKey = override.GoogleAPIKey
	}
	if override.ModelOrchestrator != "" {
		out.ModelOrchestrator = override.ModelOrchestrator
	}
	if override.ModelValidator != "" {
		out.ModelValidator = override.ModelValidator
	}
	if override.ModelPlanner != "" {
		out.ModelPlanner = override.ModelPlanner
	}
	if override.ModelDefault != "" {
		out.ModelDefault = override.ModelDefault
	}
	if override.PDFStylePrompt != "" {
		out.PDFStylePrompt = override.PDFStylePrompt
	}
	return out
}

// ─── Per-user provider ────────────────────────────────────────────────────────

// UserConfigProvider loads a fully-merged UserConfig for a given user ID.
// Implement this interface to add database or secrets-manager backed loading.
type UserConfigProvider interface {
	ForUser(ctx context.Context, userID string) (UserConfig, error)
}

// StaticUserConfigProvider returns the same UserConfig for every user.
// This is the current (v1) behaviour — replace with a database-backed
// implementation to support per-user BYOK and model preferences.
type StaticUserConfigProvider struct {
	cfg UserConfig
}

// NewStaticProvider creates a StaticUserConfigProvider from a base UserConfig.
func NewStaticProvider(cfg UserConfig) *StaticUserConfigProvider {
	return &StaticUserConfigProvider{cfg: cfg}
}

// ForUser implements UserConfigProvider.
func (p *StaticUserConfigProvider) ForUser(_ context.Context, _ string) (UserConfig, error) {
	return p.cfg, nil
}
