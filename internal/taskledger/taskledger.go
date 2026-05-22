// Package taskledger provides the small public API needed by task-ledger extensions.
//
// The retained backend stores issues directly as JSON files under
// .task-ledger/issues/**/issue.json. This package exposes project discovery, the
// file-backed storage interface, and core issue types for callers that embed
// task-ledger programmatically.
package taskledger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"task-ledger/internal/git"
	"task-ledger/internal/storage"
	filestorage "task-ledger/internal/storage/file"
	"task-ledger/internal/types"
	"task-ledger/internal/utils"
)

// RedirectFileName is the name of the file that redirects to another .task-ledger directory.
const RedirectFileName = "redirect"

// FollowRedirect checks if a .task-ledger directory contains a redirect file and follows it.
// If a redirect file exists, it returns the target .task-ledger directory path.
// If no redirect exists or there's an error, it returns the original path unchanged.
//
// The redirect file should contain a single path (relative or absolute) to the target
// .task-ledger directory. Relative paths are resolved from the parent directory of the
// original .task-ledger directory, which is the project root.
//
// Redirect chains are not followed. Only one level of redirection is supported.
func FollowRedirect(taskLedgerDir string) string {
	redirectFile := filepath.Join(taskLedgerDir, RedirectFileName)
	data, err := os.ReadFile(redirectFile) // #nosec G304,G703 -- redirect file is resolved from a discovered/configured .task-ledger directory.
	if err != nil {
		return taskLedgerDir
	}

	target := strings.TrimSpace(string(data))
	lines := strings.Split(target, "\n")
	target = ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			target = line
			break
		}
	}

	if target == "" {
		return taskLedgerDir
	}

	if !filepath.IsAbs(target) {
		projectRoot := filepath.Dir(taskLedgerDir)
		target = filepath.Join(projectRoot, target)
	}

	target = utils.CanonicalizePath(target)
	info, err := os.Stat(target) // #nosec G703 -- redirect target is intentionally user-configurable and must be statted before use.
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Warning: redirect target does not exist or is not a directory: %s\n", target)
		return taskLedgerDir
	}

	targetRedirect := filepath.Join(target, RedirectFileName)
	if _, err := os.Stat(targetRedirect); err == nil { // #nosec G703 -- redirect target is intentionally user-configurable.
		fmt.Fprintf(os.Stderr, "Warning: redirect chains not allowed, ignoring redirect in %s\n", target)
	}

	return target
}

// Issue represents a tracked work item with metadata, dependencies, and status.
type (
	Issue = types.Issue
	// Status represents the current state of an issue.
	Status = types.Status
	// IssueType represents the type of issue.
	IssueType = types.IssueType
	// Dependency represents a relationship between issues.
	Dependency = types.Dependency
	// DependencyType represents the type of dependency.
	DependencyType = types.DependencyType
	// Comment represents a user comment on an issue.
	Comment = types.Comment
	// Event represents an audit log event.
	Event = types.Event
	// EventType represents the type of audit event.
	EventType = types.EventType
	// Label represents a tag attached to an issue.
	Label = types.Label
	// BlockedIssue represents an issue with blocking dependencies.
	BlockedIssue = types.BlockedIssue
	// TreeNode represents a node in a dependency tree.
	TreeNode = types.TreeNode
	// Statistics represents project-wide metrics.
	Statistics = types.Statistics
	// IssueFilter represents filtering criteria for issue queries.
	IssueFilter = types.IssueFilter
	// WorkFilter represents filtering criteria for work queries.
	WorkFilter = types.WorkFilter
	// SortPolicy determines how ready work is ordered.
	SortPolicy = types.SortPolicy
	// EpicStatus represents the status of an epic issue.
	EpicStatus = types.EpicStatus
)

// Status constants.
const (
	StatusOpen        = types.StatusOpen
	StatusInProgress  = types.StatusInProgress
	StatusInReview    = types.StatusInReview
	StatusHumanReview = types.StatusHumanReview
	StatusBlocked     = types.StatusBlocked
	StatusDeferred    = types.StatusDeferred
	StatusClosed      = types.StatusClosed
)

// IssueType constants.
const (
	TypeBug            = types.TypeBug
	TypeFeature        = types.TypeFeature
	TypeFeatureRequest = types.TypeFeatureRequest
	TypeTask           = types.TypeTask
	TypeEpic           = types.TypeEpic
	TypeChore          = types.TypeChore
)

// DependencyType constants.
const (
	DepBlocks            = types.DepBlocks
	DepRelated           = types.DepRelated
	DepParentChild       = types.DepParentChild
	DepDiscoveredFrom    = types.DepDiscoveredFrom
	DepConditionalBlocks = types.DepConditionalBlocks
)

// SortPolicy constants.
const (
	SortPolicyHybrid   = types.SortPolicyHybrid
	SortPolicyPriority = types.SortPolicyPriority
	SortPolicyOldest   = types.SortPolicyOldest
)

// EventType constants.
const (
	EventCreated           = types.EventCreated
	EventUpdated           = types.EventUpdated
	EventStatusChanged     = types.EventStatusChanged
	EventCommented         = types.EventCommented
	EventClosed            = types.EventClosed
	EventReopened          = types.EventReopened
	EventDependencyAdded   = types.EventDependencyAdded
	EventDependencyRemoved = types.EventDependencyRemoved
	EventLabelAdded        = types.EventLabelAdded
	EventLabelRemoved      = types.EventLabelRemoved
)

