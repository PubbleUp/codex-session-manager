# codex-session-manager

`codex-session-manager` 是一个本地 Codex CLI session 管理工具，提供 TUI 和命令行两种使用方式，可简称为 `cdsm`。它用于扫描、查看、备份、移出、恢复和修复本机 Codex session，适合在多个项目之间整理历史会话。

## 功能特性

- 扫描 `$CODEX_HOME/sessions`，按 session 记录中的项目 `cwd` 聚合展示。
- 查看项目、session 状态、来源、名称、更新时间和所属目录。
- 将可见 session 备份到工具管理目录。
- 将当前 Codex 中的可见 session 移出，避免列表过载。
- 从备份、移出目录或旧 Codex 目录恢复 session。
- 一键修复指定项目，将可恢复 session 重新放回当前 Codex。
- 支持隐藏不想展示的项目，并可随时恢复。
- 记录操作审计日志，便于排查备份、恢复、移出等动作。

## 项目结构

```text
cmd/codex-session-manager/   CLI 入口
internal/app/                应用编排、输出格式和审计日志
internal/tui/                Bubble Tea TUI 页面与快捷键
internal/codex/              Codex session 解析和官方 CLI 操作
internal/scanner/            session 扫描与项目聚合
internal/backup/             备份逻辑
internal/restore/            恢复和 repair 逻辑
internal/remove/             移出和清理逻辑
internal/config/             配置读取、默认目录和隐藏项目
internal/fsutil/             文件系统工具
internal/domain/             共享领域模型
bin/                         本地构建产物
```

## 环境要求

- Go 1.24 或更高版本。
- 已安装并使用过 Codex CLI。
- 默认读取 `$CODEX_HOME`；如果未设置，则使用 `~/.codex`。

## 构建

```bash
go build -o bin/codex-session-manager ./cmd/codex-session-manager
```

运行测试：

```bash
go test ./...
```

可选：为了更方便调用，可以把 `bin/codex-session-manager` 链接或复制为 `cdsm`，并放入 `PATH`。

## 快速开始

启动 TUI：

```bash
bin/codex-session-manager
```

查看所有项目：

```bash
bin/codex-session-manager projects
```

查看当前项目 session：

```bash
bin/codex-session-manager current
```

查看工具版本：

```bash
bin/codex-session-manager version
```

备份、移出并恢复一个 session：

```bash
bin/codex-session-manager backup <session-id>
bin/codex-session-manager remove <session-id>
bin/codex-session-manager restore <session-id>
```

`<session-id>` 支持完整 ID，也支持能够唯一匹配的前缀。

## 命令行用法

```text
codex-session-manager                  启动 TUI
codex-session-manager projects         扫描并列出项目
codex-session-manager scan             同 projects
codex-session-manager list [project]   列出全部或指定项目的 session
codex-session-manager current          列出当前目录对应项目的 session
codex-session-manager hidden-projects  列出已隐藏项目
codex-session-manager hide-project <path>
codex-session-manager unhide-project <path>
codex-session-manager backup <id>
codex-session-manager restore <id>
codex-session-manager remove <id>
codex-session-manager repair [project]
codex-session-manager version
```

常见示例：

```bash
bin/codex-session-manager list /Users/me/work/my-project
bin/codex-session-manager repair
bin/codex-session-manager repair /Users/me/work/my-project
bin/codex-session-manager hide-project /Users/me/work/old-project
bin/codex-session-manager unhide-project /Users/me/work/old-project
```

## TUI 快捷键

项目页：

- 输入字符：筛选项目。
- `Backspace`：删除筛选字符。
- `Esc`：清空筛选。
- `Enter`：打开项目 session 列表。
- `Ctrl+R` 或 `F5`：重新扫描。
- `Ctrl+D`：隐藏当前项目。
- `Ctrl+U`：打开隐藏项目列表。
- `Ctrl+C`：退出。

session 页和详情页：

- `↑/↓` 或 `k/j`：移动行。
- `←/→` 或 `h/l`：切换列。
- `Enter`：打开详情。
- `s`：重新扫描。
- `g`：一键加载当前项目可恢复 session。
- `n`：重命名 session。
- `b`：备份 session。
- `a`：归档 session。
- `r`：恢复 session。
- `m`：移出或清理 session。
- `Esc`：返回项目页。
- `q`：退出。

隐藏项目页：

- `Enter` 或 `Ctrl+U`：恢复当前隐藏项目。
- `Esc`：返回项目页。
- `Ctrl+C`：退出。

## 配置

首次运行会创建工具目录：

```text
~/.codex-session-manager/
```

可选配置文件：

```text
~/.codex-session-manager/config.toml
```

支持的配置项：

```toml
codex_home = "/Users/me/.codex"
backup_dir = "/Users/me/.codex-session-manager/backups"
removed_dir = "/Users/me/.codex-session-manager/removed"
old_codex_homes = ["/Users/me/.codex-old"]
hidden_projects = ["/Users/me/work/old-project"]
include_archived = true
include_backups = true
include_removed = true
require_backup_before_remove = true
allow_overwrite = false
preview_content = false
```

默认目录：

- Codex 目录：`$CODEX_HOME` 或 `~/.codex`
- 备份目录：`~/.codex-session-manager/backups`
- 移出目录：`~/.codex-session-manager/removed`
- 审计日志：`~/.codex-session-manager/logs/audit.jsonl`

## 安全边界

工具不读取、不备份、不修改 Codex 的 `auth.json`。`remove` 默认会在移出前自动补齐备份，并把 session 移入工具管理目录，而不是直接永久删除。所有备份、移出、恢复、隐藏项目和修复操作都会写入审计日志。

## 开发

常用命令：

```bash
go test ./...
go test ./internal/scanner -run TestName
gofmt -w cmd internal
go build -o bin/codex-session-manager ./cmd/codex-session-manager
```

发布构建时可覆盖内置版本号：

```bash
go build -ldflags "-X github.com/sunlock/codex-session-manager/internal/version.Version=0.1.0" -o bin/codex-session-manager ./cmd/codex-session-manager
```

测试文件与实现文件放在同一 package 下，命名为 `*_test.go`。涉及文件操作的测试应使用临时目录，避免触碰真实 `$CODEX_HOME`。
