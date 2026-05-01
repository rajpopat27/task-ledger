// Package types defines core data structures for the task-ledger issue tracker.
package types

import (
	"fmt"
	"strings"
	"time"
)

// Issue represents a trackable work item.
type Issue struct {
	ID string `json:"id"`

	Title              string `json:"title"`
	Description        string `json:"description,omitempty"`
	Design             string `json:"design,omitempty"`
	AcceptanceCriteria string `json:"acceptance_criteria,omitempty"`
	Notes              string `json:"notes,omitempty"`

	Status    Status    `json:"status,omitempty"`
	Priority  int       `json:"priority"`
	IssueType IssueType `json:"issue_type,omitempty"`

	Assignee         string `json:"assignee,omitempty"`
	Owner            string `json:"owner,omitempty"`
	EstimatedMinutes *int   `json:"estimated_minutes,omitempty"`

	CreatedAt       time.Time  `json:"created_at"`
	CreatedBy       string     `json:"created_by,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ClosedAt        *time.Time `json:"closed_at,omitempty"`
	CloseReason     string     `json:"close_reason,omitempty"`
	ClosedBySession string     `json:"closed_by_session,omitempty"`

	DueAt      *time.Time `json:"due_at,omitempty"`
	DeferUntil *time.Time `json:"defer_until,omitempty"`

	ExternalRef  *string `json:"external_ref,omitempty"`
	SourceSystem string  `json:"source_system,omitempty"`

	Labels       []string      `json:"labels,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
	Comments     []*Comment    `json:"comments,omitempty"`

	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	DeletedBy    string     `json:"deleted_by,omitempty"`
	DeleteReason string     `json:"delete_reason,omitempty"`
	OriginalType string     `json:"original_type,omitempty"`

	// Pinned issues are persistent context markers and are omitted from ready work.
	Pinned bool `json:"pinned,omitempty"`
}

// DefaultTombstoneTTL is the default time-to-live for tombstones.
const DefaultTombstoneTTL = 30 * 24 * time.Hour

// MinTombstoneTTL is the minimum allowed TTL to prevent data loss.
const MinTombstoneTTL = 7 * 24 * time.Hour

// ClockSkewGrace is added to longer TTLs to handle clock drift between machines.
const ClockSkewGrace = 1 * time.Hour

// IsTombstone returns true if the issue has been soft-deleted.
func (i *Issue) IsTombstone() bool {
	return i.Status == StatusTombstone
}

// IsExpired returns true if the tombstone has exceeded its TTL.
func (i *Issue) IsExpired(ttl time.Duration) bool {
	if !i.IsTombstone() || i.DeletedAt == nil {
		return false
	}
	if ttl < 0 {
		return true
	}
	if ttl == 0 {
		ttl = DefaultTombstoneTTL
	}
	effectiveTTL := ttl
	if ttl > ClockSkewGrace {
		effectiveTTL = ttl + ClockSkewGrace
	}
	return time.Now().After(i.DeletedAt.Add(effectiveTTL))
}

// Validate checks if the issue has valid field values.
func (i *Issue) Validate() error {
	return i.ValidateWithCustom(nil, nil)
}

// ValidateWithCustomStatuses checks if the issue has valid field values,
// allowing custom statuses in addition to built-in ones.
func (i *Issue) ValidateWithCustomStatuses(customStatuses []string) error {
	return i.ValidateWithCustom(customStatuses, nil)
}

