package restore

import (
	"fmt"
	"path/filepath"

	"github.com/sunlock/codex-session-manager/internal/backup"
	"github.com/sunlock/codex-session-manager/internal/codex"
	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
	removesvc "github.com/sunlock/codex-session-manager/internal/remove"
)

type Result struct {
	SessionID  string
	Action     string
	SourcePath string
	TargetPath string
	Message    string
	Changed    bool
}

// RestoreSession 将一个可恢复 session 恢复到当前 CODEX_HOME/sessions。
func RestoreSession(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.ID == "" {
		return Result{}, fmt.Errorf("缺少会话 ID")
	}
	if !fsutil.FileExists(session.FilePath) {
		return Result{}, fmt.Errorf("源会话文件不存在：%s", session.FilePath)
	}

	sha, err := fsutil.SHA256File(session.FilePath)
	if err != nil {
		return Result{}, err
	}
	if session.SHA256 != "" && session.SHA256 != sha {
		return Result{}, fmt.Errorf("源文件哈希不匹配：%s", session.ID)
	}
	session.SHA256 = sha

	targetPath := codex.TargetSessionPath(cfg.CodexHome, session)
	if session.OriginalPath != "" {
		normalizedOriginal := fsutil.NormalizePath(session.OriginalPath)
		if inside(normalizedOriginal, filepath.Join(cfg.CodexHome, "sessions")) {
			targetPath = normalizedOriginal
		}
	}

	result := Result{
		SessionID:  session.ID,
		Action:     "restore",
		SourcePath: session.FilePath,
		TargetPath: targetPath,
	}

	if fsutil.FileExists(targetPath) {
		targetSHA, err := fsutil.SHA256File(targetPath)
		if err != nil {
			return result, err
		}
		if targetSHA == sha {
			if err := codex.RegisterThread(cfg, session, targetPath); err != nil {
				return result, err
			}
			result.Changed = session.Source == domain.SessionSourceInactive || isManagedRestoreSource(session)
			result.Message = "已注册到当前 Codex"
			if err := purgeRestoredManagedSource(cfg, session); err != nil {
				return result, fmt.Errorf("已恢复，但清理删除列表源记录失败：%w", err)
			}
			if isManagedRestoreSource(session) {
				result.Message = "已注册到当前 Codex，并从删除列表移除"
			}
			return result, nil
		}
		if !cfg.AllowOverwrite {
			result.Message = "目标文件已存在且内容不同"
			return result, fmt.Errorf("恢复冲突，目标文件已存在且内容不同：%s", targetPath)
		}
	}

	if err := fsutil.CopyFile(session.FilePath, targetPath); err != nil {
		return result, err
	}
	targetSHA, err := fsutil.SHA256File(targetPath)
	if err != nil {
		return result, err
	}
	if targetSHA != sha {
		return result, fmt.Errorf("恢复后哈希校验失败")
	}
	if err := codex.RegisterThread(cfg, session, targetPath); err != nil {
		return result, err
	}
	result.Changed = true
	result.Message = "已恢复"
	if err := purgeRestoredManagedSource(cfg, session); err != nil {
		return result, fmt.Errorf("已恢复，但清理删除列表源记录失败：%w", err)
	}
	if isManagedRestoreSource(session) {
		result.Message = "已恢复，并从删除列表移除"
	}
	return result, nil
}

// RestoreBackupByID 从备份目录按 session id 恢复。
func RestoreBackupByID(cfg config.Config, sessionID string) (Result, error) {
	manifestPath := filepath.Join(cfg.BackupDir, sessionID, "manifest.json")
	manifest, err := backup.LoadManifest(manifestPath)
	if err != nil {
		return Result{}, err
	}
	sessionPath := filepath.Join(cfg.BackupDir, sessionID, "session.jsonl")
	session := domain.SessionRecord{
		ID:            manifest.SessionID,
		CWD:           manifest.CWD,
		FilePath:      sessionPath,
		OriginalPath:  manifest.OriginalPath,
		Source:        domain.SessionSourceBackup,
		Status:        domain.SessionStatusRecoverable,
		CodexHome:     manifest.CodexHome,
		CreatedAt:     manifest.CreatedAt,
		UpdatedAt:     manifest.UpdatedAt,
		CLIVersion:    manifest.CLIVersion,
		ModelProvider: manifest.ModelProvider,
		SHA256:        manifest.SHA256,
	}
	return RestoreSession(cfg, session)
}

func inside(path string, dir string) bool {
	dir = fsutil.NormalizePath(dir)
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && rel != ".." && rel != "" && rel != ".."+string(filepath.Separator) && !startsWithParent(rel)
}

func startsWithParent(rel string) bool {
	return len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator)
}

func purgeRestoredManagedSource(cfg config.Config, session domain.SessionRecord) error {
	if !isManagedRestoreSource(session) {
		return nil
	}
	_, err := removesvc.PurgeManagedSession(cfg, session)
	return err
}

func isManagedRestoreSource(session domain.SessionRecord) bool {
	switch session.Source {
	case domain.SessionSourceBackup, domain.SessionSourceRemoved:
		return true
	default:
		return false
	}
}
