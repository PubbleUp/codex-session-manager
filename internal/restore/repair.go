package restore

import (
	"fmt"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

type RepairPlan struct {
	ProjectPath string
	Items       []RepairPlanItem
}

type RepairPlanItem struct {
	Session domain.SessionRecord
	Action  string
	Reason  string
}

type RepairReport struct {
	ProjectPath string
	Restored    []Result
	Skipped     []RepairPlanItem
	Conflicts   []RepairPlanItem
	Failed      []string
}

func BuildRepairPlan(cfg config.Config, inventory domain.Inventory, projectPath string) RepairPlan {
	normalizedProject := fsutil.NormalizePath(projectPath)
	visible := map[string]bool{}
	candidates := map[string]domain.SessionRecord{}

	for _, session := range inventory.Sessions {
		if fsutil.NormalizePath(session.CWD) != normalizedProject {
			continue
		}
		if session.Status == domain.SessionStatusVisible {
			visible[session.ID] = true
			continue
		}
		if session.Status == domain.SessionStatusConflict {
			candidates[session.ID] = session
			continue
		}
		if !isRecoverable(session) {
			continue
		}
		current, ok := candidates[session.ID]
		if !ok || sourceRank(session.Source) < sourceRank(current.Source) {
			candidates[session.ID] = session
		}
	}

	plan := RepairPlan{ProjectPath: normalizedProject}
	for _, session := range candidates {
		if visible[session.ID] {
			plan.Items = append(plan.Items, RepairPlanItem{Session: session, Action: "skip", Reason: "already visible"})
			continue
		}
		if session.Status == domain.SessionStatusConflict {
			plan.Items = append(plan.Items, RepairPlanItem{Session: session, Action: "conflict", Reason: session.ConflictReason})
			continue
		}
		plan.Items = append(plan.Items, RepairPlanItem{Session: session, Action: "restore", Reason: string(session.Source)})
	}
	return plan
}

func ExecuteRepair(cfg config.Config, plan RepairPlan) RepairReport {
	report := RepairReport{ProjectPath: plan.ProjectPath}
	for _, item := range plan.Items {
		switch item.Action {
		case "skip":
			report.Skipped = append(report.Skipped, item)
		case "conflict":
			report.Conflicts = append(report.Conflicts, item)
		case "restore":
			result, err := RestoreSession(cfg, item.Session)
			if err != nil {
				report.Failed = append(report.Failed, fmt.Sprintf("%s: %v", item.Session.ID, err))
				continue
			}
			report.Restored = append(report.Restored, result)
		}
	}
	return report
}

func isRecoverable(session domain.SessionRecord) bool {
	switch session.Source {
	case domain.SessionSourceBackup, domain.SessionSourceRemoved, domain.SessionSourceOldHome, domain.SessionSourceArchived:
		return true
	default:
		return false
	}
}

func sourceRank(source domain.SessionSource) int {
	switch source {
	case domain.SessionSourceBackup:
		return 1
	case domain.SessionSourceRemoved:
		return 2
	case domain.SessionSourceArchived:
		return 3
	case domain.SessionSourceOldHome:
		return 4
	default:
		return 99
	}
}
