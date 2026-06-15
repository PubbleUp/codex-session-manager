package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
)

func TestFindVisibleSessionPrefersVisibleRecord(t *testing.T) {
	id := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sessions := []domain.SessionRecord{
		{ID: id, Status: domain.SessionStatusRecoverable, Source: domain.SessionSourceBackup, FilePath: "backup"},
		{ID: id, Status: domain.SessionStatusRemoved, Source: domain.SessionSourceRemoved, FilePath: "removed"},
		{ID: id, Status: domain.SessionStatusVisible, Source: domain.SessionSourceVisible, FilePath: "visible"},
	}

	session, ok := FindVisibleSession(sessions, id[:8])
	if !ok {
		t.Fatal("expected visible session")
	}
	if session.FilePath != "visible" {
		t.Fatalf("expected visible record, got %s", session.FilePath)
	}
}

func TestRemoveInvisibleSessionDoesNotRemoveVisibleSameID(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	codexHome := filepath.Join(root, ".codex")
	oldHome := filepath.Join(root, ".codex-old")
	backupDir := filepath.Join(root, "tool", "backups")
	removedDir := filepath.Join(root, "tool", "removed")
	toolHome := filepath.Join(root, "tool")
	visiblePath := filepath.Join(codexHome, "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	invisiblePath := filepath.Join(oldHome, "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	writeAppSessionFile(t, visiblePath, sessionID, "/tmp/project")
	writeAppSessionFile(t, invisiblePath, sessionID, "/tmp/project")
	a := App{Config: config.Config{
		CodexHome:                 codexHome,
		ToolHome:                  toolHome,
		BackupDir:                 backupDir,
		RemovedDir:                removedDir,
		OldCodexHomes:             []string{oldHome},
		RequireBackupBeforeRemove: true,
	}}
	session := domain.SessionRecord{
		ID:        sessionID,
		CWD:       "/tmp/project",
		FilePath:  invisiblePath,
		Source:    domain.SessionSourceOldHome,
		Status:    domain.SessionStatusRecoverable,
		CodexHome: oldHome,
	}

	result, err := a.RemoveSession(session)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target != filepath.Join(removedDir, sessionID, "session.jsonl") {
		t.Fatalf("expected invisible session moved to removed dir, got %s", result.Target)
	}
	if _, err := os.Stat(visiblePath); err != nil {
		t.Fatalf("expected visible same-id session kept, stat err=%v", err)
	}
	if _, err := os.Stat(invisiblePath); !os.IsNotExist(err) {
		t.Fatalf("expected invisible source moved, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, sessionID)); !os.IsNotExist(err) {
		t.Fatalf("expected no backup record created, stat err=%v", err)
	}
	if entries, err := os.ReadDir(removedDir); err != nil || len(entries) != 1 {
		t.Fatalf("expected exactly one removed record, entries=%v err=%v", entries, err)
	}
}

func writeAppSessionFile(t *testing.T, path string, sessionID string, cwd string) {
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
