package restore

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
	_ "modernc.org/sqlite"
)

func TestRestoreInactiveSessionRegistersCurrentThread(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionDir := filepath.Join(codexHome, "sessions", "2026", "06", "12")
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sessionPath := filepath.Join(sessionDir, "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	writeRestoreSessionFile(t, sessionPath, sessionID, "/tmp/project")
	writeRestoreThreadDB(t, codexHome)

	session := domain.SessionRecord{
		ID:           sessionID,
		Name:         "旧会话",
		CWD:          "/tmp/project",
		FilePath:     sessionPath,
		OriginalPath: sessionPath,
		Source:       domain.SessionSourceInactive,
		Status:       domain.SessionStatusInactive,
		CreatedAt:    time.Unix(100, 0),
		UpdatedAt:    time.Unix(200, 0),
		CLIVersion:   "0.139.0",
	}
	cfg := config.Config{
		CodexHome:     codexHome,
		ModelProvider: "current-provider",
	}

	result, err := RestoreSession(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.Message != "已注册到当前 Codex" {
		t.Fatalf("expected registration result, got %+v", result)
	}

	db, err := sql.Open("sqlite", filepath.Join(codexHome, "state_5.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var provider string
	var archived int
	var rolloutPath string
	if err := db.QueryRow(`SELECT model_provider, archived, rollout_path FROM threads WHERE id = ?`, sessionID).Scan(&provider, &archived, &rolloutPath); err != nil {
		t.Fatal(err)
	}
	if provider != "current-provider" || archived != 0 || fsutil.NormalizePath(rolloutPath) != fsutil.NormalizePath(sessionPath) {
		t.Fatalf("unexpected thread row provider=%q archived=%d path=%q", provider, archived, rolloutPath)
	}
}

func TestRestoreRemovedSessionPurgesOnlySelectedDeletedRecord(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	backupDir := filepath.Join(root, "tool", "backups")
	removedDir := filepath.Join(root, "tool", "removed")
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sourcePath := filepath.Join(removedDir, sessionID, "session.jsonl")
	otherPath := filepath.Join(backupDir, sessionID, "session.jsonl")
	targetPath := filepath.Join(codexHome, "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	writeRestoreSessionFile(t, sourcePath, sessionID, "/tmp/project")
	writeRestoreSessionFile(t, otherPath, sessionID, "/tmp/project")

	session := domain.SessionRecord{
		ID:           sessionID,
		CWD:          "/tmp/project",
		FilePath:     sourcePath,
		OriginalPath: targetPath,
		Source:       domain.SessionSourceRemoved,
		Status:       domain.SessionStatusRemoved,
		CreatedAt:    time.Date(2026, 6, 12, 17, 1, 19, 0, time.Local),
		UpdatedAt:    time.Date(2026, 6, 12, 17, 2, 0, 0, time.Local),
	}
	cfg := config.Config{CodexHome: codexHome, BackupDir: backupDir, RemovedDir: removedDir}

	result, err := RestoreSession(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "已恢复，并从删除列表移除" {
		t.Fatalf("expected deleted-list cleanup message, got %+v", result)
	}
	if _, err := os.Stat(filepath.Dir(sourcePath)); !os.IsNotExist(err) {
		t.Fatalf("expected selected removed source purged, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(otherPath)); err != nil {
		t.Fatalf("expected backup source kept, stat err=%v", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected restored target, stat err=%v", err)
	}
}

func TestRestoreBackupSessionPurgesOnlySelectedDeletedRecord(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	backupDir := filepath.Join(root, "tool", "backups")
	removedDir := filepath.Join(root, "tool", "removed")
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sourcePath := filepath.Join(backupDir, sessionID, "session.jsonl")
	otherPath := filepath.Join(removedDir, sessionID, "session.jsonl")
	targetPath := filepath.Join(codexHome, "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	writeRestoreSessionFile(t, sourcePath, sessionID, "/tmp/project")
	writeRestoreSessionFile(t, otherPath, sessionID, "/tmp/project")

	session := domain.SessionRecord{
		ID:           sessionID,
		CWD:          "/tmp/project",
		FilePath:     sourcePath,
		OriginalPath: targetPath,
		Source:       domain.SessionSourceBackup,
		Status:       domain.SessionStatusRecoverable,
		CreatedAt:    time.Date(2026, 6, 12, 17, 1, 19, 0, time.Local),
		UpdatedAt:    time.Date(2026, 6, 12, 17, 2, 0, 0, time.Local),
	}
	cfg := config.Config{CodexHome: codexHome, BackupDir: backupDir, RemovedDir: removedDir}

	if _, err := RestoreSession(cfg, session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(sourcePath)); !os.IsNotExist(err) {
		t.Fatalf("expected selected backup source purged, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(otherPath)); err != nil {
		t.Fatalf("expected removed source kept, stat err=%v", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected restored target, stat err=%v", err)
	}
}

func writeRestoreSessionFile(t *testing.T, path string, sessionID string, cwd string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-06-12T17:01:19+08:00","type":"session_meta","payload":{"id":"` + sessionID + `","timestamp":"2026-06-12T17:01:19+08:00","cwd":"` + cwd + `","originator":"codex_cli_rs","cli_version":"0.139.0","source":"cli","model_provider":"old-provider"}}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeRestoreThreadDB(t *testing.T, codexHome string) {
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
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			source TEXT NOT NULL,
			model_provider TEXT NOT NULL,
			cwd TEXT NOT NULL,
			title TEXT NOT NULL,
			sandbox_policy TEXT NOT NULL,
			approval_mode TEXT NOT NULL,
			tokens_used INTEGER NOT NULL DEFAULT 0,
			has_user_event INTEGER NOT NULL DEFAULT 0,
			archived INTEGER NOT NULL DEFAULT 0,
			archived_at INTEGER,
			git_sha TEXT,
			git_branch TEXT,
			git_origin_url TEXT,
			cli_version TEXT NOT NULL DEFAULT '',
			first_user_message TEXT NOT NULL DEFAULT '',
			agent_nickname TEXT,
			agent_role TEXT,
			memory_mode TEXT NOT NULL DEFAULT 'enabled',
			model TEXT,
			reasoning_effort TEXT,
			agent_path TEXT,
			created_at_ms INTEGER,
			updated_at_ms INTEGER,
			thread_source TEXT,
			preview TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
}
