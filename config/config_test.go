package config_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alimoeeny/pubmed_search_agent/config"
)

// writeConfig writes an AppConfig as JSON to a temp file and returns its path.
func writeConfig(t *testing.T, cfg config.AppConfig) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// setEnv sets env vars for the duration of the test and restores them on cleanup.
func setEnv(t *testing.T, pairs ...string) {
	t.Helper()
	for i := 0; i < len(pairs)-1; i += 2 {
		key, val := pairs[i], pairs[i+1]
		old, existed := os.LookupEnv(key)
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
		os.Setenv(key, val)
	}
}

func TestLoadAppConfig_Defaults(t *testing.T) {
	setEnv(t,
		"NCBI_EMAIL", "test@example.com",
		"GOOGLE_API_KEY", "test-key",
	)
	// No config file — should load defaults + env vars.
	cfg, err := config.LoadAppConfig("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("LoadAppConfig failed: %v", err)
	}
	if cfg.NCBIEmail != "test@example.com" {
		t.Errorf("NCBIEmail = %q, want test@example.com", cfg.NCBIEmail)
	}
	if cfg.DefaultUser.GoogleAPIKey != "test-key" {
		t.Errorf("GoogleAPIKey = %q, want test-key", cfg.DefaultUser.GoogleAPIKey)
	}
	if cfg.PDFOutputDir != "./reports" {
		t.Errorf("PDFOutputDir = %q, want ./reports", cfg.PDFOutputDir)
	}
	if cfg.PDFPort != "8081" {
		t.Errorf("PDFPort = %q, want 8081", cfg.PDFPort)
	}
}

func TestLoadAppConfig_JSONFileLoaded(t *testing.T) {
	setEnv(t,
		"NCBI_EMAIL", "",
		"GOOGLE_API_KEY", "",
	)
	// Unset env vars that would interfere.
	os.Unsetenv("NCBI_EMAIL")
	os.Unsetenv("GOOGLE_API_KEY")

	src := config.AppConfig{
		NCBIEmail:          "file@example.com",
		PDFOutputDir:       "./custom-reports",
		PDFDownloadBaseURL: "https://example.com",
		PDFPort:            "9090",
		DefaultUser: config.UserConfig{
			GoogleAPIKey:      "file-api-key",
			ModelOrchestrator: "gemini:gemini-2.5-pro-latest",
			ModelDefault:      "gemini:gemini-3.5-flash-latest",
		},
	}
	path := writeConfig(t, src)

	cfg, err := config.LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig failed: %v", err)
	}
	if cfg.NCBIEmail != "file@example.com" {
		t.Errorf("NCBIEmail = %q", cfg.NCBIEmail)
	}
	if cfg.DefaultUser.ModelOrchestrator != "gemini:gemini-2.5-pro-latest" {
		t.Errorf("ModelOrchestrator = %q", cfg.DefaultUser.ModelOrchestrator)
	}
	if cfg.PDFPort != "9090" {
		t.Errorf("PDFPort = %q, want 9090", cfg.PDFPort)
	}
}

func TestLoadAppConfig_EnvVarsOverrideFile(t *testing.T) {
	src := config.AppConfig{
		NCBIEmail: "file@example.com",
		DefaultUser: config.UserConfig{
			GoogleAPIKey:      "file-api-key",
			ModelOrchestrator: "gemini:gemini-flash-latest",
		},
	}
	path := writeConfig(t, src)

	setEnv(t,
		"NCBI_EMAIL", "env@example.com",
		"GOOGLE_API_KEY", "env-api-key",
		"PUBMED_AGENT_MODEL_ORCHESTRATOR", "gemini:gemini-2.5-pro-latest",
	)

	cfg, err := config.LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig failed: %v", err)
	}
	if cfg.NCBIEmail != "env@example.com" {
		t.Errorf("env var NCBI_EMAIL should override file: got %q", cfg.NCBIEmail)
	}
	if cfg.DefaultUser.GoogleAPIKey != "env-api-key" {
		t.Errorf("env var GOOGLE_API_KEY should override file: got %q", cfg.DefaultUser.GoogleAPIKey)
	}
	if cfg.DefaultUser.ModelOrchestrator != "gemini:gemini-2.5-pro-latest" {
		t.Errorf("env var MODEL_ORCHESTRATOR should override file: got %q", cfg.DefaultUser.ModelOrchestrator)
	}
}

func TestLoadAppConfig_ValidationFailsMissingEmail(t *testing.T) {
	setEnv(t, "GOOGLE_API_KEY", "test-key")
	os.Unsetenv("NCBI_EMAIL")

	_, err := config.LoadAppConfig("/nonexistent/config.json")
	if err == nil {
		t.Error("expected error for missing NCBI_EMAIL, got nil")
	}
}

func TestLoadAppConfig_ValidationFailsMissingAPIKey(t *testing.T) {
	setEnv(t, "NCBI_EMAIL", "test@example.com")
	os.Unsetenv("GOOGLE_API_KEY")

	_, err := config.LoadAppConfig("/nonexistent/config.json")
	if err == nil {
		t.Error("expected error for missing GOOGLE_API_KEY, got nil")
	}
}

func TestMergeUserConfig(t *testing.T) {
	base := config.UserConfig{
		GoogleAPIKey:      "base-key",
		ModelOrchestrator: "gemini:flash",
		ModelDefault:      "gemini:flash",
	}
	override := config.UserConfig{
		GoogleAPIKey:      "user-key",
		ModelOrchestrator: "gemini:pro",
		PDFStylePrompt:    "dark minimal",
	}

	merged := config.MergeUserConfig(base, override)

	if merged.GoogleAPIKey != "user-key" {
		t.Errorf("GoogleAPIKey = %q, want user-key", merged.GoogleAPIKey)
	}
	if merged.ModelOrchestrator != "gemini:pro" {
		t.Errorf("ModelOrchestrator = %q, want gemini:pro", merged.ModelOrchestrator)
	}
	if merged.ModelDefault != "gemini:flash" {
		t.Errorf("ModelDefault should be inherited from base: got %q", merged.ModelDefault)
	}
	if merged.PDFStylePrompt != "dark minimal" {
		t.Errorf("PDFStylePrompt = %q, want dark minimal", merged.PDFStylePrompt)
	}
	// base must not be mutated
	if base.PDFStylePrompt != "" {
		t.Error("MergeUserConfig must not mutate base")
	}
}

func TestStaticUserConfigProvider(t *testing.T) {
	cfg := config.UserConfig{GoogleAPIKey: "static-key", ModelDefault: "gemini:flash"}
	provider := config.NewStaticProvider(cfg)

	got, err := provider.ForUser(context.Background(), "any-user-id")
	if err != nil {
		t.Fatalf("ForUser failed: %v", err)
	}
	if got.GoogleAPIKey != "static-key" {
		t.Errorf("GoogleAPIKey = %q", got.GoogleAPIKey)
	}
	if got.ModelDefault != "gemini:flash" {
		t.Errorf("ModelDefault = %q", got.ModelDefault)
	}
}
