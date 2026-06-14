package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

type AuditRecord struct {
	Action     string    `json:"action"`
	SessionID  string    `json:"session_id,omitempty"`
	SourcePath string    `json:"source_path,omitempty"`
	TargetPath string    `json:"target_path,omitempty"`
	Result     string    `json:"result"`
	Message    string    `json:"message,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (a App) Audit(action string, sessionID string, sourcePath string, targetPath string, result string, message string) error {
	record := AuditRecord{
		Action:     action,
		SessionID:  sessionID,
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Result:     result,
		Message:    message,
		CreatedAt:  time.Now(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	logPath := filepath.Join(a.Config.ToolHome, "logs", "audit.jsonl")
	if err := fsutil.EnsurePrivateDir(filepath.Dir(logPath)); err != nil {
		return err
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}
