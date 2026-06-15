package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sunlock/codex-session-manager/internal/app"
	"github.com/sunlock/codex-session-manager/internal/domain"
	restoresvc "github.com/sunlock/codex-session-manager/internal/restore"
)

type page int

const (
	pageProjects page = iota
	pageSessions
	pageDetail
	pageHiddenProjects
)

type scanMsg struct {
	inventory domain.Inventory
	err       error
}

type actionMsg struct {
	message       string
	forgetSession bool
	err           error
}

type renameMsg struct {
	sessionID string
	name      string
	err       error
}

type hideProjectMsg struct {
	projectPath string
	row         int
	err         error
}

type unhideProjectMsg struct {
	projectPath string
	row         int
	err         error
}

type Model struct {
	app                   app.App
	page                  page
	inventory             domain.Inventory
	selectedProject       int
	selectedHiddenProject int
	selectedColumn        int
	selectedRow           int
	columnRows            []int
	pendingProject        string
	pendingSession        string
	pendingSessionPath    string
	pendingColumn         int
	pendingRow            int
	renaming              bool
	renameInput           string
	renameSession         domain.SessionRecord
	projectFilter         string
	pendingProjectRow     int
	pendingProjectRowSet  bool
	status                string
	err                   error
	width                 int
	height                int
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	matchStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("58"))
)

func New(a app.App) Model {
	return Model{app: a, status: "正在扫描...", columnRows: newColumnRows(), pendingProjectRow: -1}
}

func (m Model) Init() tea.Cmd {
	return m.scanCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case scanMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "扫描失败"
			return m, nil
		}
		m.inventory = msg.inventory
		m.err = nil
		m.status = fmt.Sprintf("已加载 %d 个项目，%d 个会话", len(msg.inventory.Projects), len(msg.inventory.Sessions))
		m.restorePendingSelection()
		m.clampSelection()
	case actionMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "操作失败"
			return m, nil
		}
		m.err = nil
		m.status = msg.message
		if msg.forgetSession {
			m.pendingSession = ""
			m.pendingSessionPath = ""
		}
		return m, m.scanCmd()
	case renameMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "重命名失败"
			m.renaming = false
			return m, nil
		}
		m.err = nil
		m.status = "已重命名 " + msg.name
		m.renaming = false
		m.pendingSession = msg.sessionID
		return m, m.scanCmd()
	case hideProjectMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "隐藏项目失败"
			return m, nil
		}
		m.err = nil
		m.status = "已隐藏项目 " + projectDisplayName(domain.ProjectRecord{CWD: msg.projectPath})
		m.app.Config.HiddenProjects = appendHiddenProject(m.app.Config.HiddenProjects, msg.projectPath)
		m.pendingProject = ""
		m.pendingSession = ""
		m.pendingSessionPath = ""
		m.pendingProjectRow = msg.row
		m.pendingProjectRowSet = true
		return m, m.scanCmd()
	case unhideProjectMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "恢复项目失败"
			return m, nil
		}
		m.err = nil
		m.status = "已恢复项目 " + projectDisplayName(domain.ProjectRecord{CWD: msg.projectPath})
		m.app.Config.HiddenProjects = removeHiddenProject(m.app.Config.HiddenProjects, msg.projectPath)
		m.selectedHiddenProject = msg.row
		m.pendingProject = msg.projectPath
		m.pendingSession = ""
		m.pendingSessionPath = ""
		m.page = pageProjects
		return m, m.scanCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) View() string {
	var body string
	switch m.page {
	case pageProjects:
		body = m.viewProjects()
	case pageSessions:
		body = m.viewSessions()
	case pageDetail:
		body = m.viewDetail()
	case pageHiddenProjects:
		body = m.viewHiddenProjects()
	}
	status := m.status
	if m.err != nil {
		status = errorStyle.Render(m.err.Error())
	} else if status != "" {
		status = successStyle.Render(status)
	}
	if m.renaming {
		renameLine := titleStyle.Render("重命名：") + m.renameInput + mutedStyle.Render("  Enter 保存  Esc 取消")
		return lipgloss.JoinVertical(lipgloss.Left, body, "", renameLine)
	}
	help := m.helpText()
	return lipgloss.JoinVertical(lipgloss.Left, body, "", status, help)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.renaming {
		return m.handleRenameKey(msg)
	}
	if m.page == pageProjects {
		return m.handleProjectKey(msg)
	}
	if m.page == pageHiddenProjects {
		return m.handleHiddenProjectKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.page == pageProjects {
			return m, nil
		}
		m.capturePendingSelection()
		m.page = pageProjects
		m.ensureFilteredProjectSelection()
		return m, nil
	case "s":
		m.status = "正在扫描..."
		m.capturePendingSelection()
		return m, m.scanCmd()
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "enter":
		if m.page == pageProjects && len(m.inventory.Projects) > 0 {
			m.page = pageSessions
			m.selectedColumn = 0
			m.selectedRow = 0
			m.syncColumnRow()
			m.ensureSessionSelection()
		} else if m.page == pageSessions && len(m.projectSessions()) > 0 {
			m.page = pageDetail
		}
	case "left", "h":
		if m.page == pageSessions || m.page == pageDetail {
			m.moveColumn(-1)
		}
	case "right", "l":
		if m.page == pageSessions || m.page == pageDetail {
			m.moveColumn(1)
		}
	case "b":
		if m.page == pageSessions || m.page == pageDetail {
			session, ok := m.currentSession()
			if ok {
				m.capturePendingSelection()
				return m, m.backupCmd(session.ID)
			}
		}
	case "n":
		if m.page == pageSessions || m.page == pageDetail {
			session, ok := m.currentSession()
			if ok {
				m.renaming = true
				m.renameSession = session
				m.renameInput = displaySessionTitle(session, 120)
				return m, nil
			}
		}
	case "g":
		if m.page == pageSessions || m.page == pageDetail {
			project, ok := m.currentProject()
			if ok {
				m.capturePendingSelection()
				return m, m.repairCmd(project.CWD)
			}
		}
	case "a":
		if m.page == pageSessions || m.page == pageDetail {
			session, ok := m.currentSession()
			if ok {
				m.capturePendingSelection()
				return m, m.archiveCmd(session)
			}
		}
	case "m":
		if m.page == pageSessions || m.page == pageDetail {
			session, ok := m.currentSession()
			if ok {
				m.capturePendingSelection()
				return m, m.removeCmd(session)
			}
		}
	case "r":
		if m.page == pageProjects {
			project, ok := m.currentProject()
			if ok {
				m.capturePendingSelection()
				return m, m.repairCmd(project.CWD)
			}
		}
		if m.page == pageSessions || m.page == pageDetail {
			session, ok := m.currentSession()
			if ok {
				m.capturePendingSelection()
				return m, m.restoreSessionCmd(session)
			}
		}
	}
	return m, nil
}

