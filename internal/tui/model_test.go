package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sunlock/codex-session-manager/internal/app"
	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
)

func TestMoveColumnReturnsToPreviousRowInOriginalColumn(t *testing.T) {
	project := "/tmp/project"
	model := Model{
		page:            pageSessions,
		inventory:       domain.Inventory{Projects: []domain.ProjectRecord{{CWD: project}}},
		selectedColumn:  deletedColumn,
		selectedRow:     4,
		selectedProject: 0,
		columnRows:      newColumnRows(),
	}
	model.inventory.Sessions = append(model.inventory.Sessions, sessionsForColumn(project, archivedColumn, 2)...)
	model.inventory.Sessions = append(model.inventory.Sessions, sessionsForColumn(project, deletedColumn, 5)...)

	model.moveColumn(-1)
	if model.selectedColumn != archivedColumn || model.selectedRow != 1 {
		t.Fatalf("expected archived row 1, got column=%d row=%d", model.selectedColumn, model.selectedRow)
	}

	model.moveColumn(1)
	if model.selectedColumn != deletedColumn || model.selectedRow != 4 {
		t.Fatalf("expected deleted row 4, got column=%d row=%d", model.selectedColumn, model.selectedRow)
	}
}

func TestRestoreRowAfterMissingSessionKeepsSelectionInSameColumn(t *testing.T) {
	project := "/tmp/project"
	cases := []struct {
		name   string
		column int
	}{
		{name: "可见列", column: visibleColumn},
		{name: "不可见列", column: oldHomeColumn},
		{name: "归档列", column: archivedColumn},
		{name: "删除列", column: deletedColumn},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := Model{
				page:            pageSessions,
				inventory:       domain.Inventory{Projects: []domain.ProjectRecord{{CWD: project}}},
				selectedProject: 0,
				pendingProject:  project,
				pendingColumn:   tc.column,
				pendingRow:      4,
				columnRows:      newColumnRows(),
			}
			model.inventory.Sessions = append(model.inventory.Sessions, sessionsForColumn(project, tc.column, 4)...)

			model.restorePendingSelection()

			if model.selectedColumn != tc.column || model.selectedRow != 3 {
				t.Fatalf("expected column=%d row=3, got column=%d row=%d", tc.column, model.selectedColumn, model.selectedRow)
			}
		})
	}
}

func TestSessionColumnWidthExpandsWithTerminalWidth(t *testing.T) {
	model := Model{width: 166}
	if got := model.sessionColumnWidth(); got != 40 {
		t.Fatalf("expected width 40, got %d", got)
	}
}

func TestInactiveSessionGoesToInvisibleColumn(t *testing.T) {
	groups := groupSessions([]domain.SessionRecord{
		{
			ID:     "00000000-0000-0000-0000-000000000001",
			Source: domain.SessionSourceInactive,
			Status: domain.SessionStatusInactive,
		},
	})

	if len(groups[oldHomeColumn].Sessions) != 1 {
		t.Fatalf("expected inactive session in invisible column, got %+v", groups)
	}
	if label := statusLabel(groups[oldHomeColumn].Sessions[0]); label != "不可见" {
		t.Fatalf("expected invisible label, got %q", label)
	}
}

func TestGroupSessionsKeepsDeletedRecordsWithSameID(t *testing.T) {
	sessionID := "00000000-0000-0000-0000-000000000001"
	groups := groupSessions([]domain.SessionRecord{
		{
			ID:       sessionID,
			FilePath: "/tmp/backups/" + sessionID + "/session.jsonl",
			Source:   domain.SessionSourceBackup,
			Status:   domain.SessionStatusRecoverable,
		},
		{
			ID:       sessionID,
			FilePath: "/tmp/removed/" + sessionID + "/session.jsonl",
			Source:   domain.SessionSourceRemoved,
			Status:   domain.SessionStatusRemoved,
		},
	})

	if got := len(groups[deletedColumn].Sessions); got != 2 {
		t.Fatalf("expected two deleted records with same id, got %d", got)
	}
}

