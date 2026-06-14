package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

type Config struct {
	CodexHome                 string
	ToolHome                  string
	BackupDir                 string
	RemovedDir                string
	OldCodexHomes             []string
	HiddenProjects            []string
	IncludeArchived           bool
	IncludeBackups            bool
	IncludeRemoved            bool
	RequireBackupBeforeRemove bool
	AllowOverwrite            bool
	PreviewContent            bool
}

func Default() Config {
	home, _ := os.UserHomeDir()
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" && home != "" {
		codexHome = filepath.Join(home, ".codex")
	}
	toolHome := filepath.Join(home, ".codex-session-manager")
	return Config{
		CodexHome:                 fsutil.NormalizePath(codexHome),
		ToolHome:                  fsutil.NormalizePath(toolHome),
		BackupDir:                 fsutil.NormalizePath(filepath.Join(toolHome, "backups")),
		RemovedDir:                fsutil.NormalizePath(filepath.Join(toolHome, "removed")),
		IncludeArchived:           true,
		IncludeBackups:            true,
		IncludeRemoved:            true,
		RequireBackupBeforeRemove: true,
	}
}

func Load() (Config, error) {
	cfg := Default()
	path := filepath.Join(cfg.ToolHome, "config.toml")
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "home", "codex_home":
			if value != "" {
				cfg.CodexHome = fsutil.NormalizePath(value)
			}
		case "tool_home":
			if value != "" {
				cfg.ToolHome = fsutil.NormalizePath(value)
			}
		case "backup_dir":
			if value != "" {
				cfg.BackupDir = fsutil.NormalizePath(value)
			}
		case "removed_dir":
			if value != "" {
				cfg.RemovedDir = fsutil.NormalizePath(value)
			}
		case "old_codex_homes":
			cfg.OldCodexHomes = parseStringArray(value)
		case "hidden_projects":
			cfg.HiddenProjects = parseStringArray(value)
		case "include_archived":
			cfg.IncludeArchived = parseBool(value, cfg.IncludeArchived)
		case "include_backups":
			cfg.IncludeBackups = parseBool(value, cfg.IncludeBackups)
		case "include_removed":
			cfg.IncludeRemoved = parseBool(value, cfg.IncludeRemoved)
		case "require_backup_before_remove":
			cfg.RequireBackupBeforeRemove = parseBool(value, cfg.RequireBackupBeforeRemove)
		case "allow_overwrite":
			cfg.AllowOverwrite = parseBool(value, cfg.AllowOverwrite)
		case "preview_content":
			cfg.PreviewContent = parseBool(value, cfg.PreviewContent)
		}
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func ConfigPath(cfg Config) string {
	return filepath.Join(cfg.ToolHome, "config.toml")
}

func HideProject(cfg Config, projectPath string) error {
	projectPath = fsutil.NormalizePath(projectPath)
	if projectPath == "" {
		return nil
	}
	hidden := normalizePathList(cfg.HiddenProjects)
	for _, existing := range hidden {
		if existing == projectPath {
			return nil
		}
	}
	hidden = append(hidden, projectPath)
	return writeConfigArray(ConfigPath(cfg), "hidden_projects", hidden)
}

func UnhideProject(cfg Config, projectPath string) error {
	projectPath = fsutil.NormalizePath(projectPath)
	if projectPath == "" {
		return nil
	}
	hidden := make([]string, 0, len(cfg.HiddenProjects))
	for _, existing := range normalizePathList(cfg.HiddenProjects) {
		if existing == projectPath {
			continue
		}
		hidden = append(hidden, existing)
	}
	return writeConfigArray(ConfigPath(cfg), "hidden_projects", hidden)
}

func Ensure(cfg Config) error {
	for _, dir := range []string{cfg.ToolHome, cfg.BackupDir, cfg.RemovedDir, filepath.Join(cfg.ToolHome, "logs")} {
		if err := fsutil.EnsurePrivateDir(dir); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

func parseBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true
	case "false":
		return false
	default:
		return fallback
	}
}

func parseStringArray(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.Trim(strings.TrimSpace(part), `"`)
		if item != "" {
			result = append(result, fsutil.NormalizePath(item))
		}
	}
	return result
}

func writeConfigArray(path string, key string, values []string) error {
	if err := fsutil.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return err
	}
	lines, err := readConfigLines(path)
	if err != nil {
		return err
	}
	rendered := key + " = " + renderStringArray(values)
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
			lines[i] = rendered
			replaced = true
			break
		}
	}
	if !replaced {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, rendered)
	}
	data := strings.Join(lines, "\n") + "\n"
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readConfigLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	content := strings.TrimSuffix(string(data), "\n")
	if content == "" {
		return nil, nil
	}
	return strings.Split(content, "\n"), nil
}

func renderStringArray(values []string) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteByte('"')
		builder.WriteString(strings.ReplaceAll(value, `"`, `\"`))
		builder.WriteByte('"')
	}
	builder.WriteByte(']')
	return builder.String()
}

func normalizePathList(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		normalized := fsutil.NormalizePath(value)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}
