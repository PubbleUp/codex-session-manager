package claude

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sunlock/codex-session-manager/internal/config"
)

func TestScanAndDeleteSession(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "-tmp-project")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "11111111-1111-1111-1111-111111111111"
	transcript := filepath.Join(projectDir, id+".jsonl")
	data := `{"type":"user","sessionId":"` + id + `","cwd":"/tmp/project","version":"2.1.0","message":{"content":"实现会话管理"}}` + "\n"
	if err := os.WriteFile(transcript, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	history := `{"display":"实现会话管理","timestamp":1000,"project":"/tmp/project","sessionId":"` + id + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(root, "history.jsonl"), []byte(history), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ClaudeHome: root}
	inventory, err := Scan(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Sessions) != 1 || inventory.Sessions[0].Name != "实现会话管理" {
		t.Fatalf("unexpected inventory: %+v", inventory)
	}
	result, err := DeleteSession(cfg, inventory.Sessions[0])
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 1 {
		t.Fatalf("expected one deleted path, got %d", result.Deleted)
	}
	if _, err := os.Stat(transcript); !os.IsNotExist(err) {
		t.Fatalf("expected transcript deleted, err=%v", err)
	}
}
