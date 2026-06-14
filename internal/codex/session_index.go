package codex

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

type indexEntry struct {
	ID         string    `json:"id"`
	ThreadName string    `json:"thread_name"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// RenameSession 更新当前 CODEX_HOME 的 session_index.jsonl 中的会话名称。
func RenameSession(cfg config.Config, session domain.SessionRecord, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	indexPath := filepath.Join(cfg.CodexHome, "session_index.jsonl")
	if err := fsutil.EnsurePrivateDir(filepath.Dir(indexPath)); err != nil {
		return err
	}

	entries, err := readIndexEntries(indexPath)
	if err != nil {
		return err
	}
	found := false
	now := time.Now().UTC()
	for i := range entries {
		if entries[i].ID == session.ID {
			entries[i].ThreadName = name
			entries[i].UpdatedAt = now
			found = true
		}
	}
	if !found {
		entries = append(entries, indexEntry{
			ID:         session.ID,
			ThreadName: name,
			UpdatedAt:  now,
		})
	}
	return writeIndexEntries(indexPath, entries)
}

func readIndexEntries(path string) ([]indexEntry, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []indexEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var entry indexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.ID == "" {
			continue
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func writeIndexEntries(path string, entries []indexEntry) error {
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			_ = file.Close()
			_ = os.Remove(tmp)
			return err
		}
		if _, err := writer.Write(append(data, '\n')); err != nil {
			_ = file.Close()
			_ = os.Remove(tmp)
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