func (m Model) helpText() string {
	if m.page == pageProjects {
		return mutedStyle.Render("输入字符筛选项目  Backspace 删除筛选  Esc 清空筛选  ↑/↓ 选择  Enter 打开  Ctrl+D 隐藏项目  Ctrl+U 隐藏列表  Ctrl+R 扫描  Ctrl+C 退出")
	}
	if m.page == pageHiddenProjects {
		return mutedStyle.Render("↑/↓ 选择  Enter/Ctrl+U 恢复项目  Esc 返回  Ctrl+C 退出")
	}
	return mutedStyle.Render("↑/↓ 行选择  ←/→ 列选择  Enter 打开  s 扫描  g 一键加载  n 重命名  b 备份  a 归档  r 恢复  m 删除  Esc 返回  q 退出")
}

func (m Model) handleProjectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "ctrl+r", "f5":
		m.status = "正在扫描..."
		m.capturePendingSelection()
		return m, m.scanCmd()
	case "ctrl+d":
		project, ok := m.currentProject()
		if ok && len(m.filteredProjects()) > 0 {
			row := m.filteredProjectPosition(m.filteredProjects())
			m.status = "正在隐藏项目..."
			return m, m.hideProjectCmd(project.CWD, row)
		}
		return m, nil
	case "ctrl+u":
		m.page = pageHiddenProjects
		m.ensureHiddenProjectSelection()
		return m, nil
	case "esc":
		if m.projectFilter != "" {
			m.projectFilter = ""
			m.status = "已清空项目筛选"
			m.ensureFilteredProjectSelection()
		}
		return m, nil
	case "backspace", "ctrl+h":
		runes := []rune(m.projectFilter)
		if len(runes) > 0 {
			m.projectFilter = string(runes[:len(runes)-1])
			m.ensureFilteredProjectSelection()
		}
		return m, nil
	case "up":
		m.move(-1)
		return m, nil
	case "down":
		m.move(1)
		return m, nil
	case "enter":
		if len(m.filteredProjects()) > 0 {
			m.ensureFilteredProjectSelection()
			m.page = pageSessions
			m.selectedColumn = 0
			m.selectedRow = 0
			m.syncColumnRow()
			m.ensureSessionSelection()
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.projectFilter += msg.String()
			m.ensureFilteredProjectSelection()
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleHiddenProjectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.page = pageProjects
		m.ensureFilteredProjectSelection()
		return m, nil
	case "up", "k":
		m.moveHiddenProject(-1)
		return m, nil
	case "down", "j":
		m.moveHiddenProject(1)
		return m, nil
	case "enter", "ctrl+u":
		projectPath, ok := m.currentHiddenProject()
		if ok {
			row := m.selectedHiddenProject
			m.status = "正在恢复项目..."
			return m, m.unhideProjectCmd(projectPath, row)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.renaming = false
		m.renameInput = ""
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.renameInput)
		if name == "" {
			m.renaming = false
			return m, nil
		}
		m.capturePendingSelection()
		return m, m.renameCmd(m.renameSession, name)
	case "backspace", "ctrl+h":
		runes := []rune(m.renameInput)
		if len(runes) > 0 {
			m.renameInput = string(runes[:len(runes)-1])
		}
		return m, nil
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.renameInput += msg.String()
		}
		return m, nil
	}
}

