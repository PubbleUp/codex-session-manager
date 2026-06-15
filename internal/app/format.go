package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/sunlock/codex-session-manager/internal/domain"
	restoresvc "github.com/sunlock/codex-session-manager/internal/restore"
)

func FormatProjects(projects []domain.ProjectRecord) string {
	var builder strings.Builder
	for _, project := range projects {
		builder.WriteString(fmt.Sprintf(
			"%s  总数=%d 可见=%d 可恢复=%d 备份=%d 更新时间=%s\n",
			project.CWD,
			project.TotalSessions,
			project.VisibleCount,
			project.RecoverableCount,
			project.BackedUpCount,
			formatTime(project.LastUpdatedAt),
		))
	}
	return builder.String()
}

func FormatHiddenProjects(projects []string) string {
	var builder strings.Builder
	for _, project := range projects {
		builder.WriteString(project)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func FormatSessions(sessions []domain.SessionRecord, projectPath string) string {
	var builder strings.Builder
	for _, session := range sessions {
		if projectPath != "" && session.CWD != projectPath {
			continue
		}
		builder.WriteString(fmt.Sprintf(
			"%s  %-11s %-9s %-32s 更新时间=%s 项目=%s\n",
			session.ID,
			statusText(session.Status),
			sourceText(session.Source),
			displayName(session),
			formatTime(session.UpdatedAt),
			session.CWD,
		))
	}
	return builder.String()
}

func displayName(session domain.SessionRecord) string {
	if session.Name != "" {
		return session.Name
	}
	if len(session.ID) > 8 {
		return session.ID[:8]
	}
	return session.ID
}

func statusText(status domain.SessionStatus) string {
	switch status {
	case domain.SessionStatusVisible:
		return "可见"
	case domain.SessionStatusInactive:
		return "不可见"
	case domain.SessionStatusArchived:
		return "已归档"
	case domain.SessionStatusRecoverable:
		return "可恢复"
	case domain.SessionStatusBackedUp:
		return "已备份"
	case domain.SessionStatusRemoved:
		return "已删除"
	case domain.SessionStatusConflict:
		return "冲突"
	default:
		return string(status)
	}
}

func sourceText(source domain.SessionSource) string {
	switch source {
	case domain.SessionSourceVisible:
		return "当前"
	case domain.SessionSourceInactive:
		return "不可见"
	case domain.SessionSourceArchived:
		return "归档"
	case domain.SessionSourceBackup:
		return "备份"
	case domain.SessionSourceOldHome:
		return "旧目录"
	case domain.SessionSourceRemoved:
		return "删除区"
	default:
		return string(source)
	}
}

func FormatRepairReport(report restoresvc.RepairReport) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("项目：%s\n\n", report.ProjectPath))
	builder.WriteString(fmt.Sprintf("已恢复：%d\n", len(report.Restored)))
	for _, item := range report.Restored {
		builder.WriteString(fmt.Sprintf("  %s -> %s (%s)\n", item.SessionID, item.TargetPath, item.Message))
	}
	builder.WriteString(fmt.Sprintf("\n已跳过：%d\n", len(report.Skipped)))
	for _, item := range report.Skipped {
		builder.WriteString(fmt.Sprintf("  %s  %s\n", item.Session.ID, item.Reason))
	}
	builder.WriteString(fmt.Sprintf("\n冲突：%d\n", len(report.Conflicts)))
	for _, item := range report.Conflicts {
		builder.WriteString(fmt.Sprintf("  %s  %s\n", item.Session.ID, item.Reason))
	}
	builder.WriteString(fmt.Sprintf("\n失败：%d\n", len(report.Failed)))
	for _, item := range report.Failed {
		builder.WriteString(fmt.Sprintf("  %s\n", item))
	}
	return builder.String()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("2006-01-02 15:04")
}