func TestRestorePendingSelectionPrefersSameFilePath(t *testing.T) {
	project := "/tmp/project"
	sessionID := "00000000-0000-0000-0000-000000000001"
	backupPath := "/tmp/backups/" + sessionID + "/session.jsonl"
	removedPath := "/tmp/removed/" + sessionID + "/session.jsonl"
	model := Model{
		page:               pageSessions,
		selectedProject:    0,
		pendingProject:     project,
		pendingSession:     sessionID,
		pendingSessionPath: removedPath,
		pendingColumn:      deletedColumn,
		columnRows:         newColumnRows(),
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{{CWD: project}},
			Sessions: []domain.SessionRecord{
				{ID: sessionID, CWD: project, FilePath: backupPath, Source: domain.SessionSourceBackup, Status: domain.SessionStatusRecoverable},
				{ID: sessionID, CWD: project, FilePath: removedPath, Source: domain.SessionSourceRemoved, Status: domain.SessionStatusRemoved},
			},
		},
	}

	model.restorePendingSelection()
	session, ok := model.currentSession()
	if !ok || session.FilePath != removedPath {
		t.Fatalf("expected selected removed path %q, got %+v ok=%v", removedPath, session, ok)
	}
}

func TestHandleRenameKeyAcceptsSpace(t *testing.T) {
	model := Model{renaming: true, renameInput: "旧"}
	updated, _ := model.handleRenameKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	got := updated.(Model).renameInput
	if got != "旧 " {
		t.Fatalf("expected space appended, got %q", got)
	}
}

func TestProjectFilterNarrowsByTypedCharacters(t *testing.T) {
	model := projectFilterModel()

	updated, _ := model.handleProjectKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(Model)
	if model.projectFilter != "c" || len(model.filteredProjects()) != 2 {
		t.Fatalf("expected filter c with 2 matches, got filter=%q matches=%d", model.projectFilter, len(model.filteredProjects()))
	}

	updated, _ = model.handleProjectKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	model = updated.(Model)
	if model.projectFilter != "cm" || len(model.filteredProjects()) != 1 || model.selectedProject != 1 {
		t.Fatalf("expected filter cm selecting project 1, got filter=%q matches=%d selected=%d", model.projectFilter, len(model.filteredProjects()), model.selectedProject)
	}

	updated, _ = model.handleProjectKey(tea.KeyMsg{Type: tea.KeyBackspace})
	model = updated.(Model)
	if model.projectFilter != "c" || len(model.filteredProjects()) != 2 {
		t.Fatalf("expected backspace to filter c with 2 matches, got filter=%q matches=%d", model.projectFilter, len(model.filteredProjects()))
	}
}

func TestProjectFilterEnterOpensOnlyMatchingProject(t *testing.T) {
	model := projectFilterModel()
	model.projectFilter = "cm"
	model.ensureFilteredProjectSelection()

	updated, _ := model.handleProjectKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.page != pageSessions || got.selectedProject != 1 {
		t.Fatalf("expected sessions page for project 1, got page=%d selected=%d", got.page, got.selectedProject)
	}
}

func TestProjectFilterEnterDoesNotOpenWhenNoMatch(t *testing.T) {
	model := projectFilterModel()
	model.projectFilter = "missing"
	model.ensureFilteredProjectSelection()

	updated, _ := model.handleProjectKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.page != pageProjects {
		t.Fatalf("expected to stay on projects page, got page=%d", got.page)
	}
}

func TestHighlightProjectFilterMarksMatchedText(t *testing.T) {
	got := highlightProjectFilter("/tmp/cm-tool", "cm")
	want := "/tmp/" + matchStyle.Render("cm") + "-tool"
	if got != want {
		t.Fatalf("expected highlighted match, got %q want %q", got, want)
	}
}

