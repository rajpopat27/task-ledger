// Package storage defines the interface for the JSON-file issue backend.
package storage

import (
	"context"

	"task-ledger/internal/types"
)

// Transaction provides atomic multi-operation support within a file-backed command.
//
// File-backed transactions lock the project, snapshot the touched issue files,
// execute the callback, and restore the snapshot if the callback returns an error.
type Transaction interface {
	// Issue operations.
	CreateIssue(ctx context.Context, issue *types.Issue, actor string) error
	CreateIssues(ctx context.Context, issues []*types.Issue, actor string) error
	UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error
	CloseIssue(ctx context.Context, id string, reason string, actor string, session string) error
	DeleteIssue(ctx context.Context, id string) error
	DeleteIssueWithReason(ctx context.Context, id, actor, reason string) error
	GetIssue(ctx context.Context, id string) (*types.Issue, error)
	SearchIssues(ctx context.Context, query string, filter types.IssueFilter) ([]*types.Issue, error)

	// Dependency operations.
	AddDependency(ctx context.Context, dep *types.Dependency, actor string) error
	RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error
	GetDependencies(ctx context.Context, issueID string) ([]*types.Issue, error)
	GetDependents(ctx context.Context, issueID string) ([]*types.Issue, error)
	GetDependencyRecords(ctx context.Context, issueID string) ([]*types.Dependency, error)

	// Label operations.
	AddLabel(ctx context.Context, issueID, label, actor string) error
	RemoveLabel(ctx context.Context, issueID, label, actor string) error

	// Comment operations.
	AddComment(ctx context.Context, issueID, actor, comment string) error
}

// Storage defines the retained JSON-file issue operations used by the CLI.
type Storage interface {
	// Issues.
	CreateIssue(ctx context.Context, issue *types.Issue, actor string) error
	CreateIssues(ctx context.Context, issues []*types.Issue, actor string) error
	GetIssue(ctx context.Context, id string) (*types.Issue, error)
	GetIssueByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error)
	UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error
	CloseIssue(ctx context.Context, id string, reason string, actor string, session string) error
	DeleteIssue(ctx context.Context, id string) error
	DeleteIssueWithReason(ctx context.Context, id, actor, reason string) error
	SearchIssues(ctx context.Context, query string, filter types.IssueFilter) ([]*types.Issue, error)

	// Dependencies.
	AddDependency(ctx context.Context, dep *types.Dependency, actor string) error
	RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error
	GetDependencies(ctx context.Context, issueID string) ([]*types.Issue, error)
	GetDependents(ctx context.Context, issueID string) ([]*types.Issue, error)
	GetDependenciesWithMetadata(ctx context.Context, issueID string) ([]*types.IssueWithDependencyMetadata, error)
	GetDependentsWithMetadata(ctx context.Context, issueID string) ([]*types.IssueWithDependencyMetadata, error)
	GetDependencyRecords(ctx context.Context, issueID string) ([]*types.Dependency, error)
	GetAllDependencyRecords(ctx context.Context) (map[string][]*types.Dependency, error)
	GetDependencyCounts(ctx context.Context, issueIDs []string) (map[string]*types.DependencyCounts, error)
	GetDependencyTree(ctx context.Context, issueID string, maxDepth int, showAllPaths bool, reverse bool) ([]*types.TreeNode, error)
	DetectCycles(ctx context.Context) ([][]*types.Issue, error)

	// Labels.
	AddLabel(ctx context.Context, issueID, label, actor string) error
	RemoveLabel(ctx context.Context, issueID, label, actor string) error
	GetLabels(ctx context.Context, issueID string) ([]string, error)
	GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error)
	GetIssuesByLabel(ctx context.Context, label string) ([]*types.Issue, error)

	// Ready work and blocking.
	GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error)
	GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error)
	IsBlocked(ctx context.Context, issueID string) (bool, []string, error)
	GetEpicsEligibleForClosure(ctx context.Context) ([]*types.EpicStatus, error)
	GetStaleIssues(ctx context.Context, filter types.StaleFilter) ([]*types.Issue, error)
	GetNewlyUnblockedByClose(ctx context.Context, closedIssueID string) ([]*types.Issue, error)

	// Events and comments.
	AddComment(ctx context.Context, issueID, actor, comment string) error
	GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error)
	AddIssueComment(ctx context.Context, issueID, author, text string) (*types.Comment, error)
	GetIssueComments(ctx context.Context, issueID string) ([]*types.Comment, error)
	GetCommentsForIssues(ctx context.Context, issueIDs []string) (map[string][]*types.Comment, error)

	// Statistics.
	GetStatistics(ctx context.Context) (*types.Statistics, error)

	// ID generation.
	GetNextChildID(ctx context.Context, parentID string) (string, error)

	// Config. These are command-local defaults used by retained helpers.
	SetConfig(ctx context.Context, key, value string) error
	GetConfig(ctx context.Context, key string) (string, error)
	GetAllConfig(ctx context.Context) (map[string]string, error)
	DeleteConfig(ctx context.Context, key string) error
	GetCustomStatuses(ctx context.Context) ([]string, error)
	GetCustomTypes(ctx context.Context) ([]string, error)

	// Transactions.
	RunInTransaction(ctx context.Context, fn func(tx Transaction) error) error

	// Lifecycle.
	Close() error
	Path() string
}
