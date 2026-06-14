package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
)

func TestRenameSessionUpdatesExistingIndexEntry(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(codexHome, "session_index.jsonl")
	if err := os.WriteFile(indexPath, []byte(`{"id":"019ebb10-7a0e-7d70-95e8-c020b75687d8","thread_name":"旧名称","updated_at":"2026-06-13T00:00:00Z"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := RenameSession(config.Config{CodexHome: codexHome}, domain.SessionRecord{ID: "019ebb10-7a0e-7d70-95e8-c020b75687d8"}, "新名称")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"thread_name":"新名称"`) {
		t.Fatalf("expected renamed index, got %s", content)
	}
	if strings.Contains(content, "旧名称") {
		t.Fatalf("old name still present: %s", content)
	}
}

func TestRenameSessionAppendsMissingIndexEntry(t *testing.T) {
	root := t.TempDir()
	codexHome := filepath.Join(root, ".codex")
	sessionID := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	err := RenameSession(config.Config{CodexHome: codexHome}, domain.SessionRecord{ID: sessionID}, "追加名称")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(codexHome, "session_index.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, sessionID) || !strings.Contains(content, "追加名称") {
		t.Fatalf("expected appended entry, got %s", content)
	}
}