func TestFilteredProjectsSortsByProjectNameAscending(t *testing.T) {
	model := Model{
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{
				{CWD: "/tmp/zeta"},
				{CWD: "/var/alpha"},
				{CWD: "/tmp/beta"},
			},
		},
	}

	var got []string
	for _, item := range model.filteredProjects() {
		got = append(got, item.Project.CWD)
	}
	want := []string{"/var/alpha", "/tmp/beta", "/tmp/zeta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected sorted projects %v, got %v", want, got)
	}
}

func TestEscFromSessionsKeepsCurrentProjectSelected(t *testing.T) {
	model := Model{
		page:            pageSessions,
		selectedProject: 0,
		selectedColumn:  visibleColumn,
		selectedRow:     0,
		columnRows:      newColumnRows(),
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{
				{CWD: "/tmp/zeta"},
				{CWD: "/tmp/alpha"},
			},
			Sessions: []domain.SessionRecord{
				{ID: "00000000-0000-0000-0000-000000000001", CWD: "/tmp/zeta", Source: domain.SessionSourceVisible, Status: domain.SessionStatusVisible},
			},
		},
	}

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.page != pageProjects || got.selectedProject != 0 {
		t.Fatalf("expected projects page with zeta selected, got page=%d selected=%d", got.page, got.selectedProject)
	}
	project, ok := got.currentProject()
	if !ok || project.CWD != "/tmp/zeta" {
		t.Fatalf("expected current project /tmp/zeta, got %+v ok=%v", project, ok)
	}
}

func TestRenderProjectLineKeepsProjectNameReadableWithLongPath(t *testing.T) {
	model := Model{width: 64}
	project := domain.ProjectRecord{
		CWD:              "/Users/sunlock/work/some/really/long/path/with/many/segments/codex-session-manager",
		TotalSessions:    12,
		VisibleCount:     3,
		RecoverableCount: 4,
		BackedUpCount:    5,
	}

	line := model.renderProjectLine(project)
	lines := strings.Split(line, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two-line project item, got %d lines: %q", len(lines), line)
	}
	if !strings.Contains(lines[0], "codex-session-manager") {
		t.Fatalf("expected first line to keep project name, got %q", lines[0])
	}
	for _, item := range lines {
		if width := lipgloss.Width(item); width > model.projectLineWidth() {
			t.Fatalf("expected line width <= %d, got %d: %q", model.projectLineWidth(), width, item)
		}
	}
}

func TestHiddenProjectsAreSortedAndSelectable(t *testing.T) {
	model := Model{
		app: app.App{Config: config.Config{HiddenProjects: []string{"/tmp/zeta", "/tmp/alpha"}}},
	}
	projects := model.hiddenProjects()
	want := []string{"/tmp/alpha", "/tmp/zeta"}
	if !reflect.DeepEqual(projects, want) {
		t.Fatalf("expected sorted hidden projects %v, got %v", want, projects)
	}
	model.page = pageHiddenProjects
	model.selectedHiddenProject = 1
	project, ok := model.currentHiddenProject()
	if !ok || project != "/tmp/zeta" {
		t.Fatalf("expected /tmp/zeta selected, got %q ok=%v", project, ok)
	}
}

func TestUnhideProjectMsgReturnsToProjectsAndSelectsRestoredProject(t *testing.T) {
	model := Model{
		app:        app.App{Config: config.Config{HiddenProjects: []string{"/tmp/zeta"}}},
		page:       pageHiddenProjects,
		columnRows: newColumnRows(),
	}
	updated, _ := model.Update(unhideProjectMsg{projectPath: "/tmp/zeta"})
	got := updated.(Model)
	if got.page != pageProjects || len(got.app.Config.HiddenProjects) != 0 || got.pendingProject != "/tmp/zeta" {
		t.Fatalf("unexpected unhide state: page=%d hidden=%v pending=%q", got.page, got.app.Config.HiddenProjects, got.pendingProject)
	}
}

