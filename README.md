# codex-session-manager

本地 Codex CLI 和 Claude Code session 管理工具，支持 TUI 与命令行，可简称为 `cdsm`。

## 功能

- 按项目扫描和展示 Codex、Claude Code session。
- 备份、移出、恢复、归档和修复 Codex session。
- 隐藏不需要展示的 Codex 项目。
- 删除 Claude Code session 及其关联数据。
- 记录操作审计日志。

## 构建

要求 Go 1.25 或更高版本。

```bash
go build -o bin/codex-session-manager ./cmd/codex-session-manager
go test ./...
```

启动 TUI：

```bash
bin/codex-session-manager
```

一键生成生产环境的 macOS、Linux、Windows `amd64` 和 `arm64` 版本：

```bash
./scripts/package-release.sh [版本号]
```

不传版本号时默认读取 `internal/version/version.go` 中的版本。产物生成到
`bin/release/<版本号>/`，同时生成 `SHA256SUMS.txt`。例如：

```bash
./scripts/package-release.sh 0.2.0
```

## 命令

```text
codex-session-manager                         启动 TUI
codex-session-manager projects                列出 Codex 项目
codex-session-manager list [project]          列出 Codex session
codex-session-manager current                 列出当前项目的 Codex session
codex-session-manager backup <id>             备份 Codex session
codex-session-manager restore <id>            恢复 Codex session
codex-session-manager remove <id>             移出 Codex session
codex-session-manager repair [project]        修复项目的 Codex session
codex-session-manager hidden-projects         列出隐藏项目
codex-session-manager hide-project <path>     隐藏项目
codex-session-manager unhide-project <path>   恢复隐藏项目
codex-session-manager claude-projects         列出 Claude Code 项目
codex-session-manager claude-list [project]   列出 Claude Code session
codex-session-manager claude-delete <id>      删除 Claude Code session
codex-session-manager version                 显示版本
codex-session-manager update                  从 GitHub Releases 自动更新
```

`<id>` 支持完整 session ID 或 ID 前缀。

## TUI

- `Tab`：切换 Codex 与 Claude Code。
- 输入字符 / `Backspace` / `Esc`：筛选、删除或清空项目筛选。
- `↑/↓` 或 `k/j`：选择项目或 session。
- `←/→` 或 `h/l`：切换 Codex session 列。
- `Enter`：打开项目或 session 详情。
- `s`、`Ctrl+R` 或 `F5`：重新扫描。
- `Esc`：返回上一页。
- `q` 或 `Ctrl+C`：退出。

Codex 操作：

- `b`：备份。
- `a`：归档。
- `r`：恢复。
- `m`：移出或清理。
- `n`：重命名。
- `g`：加载当前项目的可恢复 session。
- `Ctrl+D` / `Ctrl+U`：隐藏项目 / 打开隐藏项目列表。

Claude Code 操作：

- `m`：永久删除 session 及其关联数据，不创建备份。

当前 `CLAUDE_SESSION_ID` 对应的活动 session 不允许删除。

## 配置

配置文件：`~/.codex-session-manager/config.toml`

```toml
codex_home = "/Users/me/.codex"
claude_home = "/Users/me/.claude"
model_provider = "codex_local_access"
backup_dir = "/Users/me/.codex-session-manager/backups"
removed_dir = "/Users/me/.codex-session-manager/removed"
old_codex_homes = ["/Users/me/.codex-old"]
hidden_projects = ["/Users/me/work/old-project"]
```

默认目录：

- Codex：`$CODEX_HOME` 或 `~/.codex`
- Claude Code：`$CLAUDE_CONFIG_DIR` 或 `~/.claude`
- 工具数据：`~/.codex-session-manager`

## 安全

- 不读取或修改 Codex、Claude Code 的认证配置。
- Codex session 移出前默认自动备份。
- Claude Code session 删除不会备份，并会清理可识别的关联数据和 `history.jsonl` 记录。
- 共享的 `paste-cache` 和全局 plans 不会自动删除。
