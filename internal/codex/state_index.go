package codex

import (
	"database/sql"
	"net/url"
	"path/filepath"
	"time"

	"github.com/sunlock/codex-session-manager/internal/config"
	"github.com/sunlock/codex-session-manager/internal/domain"
	"github.com/sunlock/codex-session-manager/internal/fsutil"
	_ "modernc.org/sqlite"
)

// ThreadRecord 表示 Codex 状态库中的线程索引记录。
type ThreadRecord struct {
	ID            string
	RolloutPath   string
	ModelProvider string
	Archived      bool
}

// ThreadIndex 保存当前 Codex 状态库中的线程索引。
type ThreadIndex struct {
	Available     bool
	ModelProvider string
	ByID          map[string]ThreadRecord
}

// ReadThreadIndex 读取当前 CODEX_HOME/state_5.sqlite 中的 Codex 线程索引。
func ReadThreadIndex(cfg config.Config) ThreadIndex {
	index := ThreadIndex{
		ModelProvider: cfg.ModelProvider,
		ByID:          map[string]ThreadRecord{},
	}
	dbPath := filepath.Join(cfg.CodexHome, "state_5.sqlite")
	if !fsutil.FileExists(dbPath) {
		return index
	}

	db, err := sql.Open("sqlite", sqliteReadOnlyDSN(dbPath))
	if err != nil {
		return index
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT id, rollout_path, model_provider, archived
		FROM threads
	`)
	if err != nil {
		return index
	}
	defer rows.Close()

	for rows.Next() {
		var record ThreadRecord
		var archived int
		if err := rows.Scan(&record.ID, &record.RolloutPath, &record.ModelProvider, &archived); err != nil {
			continue
		}
		if record.ID == "" {
			continue
		}
		record.RolloutPath = fsutil.NormalizePath(record.RolloutPath)
		record.Archived = archived != 0
		index.ByID[record.ID] = record
	}
	if err := rows.Err(); err != nil {
		return ThreadIndex{ByID: map[string]ThreadRecord{}}
	}
	index.Available = true
	return index
}

// IsActive 判断 session 文件是否仍在当前 Codex CLI 活跃线程索引中。
func (index ThreadIndex) IsActive(sessionID string, filePath string) bool {
	if !index.Available {
		return true
	}
	record, ok := index.ByID[sessionID]
	if !ok || record.Archived {
		return false
	}
	if index.ModelProvider != "" && record.ModelProvider != "" && index.ModelProvider != record.ModelProvider {
		return false
	}
	if record.RolloutPath == "" {
		return true
	}
	return samePath(record.RolloutPath, filePath)
}

func sqliteReadOnlyDSN(path string) string {
	query := url.Values{}
	query.Set("mode", "ro")
	query.Set("_pragma", "query_only(1)")
	uri := url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}
	return uri.String()
}

func samePath(left string, right string) bool {
	return fsutil.NormalizePath(left) == fsutil.NormalizePath(right)
}

// RegisterThread 将 session 注册到当前 Codex 状态库，使当前账号/provider 可以 resume。
func RegisterThread(cfg config.Config, session domain.SessionRecord, rolloutPath string) error {
	dbPath := filepath.Join(cfg.CodexHome, "state_5.sqlite")
	if !fsutil.FileExists(dbPath) {
		return nil
	}
	db, err := sql.Open("sqlite", sqliteReadWriteDSN(dbPath))
	if err != nil {
		return err
	}
	defer db.Close()

	title := session.Name
	if title == "" {
		title = session.ID
	}
	modelProvider := cfg.ModelProvider
	if modelProvider == "" {
		modelProvider = session.ModelProvider
	}
	if modelProvider == "" {
		modelProvider = "unknown"
	}
	createdAt := unixSeconds(session.CreatedAt)
	updatedAt := unixSeconds(session.UpdatedAt)
	if updatedAt < createdAt {
		updatedAt = createdAt
	}

	_, err = db.Exec(`
		INSERT INTO threads (
			id, rollout_path, created_at, updated_at, source, model_provider,
			cwd, title, sandbox_policy, approval_mode, tokens_used, has_user_event,
			archived, archived_at, cli_version, first_user_message, memory_mode,
			model, reasoning_effort, thread_source, preview
		)
		VALUES (?, ?, ?, ?, 'cli', ?, ?, ?, '{"type":"disabled"}', 'never', 0, 1, 0, NULL, ?, ?, 'enabled', NULL, NULL, 'user', ?)
		ON CONFLICT(id) DO UPDATE SET
			rollout_path = excluded.rollout_path,
			updated_at = excluded.updated_at,
			source = excluded.source,
			model_provider = excluded.model_provider,
			cwd = excluded.cwd,
			title = excluded.title,
			archived = 0,
			archived_at = NULL,
			cli_version = excluded.cli_version,
			first_user_message = CASE
				WHEN threads.first_user_message = '' THEN excluded.first_user_message
				ELSE threads.first_user_message
			END,
			thread_source = excluded.thread_source,
			preview = excluded.preview
	`, session.ID, fsutil.NormalizePath(rolloutPath), createdAt, updatedAt, modelProvider, session.CWD, title, session.CLIVersion, title, title)
	return err
}

func sqliteReadWriteDSN(path string) string {
	query := url.Values{}
	query.Set("mode", "rw")
	uri := url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}
	return uri.String()
}

func unixSeconds(value time.Time) int64 {
	if value.IsZero() {
		return time.Now().Unix()
	}
	return value.Unix()
}