// ValidateWithCustom checks if the issue has valid field values, allowing
// custom statuses and custom issue types.
func (i *Issue) ValidateWithCustom(customStatuses, customTypes []string) error {
	if i == nil {
		return fmt.Errorf("issue is nil")
	}
	if len(i.Title) == 0 {
		return fmt.Errorf("title is required")
	}
	if len(i.Title) > 500 {
		return fmt.Errorf("title must be 500 characters or less (got %d)", len(i.Title))
	}
	if i.Priority < 0 || i.Priority > 4 {
		return fmt.Errorf("priority must be between 0 and 4 (got %d)", i.Priority)
	}
	if !i.Status.IsValidWithCustom(customStatuses) {
		return fmt.Errorf("invalid status: %s", i.Status)
	}
	if !i.IssueType.IsValidWithCustom(customTypes) {
		return fmt.Errorf("invalid issue type: %s", i.IssueType)
	}
	if i.EstimatedMinutes != nil && *i.EstimatedMinutes < 0 {
		return fmt.Errorf("estimated_minutes cannot be negative")
	}
	if i.Status == StatusClosed && i.ClosedAt == nil {
		return fmt.Errorf("closed issues must have closed_at timestamp")
	}
	if i.Status != StatusClosed && i.Status != StatusTombstone && i.ClosedAt != nil {
		return fmt.Errorf("non-closed issues cannot have closed_at timestamp")
	}
	if i.Status == StatusTombstone && i.DeletedAt == nil {
		return fmt.Errorf("tombstone issues must have deleted_at timestamp")
	}
	if i.Status != StatusTombstone && i.DeletedAt != nil {
		return fmt.Errorf("non-tombstone issues cannot have deleted_at timestamp")
	}
	return nil
}

// SetDefaults applies default values for fields omitted during import.
func (i *Issue) SetDefaults() {
	if i.Status == "" {
		i.Status = StatusOpen
	}
	if i.IssueType == "" {
		i.IssueType = TypeTask
	}
}

// Status represents the current state of an issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusDeferred   Status = "deferred"
	StatusClosed     Status = "closed"
	StatusTombstone  Status = "tombstone"
	StatusPinned     Status = "pinned"
)

// IsValid checks if the status value is valid.
func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusBlocked, StatusDeferred, StatusClosed, StatusTombstone, StatusPinned:
		return true
	}
	return false
}

// IsValidWithCustom checks if the status is valid, including custom statuses.
func (s Status) IsValidWithCustom(customStatuses []string) bool {
	if s.IsValid() {
		return true
	}
	for _, custom := range customStatuses {
		if string(s) == custom {
			return true
		}
	}
	return false
}

// IssueType categorizes the kind of work.
type IssueType string

const (
	TypeBug            IssueType = "bug"
	TypeFeature        IssueType = "feature"
	TypeFeatureRequest IssueType = "feature_request"
	TypeTask           IssueType = "task"
	TypeEpic           IssueType = "epic"
	TypeChore          IssueType = "chore"
)

// IsValid checks if the issue type value is valid.
func (t IssueType) IsValid() bool {
	switch t {
	case TypeBug, TypeFeature, TypeFeatureRequest, TypeTask, TypeEpic, TypeChore:
		return true
	}
	return false
}

// IsBuiltIn returns true if the type is a built-in type.
func (t IssueType) IsBuiltIn() bool {
	return t.IsValid()
}

// IsValidWithCustom checks if the issue type is valid, including custom types.
func (t IssueType) IsValidWithCustom(customTypes []string) bool {
	if t.IsValid() {
		return true
	}
	for _, custom := range customTypes {
		if string(t) == custom {
			return true
		}
	}
	return false
}

// RequiredSection describes a recommended section for an issue type.
type RequiredSection struct {
	Heading string
	Hint    string
}

// RequiredSections returns the recommended sections for this issue type.
func (t IssueType) RequiredSections() []RequiredSection {
	switch t {
	case TypeBug:
		return []RequiredSection{
			{Heading: "## Steps to Reproduce", Hint: "Describe how to reproduce the bug"},
			{Heading: "## Acceptance Criteria", Hint: "Define criteria to verify the fix"},
		}
	case TypeTask, TypeFeature, TypeFeatureRequest:
		return []RequiredSection{
			{Heading: "## Acceptance Criteria", Hint: "Define criteria to verify completion"},
		}
	case TypeEpic:
		return []RequiredSection{
			{Heading: "## Success Criteria", Hint: "Define high-level success criteria"},
		}
	default:
		return nil
	}
}

