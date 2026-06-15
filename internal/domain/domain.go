package domain

import "time"

type SessionSource string

const (
	SessionSourceVisible  SessionSource = "visible"
	SessionSourceInactive SessionSource = "inactive"
	SessionSourceArchived SessionSource = "archived"
	SessionSourceBackup   SessionSource = "backup"
	SessionSourceOldHome  SessionSource = "old_home"
	SessionSourceRemoved  SessionSource = "removed"
)

type SessionStatus string

const (
	SessionStatusVisible     SessionStatus = "visible"
	SessionStatusInactive    SessionStatus = "inactive"
	SessionStatusArchived    SessionStatus = "archived"
	SessionStatusRecoverable SessionStatus = "recoverable"
	SessionStatusBackedUp    SessionStatus = "backed_up"
	SessionStatusRemoved     SessionStatus = "removed"
	SessionStatusConflict    SessionStatus = "conflict"
)

type SessionRecord struct {
	ID             string
	Name           string
	CWD            string
	FilePath       string
	OriginalPath   string
	Source         SessionSource
	Status         SessionStatus
	CodexHome      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CLIVersion     string
	ModelProvider  string
	SizeBytes      int64
	SHA256         string
	IsCurrentHome  bool
	IsBackedUp     bool
	ConflictReason string
}

type ProjectRecord struct {
	CWD              string
	TotalSessions    int
	VisibleCount     int
	RecoverableCount int
	BackedUpCount    int
	ConflictCount    int
	LastUpdatedAt    time.Time
}

type BackupManifest struct {
	SessionID     string    `json:"session_id"`
	OriginalPath  string    `json:"original_path"`
	CWD           string    `json:"cwd"`
	SourceType    string    `json:"source_type"`
	CodexHome     string    `json:"codex_home"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CLIVersion    string    `json:"cli_version"`
	ModelProvider string    `json:"model_provider"`
	SHA256        string    `json:"sha256"`
	BackupAt      time.Time `json:"backup_at"`
}

type Inventory struct {
	Projects []ProjectRecord
	Sessions []SessionRecord
}
