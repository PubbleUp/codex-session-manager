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
		return Result{}, fmt.Errorf("来源为%s的会话不能彻底删除", sourceText(session.Source))
	}
	dir, err := managedSessionDir(cfg, session)
	if err != nil {
		return Result{}, err
	}
	if err := os.RemoveAll(dir); err != nil {
		return Result{}, err
	}
	return Result{
		SessionID: session.ID,
		Source:    dir,
		Message:   "已彻底删除受管会话记录",
	}, nil
}

func managedSessionDir(cfg config.Config, session domain.SessionRecord) (string, error) {
	if session.ID == "" {
		return "", fmt.Errorf("缺少会话 ID")
	}
	if filepath.Base(session.FilePath) != "session.jsonl" {
		return "", fmt.Errorf("拒绝彻底删除非预期文件：%s", session.FilePath)
	}
	currentDir := filepath.Dir(session.FilePath)

	var root string
	switch session.Source {
	case domain.SessionSourceBackup:
		root = cfg.BackupDir
	case domain.SessionSourceRemoved:
		root = cfg.RemovedDir
	default:
		return "", fmt.Errorf("来源为%s的会话不能彻底删除", sourceText(session.Source))
	}
	if !inside(currentDir, root) {
		return "", fmt.Errorf("拒绝彻底删除受管目录外路径：%s", currentDir)
	}
	if !fsutil.FileExists(filepath.Join(currentDir, "session.jsonl")) {
		return "", fmt.Errorf("未找到受管会话记录：%s", session.ID)
	}
	return currentDir, nil
}

// PurgeArchivedSession 永久删除 Codex 归档目录中的 session 文件。
func PurgeArchivedSession(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.Source != domain.SessionSourceArchived {
		return Result{}, fmt.Errorf("来源为%s的会话不能按归档会话彻底删除", sourceText(session.Source))
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
		return Result{}, fmt.Errorf("拒绝彻底删除 archived_sessions 外的归档路径：%s", session.FilePath)
	}
	if filepath.Base(session.FilePath) == "" || filepath.Ext(session.FilePath) != ".jsonl" {
		return Result{}, fmt.Errorf("拒绝彻底删除非预期归档文件：%s", session.FilePath)
	}
	if err := os.Remove(session.FilePath); err != nil {
		return Result{}, err
	}
	return Result{
		SessionID: session.ID,
		Source:    session.FilePath,
		Message:   "已彻底删除归档会话",
	}, nil
}

// RemoveFromCurrentCodex 将当前可见 session 移出当前 CODEX_HOME。
func RemoveFromCurrentCodex(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.Status != domain.SessionStatusVisible {
		return Result{}, fmt.Errorf("会话不在当前 CODEX_HOME 的可见列表中")
	}
	if cfg.RequireBackupBeforeRemove && !session.IsBackedUp {
		if _, err := backupsvc.BackupSession(cfg, session); err != nil {
			return Result{}, err
		}
	}
	return moveSessionToRemoved(cfg, session, "已从当前 Codex 移出", "已从当前 Codex 移出，并复用已有删除区副本")
}

// MoveInvisibleSessionToRemoved 将当前选中的不可见会话移入删除区。
func MoveInvisibleSessionToRemoved(cfg config.Config, session domain.SessionRecord) (Result, error) {
	if session.Source != domain.SessionSourceInactive && session.Source != domain.SessionSourceOldHome {
		return Result{}, fmt.Errorf("来源为%s的会话不能按不可见会话删除", sourceText(session.Source))
	}
	return moveSessionToRemoved(cfg, session, "已从不可见列表移入删除区", "已从不可见列表删除，并复用已有删除区副本")
}

func moveSessionToRemoved(cfg config.Config, session domain.SessionRecord, movedMessage string, reusedMessage string) (Result, error) {
	if session.ID == "" {
		return Result{}, fmt.Errorf("缺少会话 ID")
	}
	if !fsutil.FileExists(session.FilePath) {
		return Result{}, fmt.Errorf("会话文件不存在：%s", session.FilePath)
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
				Message:   reusedMessage,
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
		return Result{}, fmt.Errorf("移出后哈希校验失败")
	}

	if err := writeRemovedManifest(targetDir, session, sha); err != nil {
		return Result{}, err
	}

	return Result{
		SessionID: session.ID,
		Source:    session.FilePath,
		Target:    targetPath,
		Message:   movedMessage,
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

func sourceText(source domain.SessionSource) string {
	switch source {
	case domain.SessionSourceVisible:
		return "当前可见列表"
	case domain.SessionSourceInactive:
		return "不可见列表"
	case domain.SessionSourceArchived:
		return "归档目录"
	case domain.SessionSourceBackup:
		return "工具备份"
	case domain.SessionSourceOldHome:
		return "旧 CODEX_HOME"
	case domain.SessionSourceRemoved:
		return "工具删除区"
	default:
		return string(source)
	}
}
