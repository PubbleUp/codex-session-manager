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
			"%s  total=%d visible=%d recoverable=%d backup=%d updated=%s\n",
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
			"%s  %-11s %-9s %-32s updated=%s cwd=%s\n",
			session.ID,
			session.Status,
			session.Source,
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
