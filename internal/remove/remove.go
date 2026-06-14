package remove

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	backupsvc "github.com/sunlock/codex-session-manager/internal/backup"
	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

type Result struct {
	SessionID string
	Source    string
	Target    string
	Message   string
}

// PurgeManagedSession 永久删除工具管理的备份或删除区记录。
func PurgeManagedSession(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.Source != domain.SessionSourceBackup && session.Source != domain.SessionSourceRemoved {
		return Result{}, fmt.Errorf("session source %s cannot be purged", session.Source)
	}
	dirs, err := managedSessionDirs(cfg, session)
	if err != nil {
		return Result{}, err
	}
	if len(dirs) == 0 {
		return Result{}, fmt.Errorf("managed session record not found: %s", session.ID)
	}
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			return Result{}, err
		}
	}
	return Result{
		SessionID: session.ID,
		Source:    strings.Join(dirs, ","),
		Message:   "purged managed session record",
	}, nil
}

func managedSessionDirs(cfg config.Config, session domain.SessionRecord) ([]string, error) {
	if session.ID == "" {
		return nil, fmt.Errorf("missing session id")
	}
	if filepath.Base(session.FilePath) != "session.jsonl" {
		return nil, fmt.Errorf("refuse to purge unexpected file: %s", session.FilePath)
	}
	currentDir := filepath.Dir(session.FilePath)
	if !inside(currentDir, cfg.BackupDir) && !inside(currentDir, cfg.RemovedDir) {
		return nil, fmt.Errorf("refuse to purge path outside managed dir: %s", currentDir)
	}

	seen := map[string]bool{}
	var dirs []string
	addDir := func(dir string) error {
		if dir == "" || seen[dir] {
			return nil
		}
		if !inside(dir, cfg.BackupDir) && !inside(dir, cfg.RemovedDir) {
			return fmt.Errorf("refuse to purge path outside managed dir: %s", dir)
		}
		sessionPath := filepath.Join(dir, "session.jsonl")
		if !fsutil.FileExists(sessionPath) {
			return nil
		}
		seen[dir] = true
		dirs = append(dirs, dir)
		return nil
	}

	if err := addDir(currentDir); err != nil {
		return nil, err
	}
	if err := addDir(filepath.Join(cfg.BackupDir, session.ID)); err != nil {
		return nil, err
	}
	if err := addRemovedDirs(cfg.RemovedDir, session.ID, addDir); err != nil {
		return nil, err
	}
	return dirs, nil
}

func addRemovedDirs(root string, sessionID string, addDir func(string) error) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name != sessionID && !strings.HasPrefix(name, sessionID+"-") {
			continue
		}
		if err := addDir(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
}

// PurgeArchivedSession 永久删除 Codex 归档目录中的 session 文件。
func PurgeArchivedSession(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.Source != domain.SessionSourceArchived {
		return Result{}, fmt.Errorf("session source %s cannot be purged as archived", session.Source)
	}
	allowedRoots := []string{filepath.Join(cfg.CodexHome, "archived_sessions")}
	for _, oldHome := range cfg.OldCodexHomes {
		allowedRoots = append(allowedRoots, filepath.Join(oldHome, "archived_sessions"))
	}
	allowed := false
	for _, root := range allowedRoots {
		if inside(session.FilePath, root) {
			allowed = true
			break
		}
	}
	if !allowed {
		return Result{}, fmt.Errorf("refuse to purge archived path outside archived_sessions: %s", session.FilePath)
	}
	if filepath.Base(session.FilePath) == "" || filepath.Ext(session.FilePath) != ".jsonl" {
		return Result{}, fmt.Errorf("refuse to purge unexpected archived file: %s", session.FilePath)
	}
	if err := os.Remove(session.FilePath); err != nil {
		return Result{}, err
	}
	return Result{
		SessionID: session.ID,
		Source:    session.FilePath,
		Message:   "purged archived session",
	}, nil
}

// RemoveFromCurrentCodex 将当前可见 session 移出当前 CODEX_HOME。
func RemoveFromCurrentCodex(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.Status != domain.SessionStatusVisible {
		return Result{}, fmt.Errorf("session is not visible in current CODEX_HOME")
	}
	if cfg.RequireBackupBeforeRemove && !session.IsBackedUp {
		if _, err := backupsvc.BackupSession(cfg, session); err != nil {
			return Result{}, err
		}
	}

	targetDir := filepath.Join(cfg.RemovedDir, session.ID)
	targetPath := filepath.Join(targetDir, "session.jsonl")
	sha, err := fsutil.SHA256File(session.FilePath)
	if err != nil {
		return Result{}, err
	}

	if fsutil.FileExists(targetPath) {
		targetSHA, err := fsutil.SHA256File(targetPath)
		if err != nil {
			return Result{}, err
		}
		if targetSHA == sha {
			if err := os.Remove(session.FilePath); err != nil {
				return Result{}, err
			}
			if err := writeRemovedManifest(targetDir, session, sha); err != nil {
				return Result{}, err
			}
			return Result{
				SessionID: session.ID,
				Source:    session.FilePath,
				Target:    targetPath,
				Message:   "removed from current Codex; existing removed copy reused",
			}, nil
		}
		if !cfg.AllowOverwrite {
			stamp := time.Now().Format("20060102150405")
			targetDir = filepath.Join(cfg.RemovedDir, session.ID+"-"+stamp)
			targetPath = filepath.Join(targetDir, "session.jsonl")
		}
	}

	if err := fsutil.MoveFile(session.FilePath, targetPath); err != nil {
		return Result{}, err
	}
	targetSHA, err := fsutil.SHA256File(targetPath)
	if err != nil {
		return Result{}, err
	}
	if targetSHA != sha {
		return Result{}, fmt.Errorf("removed hash mismatch")
	}

	if err := writeRemovedManifest(targetDir, session, sha); err != nil {
		return Result{}, err
	}

	return Result{
		SessionID: session.ID,
		Source:    session.FilePath,
		Target:    targetPath,
		Message:   "removed from current Codex",
	}, nil
}

func writeRemovedManifest(targetDir string, session domain.SessionRecord, sha string) error {
	manifest := domain.BackupManifest{
		SessionID:     session.ID,
		OriginalPath:  session.FilePath,
		CWD:           session.CWD,
		SourceType:    string(domain.SessionSourceRemoved),
		CodexHome:     session.CodexHome,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
		CLIVersion:    session.CLIVersion,
		ModelProvider: session.ModelProvider,
		SHA256:        sha,
		BackupAt:      time.Now(),
	}
	return backupsvc.WriteManifest(filepath.Join(targetDir, "manifest.json"), manifest)
}

func inside(path string, root string) bool {
	path = fsutil.NormalizePath(path)
	root = fsutil.NormalizePath(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