func (m Model) scanCmd() tea.Cmd {
	return func() tea.Msg {
		inventory, err := m.app.Scan()
		return scanMsg{inventory: inventory, err: err}
	}
}

func (m *Model) capturePendingSelection() {
	if project, ok := m.currentProject(); ok {
		m.pendingProject = project.CWD
	}
	if session, ok := m.currentSession(); ok {
		m.pendingSession = session.ID
		m.pendingSessionPath = session.FilePath
		m.pendingColumn = m.selectedColumn
		m.pendingRow = m.selectedRow
	}
}

func (m *Model) restorePendingSelection() {
	m.ensureColumnRows()
	if m.pendingProjectRowSet {
		m.restoreProjectRowAfterMissingProject()
		m.pendingProjectRow = -1
		m.pendingProjectRowSet = false
		return
	}
	if m.pendingProject != "" {
		for i, project := range m.inventory.Projects {
			if project.CWD == m.pendingProject {
				m.selectedProject = i
				break
			}
		}
	}
	if m.pendingSession != "" {
		if m.selectSession(m.pendingSession, m.pendingSessionPath) {
			m.pendingProject = ""
			m.pendingSession = ""
			m.pendingSessionPath = ""
			return
		}
	}
	if m.pendingProject != "" {
		m.restoreRowAfterMissingSession()
	}
	m.pendingProject = ""
	m.pendingSession = ""
	m.pendingSessionPath = ""
}

func (m Model) backupCmd(sessionID string) tea.Cmd {
	return func() tea.Msg {
		manifest, err := m.app.Backup(sessionID)
		return actionMsg{message: "已备份 " + manifest.SessionID, err: err}
	}
}

func (m Model) removeCmd(session domain.SessionRecord) tea.Cmd {
	return func() tea.Msg {
		result, err := m.app.RemoveSession(session)
		message := "已移出 "
		if session.Source == domain.SessionSourceBackup || session.Source == domain.SessionSourceRemoved || session.Source == domain.SessionSourceArchived {
			message = "已彻底删除 "
		}
		return actionMsg{message: message + result.SessionID, forgetSession: true, err: err}
	}
}

func (m Model) archiveCmd(session domain.SessionRecord) tea.Cmd {
	return func() tea.Msg {
		result, err := m.app.ArchiveSession(session)
		return actionMsg{message: "已归档 " + result.SessionID, err: err}
	}
}

func (m Model) renameCmd(session domain.SessionRecord, name string) tea.Cmd {
	return func() tea.Msg {
		err := m.app.RenameSession(session, name)
		return renameMsg{sessionID: session.ID, name: name, err: err}
	}
}

func (m Model) restoreSessionCmd(session domain.SessionRecord) tea.Cmd {
	return func() tea.Msg {
		result, err := m.app.RestoreSession(session)
		return actionMsg{message: "已恢复 " + result.SessionID, err: err}
	}
}

func (m Model) repairCmd(projectPath string) tea.Cmd {
	return func() tea.Msg {
		report, err := m.app.Repair(projectPath)
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{message: repairSummary(report)}
	}
}

func (m Model) hideProjectCmd(projectPath string, row int) tea.Cmd {
	return func() tea.Msg {
		err := m.app.HideProject(projectPath)
		return hideProjectMsg{projectPath: projectPath, row: row, err: err}
	}
}