func TestTabSwitchesBetweenCodexAndClaudeInventories(t *testing.T) {
	codexInventory := domain.Inventory{Projects: []domain.ProjectRecord{{CWD: "/tmp/codex"}}}
	claudeInventory := domain.Inventory{Projects: []domain.ProjectRecord{{CWD: "/tmp/claude"}}}
	model := Model{
		provider:        providerCodex,
		page:            pageSessions,
		inventory:       codexInventory,
		codexInventory:  codexInventory,
		claudeInventory: claudeInventory,
		columnRows:      newColumnRows(),
	}

	updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(Model)
	if got.provider != providerClaude || got.page != pageProjects || got.inventory.Projects[0].CWD != "/tmp/claude" {
		t.Fatalf("unexpected Claude tab state: provider=%d page=%d inventory=%+v", got.provider, got.page, got.inventory)
	}

	updated, _ = got.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(Model)
	if got.provider != providerCodex || got.inventory.Projects[0].CWD != "/tmp/codex" {
		t.Fatalf("unexpected Codex tab state: provider=%d inventory=%+v", got.provider, got.inventory)
	}
}

func TestClaudeModeUsesSingleSessionColumn(t *testing.T) {
	model := Model{provider: providerClaude, width: 80}
	columns := model.sessionColumns([]domain.SessionRecord{{
		ID:     "11111111-1111-1111-1111-111111111111",
		Name:   "Claude session",
		Source: domain.SessionSourceVisible,
		Status: domain.SessionStatusVisible,
	}})
	if len(columns) != 1 || !strings.Contains(columns[0], "本地会话") {
		t.Fatalf("expected one Claude session column, got %v", columns)
	}
}

func TestClaudeDeleteKeyDeletesSessionWithoutBackup(t *testing.T) {
	root := t.TempDir()
	project := "/tmp/claude-project"
	id := "11111111-1111-1111-1111-111111111111"
	projectDir := filepath.Join(root, "claude", "projects", "-tmp-claude-project")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(projectDir, id+".jsonl")
	if err := os.WriteFile(transcript, []byte(`{"type":"user","sessionId":"`+id+`","cwd":"`+project+`","message":{"content":"测试删除"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "claude", "history.jsonl"), []byte(`{"display":"测试删除","project":"`+project+`","sessionId":"`+id+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := Model{
		app: app.App{Config: config.Config{
			ClaudeHome: filepath.Join(root, "claude"),
			ToolHome:   filepath.Join(root, "tool"),
		}},
		provider:        providerClaude,
		page:            pageSessions,
		selectedProject: 0,
		selectedColumn:  visibleColumn,
		columnRows:      newColumnRows(),
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{{CWD: project}},
			Sessions: []domain.SessionRecord{{
				ID:       id,
				Name:     "测试删除",
				CWD:      project,
				FilePath: transcript,
				Source:   domain.SessionSourceVisible,
				Status:   domain.SessionStatusVisible,
			}},
		},
	}

	updated, cmd := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd == nil {
		t.Fatal("expected Claude delete command")
	}
	msg := cmd()
	action, ok := msg.(actionMsg)
	if !ok || action.err != nil || !strings.Contains(action.message, "已删除") {
		t.Fatalf("unexpected delete result: %#v", msg)
	}
	if _, err := os.Stat(transcript); !os.IsNotExist(err) {
		t.Fatalf("expected transcript deleted, err=%v", err)
	}
	if updated.(Model).provider != providerClaude {
		t.Fatal("expected model to remain in Claude mode")
	}
	if _, err := os.Stat(filepath.Join(root, "backups")); !os.IsNotExist(err) {
		t.Fatalf("expected no backup directory, err=%v", err)
	}
}

func TestEscFromDetailReturnsToSessionList(t *testing.T) {
	for _, currentProvider := range []provider{providerCodex, providerClaude} {
		model := Model{
			provider:        currentProvider,
			page:            pageDetail,
			selectedProject: 0,
			selectedColumn:  visibleColumn,
			columnRows:      newColumnRows(),
			inventory: domain.Inventory{
				Projects: []domain.ProjectRecord{{CWD: "/tmp/project"}},
				Sessions: []domain.SessionRecord{{
					ID:     "11111111-1111-1111-1111-111111111111",
					CWD:    "/tmp/project",
					Source: domain.SessionSourceVisible,
					Status: domain.SessionStatusVisible,
				}},
			},
		}

		updated, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
		got := updated.(Model)
		if got.page != pageSessions {
			t.Fatalf("provider %d: expected sessions page, got %d", currentProvider, got.page)
		}
	}
}

func TestDeleteLastProjectSessionReturnsToProjectList(t *testing.T) {
	model := Model{
		provider:       providerClaude,
		page:           pageSessions,
		pendingProject: "/tmp/deleted-project",
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{{CWD: "/tmp/deleted-project"}, {CWD: "/tmp/next-project"}},
		},
		columnRows: newColumnRows(),
	}

	updated, _ := model.Update(actionMsg{
		forgetSession:                   true,
		returnToProjectsIfProjectAbsent: true,
	})
	model = updated.(Model)
	updated, _ = model.Update(scanMsg{
		provider: providerClaude,
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{{CWD: "/tmp/next-project"}},
		},
	})
	got := updated.(Model)
	if got.page != pageProjects {
		t.Fatalf("expected projects page after deleting project's last session, got %d", got.page)
	}
}

