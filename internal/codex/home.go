package codex

import (
	"path/filepath"
	"strings"

	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

// ClassifySource 根据文件路径识别 session 来源和状态。
func ClassifySource(path string, currentHome string, backupDir string, removedDir string, oldHomes []string) (domain.SessionSource, domain.SessionStatus, bool, string) {
	normalized := fsutil.NormalizePath(path)
	currentHome = fsutil.NormalizePath(currentHome)
	backupDir = fsutil.NormalizePath(backupDir)
	removedDir = fsutil.NormalizePath(removedDir)

	if inside(normalized, filepath.Join(currentHome, "sessions")) {
		return domain.SessionSourceVisible, domain.SessionStatusVisible, true, currentHome
	}
	if inside(normalized, filepath.Join(currentHome, "archived_sessions")) {
		return domain.SessionSourceArchived, domain.SessionStatusArchived, true, currentHome
	}
	if inside(normalized, backupDir) {
		return domain.SessionSourceBackup, domain.SessionStatusRecoverable, false, ""
	}
	if inside(normalized, removedDir) {
		return domain.SessionSourceRemoved, domain.SessionStatusRemoved, false, ""
	}
	for _, oldHome := range oldHomes {
		oldHome = fsutil.NormalizePath(oldHome)
		if inside(normalized, filepath.Join(oldHome, "sessions")) || inside(normalized, filepath.Join(oldHome, "archived_sessions")) {
			return domain.SessionSourceOldHome, domain.SessionStatusRecoverable, false, oldHome
		}
	}
	return domain.SessionSourceOldHome, domain.SessionStatusRecoverable, false, ""
}

// TargetSessionPath 计算 session 恢复到当前 CODEX_HOME 后的目标文件路径。
func TargetSessionPath(currentHome string, session domain.SessionRecord) string {
	created := session.CreatedAt
	year := created.Format("2006")
	month := created.Format("01")
	day := created.Format("02")
	stamp := created.Format("2006-01-02T15-04-05")
	name := "rollout-" + stamp + "-" + session.ID + ".jsonl"
	return filepath.Join(currentHome, "sessions", year, month, day, name)
}

func inside(path string, dir string) bool {
	dir = fsutil.NormalizePath(dir)
	if path == dir {
		return true
	}
	prefix := dir + string(filepath.Separator)
	return strings.HasPrefix(path, prefix)
}