func (m Model) unhideProjectCmd(projectPath string, row int) tea.Cmd {
	return func() tea.Msg {
		err := m.app.UnhideProject(projectPath)
		return unhideProjectMsg{projectPath: projectPath, row: row, err: err}
	}
}

func repairSummary(report restoresvc.RepairReport) string {
	return fmt.Sprintf("加载完成：恢复=%d 跳过=%d 冲突=%d 失败=%d", len(report.Restored), len(report.Skipped), len(report.Conflicts), len(report.Failed))
}

func (m *Model) move(delta int) {
	switch m.page {
	case pageProjects:
		before := m.selectedProject
		m.moveProject(delta)
		if before != m.selectedProject {
			m.resetSessionSelection()
		}
	case pageSessions, pageDetail:
		m.moveRow(delta)
	}
}

func (m *Model) clampSelection() {
	m.ensureFilteredProjectSelection()
	m.ensureSessionSelection()
}

func (m *Model) resetSessionSelection() {
	m.selectedColumn = 0
	m.selectedRow = 0
	m.columnRows = newColumnRows()
}

func (m Model) currentProject() (domain.ProjectRecord, bool) {
	if m.selectedProject < 0 || m.selectedProject >= len(m.inventory.Projects) {
		return domain.ProjectRecord{}, false
	}
	return m.inventory.Projects[m.selectedProject], true
}

func (m *Model) moveProject(delta int) {
	filtered := m.filteredProjects()
	if len(filtered) == 0 {
		m.selectedProject = 0
		return
	}
	position := m.filteredProjectPosition(filtered)
	position += delta
	if position < 0 {
		position = 0
	}
	if position >= len(filtered) {
		position = len(filtered) - 1
	}
	m.selectedProject = filtered[position].Index
}

func (m *Model) ensureFilteredProjectSelection() {
	filtered := m.filteredProjects()
	if len(filtered) == 0 {
		m.selectedProject = 0
		return
	}
	if m.selectedProject < 0 || m.selectedProject >= len(m.inventory.Projects) || !m.projectMatchesFilter(m.inventory.Projects[m.selectedProject]) {
		m.selectedProject = filtered[0].Index
		return
	}
}

func (m Model) filteredProjectPosition(filtered []filteredProject) int {
	for i, project := range filtered {
		if project.Index == m.selectedProject {
			return i
		}
	}
	return 0
}

func (m *Model) restoreProjectRowAfterMissingProject() {
	filtered := m.filteredProjects()
	if len(filtered) == 0 {
		m.selectedProject = 0
		return
	}
	row := clampRow(m.pendingProjectRow, len(filtered))
	m.selectedProject = filtered[row].Index
}

func (m *Model) moveHiddenProject(delta int) {
	projects := m.hiddenProjects()
	if len(projects) == 0 {
		m.selectedHiddenProject = 0
		return
	}
	m.selectedHiddenProject += delta
	if m.selectedHiddenProject < 0 {
		m.selectedHiddenProject = 0
	}
	if m.selectedHiddenProject >= len(projects) {
		m.selectedHiddenProject = len(projects) - 1
	}
}

func (m *Model) ensureHiddenProjectSelection() {
	projects := m.hiddenProjects()
	if len(projects) == 0 {
		m.selectedHiddenProject = 0
		return
	}
	m.selectedHiddenProject = clampRow(m.selectedHiddenProject, len(projects))
}

func (m Model) currentHiddenProject() (string, bool) {
	projects := m.hiddenProjects()
	if m.selectedHiddenProject < 0 || m.selectedHiddenProject >= len(projects) {
		return "", false
	}
	return projects[m.selectedHiddenProject], true
}

func (m Model) currentSession() (domain.SessionRecord, bool) {
	columns := groupSessions(m.projectSessions())
	if m.selectedColumn < 0 || m.selectedColumn >= len(columns) {
		return domain.SessionRecord{}, false
	}
	sessions := columns[m.selectedColumn].Sessions
	if m.selectedRow < 0 || m.selectedRow >= len(sessions) {
		return domain.SessionRecord{}, false
	}
	return sessions[m.selectedRow], true
}

