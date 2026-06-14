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
