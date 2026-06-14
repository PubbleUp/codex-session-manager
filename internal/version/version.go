package version

import "fmt"

// Version 是当前应用内置版本号，可在发布构建时通过 -ldflags 覆盖。
var Version = "0.1.0"

// Format 返回命令行 version 子命令展示的版本信息。
func Format() string {
	return fmt.Sprintf("codex-session-manager %s\n", Version)
}