func (m Model) projectSessions() []domain.SessionRecord {
	project, ok := m.currentProject()
	if !ok {
		return nil
	}
	var sessions []domain.SessionRecord
	for _, session := range m.inventory.Sessions {
		if session.CWD == project.CWD {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func (m *Model) moveRow(delta int) {
	columns := groupSessions(m.projectSessions())
	if len(columns) == 0 {
		return
	}
	m.ensureSessionSelection()
	if m.selectedColumn < 0 || m.selectedColumn >= len(columns) {
		return
	}
	maxRows := len(columns[m.selectedColumn].Sessions)
	if maxRows == 0 {
		return
	}
	m.selectedRow += delta
	if m.selectedRow < 0 {
		m.selectedRow = 0
	}
	if m.selectedRow >= maxRows {
		m.selectedRow = maxRows - 1
	}
	m.syncColumnRow()
}

func (m *Model) moveColumn(delta int) {
	columns := groupSessions(m.projectSessions())
	if len(columns) == 0 {
		return
	}
	m.ensureSessionSelection()
	m.syncColumnRow()
	next := m.selectedColumn
	for i := 0; i < len(sessionColumnTitles); i++ {
		next += delta
		if next < 0 {
			next = len(sessionColumnTitles) - 1
		}
		if next >= len(sessionColumnTitles) {
			next = 0
		}
		if len(columns[next].Sessions) > 0 {
			m.selectedColumn = next
			m.selectedRow = m.rowForColumn(next, columns)
			return
		}
	}
}

func (m *Model) ensureSessionSelection() {
	columns := groupSessions(m.projectSessions())
	if len(columns) == 0 {
		m.selectedColumn = 0
		m.selectedRow = 0
		return
	}
	if m.selectedColumn < 0 || m.selectedColumn >= len(columns) || len(columns[m.selectedColumn].Sessions) == 0 {
		m.selectedColumn = firstNonEmptyColumn(columns)
		m.selectedRow = m.rowForColumn(m.selectedColumn, columns)
	}
	if m.selectedColumn < 0 {
		m.selectedColumn = 0
		m.selectedRow = 0
		return
	}
	if m.selectedRow < 0 {
		m.selectedRow = 0
	}
	if m.selectedRow >= len(columns[m.selectedColumn].Sessions) {
		m.selectedRow = len(columns[m.selectedColumn].Sessions) - 1
	}
	m.syncColumnRow()
}

func (m *Model) selectSession(sessionID string, filePath string) bool {
	columns := groupSessions(m.projectSessions())
	if filePath != "" {
		if m.selectSessionInColumns(columns, func(session domain.SessionRecord) bool {
			return session.ID == sessionID && session.FilePath == filePath
		}) {
			return true
		}
	}
	return m.selectSessionInColumns(columns, func(session domain.SessionRecord) bool {
		return session.ID == sessionID
	})
}

func (m *Model) selectSessionInColumns(columns []sessionColumn, matches func(domain.SessionRecord) bool) bool {
	if m.pendingColumn >= 0 && m.pendingColumn < len(columns) {
		for rowIndex, session := range columns[m.pendingColumn].Sessions {
			if matches(session) {
				m.selectedColumn = m.pendingColumn
				m.selectedRow = rowIndex
				m.syncColumnRow()
				return true
			}
		}
	}
	for _, column := range fallbackColumnOrder(m.pendingColumn) {
		if column < 0 || column >= len(columns) {
			continue
		}
		for rowIndex, session := range columns[column].Sessions {
			if matches(session) {
				m.selectedColumn = column
				m.selectedRow = rowIndex
				m.syncColumnRow()
				return true
			}
		}
	}
	m.ensureSessionSelection()
	return false
}

func firstNonEmptyColumn(columns []sessionColumn) int {
	for i, column := range columns {
		if len(column.Sessions) > 0 {
			return i
		}
	}
	return -1
}

func (m *Model) restoreRowAfterMissingSession() {
	columns := groupSessions(m.projectSessions())
	if len(columns) == 0 {
		m.selectedColumn = 0
		m.selectedRow = 0
		m.syncColumnRow()
		return
	}
	if m.pendingColumn >= 0 && m.pendingColumn < len(columns) && len(columns[m.pendingColumn].Sessions) > 0 {
		m.selectedColumn = m.pendingColumn
		m.selectedRow = clampRow(m.pendingRow, len(columns[m.pendingColumn].Sessions))
		m.syncColumnRow()
		return
	}
	m.selectedRow = 0
	m.ensureSessionSelection()
}

func clampRow(row int, length int) int {
	if length <= 0 {
		return 0
	}
	if row < 0 {
		return 0
	}
	if row >= length {
		return length - 1
	}
	return row
}

func (m *Model) ensureColumnRows() {
	if len(m.columnRows) == len(sessionColumnTitles) {
		return
	}
	rows := newColumnRows()
	copy(rows, m.columnRows)
	m.columnRows = rows
}

func (m *Model) syncColumnRow() {
	m.ensureColumnRows()
	if m.selectedColumn < 0 || m.selectedColumn >= len(m.columnRows) {
		return
	}
	m.columnRows[m.selectedColumn] = m.selectedRow
}

func (m *Model) rowForColumn(column int, columns []sessionColumn) int {
	m.ensureColumnRows()
	if column < 0 || column >= len(columns) || len(columns[column].Sessions) == 0 {
		return 0
	}
	row := m.selectedRow
	if column < len(m.columnRows) && m.columnRows[column] >= 0 {
		row = m.columnRows[column]
	}
	if row < 0 {
		row = 0
	}
	if row >= len(columns[column].Sessions) {
		row = len(columns[column].Sessions) - 1
	}
	return row
}

func newColumnRows() []int {
	rows := make([]int, len(sessionColumnTitles))
	for i := range rows {
		rows[i] = -1
	}
	return rows
}

func (m Model) viewProjects() string {
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("codex-session-manager / 项目列表"))
	builder.WriteString("\n\n")
	if len(m.inventory.Projects) == 0 {
		builder.WriteString("没有扫描到项目。\n")
		return builder.String()
	}
	filtered := m.filteredProjects()
	if m.projectFilter != "" {
		builder.WriteString(mutedStyle.Render(fmt.Sprintf("筛选：%s  匹配=%d/%d", m.projectFilter, len(filtered), len(m.inventory.Projects))))
		builder.WriteString("\n\n")
	}
	if len(filtered) == 0 {
		builder.WriteString("没有匹配的项目。\n")
		return builder.String()
	}
	for _, item := range filtered {
		project := item.Project
		line := m.renderProjectLine(project)
		if item.Index == m.selectedProject {
			line = selectedStyle.Render(line)
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func (m Model) viewHiddenProjects() string {
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("隐藏项目"))
	builder.WriteString("\n\n")
	projects := m.hiddenProjects()
	if len(projects) == 0 {
		builder.WriteString("没有隐藏项目。\n")
		return builder.String()
	}
	for i, projectPath := range projects {
		project := domain.ProjectRecord{CWD: projectPath}
		line := projectDisplayName(project) + "\n" + mutedStyle.Render(strings.Repeat(" ", projectPathIndent)+trim(projectPath, m.projectLineWidth()-projectPathIndent))
		if i == m.selectedHiddenProject {
			line = selectedStyle.Render(line)
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func (m Model) renderProjectLine(project domain.ProjectRecord) string {
	width := m.projectLineWidth()
	name := highlightProjectFilter(trim(projectDisplayName(project), width), m.projectFilter)
	firstLine := lipgloss.NewStyle().Width(width).Render(name)

	stats := fmt.Sprintf("总数=%d 可见=%d 可恢复=%d 备份=%d", project.TotalSessions, project.VisibleCount, project.RecoverableCount, project.BackedUpCount)
	prefix := strings.Repeat(" ", projectPathIndent) + trim(stats, width-projectPathIndent)
	pathWidth := width - lipgloss.Width(prefix) - projectColumnGap
	secondLine := prefix
	if pathWidth > 0 {
		path := highlightProjectFilter(trim(project.CWD, pathWidth), m.projectFilter)
		secondLine += strings.Repeat(" ", projectColumnGap) + path
	}
	secondLine = mutedStyle.Render(secondLine)
	return lipgloss.JoinVertical(lipgloss.Left, firstLine, secondLine)
}

func (m Model) projectLineWidth() int {
	if m.width <= 0 {
		return defaultProjectLineWidth
	}
	if m.width < minProjectLineWidth {
		return minProjectLineWidth
	}
	return m.width
}

const (
	defaultProjectLineWidth = 96
	minProjectLineWidth     = 48
	projectColumnGap        = 2
	projectPathIndent       = 2
)

type filteredProject struct {
	Index   int
	Project domain.ProjectRecord
}

func (m Model) filteredProjects() []filteredProject {
	projects := make([]filteredProject, 0, len(m.inventory.Projects))
	for index, project := range m.inventory.Projects {
		if !m.projectMatchesFilter(project) {
			continue
		}
		projects = append(projects, filteredProject{Index: index, Project: project})
	}
	sort.SliceStable(projects, func(i, j int) bool {
		leftName := strings.ToLower(projectDisplayName(projects[i].Project))
		rightName := strings.ToLower(projectDisplayName(projects[j].Project))
		if leftName == rightName {
			return strings.ToLower(projects[i].Project.CWD) < strings.ToLower(projects[j].Project.CWD)
		}
		return leftName < rightName
	})
	return projects
}

func (m Model) hiddenProjects() []string {
	projects := make([]string, 0, len(m.app.Config.HiddenProjects))
	seen := map[string]bool{}
	for _, projectPath := range m.app.Config.HiddenProjects {
		projectPath = strings.TrimSpace(projectPath)
		if projectPath == "" || seen[projectPath] {
			continue
		}
		seen[projectPath] = true
		projects = append(projects, projectPath)
	}
	sort.SliceStable(projects, func(i, j int) bool {
		left := projectDisplayName(domain.ProjectRecord{CWD: projects[i]})
		right := projectDisplayName(domain.ProjectRecord{CWD: projects[j]})
		if strings.EqualFold(left, right) {
			return strings.ToLower(projects[i]) < strings.ToLower(projects[j])
		}
		return strings.ToLower(left) < strings.ToLower(right)
	})
	return projects
}

func appendHiddenProject(projects []string, projectPath string) []string {
	projectPath = strings.TrimSpace(projectPath)
	if projectPath == "" {
		return projects
	}
	for _, existing := range projects {
		if existing == projectPath {
			return projects
		}
	}
	return append(projects, projectPath)
}

func removeHiddenProject(projects []string, projectPath string) []string {
	result := make([]string, 0, len(projects))
	for _, existing := range projects {
		if existing == projectPath {
			continue
		}
		result = append(result, existing)
	}
	return result
}

func projectDisplayName(project domain.ProjectRecord) string {
	name := filepath.Base(strings.TrimRight(project.CWD, string(filepath.Separator)))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return project.CWD
	}
	return name
}

func (m Model) projectMatchesFilter(project domain.ProjectRecord) bool {
	filter := m.projectFilter
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(project.CWD), strings.ToLower(filter))
}

func highlightProjectFilter(value string, filter string) string {
	if filter == "" {
		return value
	}
	lowerValue := strings.ToLower(value)
	lowerFilter := strings.ToLower(filter)
	index := strings.Index(lowerValue, lowerFilter)
	if index < 0 {
		return value
	}
	end := index + len(lowerFilter)
	return value[:index] + matchStyle.Render(value[index:end]) + value[end:]
}

func (m Model) viewSessions() string {
	project, ok := m.currentProject()
	if !ok {
		return "没有选中项目。"
	}
	sessions := m.projectSessions()
	var builder strings.Builder
	builder.WriteString(titleStyle.Render("会话列表"))
	builder.WriteString(" ")
	builder.WriteString(mutedStyle.Render(project.CWD))
	builder.WriteString("\n\n")
	if len(sessions) == 0 {
		builder.WriteString("没有扫描到会话。\n")
		return builder.String()
	}

	selected, _ := m.currentSession()
	columns := m.sessionColumns(sessions)
	builder.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, columns...))
	builder.WriteString("\n\n")
	if selected.ID != "" {
		builder.WriteString(mutedStyle.Render("当前选中："))
		builder.WriteString(displaySessionTitle(selected, 64))
		builder.WriteString(mutedStyle.Render("  [" + selected.ID + "]"))
	}
	return builder.String()
}

func (m Model) viewDetail() string {
	session, ok := m.currentSession()
	if !ok {
		return "没有选中会话。"
	}
	return fmt.Sprintf(
		"%s\n\nID：%s\n名称：%s\n状态：%s\n来源：%s\n项目：%s\n文件：%s\n原路径：%s\nCLI：%s\n服务商：%s\n大小：%d\nSHA256：%s\n",
		titleStyle.Render("会话详情"),
		session.ID,
		displaySessionTitle(session, 120),
		statusLabel(session),
		sourceLabel(session.Source),
		session.CWD,
		session.FilePath,
		session.OriginalPath,
		session.CLIVersion,
		session.ModelProvider,
		session.SizeBytes,
		session.SHA256,
	)
}

func (m Model) sessionColumns(sessions []domain.SessionRecord) []string {
	groups := groupSessions(sessions)
	columnWidth := m.sessionColumnWidth()

	columns := make([]string, 0, len(sessionColumnTitles))
	for columnIndex, title := range sessionColumnTitles {
		column := renderSessionColumn(title, groups[columnIndex].Sessions, m.selectedColumn, m.selectedRow, columnIndex, columnWidth)
		if columnIndex < len(sessionColumnTitles)-1 {
			column = lipgloss.NewStyle().MarginRight(sessionColumnGap).Render(column)
		}
		columns = append(columns, column)
	}
	return columns
}

func (m Model) sessionColumnWidth() int {
	if m.width <= 0 {
		return defaultSessionColumnWidth
	}
	available := m.width - sessionColumnGap*(len(sessionColumnTitles)-1)
	if available <= 0 {
		return minSessionColumnWidth
	}
	width := available / len(sessionColumnTitles)
	if width < minSessionColumnWidth {
		return minSessionColumnWidth
	}
	return width
}

const (
	visibleColumn = iota
	oldHomeColumn
	archivedColumn
	deletedColumn
)

const (
	defaultSessionColumnWidth = 30
	minSessionColumnWidth     = 18
	sessionColumnGap          = 2
)

var sessionColumnTitles = []string{"可见", "不可见", "归档", "删除"}

type sessionColumn struct {
	Title    string
	Sessions []domain.SessionRecord
}

func groupSessions(sessions []domain.SessionRecord) []sessionColumn {
	groups := make([]sessionColumn, len(sessionColumnTitles))
	for i, title := range sessionColumnTitles {
		groups[i].Title = title
	}

	for _, session := range sessions {
		column := sessionColumnIndex(session)
		groups[column].Sessions = append(groups[column].Sessions, session)
	}
	return groups
}

func sessionColumnIndex(session domain.SessionRecord) int {
	switch {
	case session.Status == domain.SessionStatusConflict:
		return deletedColumn
	case session.Source == domain.SessionSourceVisible:
		return visibleColumn
	case session.Source == domain.SessionSourceInactive:
		return oldHomeColumn
	case session.Source == domain.SessionSourceOldHome:
		return oldHomeColumn
	case session.Source == domain.SessionSourceArchived:
		return archivedColumn
	case session.Source == domain.SessionSourceRemoved:
		return deletedColumn
	case session.Source == domain.SessionSourceBackup:
		return deletedColumn
	default:
		return deletedColumn
	}
}

func fallbackColumnOrder(preferred int) []int {
	order := []int{preferred, visibleColumn, archivedColumn, oldHomeColumn, deletedColumn}
	seen := map[int]bool{}
	result := make([]int, 0, len(order))
	for _, column := range order {
		if column < 0 || column >= len(sessionColumnTitles) || seen[column] {
			continue
		}
		seen[column] = true
		result = append(result, column)
	}
	return result
}

func renderSessionColumn(title string, sessions []domain.SessionRecord, selectedColumn int, selectedRow int, columnIndex int, width int) string {
	var builder strings.Builder
	header := fmt.Sprintf("%s (%d)", title, len(sessions))
	builder.WriteString(titleStyle.Width(width).Render(trim(header, width)))
	builder.WriteByte('\n')
	if len(sessions) == 0 {
		builder.WriteString(mutedStyle.Width(width).Render(""))
		return builder.String()
	}
	for rowIndex, session := range sessions {
		nameWidth := width - 12
		if nameWidth < 4 {
			nameWidth = width
		}
		line := fmt.Sprintf("%s  %s", displaySessionTitle(session, nameWidth), shortID(session.ID))
		line = trim(line, width)
		style := lipgloss.NewStyle().Width(width)
		if rowIndex == selectedRow && columnIndex == selectedColumn {
			style = selectedStyle.Width(width)
		}
		builder.WriteString(style.Render(line))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func displaySessionTitle(session domain.SessionRecord, max int) string {
	name := strings.TrimSpace(session.Name)
	if name == "" {
		name = shortID(session.ID)
	}
	return trim(name, max)
}

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func statusLabel(session domain.SessionRecord) string {
	if session.Status == domain.SessionStatusConflict {
		return "冲突"
	}
	switch session.Source {
	case domain.SessionSourceVisible:
		return "可见"
	case domain.SessionSourceInactive:
		return "不可见"
	case domain.SessionSourceOldHome:
		return "不可见"
	case domain.SessionSourceArchived:
		return "归档"
	case domain.SessionSourceRemoved:
		return "删除"
	case domain.SessionSourceBackup:
		return "删除"
	default:
		return string(session.Status)
	}
}

func sourceLabel(source domain.SessionSource) string {
	switch source {
	case domain.SessionSourceVisible:
		return "当前 CODEX_HOME"
	case domain.SessionSourceInactive:
		return "当前 CODEX_HOME / 当前 CLI 不可见"
	case domain.SessionSourceOldHome:
		return "旧 CODEX_HOME / 不可见"
	case domain.SessionSourceArchived:
		return "归档目录"
	case domain.SessionSourceRemoved:
		return "工具删除区"
	case domain.SessionSourceBackup:
		return "工具备份"
	default:
		return string(source)
	}
}

func trim(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "…"
}
