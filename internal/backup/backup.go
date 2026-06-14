package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

// BackupSession 将 session 文件备份到工具备份目录。
func BackupSession(cfg config.Config, session domain.SessionRecord) (domain.BackupManifest, error) {
	if session.ID == "" {
		return domain.BackupManifest{}, fmt.Errorf("missing session id")
	}
	if !fsutil.FileExists(session.FilePath) {
		return domain.BackupManifest{}, fmt.Errorf("session file not found: %s", session.FilePath)
	}
	sha, err := fsutil.SHA256File(session.FilePath)
	if err != nil {
		return domain.BackupManifest{}, err
	}
	session.SHA256 = sha

	backupDir := filepath.Join(cfg.BackupDir, session.ID)
	sessionPath := filepath.Join(backupDir, "session.jsonl")
	manifestPath := filepath.Join(backupDir, "manifest.json")

	if fsutil.FileExists(sessionPath) {
		existingSHA, err := fsutil.SHA256File(sessionPath)
		if err != nil {
			return domain.BackupManifest{}, err
		}
		if existingSHA != sha && !cfg.AllowOverwrite {
			return domain.BackupManifest{}, fmt.Errorf("backup conflict: %s already exists with different content", session.ID)
		}
	}

	if err := fsutil.EnsurePrivateDir(backupDir); err != nil {
		return domain.BackupManifest{}, err
	}
	if err := fsutil.CopyFile(session.FilePath, sessionPath); err != nil {
		return domain.BackupManifest{}, err
	}
	copiedSHA, err := fsutil.SHA256File(sessionPath)
	if err != nil {
		return domain.BackupManifest{}, err
	}
	if copiedSHA != sha {
		return domain.BackupManifest{}, fmt.Errorf("backup hash mismatch")
	}

	manifest := domain.BackupManifest{
		SessionID:     session.ID,
		OriginalPath:  session.FilePath,
		CWD:           session.CWD,
		SourceType:    string(session.Source),
		CodexHome:     session.CodexHome,
		CreatedAt:     session.CreatedAt,
		UpdatedAt:     session.UpdatedAt,
		CLIVersion:    session.CLIVersion,
		ModelProvider: session.ModelProvider,
		SHA256:        sha,
		BackupAt:      time.Now(),
	}
	if err := writeManifest(manifestPath, manifest); err != nil {
		return domain.BackupManifest{}, err
	}
	return manifest, nil
}

func LoadManifest(path string) (domain.BackupManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.BackupManifest{}, err
	}
	var manifest domain.BackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return domain.BackupManifest{}, err
	}
	return manifest, nil
}

func WriteManifest(path string, manifest domain.BackupManifest) error {
	return writeManifest(path, manifest)
}

func writeManifest(path string, manifest domain.BackupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
