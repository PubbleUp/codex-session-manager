package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

type historyEntry struct {
	Display   string         `json:"display"`
	Project   string         `json:"project"`
	SessionID string         `json:"sessionId"`
	Timestamp int64          `json:"timestamp"`
	Pasted    map[string]any `json:"pastedContents"`
}

type transcriptEntry struct {
	Type        string `json:"type"`
	SessionID   string `json:"sessionId"`
	CWD         string `json:"cwd"`
	Version     string `json:"version"`
	Timestamp   string `json:"timestamp"`
	CustomTitle string `json:"customTitle"`
	Message     struct {
		Content any `json:"content"`
	} `json:"message"`
}

type DeleteResult struct {
	SessionID string
	Deleted   int
}

func Scan(cfg config.Config) (domain.Inventory, error) {
	history, err := readHistory(filepath.Join(cfg.ClaudeHome, "history.jsonl"))
	if err != nil {
		return domain.Inventory{}, err
	}
	metadata := map[string][]historyEntry{}
	for _, entry := range history {
		if entry.SessionID != "" {
			metadata[entry.SessionID] = append(metadata[entry.SessionID], entry)
		}
	}

	paths, err := filepath.Glob(filepath.Join(cfg.ClaudeHome, "projects", "*", "*.jsonl"))
	if err != nil {
		return domain.Inventory{}, err
	}
	sessions := make([]domain.SessionRecord, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		if id == "" || strings.HasPrefix(id, "agent-") || seen[id] {
			continue
		}
		record := domain.SessionRecord{
			ID:            id,
			FilePath:      fsutil.NormalizePath(path),
			OriginalPath:  fsutil.NormalizePath(path),
			Source:        domain.SessionSourceVisible,
			Status:        domain.SessionStatusVisible,
			CodexHome:     cfg.ClaudeHome,
			ModelProvider: "claude-code",
		}
		if info, statErr := os.Stat(path); statErr == nil {
			record.SizeBytes = info.Size()
			record.UpdatedAt = info.ModTime()
		}
		populateFromHistory(&record, metadata[id])
		populateFromTranscript(&record)
		if record.CWD == "" {
			continue
		}
		seen[id] = true
		sessions = append(sessions, record)
	}

	projects := aggregateProjects(sessions)
	sort.Slice(projects, func(i, j int) bool { return projects[i].LastUpdatedAt.After(projects[j].LastUpdatedAt) })
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt) })
	return domain.Inventory{Projects: projects, Sessions: sessions}, nil
}

func DeleteSession(cfg config.Config, session domain.SessionRecord) (DeleteResult, error) {
	if session.ID == "" {
		return DeleteResult{}, fmt.Errorf("缺少 Claude Code 会话 ID")
	}
	if session.ID == os.Getenv("CLAUDE_SESSION_ID") {
		return DeleteResult{}, fmt.Errorf("拒绝删除当前活动的 Claude Code 会话：%s", session.ID)
	}
	paths, err := associatedPaths(cfg.ClaudeHome, session.ID)
	if err != nil {
		return DeleteResult{}, err
	}
	if len(paths) == 0 {
		return DeleteResult{}, fmt.Errorf("未找到 Claude Code 会话：%s", session.ID)
	}
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return DeleteResult{}, err
		}
	}
	if err := rewriteHistoryWithoutSession(filepath.Join(cfg.ClaudeHome, "history.jsonl"), session.ID); err != nil {
		return DeleteResult{}, err
	}
	return DeleteResult{SessionID: session.ID, Deleted: len(paths)}, nil
}

func readHistory(path string) ([]historyEntry, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var result []historyEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var entry historyEntry
		if json.Unmarshal(scanner.Bytes(), &entry) == nil {
			result = append(result, entry)
		}
	}
	return result, scanner.Err()
}

