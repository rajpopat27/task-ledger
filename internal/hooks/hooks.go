// Package hooks provides a hook system for extensibility.
// Hooks are executable scripts in .task-ledger/hooks/ that run after certain events.
package hooks

import (
	"os"
	"path/filepath"
	"time"

	"task-ledger/internal/types"
)

// Event types
const (
	EventCreate = "create"
	EventUpdate = "update"
	EventClose  = "close"
)

// Hook file names
const (
	HookOnCreate = "on_create"
	HookOnUpdate = "on_update"
	HookOnClose  = "on_close"
)

// Runner handles hook execution
type Runner struct {
	hooksDir string
	timeout  time.Duration
}

// NewRunner creates a new hook runner.
// hooksDir is typically .task-ledger/hooks/ relative to workspace root.
func NewRunner(hooksDir string) *Runner {
	return &Runner{
		hooksDir: hooksDir,
		timeout:  10 * time.Second,
	}
}

// Run executes a hook if it exists.
// Runs asynchronously - returns immediately, hook runs in background.
func (r *Runner) Run(event string, issue *types.Issue) {
	hookPath, ok := r.runnableHook(event)
	if !ok {
		return
	}

	// Run asynchronously (ignore error as this is fire-and-forget)
	go func() {
		_ = r.runHook(hookPath, event, issue)
	}()
}

func (r *Runner) runnableHook(event string) (string, bool) {
	hookName := eventToHook(event)
	if hookName == "" {
		return "", false
	}

	hookPath := filepath.Join(r.hooksDir, hookName)

	// Check if hook exists and is executable
	info, err := os.Stat(hookPath)
	if err != nil || info.IsDir() {
		return "", false
	}

	if info.Mode()&0111 == 0 {
		return "", false
	}

	return hookPath, true
}

func eventToHook(event string) string {
	switch event {
	case EventCreate:
		return HookOnCreate
	case EventUpdate:
		return HookOnUpdate
	case EventClose:
		return HookOnClose
	default:
		return ""
	}
}
