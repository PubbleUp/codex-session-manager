package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
)

func TestBackupSession(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".codex", "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-019ebb10-7a0e-7d70-95e8-c020b75687d8.jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		CodexHome: filepath.Join(root, ".codex"),
		BackupDir: filepath.Join(root, "tool", "backups"),
	}
	session := domain.SessionRecord{
		ID:        "019ebb10-7a0e-7d70-95e8-c020b75687d8",
		CWD:       "/tmp/project",
		FilePath:  source,
		Source:    domain.SessionSourceVisible,
		Status:    domain.SessionStatusVisible,
		CreatedAt: time.Date(2026, 6, 12, 17, 1, 19, 0, time.Local),
		UpdatedAt: time.Date(2026, 6, 12, 17, 2, 0, 0, time.Local),
	}

	manifest, err := BackupSession(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.SessionID != session.ID {
		t.Fatalf("unexpected manifest session id: %s", manifest.SessionID)
	}
	if _, err := os.Stat(filepath.Join(cfg.BackupDir, session.ID, "session.jsonl")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfg.BackupDir, session.ID, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}
