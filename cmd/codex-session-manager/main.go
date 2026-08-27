package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sunlock/codex-session-manager/internal/app"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
	"github.com/sunlock/codex-session-manager/internal/tui"
	"github.com/sunlock/codex-session-manager/internal/version"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			printHelp()
			return
		case "version", "-v", "--version":
			fmt.Print(version.Format())
			return
		}
	}

	a, err := app.New()
	if err != nil {
		exitErr(err)
	}

	if len(args) == 0 {
		if _, err := tea.NewProgram(tui.New(a), tea.WithAltScreen()).Run(); err != nil {
			exitErr(err)
		}
		return
	}

	switch args[0] {
	case "scan", "projects":
		inventory, err := a.Scan()
		if err != nil {
			exitErr(err)
		}
		fmt.Print(app.FormatProjects(inventory.Projects))
	case "claude-projects":
		inventory, err := a.ScanClaude()
		if err != nil {
			exitErr(err)
		}
		fmt.Print(app.FormatProjects(inventory.Projects))
	case "claude-list":
		inventory, err := a.ScanClaude()
		if err != nil {
			exitErr(err)
		}
		projectPath := ""
		if len(args) > 1 {
			projectPath = fsutil.NormalizePath(args[1])
		}
		fmt.Print(app.FormatSessions(inventory.Sessions, projectPath))
	case "claude-delete":
		requireArgs(args, 2, "用法：codex-session-manager claude-delete <会话ID>")
		inventory, err := a.ScanClaude()
		if err != nil {
			exitErr(err)
		}
		session, ok := app.FindSession(inventory.Sessions, args[1])
		if !ok {
			exitErr(fmt.Errorf("未找到 Claude Code 会话：%s", args[1]))
		}
		result, err := a.DeleteClaudeSession(session)
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("已删除 %s（%d 个路径）\n", result.SessionID, result.Deleted)
	case "hidden-projects":
		fmt.Print(app.FormatHiddenProjects(a.Config.HiddenProjects))
	case "list":
		inventory, err := a.Scan()
		if err != nil {
			exitErr(err)
		}
		projectPath := ""
		if len(args) > 1 {
			projectPath = fsutil.NormalizePath(args[1])
		}
		fmt.Print(app.FormatSessions(inventory.Sessions, projectPath))
	case "current":
		cwd, err := os.Getwd()
		if err != nil {
			exitErr(err)
		}
		inventory, err := a.Scan()
		if err != nil {
			exitErr(err)
		}
		fmt.Print(app.FormatSessions(inventory.Sessions, fsutil.NormalizePath(cwd)))
	case "backup":
		requireArgs(args, 2, "用法：codex-session-manager backup <会话ID>")
		manifest, err := a.Backup(args[1])
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("已备份 %s\n", manifest.SessionID)
	case "restore":
		requireArgs(args, 2, "用法：codex-session-manager restore <会话ID>")
		result, err := a.Restore(args[1])
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("%s: %s -> %s\n", result.Message, result.SourcePath, result.TargetPath)
	case "remove":
		requireArgs(args, 2, "用法：codex-session-manager remove <会话ID>")
		result, err := a.Remove(args[1])
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("%s: %s -> %s\n", result.Message, result.Source, result.Target)
	case "repair":
		projectPath := ""
		if len(args) > 1 {
			projectPath = args[1]
		}
		report, err := a.Repair(projectPath)
		if err != nil {
			exitErr(err)
		}
		fmt.Print(app.FormatRepairReport(report))
	case "hide-project":
		requireArgs(args, 2, "用法：codex-session-manager hide-project <项目路径>")
		projectPath := fsutil.NormalizePath(args[1])
		if err := a.HideProject(projectPath); err != nil {
			exitErr(err)
		}
		fmt.Printf("已隐藏项目 %s\n", projectPath)
	case "unhide-project":
		requireArgs(args, 2, "用法：codex-session-manager unhide-project <项目路径>")
		projectPath := fsutil.NormalizePath(args[1])
		if err := a.UnhideProject(projectPath); err != nil {
			exitErr(err)
		}
		fmt.Printf("已恢复项目 %s\n", projectPath)
	default:
		printHelp()
		os.Exit(2)
	}
}

func requireArgs(args []string, count int, usage string) {
	if len(args) < count {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
}

func printHelp() {
	fmt.Print(`codex-session-manager 用于管理本地 Codex CLI 和 Claude Code 会话。

用法：
  codex-session-manager                  启动 TUI
  codex-session-manager projects         列出项目
  codex-session-manager scan             扫描并列出项目
  codex-session-manager list [项目路径]   列出会话
  codex-session-manager current          列出当前项目会话
  codex-session-manager hidden-projects  列出隐藏项目
  codex-session-manager hide-project <项目路径>  从扫描中隐藏项目
  codex-session-manager unhide-project <项目路径> 恢复隐藏项目
  codex-session-manager backup <会话ID>   备份会话
  codex-session-manager restore <会话ID>  恢复会话
  codex-session-manager remove <会话ID>   从当前 Codex 移出可见会话
  codex-session-manager repair [项目路径] 恢复项目下所有可恢复会话
	  codex-session-manager claude-projects  列出 Claude Code 项目
	  codex-session-manager claude-list [项目路径] 列出 Claude Code 会话
	  codex-session-manager claude-delete <会话ID> 删除 Claude Code 会话
  codex-session-manager version          显示版本号
`)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "错误：", err)
	os.Exit(1)
}
