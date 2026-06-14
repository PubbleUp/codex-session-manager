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
		requireArgs(args, 2, "usage: codex-session-manager backup <session-id>")
		manifest, err := a.Backup(args[1])
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("backed up %s\n", manifest.SessionID)
	case "restore":
		requireArgs(args, 2, "usage: codex-session-manager restore <session-id>")
		result, err := a.Restore(args[1])
		if err != nil {
			exitErr(err)
		}
		fmt.Printf("%s: %s -> %s\n", result.Message, result.SourcePath, result.TargetPath)
	case "remove":
		requireArgs(args, 2, "usage: codex-session-manager remove <session-id>")
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
		requireArgs(args, 2, "usage: codex-session-manager hide-project <project-path>")
		projectPath := fsutil.NormalizePath(args[1])
		if err := a.HideProject(projectPath); err != nil {
			exitErr(err)
		}
		fmt.Printf("hidden project %s\n", projectPath)
	case "unhide-project":
		requireArgs(args, 2, "usage: codex-session-manager unhide-project <project-path>")
		projectPath := fsutil.NormalizePath(args[1])
		if err := a.UnhideProject(projectPath); err != nil {
			exitErr(err)
		}
		fmt.Printf("restored project %s\n", projectPath)
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
	fmt.Print(`codex-session-manager manages local Codex CLI sessions.

Usage:
  codex-session-manager                  Start TUI
  codex-session-manager projects         List projects
  codex-session-manager scan             Scan and list projects
  codex-session-manager list [project]   List sessions
  codex-session-manager current          List current project sessions
  codex-session-manager hidden-projects  List hidden projects
  codex-session-manager hide-project <path>    Hide a project from scans
  codex-session-manager unhide-project <path>  Restore a hidden project
  codex-session-manager backup <id>      Backup a session
  codex-session-manager restore <id>     Restore a backed up session
  codex-session-manager remove <id>      Move a visible session out of current Codex
  codex-session-manager repair [project] Restore all recoverable sessions for a project
  codex-session-manager version          Show version
`)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
