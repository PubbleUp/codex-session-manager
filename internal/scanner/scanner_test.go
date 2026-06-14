package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
)

func TestScanFindsNestedSessions(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "12")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeSession(t, filepath.Join(sessionDir, "rollout-2026-06-12T17-01-19-019ebb10-7a0e-7d70-95e8-c020b75687d8.jsonl"), "/tmp/project")

	cfg := config.Config{
		CodexHome:                 codexHome,
		ToolHome:                  filepath.Join(root, "tool"),
		BackupDir:                 filepath.Join(root, "tool", "backups"),
		RemovedDir:                filepath.Join(root, "tool", "removed"),
		IncludeArchived:           true,
		IncludeBackups:            true,
		IncludeRemoved:            true,
		RequireBackupBeforeRemove: true,
	}
	inventory, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Sessions) != 1 {
		t.Fatalf("unexpected session count: %d", len(inventory.Sessions))
	}
	if inventory.Sessions[0].Status != domain.SessionStatusVisible {
		t.Fatalf("unexpected status: %s", inventory.Sessions[0].Status)
	}
	if len(inventory.Projects) != 1 {
		t.Fatalf("unexpected project count: %d", len(inventory.Projects))
	}
}

func TestScanSkipsHiddenProjects(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "12")
	hiddenProject := filepath.Join(root, "hidden")
	visibleProject := filepath.Join(root, "visible")
	writeSessionWithID(t, filepath.Join(sessionDir, "rollout-2026-06-12T17-01-19-019ebb10-7a0e-7d70-95e8-c020b75687d8.jsonl"), hiddenProject, "019ebb10-7a0e-7d70-95e8-c020b75687d8")
	writeSessionWithID(t, filepath.Join(sessionDir, "rollout-2026-06-12T17-01-20-019ebb10-7a0e-7d70-95e8-c020b75687d9.jsonl"), visibleProject, "019ebb10-7a0e-7d70-95e8-c020b75687d9")

	cfg := config.Config{
		CodexHome:      codexHome,
		ToolHome:       filepath.Join(root, "tool"),
		BackupDir:      filepath.Join(root, "tool", "backups"),
		RemovedDir:     filepath.Join(root, "tool", "removed"),
		HiddenProjects: []string{hiddenProject},
	}
	inventory, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Projects) != 1 || inventory.Projects[0].CWD != visibleProject {
		t.Fatalf("expected only visible project, got %+v", inventory.Projects)
	}
	if len(inventory.Sessions) != 1 || inventory.Sessions[0].CWD != visibleProject {
		t.Fatalf("expected only visible session, got %+v", inventory.Sessions)
	}
}

func writeSession(t *testing.T, path string, cwd string) {
	t.Helper()
	writeSessionWithID(t, path, cwd, "019ebb10-7a0e-7d70-95e8-c020b75687d8")
}

func writeSessionWithID(t *testing.T, path string, cwd string, sessionID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-06-12T17:01:19+08:00","type":"session_meta","payload":{"id":"` + sessionID + `","timestamp":"2026-06-12T17:01:19+08:00","cwd":"` + cwd + `","originator":"codex_cli_rs","cli_version":"0.136.0","source":"cli","model_provider":"openai"}}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