// Dependency represents a relationship between issues.
type Dependency struct {
	IssueID     string         `json:"issue_id"`
	DependsOnID string         `json:"depends_on_id"`
	Type        DependencyType `json:"type"`
	CreatedAt   time.Time      `json:"created_at"`
	CreatedBy   string         `json:"created_by,omitempty"`
	Metadata    string         `json:"metadata,omitempty"`
	ThreadID    string         `json:"thread_id,omitempty"`
}

// DependencyCounts holds counts for dependencies and dependents.
type DependencyCounts struct {
	DependencyCount int `json:"dependency_count"`
	DependentCount  int `json:"dependent_count"`
}

// IssueWithDependencyMetadata extends Issue with dependency relationship type.
type IssueWithDependencyMetadata struct {
	Issue
	DependencyType DependencyType `json:"dependency_type"`
}

// IssueWithCounts extends Issue with dependency relationship counts.
type IssueWithCounts struct {
	*Issue
	DependencyCount int `json:"dependency_count"`
	DependentCount  int `json:"dependent_count"`
}

// IssueDetails extends Issue with labels, dependencies, dependents, and comments.
type IssueDetails struct {
	Issue
	Labels       []string                       `json:"labels,omitempty"`
	Dependencies []*IssueWithDependencyMetadata `json:"dependencies,omitempty"`
	Dependents   []*IssueWithDependencyMetadata `json:"dependents,omitempty"`
	Comments     []*Comment                     `json:"comments,omitempty"`
	Parent       *string                        `json:"parent,omitempty"`
}

// DependencyType categorizes the relationship.
type DependencyType string

const (
	DepBlocks            DependencyType = "blocks"
	DepParentChild       DependencyType = "parent-child"
	DepConditionalBlocks DependencyType = "conditional-blocks"

	DepRelated        DependencyType = "related"
	DepDiscoveredFrom DependencyType = "discovered-from"
	DepRepliesTo      DependencyType = "replies-to"
	DepRelatesTo      DependencyType = "relates-to"
	DepDuplicates     DependencyType = "duplicates"
	DepSupersedes     DependencyType = "supersedes"
	DepTracks         DependencyType = "tracks"
	DepUntil          DependencyType = "until"
	DepCausedBy       DependencyType = "caused-by"
	DepValidates      DependencyType = "validates"
)

// IsValid checks if the dependency type value is valid.
func (d DependencyType) IsValid() bool {
	return len(d) > 0 && len(d) <= 50
}

// IsWellKnown checks if the dependency type is a built-in constant.
func (d DependencyType) IsWellKnown() bool {
	switch d {
	case DepBlocks, DepParentChild, DepConditionalBlocks, DepRelated, DepDiscoveredFrom,
		DepRepliesTo, DepRelatesTo, DepDuplicates, DepSupersedes, DepTracks,
		DepUntil, DepCausedBy, DepValidates:
		return true
	}
	return false
}

// AffectsReadyWork returns true if this dependency type blocks work.
func (d DependencyType) AffectsReadyWork() bool {
	return d == DepBlocks || d == DepParentChild || d == DepConditionalBlocks
}

// FailureCloseKeywords are keywords that indicate an issue was closed due to failure.
var FailureCloseKeywords = []string{
	"failed",
	"rejected",
	"wontfix",
	"won't fix",
	"canceled",
	"cancelled", //nolint:misspell // Keep British spelling as accepted user input.
	"abandoned",
	"blocked",
	"error",
	"timeout",
	"aborted",
}

