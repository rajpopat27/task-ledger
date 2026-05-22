// Package taskledger provides a minimal public API for task-ledger extension code.
package taskledger

import (
	internal "task-ledger/internal/taskledger"
	"task-ledger/internal/types"
)

// Storage is the interface for file-backed taskledger storage operations.
type Storage = internal.Storage

// Transaction provides atomic multi-operation support within a file-backed command.
type Transaction = internal.Transaction

// OpenStorage opens a JSON-file-backed Task Ledger project from its .task-ledger directory.
func OpenStorage(taskLedgerDir string) (Storage, error) {
	return internal.OpenStorage(taskLedgerDir)
}

// FindTaskLedgerDir finds the .task-ledger directory in the current directory tree.
func FindTaskLedgerDir() string {
	return internal.FindTaskLedgerDir()
}

// ProjectInfo contains information about a Task Ledger project.
type ProjectInfo = internal.ProjectInfo

// FindAllProjects finds the closest Task Ledger project from the current directory.
func FindAllProjects() []ProjectInfo {
	return internal.FindAllProjects()
}

// Core types from internal/types.
type (
	Issue            = types.Issue
	Status           = types.Status
	IssueType        = types.IssueType
	Dependency       = types.Dependency
	DependencyType   = types.DependencyType
	Label            = types.Label
	Comment          = types.Comment
	Event            = types.Event
	EventType        = types.EventType
	BlockedIssue     = types.BlockedIssue
	TreeNode         = types.TreeNode
	IssueFilter      = types.IssueFilter
	WorkFilter       = types.WorkFilter
	StaleFilter      = types.StaleFilter
	DependencyCounts = types.DependencyCounts
	IssueWithCounts  = types.IssueWithCounts
	SortPolicy       = types.SortPolicy
	EpicStatus       = types.EpicStatus
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
