package scanner

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sunlock/codex-session-manager/internal/codex"
	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
)

// Scan 扫描所有配置来源，并按项目聚合 session。
func Scan(cfg config.Config) (domain.Inventory, error) {
	paths, err := sessionFiles(cfg)
	if err != nil {
		return domain.Inventory{}, err
	}

	sessions := make([]domain.SessionRecord, 0, len(paths))
	backupIDs := readBackupIDs(cfg.BackupDir)
	indexNames := readSessionIndex(cfg)
	threadIndex := codex.ReadThreadIndex(cfg)
	hiddenProjects := hiddenProjectSet(cfg.HiddenProjects)
	hashByID := map[string]string{}

	for _, path := range paths {
		record, err := codex.ParseSessionFile(path)
		if err != nil {
			continue
		}
		record.FilePath = fsutil.NormalizePath(record.FilePath)
		record.Source, record.Status, record.IsCurrentHome, record.CodexHome = codex.ClassifySource(
			record.FilePath,
			cfg.CodexHome,
			cfg.BackupDir,
			cfg.RemovedDir,
			cfg.OldCodexHomes,
		)
		applyThreadVisibility(&record, threadIndex)
		record.CWD = fsutil.NormalizePath(record.CWD)
		if hiddenProjects[record.CWD] {
			continue
		}
		if index, ok := indexNames[record.ID]; ok {
			if index.Name != "" {
				record.Name = index.Name
			}
			if index.UpdatedAt.After(record.UpdatedAt) {
				record.UpdatedAt = index.UpdatedAt
			}
		}
		record.IsBackedUp = backupIDs[record.ID]
		if sha, err := fsutil.SHA256File(record.FilePath); err == nil {
			record.SHA256 = sha
		}
		if previous, ok := hashByID[record.ID]; ok && previous != "" && record.SHA256 != "" && previous != record.SHA256 {
			record.Status = domain.SessionStatusConflict
			record.ConflictReason = "相同会话 ID 存在不同内容"
		}
		if record.SHA256 != "" {
			hashByID[record.ID] = record.SHA256
		}
		sessions = append(sessions, record)
	}

	projects := aggregateProjects(sessions)
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastUpdatedAt.After(projects[j].LastUpdatedAt)
	})
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	return domain.Inventory{Projects: projects, Sessions: sessions}, nil
}

func applyThreadVisibility(record *domain.SessionRecord, threadIndex codex.ThreadIndex) {
	if record.Source != domain.SessionSourceVisible {
		return
	}
	if threadIndex.IsActive(record.ID, record.FilePath) {
		return
	}
	record.Source = domain.SessionSourceInactive
	record.Status = domain.SessionStatusInactive
	record.IsCurrentHome = false
}

func hiddenProjectSet(projects []string) map[string]bool {
	result := map[string]bool{}
	for _, project := range projects {
		project = fsutil.NormalizePath(project)
		if project == "" {
			continue
		}
		result[project] = true
	}
	return result
}

func sessionFiles(cfg config.Config) ([]string, error) {
	roots := []string{
		filepath.Join(cfg.CodexHome, "sessions"),
	}
	if cfg.IncludeArchived {
		roots = append(roots, filepath.Join(cfg.CodexHome, "archived_sessions"))
	}
	for _, oldHome := range cfg.OldCodexHomes {
		roots = append(roots, filepath.Join(oldHome, "sessions"))
		if cfg.IncludeArchived {
			roots = append(roots, filepath.Join(oldHome, "archived_sessions"))
		}
	}
	if cfg.IncludeBackups {
		roots = append(roots, cfg.BackupDir)
	}
	if cfg.IncludeRemoved {
		roots = append(roots, cfg.RemovedDir)
	}

	seen := map[string]bool{}
	var result []string
	for _, root := range roots {
		if !fsutil.DirExists(root) {
			continue
		}
		if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
				return nil
			}
			if (root == cfg.BackupDir || root == cfg.RemovedDir) && filepath.Base(path) != "session.jsonl" {
				return nil
			}
			normalized := fsutil.NormalizePath(path)
			if !seen[normalized] {
				seen[normalized] = true
				result = append(result, normalized)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func aggregateProjects(sessions []domain.SessionRecord) []domain.ProjectRecord {
	byCWD := map[string]*domain.ProjectRecord{}
	for _, session := range sessions {
		if session.CWD == "" {
			continue
		}
		project := byCWD[session.CWD]
		if project == nil {
			project = &domain.ProjectRecord{CWD: session.CWD}
			byCWD[session.CWD] = project
		}
		project.TotalSessions++
		if session.Status == domain.SessionStatusVisible {
			project.VisibleCount++
		}
		if session.Status == domain.SessionStatusRecoverable || session.Status == domain.SessionStatusRemoved || session.Status == domain.SessionStatusArchived || session.Status == domain.SessionStatusInactive {
			project.RecoverableCount++
		}
		if session.IsBackedUp || session.Source == domain.SessionSourceBackup {
			project.BackedUpCount++
		}
		if session.Status == domain.SessionStatusConflict {
			project.ConflictCount++
		}
		if session.UpdatedAt.After(project.LastUpdatedAt) {
			project.LastUpdatedAt = session.UpdatedAt
		}
	}

	projects := make([]domain.ProjectRecord, 0, len(byCWD))
	for _, project := range byCWD {
		projects = append(projects, *project)
	}
	return projects
}

func readBackupIDs(backupDir string) map[string]bool {
	result := map[string]bool{}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return result
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(backupDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var manifest domain.BackupManifest
		if err := json.Unmarshal(data, &manifest); err == nil && manifest.SessionID != "" {
			result[manifest.SessionID] = true
		}
	}
	return result
}

type sessionIndexRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"thread_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

func readSessionIndex(cfg config.Config) map[string]sessionIndexRecord {
	result := map[string]sessionIndexRecord{}
	homes := append([]string{cfg.CodexHome}, cfg.OldCodexHomes...)
	for _, home := range homes {
		path := filepath.Join(home, "session_index.jsonl")
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			var record sessionIndexRecord
			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				continue
			}
			if record.ID == "" {
				continue
			}
			existing, ok := result[record.ID]
			if !ok || record.UpdatedAt.After(existing.UpdatedAt) {
				result[record.ID] = record
			}
		}
		_ = file.Close()
	}
	return result
}