// Storage provides file-backed issue operations.
type Storage = storage.Storage

// Transaction provides atomic multi-operation support within a file-backed command.
type Transaction = storage.Transaction

// OpenStorage opens a JSON-file-backed Task Ledger project from its .task-ledger directory.
func OpenStorage(taskLedgerDir string) (Storage, error) {
	return filestorage.Open(taskLedgerDir)
}

// hasTaskLedgerProjectFiles checks if a .task-ledger directory contains project-owned files.
func hasTaskLedgerProjectFiles(taskLedgerDir string) bool {
	if info, err := os.Stat(filepath.Join(taskLedgerDir, "issues")); err == nil && info.IsDir() { // #nosec G703 -- taskLedgerDir is discovered/configured project storage.
		return true
	}
	if _, err := os.Stat(filepath.Join(taskLedgerDir, "metadata.json")); err == nil { // #nosec G703 -- taskLedgerDir is discovered/configured project storage.
		return true
	}
	if _, err := os.Stat(filepath.Join(taskLedgerDir, "config.yaml")); err == nil { // #nosec G703 -- taskLedgerDir is discovered/configured project storage.
		return true
	}
	return false
}

// FindTaskLedgerDir finds the active .task-ledger directory in the current directory tree.
// It returns an empty string if no JSON-file project is found.
func FindTaskLedgerDir() string {
	if taskLedgerDir := os.Getenv("TASK_LEDGER_DIR"); taskLedgerDir != "" {
		return usableTaskLedgerDir(utils.CanonicalizePath(taskLedgerDir))
	}

	mainRepoRoot, mainRepoTaskLedgerDir := worktreeMainRepoTaskLedgerDir()
	if mainRepoTaskLedgerDir != "" {
		return mainRepoTaskLedgerDir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	gitRoot := findGitRoot()
	if git.IsWorktree() && mainRepoRoot != "" {
		gitRoot = mainRepoRoot
	}

	return findTaskLedgerDirUpward(cwd, gitRoot)
}

func usableTaskLedgerDir(taskLedgerDir string) string {
	taskLedgerDir = FollowRedirect(taskLedgerDir)
	if info, err := os.Stat(taskLedgerDir); err == nil && info.IsDir() && hasTaskLedgerProjectFiles(taskLedgerDir) { // #nosec G703 -- taskLedgerDir is discovered/configured project storage.
		return taskLedgerDir
	}
	return ""
}

func worktreeMainRepoTaskLedgerDir() (string, string) {
	if !git.IsWorktree() {
		return "", ""
	}
	mainRepoRoot, err := git.GetMainRepoRoot()
	if err != nil || mainRepoRoot == "" {
		return "", ""
	}
	return mainRepoRoot, usableTaskLedgerDir(filepath.Join(mainRepoRoot, ".task-ledger"))
}

func findTaskLedgerDirUpward(cwd, gitRoot string) string {
	for dir := cwd; ; {
		if taskLedgerDir := usableTaskLedgerDir(filepath.Join(dir, ".task-ledger")); taskLedgerDir != "" {
			return taskLedgerDir
		}

		if gitRoot != "" && dir == gitRoot {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// ProjectInfo contains information about a discovered Task Ledger project.
type ProjectInfo struct {
	Path          string // Full path to the .task-ledger directory.
	TaskLedgerDir string // Same as Path, retained for callers that prefer explicit naming.
	IssueCount    int    // Number of issues, or -1 if the project cannot be opened.
}

// findGitRoot returns the root directory of the current git repository.
func findGitRoot() string {
	return git.GetRepoRoot()
}

// FindAllProjects scans the current directory hierarchy for the closest Task Ledger project.
func FindAllProjects() []ProjectInfo {
	projects := []ProjectInfo{}
	seen := make(map[string]bool)

	dir, err := os.Getwd()
	if err != nil {
		return projects
	}

	gitRoot := findGitRoot()

	for {
		taskLedgerDir := FollowRedirect(filepath.Join(dir, ".task-ledger"))
		if info, err := os.Stat(taskLedgerDir); err == nil && info.IsDir() && hasTaskLedgerProjectFiles(taskLedgerDir) {
			canonicalPath := taskLedgerDir
			if resolved, err := filepath.EvalSymlinks(taskLedgerDir); err == nil {
				canonicalPath = resolved
			}
			if !seen[canonicalPath] {
				seen[canonicalPath] = true
				projects = append(projects, ProjectInfo{
					Path:          taskLedgerDir,
					TaskLedgerDir: taskLedgerDir,
					IssueCount:    countIssues(taskLedgerDir),
				})
			}
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		if gitRoot != "" && dir == gitRoot {
			break
		}
		dir = parent
	}

	return projects
}

func countIssues(taskLedgerDir string) int {
	store, err := filestorage.Open(taskLedgerDir)
	if err != nil {
		return -1
	}
	defer func() { _ = store.Close() }()

	issues, err := store.SearchIssues(context.Background(), "", types.IssueFilter{})
	if err != nil {
		return -1
	}
	return len(issues)
}