func TestDeleteSessionStaysOnProjectWhenSessionsRemain(t *testing.T) {
	model := Model{
		provider:       providerClaude,
		page:           pageSessions,
		pendingProject: "/tmp/project",
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{{CWD: "/tmp/project"}},
		},
		columnRows: newColumnRows(),
	}

	updated, _ := model.Update(actionMsg{
		forgetSession:                   true,
		returnToProjectsIfProjectAbsent: true,
	})
	model = updated.(Model)
	updated, _ = model.Update(scanMsg{
		provider: providerClaude,
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{{CWD: "/tmp/project"}},
			Sessions: []domain.SessionRecord{{CWD: "/tmp/project"}},
		},
	})
	got := updated.(Model)
	if got.page != pageSessions {
		t.Fatalf("expected sessions page while project still has sessions, got %d", got.page)
	}
}

func projectFilterModel() Model {
	return Model{
		page:            pageProjects,
		selectedProject: 0,
		columnRows:      newColumnRows(),
		inventory: domain.Inventory{
			Projects: []domain.ProjectRecord{
				{CWD: "/tmp/codex"},
				{CWD: "/tmp/cm-tool"},
				{CWD: "/tmp/wiki"},
			},
		},
	}
}

func sessionsForColumn(project string, column int, count int) []domain.SessionRecord {
	sessions := make([]domain.SessionRecord, 0, count)
	for i := 0; i < count; i++ {
		session := domain.SessionRecord{
			ID:   fmt.Sprintf("00000000-0000-0000-0000-%012d", column*100+i),
			Name: fmt.Sprintf("会话 %d-%d", column, i),
			CWD:  project,
		}
		switch column {
		case visibleColumn:
			session.Source = domain.SessionSourceVisible
			session.Status = domain.SessionStatusVisible
		case oldHomeColumn:
			session.Source = domain.SessionSourceOldHome
			session.Status = domain.SessionStatusRecoverable
		case archivedColumn:
			session.Source = domain.SessionSourceArchived
			session.Status = domain.SessionStatusArchived
		case deletedColumn:
			session.Source = domain.SessionSourceRemoved
			session.Status = domain.SessionStatusRemoved
		}
		sessions = append(sessions, session)
	}
	return sessions
}