func populateFromHistory(record *domain.SessionRecord, entries []historyEntry) {
	for _, entry := range entries {
		if record.CWD == "" && entry.Project != "" {
			record.CWD = fsutil.NormalizePath(entry.Project)
		}
		if record.Name == "" && meaningfulTitle(entry.Display) {
			record.Name = strings.TrimSpace(entry.Display)
		}
		when := time.UnixMilli(entry.Timestamp)
		if record.CreatedAt.IsZero() || when.Before(record.CreatedAt) {
			record.CreatedAt = when
		}
		if when.After(record.UpdatedAt) {
			record.UpdatedAt = when
		}
	}
}

func populateFromTranscript(record *domain.SessionRecord) {
	file, err := os.Open(record.FilePath)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var entry transcriptEntry
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if entry.CWD != "" && record.CWD == "" {
			record.CWD = fsutil.NormalizePath(entry.CWD)
		}
		if entry.Version != "" {
			record.CLIVersion = entry.Version
		}
		if entry.Type == "custom-title" && strings.TrimSpace(entry.CustomTitle) != "" {
			record.Name = strings.TrimSpace(entry.CustomTitle)
		}
		if record.Name == "" && entry.Type == "user" {
			if title := contentTitle(entry.Message.Content); title != "" {
				record.Name = title
			}
		}
	}
}

func contentTitle(content any) string {
	var text string
	switch value := content.(type) {
	case string:
		text = value
	case []any:
		for _, item := range value {
			block, ok := item.(map[string]any)
			if ok && block["type"] == "text" {
				if part, ok := block["text"].(string); ok {
					text += " " + part
				}
			}
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if !meaningfulTitle(text) {
		return ""
	}
	if len([]rune(text)) > 100 {
		return string([]rune(text)[:97]) + "..."
	}
	return text
}

func meaningfulTitle(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "<")
}

func aggregateProjects(sessions []domain.SessionRecord) []domain.ProjectRecord {
	byPath := map[string]*domain.ProjectRecord{}
	for _, session := range sessions {
		project := byPath[session.CWD]
		if project == nil {
			project = &domain.ProjectRecord{CWD: session.CWD}
			byPath[session.CWD] = project
		}
		project.TotalSessions++
		project.VisibleCount++
		if session.UpdatedAt.After(project.LastUpdatedAt) {
			project.LastUpdatedAt = session.UpdatedAt
		}
	}
	result := make([]domain.ProjectRecord, 0, len(byPath))
	for _, project := range byPath {
		result = append(result, *project)
	}
	return result
}

func associatedPaths(root string, sessionID string) ([]string, error) {
	patterns := []string{
		filepath.Join(root, "projects", "*", sessionID+".jsonl"),
		filepath.Join(root, "tasks", sessionID),
		filepath.Join(root, "todos", "*"+sessionID+"*"),
		filepath.Join(root, "session-env", sessionID),
		filepath.Join(root, "file-history", sessionID),
		filepath.Join(root, "teams", sessionID),
		filepath.Join(root, "debug", sessionID+".txt"),
		filepath.Join(root, "telemetry", "1p_failed_events."+sessionID+".*.json"),
	}
	seen := map[string]bool{}
	var result []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			match = fsutil.NormalizePath(match)
			if !seen[match] {
				seen[match] = true
				result = append(result, match)
			}
		}
	}
	markers, _ := filepath.Glob(filepath.Join(root, "sessions", "*.json"))
	for _, marker := range markers {
		data, err := os.ReadFile(marker)
		if err != nil {
			continue
		}
		var value struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(data, &value) == nil && value.SessionID == sessionID {
			result = append(result, fsutil.NormalizePath(marker))
		}
	}
	return result, nil
}

func rewriteHistoryWithoutSession(path, sessionID string) error {
	input, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	output, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		input.Close()
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var entry historyEntry
		if json.Unmarshal(line, &entry) == nil && entry.SessionID == sessionID {
			continue
		}
		if _, err := output.Write(append(line, '\n')); err != nil {
			input.Close()
			output.Close()
			os.Remove(tmp)
			return err
		}
	}
	scanErr := scanner.Err()
	closeInErr := input.Close()
	closeOutErr := output.Close()
	if err := firstError(scanErr, closeInErr, closeOutErr); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func firstError(errors ...error) error {
	for _, err := range errors {
		if err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}
