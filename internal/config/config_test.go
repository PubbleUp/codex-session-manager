package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHideAndUnhideProjectUpdatesConfigFile(t *testing.T) {
	root := t.TempDir()
	cfg := Config{ToolHome: filepath.Join(root, "tool")}
	project := filepath.Join(root, "project")

	if err := HideProject(cfg, project); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ConfigPath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hidden_projects") || !strings.Contains(string(data), project) {
		t.Fatalf("expected hidden project written, got %s", data)
	}

	cfg.HiddenProjects = []string{project}
	if err := UnhideProject(cfg, project); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(ConfigPath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), project) {
		t.Fatalf("expected hidden project removed, got %s", data)
	}
}

func TestReadCodexModelProvider(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.toml")
	content := `
model = "gpt-5"
model_provider = "codex_local_access"

[model_providers.codex_local_access]
base_url = "http://127.0.0.1:3390"
`
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got := readCodexModelProvider(configPath)
	if got != "codex_local_access" {
		t.Fatalf("expected provider codex_local_access, got %q", got)
	}
}

func TestLoadReadsProviderFromConfiguredCodexHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	codexHome := filepath.Join(root, "custom-codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(`model_provider = "custom_provider"`), 0o600); err != nil {
		t.Fatal(err)
	}
	toolHome := filepath.Join(root, ".codex-session-manager")
	if err := os.MkdirAll(toolHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolHome, "config.toml"), []byte(`codex_home = "`+codexHome+`"`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != "custom_provider" {
		t.Fatalf("expected provider from configured codex home, got %q", cfg.ModelProvider)
	}
}

func TestLoadAllowsToolProviderOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	codexHome := filepath.Join(root, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte(`model_provider = "codex_provider"`), 0o600); err != nil {
		t.Fatal(err)
	}
	toolHome := filepath.Join(root, ".codex-session-manager")
	if err := os.MkdirAll(toolHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(toolHome, "config.toml"), []byte(`model_provider = "tool_provider"`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelProvider != "tool_provider" {
		t.Fatalf("expected tool provider override, got %q", cfg.ModelProvider)
	}
}
