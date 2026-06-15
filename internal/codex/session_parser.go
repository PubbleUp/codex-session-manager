package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sunlock/codex-session-manager/internal/domain"
)

var (
	sessionIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	fileTimePattern  = regexp.MustCompile(`rollout-(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2})-`)
)

type jsonLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type metaPayload struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	CWD           string `json:"cwd"`
	Originator    string `json:"originator"`
	CLIVersion    string `json:"cli_version"`
	Source        string `json:"source"`
	ModelProvider string `json:"model_provider"`
}

type messagePayload struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// ParseSessionFile 解析 Codex session 文件的元数据，只读取文件开头部分。
func ParseSessionFile(path string) (domain.SessionRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return domain.SessionRecord{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return domain.SessionRecord{}, err
	}

	record := domain.SessionRecord{
		FilePath:  path,
		UpdatedAt: info.ModTime(),
		SizeBytes: info.Size(),
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineCount := 0
	for scanner.Scan() {
		lineCount++
		var item jsonLine
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			if lineCount == 1 {
				return record, err
			}
			continue
		}

		if record.CreatedAt.IsZero() {
			record.CreatedAt = parseTime(item.Timestamp)
		}

		var meta metaPayload
		if err := json.Unmarshal(item.Payload, &meta); err == nil {
			if meta.ID != "" || meta.CWD != "" || meta.CLIVersion != "" {
				applyMeta(&record, meta)
			}
		}

		if record.Name == "" {
			if name := extractName(item.Payload); name != "" {
				record.Name = name
			}
		}

		if record.ID != "" && record.CWD != "" && !record.CreatedAt.IsZero() {
			break
		}
		if lineCount >= 50 {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return record, err
	}

	if record.ID == "" {
		record.ID = ExtractSessionID(path)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = ExtractTimeFromFilename(path)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = info.ModTime()
	}
	if record.Name == "" && record.ID != "" {
		record.Name = ShortID(record.ID)
	}
	if record.ID == "" {
		return record, errors.New("缺少会话 ID")
	}
	return record, nil
}

// ExtractSessionID 从文件名或路径中提取 session UUID。
func ExtractSessionID(path string) string {
	return sessionIDPattern.FindString(filepath.Base(path))
}

// ExtractTimeFromFilename 从 Codex rollout 文件名中提取创建时间。
func ExtractTimeFromFilename(path string) time.Time {
	match := fileTimePattern.FindStringSubmatch(filepath.Base(path))
	if len(match) != 2 {
		return time.Time{}
	}
	value := strings.ReplaceAll(match[1], "-", ":")
	value = strings.Replace(value, ":", "-", 2)
	value = strings.Replace(value, ":", "-", 1)
	parsed, err := time.ParseInLocation("2006-01-02T15-04-05", match[1], time.Local)
	if err == nil {
		return parsed
	}
	parsed, _ = time.ParseInLocation("2006-01-02T15:04:05", value, time.Local)
	return parsed
}

// ShortID 返回便于 TUI 展示的 session 短 ID。
func ShortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func applyMeta(record *domain.SessionRecord, meta metaPayload) {
	if meta.ID != "" {
		record.ID = meta.ID
	}
	if meta.CWD != "" {
		record.CWD = meta.CWD
	}
	if meta.CLIVersion != "" {
		record.CLIVersion = meta.CLIVersion
	}
	if meta.ModelProvider != "" {
		record.ModelProvider = meta.ModelProvider
	}
	if meta.Timestamp != "" && record.CreatedAt.IsZero() {
		record.CreatedAt = parseTime(meta.Timestamp)
	}
}

func extractName(payload json.RawMessage) string {
	var message messagePayload
	if err := json.Unmarshal(payload, &message); err != nil {
		return ""
	}
	if message.Role != "user" {
		return ""
	}
	for _, content := range message.Content {
		text := strings.TrimSpace(content.Text)
		if text == "" {
			continue
		}
		if len(text) > 60 {
			text = text[:60]
		}
		return strings.ReplaceAll(text, "\n", " ")
	}
	return ""
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999-07:00",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
