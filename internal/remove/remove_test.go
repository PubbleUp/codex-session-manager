package remove

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	backupsvc "github.com/sunlock/codex-session-manager/internal/backup"
	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

func TestRemoveReusesExistingRemovedCopyWithSameHash(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	source := filepath.Join(root, ".codex", "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	removed := filepath.Join(root, "tool", "removed", sessionID, "session.jsonl")
	content := []byte("same session content\n")

	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, content, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fsutil.CopyFile(source, removed); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		CodexHome:                 filepath.Join(root, ".codex"),
		BackupDir:                 filepath.Join(root, "tool", "backups"),
		RemovedDir:                filepath.Join(root, "tool", "removed"),
		RequireBackupBeforeRemove: false,
	}
	session := domain.SessionRecord{
		ID:        sessionID,
		CWD:       "/tmp/project",
		FilePath:  source,
		Source:    domain.SessionSourceVisible,
		Status:    domain.SessionStatusVisible,
		CreatedAt: time.Date(2026, 6, 12, 17, 1, 19, 0, time.Local),
		UpdatedAt: time.Date(2026, 6, 12, 17, 2, 0, 0, time.Local),
	}

	result, err := RemoveFromCurrentCodex(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target != removed {
		t.Fatalf("expected existing removed target, got %s", result.Target)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("expected source removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "tool", "removed", sessionID, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveCreatesVersionedRemovedDirWhenContentDiffers(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	source := filepath.Join(root, ".codex", "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	removed := filepath.Join(root, "tool", "removed", sessionID, "session.jsonl")

	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new content\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(removed), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(removed, []byte("old content\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		CodexHome:                 filepath.Join(root, ".codex"),
		BackupDir:                 filepath.Join(root, "tool", "backups"),
		RemovedDir:                filepath.Join(root, "tool", "removed"),
		RequireBackupBeforeRemove: false,
		AllowOverwrite:            false,
	}
	session := domain.SessionRecord{
		ID:        sessionID,
		CWD:       "/tmp/project",
		FilePath:  source,
		Source:    domain.SessionSourceVisible,
		Status:    domain.SessionStatusVisible,
		CreatedAt: time.Date(2026, 6, 12, 17, 1, 19, 0, time.Local),
		UpdatedAt: time.Date(2026, 6, 12, 17, 2, 0, 0, time.Local),
	}

	result, err := RemoveFromCurrentCodex(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if result.Target == removed {
		t.Fatal("expected versioned removed target")
	}
	if _, err := os.Stat(result.Target); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveStillBackupsWhenRequired(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	source := filepath.Join(root, ".codex", "sessions", "2026", "06", "12", "rollout-2026-06-12T17-01-19-"+sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(source), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("content\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		CodexHome:                 filepath.Join(root, ".codex"),
		BackupDir:                 filepath.Join(root, "tool", "backups"),
		RemovedDir:                filepath.Join(root, "tool", "removed"),
		RequireBackupBeforeRemove: true,
	}
	session := domain.SessionRecord{
		ID:        sessionID,
		CWD:       "/tmp/project",
		FilePath:  source,
		Source:    domain.SessionSourceVisible,
		Status:    domain.SessionStatusVisible,
		CreatedAt: time.Date(2026, 6, 12, 17, 1, 19, 0, time.Local),
		UpdatedAt: time.Date(2026, 6, 12, 17, 2, 0, 0, time.Local),
	}

	if _, err := RemoveFromCurrentCodex(cfg, session); err != nil {
		t.Fatal(err)
	}
	if _, err := backupsvc.LoadManifest(filepath.Join(cfg.BackupDir, sessionID, "manifest.json")); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeManagedSessionRemovesOnlySelectedBackupDir(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	backupDir := filepath.Join(root, "tool", "backups")
	removedDir := filepath.Join(root, "tool", "removed")
	sessionDir := filepath.Join(backupDir, sessionID)
	sessionPath := filepath.Join(sessionDir, "session.jsonl")
	removedSessionDir := filepath.Join(removedDir, sessionID)
	removedSessionPath := filepath.Join(removedSessionDir, "session.jsonl")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("backup\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(removedSessionDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(removedSessionPath, []byte("removed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{BackupDir: backupDir, RemovedDir: removedDir}
	session := domain.SessionRecord{
		ID:       sessionID,
		FilePath: sessionPath,
		Source:   domain.SessionSourceBackup,
	}

	if _, err := PurgeManagedSession(cfg, session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("expected backup dir purged, stat err=%v", err)
	}
	if _, err := os.Stat(removedSessionDir); err != nil {
		t.Fatalf("expected removed dir kept, stat err=%v", err)
	}
}

func TestPurgeManagedSessionRemovesOnlySelectedRemovedDir(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	backupDir := filepath.Join(root, "tool", "backups")
	removedDir := filepath.Join(root, "tool", "removed")
	removedSessionDir := filepath.Join(removedDir, sessionID)
	versionedRemovedSessionDir := filepath.Join(removedDir, sessionID+"-20260613235959")
	for _, dir := range []string{removedSessionDir, versionedRemovedSessionDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte("removed\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Config{BackupDir: backupDir, RemovedDir: removedDir}
	session := domain.SessionRecord{
		ID:       sessionID,
		FilePath: filepath.Join(removedSessionDir, "session.jsonl"),
		Source:   domain.SessionSourceRemoved,
	}

	if _, err := PurgeManagedSession(cfg, session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(removedSessionDir); !os.IsNotExist(err) {
		t.Fatalf("expected selected removed dir purged, stat err=%v", err)
	}
	if _, err := os.Stat(versionedRemovedSessionDir); err != nil {
		t.Fatalf("expected versioned removed dir kept, stat err=%v", err)
	}
}

func TestPurgeManagedSessionRejectsVisibleSession(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{BackupDir: filepath.Join(root, "tool", "backups"), RemovedDir: filepath.Join(root, "tool", "removed")}
	session := domain.SessionRecord{
		ID:       "019ebb10-7a0e-7d70-95e8-c020b75687d8",
		FilePath: filepath.Join(root, ".codex", "sessions", "session.jsonl"),
		Source:   domain.SessionSourceVisible,
	}
	if _, err := PurgeManagedSession(cfg, session); err == nil {
		t.Fatal("expected visible session purge rejected")
	}
}

func TestPurgeManagedSessionRejectsPathOutsideManagedDir(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{BackupDir: filepath.Join(root, "tool", "backups"), RemovedDir: filepath.Join(root, "tool", "removed")}
	session := domain.SessionRecord{
		ID:       "019ebb10-7a0e-7d70-95e8-c020b75687d8",
		FilePath: filepath.Join(root, "outside", "session.jsonl"),
		Source:   domain.SessionSourceBackup,
	}
	if _, err := PurgeManagedSession(cfg, session); err == nil {
		t.Fatal("expected outside path purge rejected")
	}
}

func TestPurgeManagedSessionRejectsSourceDirectoryMismatch(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	cfg := config.Config{BackupDir: filepath.Join(root, "tool", "backups"), RemovedDir: filepath.Join(root, "tool", "removed")}
	sessionPath := filepath.Join(cfg.RemovedDir, sessionID, "session.jsonl")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("removed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := domain.SessionRecord{
		ID:       sessionID,
		FilePath: sessionPath,
		Source:   domain.SessionSourceBackup,
	}

	if _, err := PurgeManagedSession(cfg, session); err == nil {
		t.Fatal("expected source directory mismatch rejected")
	}
}

func TestPurgeArchivedSessionRemovesArchivedFile(t *testing.T) {
	root := t.TempDir()
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	archivedPath := filepath.Join(root, ".codex", "archived_sessions", "2026", "06", "12", "rollout-"+sessionID+".jsonl")
	if err := os.MkdirAll(filepath.Dir(archivedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivedPath, []byte("archived\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{CodexHome: filepath.Join(root, ".codex")}
	session := domain.SessionRecord{
		ID:       sessionID,
		FilePath: archivedPath,
		Source:   domain.SessionSourceArchived,
	}

	if _, err := PurgeArchivedSession(cfg, session); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(archivedPath); !os.IsNotExist(err) {
		t.Fatalf("expected archived file removed, stat err=%v", err)
	}
}

func TestPurgeArchivedSessionRejectsNonArchivedPath(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{CodexHome: filepath.Join(root, ".codex")}
	session := domain.SessionRecord{
		ID:       "019ebb10-7a0e-7d70-95e8-c020b75687d8",
		FilePath: filepath.Join(root, ".codex", "sessions", "session.jsonl"),
		Source:   domain.SessionSourceArchived,
	}

	if _, err := PurgeArchivedSession(cfg, session); err == nil {
		t.Fatal("expected non-archived path rejected")
	}
}
