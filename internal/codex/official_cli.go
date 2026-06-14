package codex

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
)

type CommandResult struct {
	SessionID string
	Message   string
	Output    string
}

// ArchiveSession 调用 Codex CLI 官方 archive 能力归档当前可见 session。
func ArchiveSession(cfg config.Config, session domain.SessionRecord) (CommandResult, error) {
	if session.Source != domain.SessionSourceVisible {
		return CommandResult{}, fmt.Errorf("only visible sessions can be archived")
	}
	cmd := exec.Command("codex", "archive", session.ID)
	cmd.Env = append(os.Environ(), "CODEX_HOME="+cfg.CodexHome)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CommandResult{}, fmt.Errorf("codex archive failed: %w: %s", err, string(output))
	}
	return CommandResult{
		SessionID: session.ID,
		Message:   "archived",
		Output:    string(output),
	}, nil
}
