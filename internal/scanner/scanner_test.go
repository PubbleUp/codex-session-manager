package scanner

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	_ "modernc.org/sqlite"
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

func TestScanMarksSessionInactiveWhenMissingFromThreadIndex(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "12")
	sessionPath := filepath.Join(sessionDir, "rollout-2026-06-12T17-01-19-019ebb10-7a0e-7d70-95e8-c020b75687d8.jsonl")
	writeSession(t, sessionPath, "/tmp/project")
	writeThreadIndex(t, codexHome)

	inventory, err := Scan(testConfig(root, codexHome))
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Sessions) != 1 {
		t.Fatalf("unexpected session count: %d", len(inventory.Sessions))
	}
	session := inventory.Sessions[0]
	if session.Source != domain.SessionSourceInactive || session.Status != domain.SessionStatusInactive {
		t.Fatalf("expected inactive session, got source=%s status=%s", session.Source, session.Status)
	}
	if inventory.Projects[0].VisibleCount != 0 || inventory.Projects[0].RecoverableCount != 1 {
		t.Fatalf("unexpected project stats: %+v", inventory.Projects[0])
	}
}

func TestScanKeepsSessionVisibleWhenThreadIndexMatches(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "12")
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sessionPath := filepath.Join(sessionDir, "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	writeSessionWithID(t, sessionPath, "/tmp/project", sessionID)
	writeThreadIndex(t, codexHome, threadFixture{ID: sessionID, RolloutPath: sessionPath})

	inventory, err := Scan(testConfig(root, codexHome))
	if err != nil {
		t.Fatal(err)
	}
	session := inventory.Sessions[0]
	if session.Source != domain.SessionSourceVisible || session.Status != domain.SessionStatusVisible {
		t.Fatalf("expected visible session, got source=%s status=%s", session.Source, session.Status)
	}
	if inventory.Projects[0].VisibleCount != 1 {
		t.Fatalf("unexpected visible count: %+v", inventory.Projects[0])
	}
}

func TestScanMarksSessionInactiveWhenProviderDiffers(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "12")
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sessionPath := filepath.Join(sessionDir, "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	writeSessionWithID(t, sessionPath, "/tmp/project", sessionID)
	writeThreadIndex(t, codexHome, threadFixture{ID: sessionID, RolloutPath: sessionPath, ModelProvider: "old-provider"})

	cfg := testConfig(root, codexHome)
	cfg.ModelProvider = "current-provider"
	inventory, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	session := inventory.Sessions[0]
	if session.Source != domain.SessionSourceInactive || session.Status != domain.SessionStatusInactive {
		t.Fatalf("expected inactive session, got source=%s status=%s", session.Source, session.Status)
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

func testConfig(root string, codexHome string) config.Config {
	return config.Config{
		CodexHome:                 codexHome,
		ToolHome:                  filepath.Join(root, "tool"),
		BackupDir:                 filepath.Join(root, "tool", "backups"),
		RemovedDir:                filepath.Join(root, "tool", "removed"),
		IncludeArchived:           true,
		IncludeBackups:            true,
		IncludeRemoved:            true,
		RequireBackupBeforeRemove: true,
	}
}

type threadFixture struct {
	ID            string
	RolloutPath   string
	ModelProvider string
	Archived      int
}

func writeThreadIndex(t *testing.T, codexHome string, threads ...threadFixture) {
	t.Helper()
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(codexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE threads (
			id TEXT PRIMARY KEY,
			rollout_path TEXT NOT NULL,
			model_provider TEXT NOT NULL,
			archived INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, thread := range threads {
		provider := thread.ModelProvider
		if provider == "" {
			provider = "openai"
		}
		_, err := db.Exec(
			`INSERT INTO threads (id, rollout_path, model_provider, archived) VALUES (?, ?, ?, ?)`,
			thread.ID,
			thread.RolloutPath,
			provider,
			thread.Archived,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
}