// IsFailureClose returns true if the close reason indicates the issue failed.
func IsFailureClose(closeReason string) bool {
	if closeReason == "" {
		return false
	}
	lower := strings.ToLower(closeReason)
	for _, keyword := range FailureCloseKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

// Label represents a tag on an issue.
type Label struct {
	IssueID string `json:"issue_id"`
	Label   string `json:"label"`
}

// Comment represents a comment on an issue.
type Comment struct {
	ID        int64     `json:"id"`
	IssueID   string    `json:"issue_id"`
	Author    string    `json:"author"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// Event represents an audit trail entry.
type Event struct {
	ID        int64     `json:"id"`
	IssueID   string    `json:"issue_id"`
	EventType EventType `json:"event_type"`
	Actor     string    `json:"actor"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	Comment   *string   `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EventType categorizes audit trail events.
type EventType string

const (
	EventCreated           EventType = "created"
	EventUpdated           EventType = "updated"
	EventStatusChanged     EventType = "status_changed"
	EventCommented         EventType = "commented"
	EventClosed            EventType = "closed"
	EventReopened          EventType = "reopened"
	EventDependencyAdded   EventType = "dependency_added"
	EventDependencyRemoved EventType = "dependency_removed"
	EventLabelAdded        EventType = "label_added"
	EventLabelRemoved      EventType = "label_removed"
)

// BlockedIssue extends Issue with blocking information.
type BlockedIssue struct {
	Issue
	BlockedByCount int      `json:"blocked_by_count"`
	BlockedBy      []string `json:"blocked_by"`
}

// TreeNode represents a node in a dependency tree.
type TreeNode struct {
	Issue
	Depth     int    `json:"depth"`
	ParentID  string `json:"parent_id"`
	Truncated bool   `json:"truncated"`
}

// Statistics provides aggregate metrics.
type Statistics struct {
	TotalIssues             int     `json:"total_issues"`
	OpenIssues              int     `json:"open_issues"`
	InProgressIssues        int     `json:"in_progress_issues"`
	ClosedIssues            int     `json:"closed_issues"`
	BlockedIssues           int     `json:"blocked_issues"`
	DeferredIssues          int     `json:"deferred_issues"`
	ReadyIssues             int     `json:"ready_issues"`
	TombstoneIssues         int     `json:"tombstone_issues"`
	PinnedIssues            int     `json:"pinned_issues"`
	EpicsEligibleForClosure int     `json:"epics_eligible_for_closure"`
	AverageLeadTime         float64 `json:"average_lead_time_hours"`
}

// IssueFilter is used to filter issue queries.
type IssueFilter struct {
	Status      *Status
	Priority    *int
	IssueType   *IssueType
	Assignee    *string
	Labels      []string
	LabelsAny   []string
	TitleSearch string
	IDs         []string
	IDPrefix    string
	Limit       int

	TitleContains       string
	DescriptionContains string
	NotesContains       string

	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	UpdatedAfter  *time.Time
	UpdatedBefore *time.Time
	ClosedAfter   *time.Time
	ClosedBefore  *time.Time

	EmptyDescription bool
	NoAssignee       bool
	NoLabels         bool

	PriorityMin *int
	PriorityMax *int

	IncludeTombstones bool
	Pinned            *bool
	ParentID          *string
	ExcludeStatus     []Status
	ExcludeTypes      []IssueType

	Deferred    bool
	DeferAfter  *time.Time
	DeferBefore *time.Time
	DueAfter    *time.Time
	DueBefore   *time.Time
	Overdue     bool
}

// SortPolicy determines how ready work is ordered.
type SortPolicy string

const (
	SortPolicyHybrid   SortPolicy = "hybrid"
	SortPolicyPriority SortPolicy = "priority"
	SortPolicyOldest   SortPolicy = "oldest"
)

// IsValid checks if the sort policy value is valid.
func (s SortPolicy) IsValid() bool {
	switch s {
	case SortPolicyHybrid, SortPolicyPriority, SortPolicyOldest, "":
		return true
	}
	return false
}

// WorkFilter is used to filter ready work queries.
type WorkFilter struct {
	Status     Status
	Type       string
	Priority   *int
	Assignee   *string
	Unassigned bool
	Labels     []string
	LabelsAny  []string
	Limit      int
	SortPolicy SortPolicy

	ParentID        *string
	IncludeDeferred bool
}

// StaleFilter is used to filter stale issue queries.
type StaleFilter struct {
	Days   int
	Status string
	Limit  int
}

// EpicStatus represents an epic with its completion status.
type EpicStatus struct {
	Epic             *Issue `json:"epic"`
	TotalChildren    int    `json:"total_children"`
	ClosedChildren   int    `json:"closed_children"`
	EligibleForClose bool   `json:"eligible_for_close"`
}
