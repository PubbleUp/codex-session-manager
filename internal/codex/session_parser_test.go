package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSessionFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout-2026-06-12T17-01-19-019ebb10-7a0e-7d70-95e8-c020b75687d8.jsonl")
	content := `{"timestamp":"2026-06-12T17:01:19+08:00","type":"session_meta","payload":{"id":"019ebb10-7a0e-7d70-95e8-c020b75687d8","timestamp":"2026-06-12T17:01:19+08:00","cwd":"/tmp/project","originator":"codex_cli_rs","cli_version":"0.136.0","source":"cli","model_provider":"openai"}}
{"timestamp":"2026-06-12T17:01:20+08:00","type":"event_msg","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello codex"}]}}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	session, err := ParseSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "019ebb10-7a0e-7d70-95e8-c020b75687d8" {
		t.Fatalf("unexpected id: %s", session.ID)
	}
	if session.CWD != "/tmp/project" {
		t.Fatalf("unexpected cwd: %s", session.CWD)
	}
	if session.CLIVersion != "0.136.0" {
		t.Fatalf("unexpected cli version: %s", session.CLIVersion)
	}
	if session.ModelProvider != "openai" {
		t.Fatalf("unexpected provider: %s", session.ModelProvider)
	}
}

func TestExtractSessionID(t *testing.T) {
	id := ExtractSessionID("rollout-2026-06-12T17-01-19-019ebb10-7a0e-7d70-95e8-c020b75687d8.jsonl")
	if id != "019ebb10-7a0e-7d70-95e8-c020b75687d8" {
		t.Fatalf("unexpected id: %s", id)
	}
}
