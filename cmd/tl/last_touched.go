package main

import (
	"os"
	"path/filepath"
	"strings"

	"task-ledger/internal/taskledger"
)

const lastTouchedFile = "last-touched"

// GetLastTouchedID returns the ID of the last touched issue.
// Returns empty string if no last touched issue exists or the file is unreadable.
func GetLastTouchedID() string {
	taskLedgerDir := taskledger.FindTaskLedgerDir()
	if taskLedgerDir == "" {
		return ""
	}

	lastTouchedPath := filepath.Join(taskLedgerDir, lastTouchedFile)
	data, err := os.ReadFile(lastTouchedPath) // #nosec G304 -- path constructed from taskLedgerDir
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// SetLastTouchedID saves the ID of the last touched issue.
// Silently ignores errors (best-effort tracking).
func SetLastTouchedID(issueID string) {
	if issueID == "" {
		return
	}

	taskLedgerDir := taskledger.FindTaskLedgerDir()
	if taskLedgerDir == "" {
		return
	}

	lastTouchedPath := filepath.Join(taskLedgerDir, lastTouchedFile)
	// Write with restrictive permissions (local-only state)
	_ = os.WriteFile(lastTouchedPath, []byte(issueID+"\n"), 0600)
}
