package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	backupsvc "github.com/sunlock/codex-session-manager/internal/backup"
	"github.com/sunlock/codex-session-manager/internal/codex"
	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	removesvc "github.com/sunlock/codex-session-manager/internal/remove"
	restoresvc "github.com/sunlock/codex-session-manager/internal/restore"
	"github.com/sunlock/codex-session-manager/internal/scanner"
)

type App struct {
	Config config.Config
}

func New() (App, error) {
	cfg, err := config.Load()
	if err != nil {
		return App{}, err
	}
	if err := config.Ensure(cfg); err != nil {
		return App{}, err
	}
	return App{Config: cfg}, nil
}

func (a App) Scan() (domain.Inventory, error) {
	return scanner.Scan(a.Config)
}

func (a App) HideProject(projectPath string) error {
	if err := config.HideProject(a.Config, projectPath); err != nil {
		_ = a.Audit("hide-project", "", projectPath, "", "失败", err.Error())
		return err
	}
	_ = a.Audit("hide-project", "", projectPath, "", "成功", "")
	return nil
}

func (a App) UnhideProject(projectPath string) error {
	if err := config.UnhideProject(a.Config, projectPath); err != nil {
		_ = a.Audit("unhide-project", "", projectPath, "", "失败", err.Error())
		return err
	}
	_ = a.Audit("unhide-project", "", projectPath, "", "成功", "")
	return nil
}

func (a App) Backup(sessionID string) (domain.BackupManifest, error) {
	inventory, err := a.Scan()
	if err != nil {
		return domain.BackupManifest{}, err
	}
	session, ok := FindSession(inventory.Sessions, sessionID)
	if !ok {
		return domain.BackupManifest{}, fmt.Errorf("未找到会话：%s", sessionID)
	}
	manifest, err := backupsvc.BackupSession(a.Config, session)
	if err != nil {
		_ = a.Audit("backup", session.ID, session.FilePath, "", "失败", err.Error())
		return domain.BackupManifest{}, err
	}
	_ = a.Audit("backup", session.ID, session.FilePath, filepath.Join(a.Config.BackupDir, session.ID), "成功", "")
	return manifest, nil
}

func (a App) Restore(sessionID string) (restoresvc.Result, error) {
	result, err := restoresvc.RestoreBackupByID(a.Config, sessionID)
	if err != nil {
		_ = a.Audit("restore", sessionID, "", "", "失败", err.Error())
		return result, err
	}
	_ = a.Audit("restore", sessionID, result.SourcePath, result.TargetPath, "成功", result.Message)
	return result, nil
}

func (a App) RestoreSession(session domain.SessionRecord) (restoresvc.Result, error) {
	result, err := restoresvc.RestoreSession(a.Config, session)
	if err != nil {
		_ = a.Audit("restore", session.ID, session.FilePath, "", "失败", err.Error())
		return result, err
	}
	_ = a.Audit("restore", session.ID, result.SourcePath, result.TargetPath, "成功", result.Message)
	return result, nil
}

func (a App) Remove(sessionID string) (removesvc.Result, error) {
	inventory, err := a.Scan()
	if err != nil {
		return removesvc.Result{}, err
	}
	session, ok := FindVisibleSession(inventory.Sessions, sessionID)
	if !ok {
		return removesvc.Result{}, fmt.Errorf("未找到当前可见会话：%s", sessionID)
	}
	result, err := removesvc.RemoveFromCurrentCodex(a.Config, session)
	if err != nil {
		_ = a.Audit("remove", session.ID, session.FilePath, "", "失败", err.Error())
		return result, err
	}
	_ = a.Audit("remove", session.ID, result.Source, result.Target, "成功", result.Message)
	return result, nil
}

func (a App) RemoveSession(session domain.SessionRecord) (removesvc.Result, error) {
	switch session.Source {
	case domain.SessionSourceBackup, domain.SessionSourceRemoved:
		result, err := removesvc.PurgeManagedSession(a.Config, session)
		if err != nil {
			_ = a.Audit("purge", session.ID, session.FilePath, "", "失败", err.Error())
			return result, err
		}
		_ = a.Audit("purge", session.ID, result.Source, "", "成功", result.Message)
		return result, nil
	case domain.SessionSourceArchived:
		result, err := removesvc.PurgeArchivedSession(a.Config, session)
		if err != nil {
			_ = a.Audit("purge-archived", session.ID, session.FilePath, "", "失败", err.Error())
			return result, err
		}
		_ = a.Audit("purge-archived", session.ID, result.Source, "", "成功", result.Message)
		return result, nil
	case domain.SessionSourceVisible:
		result, err := removesvc.RemoveFromCurrentCodex(a.Config, session)
		if err != nil {
			_ = a.Audit("remove", session.ID, session.FilePath, "", "失败", err.Error())
			return result, err
		}
		_ = a.Audit("remove", session.ID, result.Source, result.Target, "成功", result.Message)
		return result, nil
	case domain.SessionSourceInactive, domain.SessionSourceOldHome:
		result, err := removesvc.MoveInvisibleSessionToRemoved(a.Config, session)
		if err != nil {
			_ = a.Audit("remove-invisible", session.ID, session.FilePath, "", "失败", err.Error())
			return result, err
		}
		_ = a.Audit("remove-invisible", session.ID, result.Source, result.Target, "成功", result.Message)
		return result, nil
	default:
		return removesvc.Result{}, fmt.Errorf("当前来源的会话不能删除")
	}
}

func (a App) ArchiveSession(session domain.SessionRecord) (codex.CommandResult, error) {
	result, err := codex.ArchiveSession(a.Config, session)
	if err != nil {
		_ = a.Audit("archive", session.ID, session.FilePath, "", "失败", err.Error())
		return result, err
	}
	_ = a.Audit("archive", session.ID, session.FilePath, "", "成功", result.Message)
	return result, nil
}

func (a App) RenameSession(session domain.SessionRecord, name string) error {
	if err := codex.RenameSession(a.Config, session, name); err != nil {
		_ = a.Audit("rename", session.ID, session.FilePath, "", "失败", err.Error())
		return err
	}
	_ = a.Audit("rename", session.ID, session.FilePath, "", "成功", name)
	return nil
}

func (a App) Repair(projectPath string) (restoresvc.RepairReport, error) {
	if projectPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return restoresvc.RepairReport{}, err
		}
		projectPath = cwd
	}
	inventory, err := a.Scan()
	if err != nil {
		return restoresvc.RepairReport{}, err
	}
	plan := restoresvc.BuildRepairPlan(a.Config, inventory, projectPath)
	report := restoresvc.ExecuteRepair(a.Config, plan)
	for _, result := range report.Restored {
		_ = a.Audit("repair", result.SessionID, result.SourcePath, result.TargetPath, "成功", result.Message)
	}
	for _, failed := range report.Failed {
		_ = a.Audit("repair", "", "", "", "失败", failed)
	}
	return report, nil
}

func FindSession(sessions []domain.SessionRecord, idOrPrefix string) (domain.SessionRecord, bool) {
	for _, session := range sessions {
		if session.ID == idOrPrefix || strings.HasPrefix(session.ID, idOrPrefix) {
			return session, true
		}
	}
	return domain.SessionRecord{}, false
}

func FindVisibleSession(sessions []domain.SessionRecord, idOrPrefix string) (domain.SessionRecord, bool) {
	for _, session := range sessions {
		if session.Status != domain.SessionStatusVisible {
			continue
		}
		if session.ID == idOrPrefix || strings.HasPrefix(session.ID, idOrPrefix) {
			return session, true
		}
	}
	return domain.SessionRecord{}, false
}
