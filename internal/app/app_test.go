package app

import (
	"testing"

	"github.com/sunlock/codex-session-manager/internal/domain"
)

func TestFindVisibleSessionPrefersVisibleRecord(t *testing.T) {
	id := "019ebb10-7a0e-7d70-95e8-c020b75687d8"
	sessions := []domain.SessionRecord{
		{ID: id, Status: domain.SessionStatusRecoverable, Source: domain.SessionSourceBackup, FilePath: "backup"},
		{ID: id, Status: domain.SessionStatusRemoved, Source: domain.SessionSourceRemoved, FilePath: "removed"},
		{ID: id, Status: domain.SessionStatusVisible, Source: domain.SessionSourceVisible, FilePath: "visible"},
	}

	session, ok := FindVisibleSession(sessions, id[:8])
	if !ok {
		t.Fatal("expected visible session")
	}
	if session.FilePath != "visible" {
		t.Fatalf("expected visible record, got %s", session.FilePath)
	}
}
