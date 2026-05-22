package file

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"task-ledger/internal/config"
	"task-ledger/internal/types"
)

// issueIndex is the command-local query/mutation model rebuilt from issue.json
// files for each CLI process. Disk remains the durable source of truth.
type issueIndex struct {
	mu sync.RWMutex // Protects all maps

	// Core data
	issues       map[string]*types.Issue        // ID -> Issue
	dependencies map[string][]*types.Dependency // IssueID -> Dependencies
	labels       map[string][]string            // IssueID -> Labels
	events       map[string][]*types.Event      // IssueID -> Events
	comments     map[string][]*types.Comment    // IssueID -> Comments
	config       map[string]string              // Config key-value pairs
	counters     map[string]int                 // Prefix -> Last ID

	// Indexes for O(1) lookups
	externalRefToID map[string]string // ExternalRef -> IssueID

	closed bool
}

// newIssueIndex creates a command-local issue index
func newIssueIndex() *issueIndex {
	return &issueIndex{
		issues:          make(map[string]*types.Issue),
		dependencies:    make(map[string][]*types.Dependency),
		labels:          make(map[string][]string),
		events:          make(map[string][]*types.Event),
		comments:        make(map[string][]*types.Comment),
		config:          make(map[string]string),
		counters:        make(map[string]int),
		externalRefToID: make(map[string]string),
	}
}

// LoadFromIssues populates the command-local issue index from a slice of issues
// This is used when loading from file JSON at startup
func (m *issueIndex) LoadFromIssues(issues []*types.Issue) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.validateLoadIssuesLocked(issues); err != nil {
		return err
	}

	for _, issue := range issues {
		if issue == nil {
			continue
		}
		m.addIssueToIndexLocked(issue)
	}

	return nil
}

func (m *issueIndex) validateLoadIssuesLocked(issues []*types.Issue) error {
	batchIDs := make(map[string]struct{})
	batchExternalRefs := make(map[string]string)
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		if _, exists := m.issues[issue.ID]; exists {
			return fmt.Errorf("issue %s already exists", issue.ID)
		}
		if _, exists := batchIDs[issue.ID]; exists {
			return fmt.Errorf("duplicate issue id %s", issue.ID)
		}
		batchIDs[issue.ID] = struct{}{}
		if issue.ExternalRef != nil && *issue.ExternalRef != "" {
			ref := *issue.ExternalRef
			if err := m.checkExternalRefAvailableLocked(ref, issue.ID); err != nil {
				return err
			}
			if otherID, exists := batchExternalRefs[ref]; exists && otherID != issue.ID {
				return fmt.Errorf("external_ref %s already belongs to %s; cannot assign to %s", ref, otherID, issue.ID)
			}
			batchExternalRefs[ref] = issue.ID
		}
	}

	return nil
}

func (m *issueIndex) addIssueToIndexLocked(issue *types.Issue) {
	m.issues[issue.ID] = issue

	if issue.ExternalRef != nil && *issue.ExternalRef != "" {
		m.externalRefToID[*issue.ExternalRef] = issue.ID
	}
	if len(issue.Dependencies) > 0 {
		m.dependencies[issue.ID] = issue.Dependencies
	}
	if len(issue.Labels) > 0 {
		m.labels[issue.ID] = issue.Labels
	}
	if len(issue.Comments) > 0 {
		m.comments[issue.ID] = issue.Comments
	}

	m.advanceCountersLocked(issue.ID)
}

func (m *issueIndex) advanceCountersLocked(issueID string) {
	prefix, num := extractPrefixAndNumber(issueID)
	if prefix != "" && num > 0 && m.counters[prefix] < num {
		m.counters[prefix] = num
	}

	if parentID, childNum, ok := extractParentAndChildNumber(issueID); ok && m.counters[parentID] < childNum {
		m.counters[parentID] = childNum
	}
}

func (m *issueIndex) checkExternalRefAvailableLocked(ref, issueID string) error {
	if ref == "" {
		return nil
	}
	if existingID, exists := m.externalRefToID[ref]; exists && existingID != issueID {
		return fmt.Errorf("external_ref %s already belongs to %s; cannot assign to %s", ref, existingID, issueID)
	}
	return nil
}

// GetAllIssues returns all issues in the command-local index (for export to file JSON)
func (m *issueIndex) GetAllIssues() []*types.Issue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	issues := make([]*types.Issue, 0, len(m.issues))
	for _, issue := range m.issues {
		issues = append(issues, m.copyIssueLocked(issue, issueCopyOptions{
			dependencies: true,
			labels:       true,
			comments:     true,
		}))
	}

	// Sort by ID for consistent output
	sort.Slice(issues, func(i, j int) bool {
		return issues[i].ID < issues[j].ID
	})

	return issues
}

// extractPrefixAndNumber extracts prefix and number from issue ID like "tl-123" -> ("tl", 123)
func extractPrefixAndNumber(id string) (string, int) {
	lastDash := strings.LastIndex(id, "-")
	if lastDash == -1 {
		return "", 0
	}

	prefix := id[:lastDash]
	suffix := id[lastDash+1:]

	var num int
	_, err := fmt.Sscanf(suffix, "%d", &num)
	if err != nil {
		return "", 0
	}
	return prefix, num
}

// extractParentAndChildNumber extracts the parent ID and numeric child counter from an issue ID like
// "tl-a3f8e9.2" -> ("tl-a3f8e9", 2, true).
func extractParentAndChildNumber(id string) (string, int, bool) {
	lastDot := strings.LastIndex(id, ".")
	if lastDot == -1 {
		return "", 0, false
	}

	parentID := id[:lastDot]
	suffix := id[lastDot+1:]

	var num int
	if _, err := fmt.Sscanf(suffix, "%d", &num); err != nil {
		return "", 0, false
	}

	return parentID, num, true
}

// CreateIssue creates a new issue
func (m *issueIndex) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate
	if err := issue.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Set timestamps
	now := time.Now()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	// Generate ID if not set
	if issue.ID == "" {
		prefix := m.config["issue_prefix"]
		if prefix == "" {
			prefix = "tl" // Default fallback
		}

		// Get next ID
		m.counters[prefix]++
		issue.ID = fmt.Sprintf("%s-%d", prefix, m.counters[prefix])
	}

	// Check for duplicate
	if _, exists := m.issues[issue.ID]; exists {
		return fmt.Errorf("issue %s already exists", issue.ID)
	}
	if issue.ExternalRef != nil {
		if err := m.checkExternalRefAvailableLocked(*issue.ExternalRef, issue.ID); err != nil {
			return err
		}
	}

	// Store issue
	m.issues[issue.ID] = issue

	// Index external ref for O(1) lookup
	if issue.ExternalRef != nil && *issue.ExternalRef != "" {
		m.externalRefToID[*issue.ExternalRef] = issue.ID
	}

	// Record event
	event := &types.Event{
		IssueID:   issue.ID,
		EventType: types.EventCreated,
		Actor:     actor,
		CreatedAt: now,
	}
	m.events[issue.ID] = append(m.events[issue.ID], event)

	return nil
}

// CreateIssues creates multiple issues atomically
func (m *issueIndex) CreateIssues(ctx context.Context, issues []*types.Issue, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate all first
	for i, issue := range issues {
		if err := issue.Validate(); err != nil {
			return fmt.Errorf("validation failed for issue %d: %w", i, err)
		}
	}

	now := time.Now()
	counters := make(map[string]int, len(m.counters))
	for prefix, counter := range m.counters {
		counters[prefix] = counter
	}

	// Track IDs and external refs in this batch to detect duplicates before mutating maps.
	batchIDs := make(map[string]bool)
	batchExternalRefs := make(map[string]string)

	// Generate IDs for issues that need them
	for _, issue := range issues {
		issue.CreatedAt = now
		issue.UpdatedAt = now

		if issue.ID == "" {
			prefix := m.config["issue_prefix"]
			if prefix == "" {
				prefix = "tl"
			}
			counters[prefix]++
			issue.ID = fmt.Sprintf("%s-%d", prefix, counters[prefix])
		}

		// Check for duplicates in existing issues
		if _, exists := m.issues[issue.ID]; exists {
			return fmt.Errorf("issue %s already exists", issue.ID)
		}

		// Check for duplicates within this batch
		if batchIDs[issue.ID] {
			return fmt.Errorf("duplicate ID within batch: %s", issue.ID)
		}
		batchIDs[issue.ID] = true

		if issue.ExternalRef != nil && *issue.ExternalRef != "" {
			ref := *issue.ExternalRef
			if err := m.checkExternalRefAvailableLocked(ref, issue.ID); err != nil {
				return err
			}
			if existingID, exists := batchExternalRefs[ref]; exists && existingID != issue.ID {
				return fmt.Errorf("external_ref %s already belongs to %s; cannot assign to %s", ref, existingID, issue.ID)
			}
			batchExternalRefs[ref] = issue.ID
		}
	}

	m.counters = counters

	// Store all issues
	for _, issue := range issues {
		m.issues[issue.ID] = issue

		// Index external ref for O(1) lookup
		if issue.ExternalRef != nil && *issue.ExternalRef != "" {
			m.externalRefToID[*issue.ExternalRef] = issue.ID
		}

		// Record event
		event := &types.Event{
			IssueID:   issue.ID,
			EventType: types.EventCreated,
			Actor:     actor,
			CreatedAt: now,
		}
		m.events[issue.ID] = append(m.events[issue.ID], event)
	}

	return nil
}

// GetIssue retrieves an issue by ID
func (m *issueIndex) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	issue, exists := m.issues[id]
	if !exists {
		return nil, nil
	}

	return m.copyIssueLocked(issue, issueCopyOptions{
		dependencies: true,
		labels:       true,
	}), nil
}

// GetIssueByExternalRef retrieves an issue by external reference
func (m *issueIndex) GetIssueByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// O(1) lookup using index
	issueID, exists := m.externalRefToID[externalRef]
	if !exists {
		return nil, nil
	}

	issue, exists := m.issues[issueID]
	if !exists {
		return nil, nil
	}

	return m.copyIssueLocked(issue, issueCopyOptions{
		dependencies: true,
		labels:       true,
	}), nil
}

// UpdateIssue updates fields on an issue
func (m *issueIndex) UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, exists := m.issues[id]
	if !exists {
		return fmt.Errorf("issue %s not found", id)
	}

	now := time.Now()
	issue.UpdatedAt = now

	// Apply updates
	for key, value := range updates {
		if err := m.applyIssueUpdateLocked(issue, id, key, value, now); err != nil {
			return err
		}
	}

	// Record event
	eventType := types.EventUpdated
	if status, hasStatus := updates["status"]; hasStatus {
		if status == string(types.StatusClosed) {
			eventType = types.EventClosed
		}
	}

	event := &types.Event{
		IssueID:   id,
		EventType: eventType,
		Actor:     actor,
		CreatedAt: now,
	}
	m.events[id] = append(m.events[id], event)

	return nil
}

func (m *issueIndex) applyIssueUpdateLocked(issue *types.Issue, id, key string, value interface{}, now time.Time) error {
	switch key {
	case "title":
		setStringField(value, &issue.Title)
	case "description":
		setStringField(value, &issue.Description)
	case "design":
		setStringField(value, &issue.Design)
	case "acceptance_criteria":
		setStringField(value, &issue.AcceptanceCriteria)
	case "notes":
		setStringField(value, &issue.Notes)
	case "status":
		setIssueStatus(issue, value, now)
	case "priority":
		if v, ok := value.(int); ok {
			issue.Priority = v
		}
	case "issue_type":
		if v, ok := value.(string); ok {
			issue.IssueType = types.IssueType(v)
		}
	case "assignee":
		setOptionalStringField(value, &issue.Assignee)
	case "estimated_minutes":
		setOptionalIntPointer(value, &issue.EstimatedMinutes)
	case "external_ref":
		return m.setExternalRefLocked(issue, id, value)
	case "close_reason":
		setStringField(value, &issue.CloseReason)
	case "closed_by_session":
		setStringField(value, &issue.ClosedBySession)
	case "due_at":
		setOptionalTimePointer(value, &issue.DueAt)
	case "defer_until":
		setOptionalTimePointer(value, &issue.DeferUntil)
	}
	return nil
}

func setStringField(value interface{}, target *string) {
	if v, ok := value.(string); ok {
		*target = v
	}
}

func setOptionalStringField(value interface{}, target *string) {
	if value == nil {
		*target = ""
		return
	}
	setStringField(value, target)
}

func setOptionalIntPointer(value interface{}, target **int) {
	if value == nil {
		*target = nil
		return
	}
	if v, ok := value.(int); ok {
		*target = &v
	}
}

func setOptionalTimePointer(value interface{}, target **time.Time) {
	if value == nil {
		*target = nil
		return
	}
	if v, ok := value.(time.Time); ok {
		*target = &v
	}
}

func setIssueStatus(issue *types.Issue, value interface{}, now time.Time) {
	v, ok := value.(string)
	if !ok {
		return
	}

	oldStatus := issue.Status
	issue.Status = types.Status(v)
	if issue.Status == types.StatusClosed && oldStatus != types.StatusClosed {
		issue.ClosedAt = &now
	} else if issue.Status != types.StatusClosed && oldStatus == types.StatusClosed {
		issue.ClosedAt = nil
	}
}

func (m *issueIndex) setExternalRefLocked(issue *types.Issue, id string, value interface{}) error {
	if value == nil {
		clearExternalRef(issue, m.externalRefToID)
		return nil
	}

	v, ok := value.(string)
	if !ok {
		return nil
	}
	if err := m.checkExternalRefAvailableLocked(v, id); err != nil {
		return err
	}

	clearExternalRef(issue, m.externalRefToID)
	if v != "" {
		m.externalRefToID[v] = id
	}
	issue.ExternalRef = &v
	return nil
}

func clearExternalRef(issue *types.Issue, externalRefToID map[string]string) {
	if issue.ExternalRef != nil && *issue.ExternalRef != "" {
		delete(externalRefToID, *issue.ExternalRef)
	}
	issue.ExternalRef = nil
}

// CloseIssue closes an issue with a reason.
// The session parameter tracks which Claude Code session closed the issue (can be empty).
func (m *issueIndex) CloseIssue(ctx context.Context, id string, reason string, actor string, session string) error {
	updates := map[string]interface{}{
		"status":       string(types.StatusClosed),
		"close_reason": reason,
	}
	if session != "" {
		updates["closed_by_session"] = session
	}
	return m.UpdateIssue(ctx, id, updates, actor)
}

// CreateTombstone converts an existing issue to a tombstone record.
// This is a soft-delete that preserves the issue with status="tombstone".
func (m *issueIndex) CreateTombstone(ctx context.Context, id string, actor string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	issue, ok := m.issues[id]
	if !ok {
		return fmt.Errorf("issue not found: %s", id)
	}

	now := time.Now()
	issue.OriginalType = string(issue.IssueType)
	issue.Status = types.StatusTombstone
	issue.DeletedAt = &now
	issue.DeletedBy = actor
	issue.DeleteReason = reason
	issue.UpdatedAt = now

	// Record tombstone creation event
	event := &types.Event{
		IssueID:   id,
		EventType: "deleted",
		Actor:     actor,
		Comment:   &reason,
		CreatedAt: now,
	}
	m.events[id] = append(m.events[id], event)

	return nil
}

// DeleteIssue permanently deletes an issue and all associated data
func (m *issueIndex) DeleteIssue(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if issue exists
	issue, ok := m.issues[id]
	if !ok {
		return fmt.Errorf("issue not found: %s", id)
	}

	// Remove external ref index entry
	if issue.ExternalRef != nil && *issue.ExternalRef != "" {
		delete(m.externalRefToID, *issue.ExternalRef)
	}

	// Delete the issue
	delete(m.issues, id)

	// Delete associated data
	delete(m.dependencies, id)
	delete(m.labels, id)
	delete(m.events, id)
	delete(m.comments, id)

	return nil
}

// SearchIssues finds issues matching query and filters
func (m *issueIndex) SearchIssues(ctx context.Context, query string, filter types.IssueFilter) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	search := newIssueSearch(query, filter)
	results := make([]*types.Issue, 0)
	for _, issue := range m.issues {
		if m.matchesIssueSearchLocked(issue, search) {
			results = append(results, m.copyIssueLocked(issue, issueCopyOptions{
				dependencies: true,
				labels:       true,
			}))
		}
	}

	sortSearchResults(results)
	return limitIssueResults(results, filter.Limit), nil
}

type issueSearch struct {
	filter              types.IssueFilter
	query               string
	titleSearch         string
	titleContains       string
	descriptionContains string
	notesContains       string
	now                 time.Time
}

func newIssueSearch(query string, filter types.IssueFilter) issueSearch {
	return issueSearch{
		filter:              filter,
		query:               strings.ToLower(query),
		titleSearch:         strings.ToLower(filter.TitleSearch),
		titleContains:       strings.ToLower(filter.TitleContains),
		descriptionContains: strings.ToLower(filter.DescriptionContains),
		notesContains:       strings.ToLower(filter.NotesContains),
		now:                 time.Now(),
	}
}

func (m *issueIndex) matchesIssueSearchLocked(issue *types.Issue, search issueSearch) bool {
	return matchesCoreIssueFilters(issue, search.filter) &&
		matchesTextFilters(issue, search) &&
		matchesIssueTimeFilters(issue, search.filter, search.now) &&
		m.matchesSearchMetadataLocked(issue, search.filter)
}

func matchesCoreIssueFilters(issue *types.Issue, filter types.IssueFilter) bool {
	return matchesStatusFilters(issue, filter) &&
		matchesPriorityFilters(issue, filter) &&
		matchesIssueTypeFilters(issue, filter) &&
		matchesAssignmentFilters(issue, filter) &&
		matchesPinnedFilter(issue, filter) &&
		matchesDescriptionPresenceFilter(issue, filter)
}

func matchesStatusFilters(issue *types.Issue, filter types.IssueFilter) bool {
	if issue.Status == types.StatusTombstone && !filter.IncludeTombstones {
		return false
	}
	if filter.Status != nil && issue.Status != *filter.Status {
		return false
	}
	return len(filter.ExcludeStatus) == 0 || !statusIn(issue.Status, filter.ExcludeStatus)
}

func matchesPriorityFilters(issue *types.Issue, filter types.IssueFilter) bool {
	if filter.Priority != nil && issue.Priority != *filter.Priority {
		return false
	}
	if filter.PriorityMin != nil && issue.Priority < *filter.PriorityMin {
		return false
	}
	if filter.PriorityMax != nil && issue.Priority > *filter.PriorityMax {
		return false
	}
	return true
}

func matchesIssueTypeFilters(issue *types.Issue, filter types.IssueFilter) bool {
	if filter.IssueType != nil && issue.IssueType != *filter.IssueType {
		return false
	}
	return len(filter.ExcludeTypes) == 0 || !issueTypeIn(issue.IssueType, filter.ExcludeTypes)
}

func matchesAssignmentFilters(issue *types.Issue, filter types.IssueFilter) bool {
	if filter.Assignee != nil && issue.Assignee != *filter.Assignee {
		return false
	}
	return !filter.NoAssignee || issue.Assignee == ""
}

func matchesPinnedFilter(issue *types.Issue, filter types.IssueFilter) bool {
	return filter.Pinned == nil || issue.Pinned == *filter.Pinned
}

func matchesDescriptionPresenceFilter(issue *types.Issue, filter types.IssueFilter) bool {
	return !filter.EmptyDescription || strings.TrimSpace(issue.Description) == ""
}

func matchesTextFilters(issue *types.Issue, search issueSearch) bool {
	if search.query != "" &&
		!containsLower(issue.Title, search.query) &&
		!containsLower(issue.Description, search.query) &&
		!containsLower(issue.ID, search.query) {
		return false
	}
	if search.titleSearch != "" && !containsLower(issue.Title, search.titleSearch) {
		return false
	}
	if search.titleContains != "" && !containsLower(issue.Title, search.titleContains) {
		return false
	}
	if search.descriptionContains != "" && !containsLower(issue.Description, search.descriptionContains) {
		return false
	}
	if search.notesContains != "" && !containsLower(issue.Notes, search.notesContains) {
		return false
	}
	return true
}

func containsLower(value, lowerNeedle string) bool {
	return strings.Contains(strings.ToLower(value), lowerNeedle)
}

func matchesIssueTimeFilters(issue *types.Issue, filter types.IssueFilter, now time.Time) bool {
	if !timeInRange(issue.CreatedAt, filter.CreatedAfter, filter.CreatedBefore) {
		return false
	}
	if !timeInRange(issue.UpdatedAt, filter.UpdatedAfter, filter.UpdatedBefore) {
		return false
	}
	if !optionalTimeInRange(issue.ClosedAt, filter.ClosedAfter, filter.ClosedBefore) {
		return false
	}
	if filter.Deferred && issue.DeferUntil == nil {
		return false
	}
	if !optionalTimeInRange(issue.DeferUntil, filter.DeferAfter, filter.DeferBefore) {
		return false
	}
	if !optionalTimeInRange(issue.DueAt, filter.DueAfter, filter.DueBefore) {
		return false
	}
	if filter.Overdue && !isOverdue(issue, now) {
		return false
	}
	return true
}

func timeInRange(value time.Time, after, before *time.Time) bool {
	if after != nil && value.Before(*after) {
		return false
	}
	if before != nil && value.After(*before) {
		return false
	}
	return true
}

func optionalTimeInRange(value, after, before *time.Time) bool {
	if after == nil && before == nil {
		return true
	}
	if value == nil {
		return false
	}
	return timeInRange(*value, after, before)
}

func isOverdue(issue *types.Issue, now time.Time) bool {
	return issue.DueAt != nil &&
		issue.DueAt.Before(now) &&
		issue.Status != types.StatusClosed &&
		issue.Status != types.StatusTombstone
}

func (m *issueIndex) matchesSearchMetadataLocked(issue *types.Issue, filter types.IssueFilter) bool {
	issueLabels := m.labels[issue.ID]
	if filter.NoLabels && len(issueLabels) > 0 {
		return false
	}
	if len(filter.Labels) > 0 && !hasAllLabels(issueLabels, filter.Labels) {
		return false
	}
	if len(filter.IDs) > 0 && !stringIn(issue.ID, filter.IDs) {
		return false
	}
	if filter.IDPrefix != "" && !strings.HasPrefix(issue.ID, filter.IDPrefix) {
		return false
	}
	return m.matchesParentFilterLocked(issue.ID, filter.ParentID)
}

func hasAllLabels(issueLabels, requiredLabels []string) bool {
	for _, requiredLabel := range requiredLabels {
		if !stringIn(requiredLabel, issueLabels) {
			return false
		}
	}
	return true
}

func hasAnyLabel(issueLabels, requiredLabels []string) bool {
	for _, requiredLabel := range requiredLabels {
		if stringIn(requiredLabel, issueLabels) {
			return true
		}
	}
	return false
}

func stringIn(value string, values []string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func (m *issueIndex) matchesParentFilterLocked(issueID string, parentID *string) bool {
	if parentID == nil {
		return true
	}
	for _, dep := range m.dependencies[issueID] {
		if dep.Type == types.DepParentChild && dep.DependsOnID == *parentID {
			return true
		}
	}
	return false
}

func sortSearchResults(results []*types.Issue) {
	sort.Slice(results, func(i, j int) bool {
		if results[i].Priority != results[j].Priority {
			return results[i].Priority < results[j].Priority
		}
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
}

func limitIssueResults(results []*types.Issue, limit int) []*types.Issue {
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func statusIn(status types.Status, statuses []types.Status) bool {
	for _, candidate := range statuses {
		if status == candidate {
			return true
		}
	}
	return false
}

func issueTypeIn(issueType types.IssueType, issueTypes []types.IssueType) bool {
	for _, candidate := range issueTypes {
		if issueType == candidate {
			return true
		}
	}
	return false
}

// AddDependency adds a dependency between issues
func (m *issueIndex) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check that both issues exist
	if _, exists := m.issues[dep.IssueID]; !exists {
		return fmt.Errorf("issue %s not found", dep.IssueID)
	}
	if _, exists := m.issues[dep.DependsOnID]; !exists && !isExternalDependencyID(dep.DependsOnID) {
		return fmt.Errorf("issue %s not found", dep.DependsOnID)
	}

	// Check for duplicates
	for _, existing := range m.dependencies[dep.IssueID] {
		if existing.DependsOnID == dep.DependsOnID && existing.Type == dep.Type {
			return fmt.Errorf("dependency already exists")
		}
	}

	m.dependencies[dep.IssueID] = append(m.dependencies[dep.IssueID], dep)

	return nil
}

func isExternalDependencyID(id string) bool {
	return strings.HasPrefix(id, "external:")
}

// RemoveDependency removes a dependency
func (m *issueIndex) RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	deps := m.dependencies[issueID]
	newDeps := make([]*types.Dependency, 0)

	for _, dep := range deps {
		if dep.DependsOnID != dependsOnID {
			newDeps = append(newDeps, dep)
		}
	}

	m.dependencies[issueID] = newDeps

	return nil
}

// GetDependencies gets issues that this issue depends on
func (m *issueIndex) GetDependencies(ctx context.Context, issueID string) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.Issue
	for _, dep := range m.dependencies[issueID] {
		if issue, exists := m.issues[dep.DependsOnID]; exists {
			issueCopy := *issue
			results = append(results, &issueCopy)
		} else if isExternalDependencyID(dep.DependsOnID) {
			results = append(results, externalDependencyIssue(dep.DependsOnID))
		}
	}

	return results, nil
}

// GetDependents gets issues that depend on this issue
func (m *issueIndex) GetDependents(ctx context.Context, issueID string) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.Issue
	for id, deps := range m.dependencies {
		for _, dep := range deps {
			if dep.DependsOnID == issueID {
				if issue, exists := m.issues[id]; exists {
					issueCopy := *issue
					results = append(results, &issueCopy)
				}
				break
			}
		}
	}

	return results, nil
}

// GetDependenciesWithMetadata gets issues that this issue depends on, with dependency type
func (m *issueIndex) GetDependenciesWithMetadata(ctx context.Context, issueID string) ([]*types.IssueWithDependencyMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.IssueWithDependencyMetadata
	for _, dep := range m.dependencies[issueID] {
		if issue, exists := m.issues[dep.DependsOnID]; exists {
			issueCopy := *issue
			results = append(results, &types.IssueWithDependencyMetadata{
				Issue:          issueCopy,
				DependencyType: dep.Type,
			})
		} else if isExternalDependencyID(dep.DependsOnID) {
			issue := externalDependencyIssue(dep.DependsOnID)
			results = append(results, &types.IssueWithDependencyMetadata{
				Issue:          *issue,
				DependencyType: dep.Type,
			})
		}
	}

	return results, nil
}

// GetDependentsWithMetadata gets issues that depend on this issue, with dependency type
func (m *issueIndex) GetDependentsWithMetadata(ctx context.Context, issueID string) ([]*types.IssueWithDependencyMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.IssueWithDependencyMetadata
	for id, deps := range m.dependencies {
		for _, dep := range deps {
			if dep.DependsOnID == issueID {
				if issue, exists := m.issues[id]; exists {
					issueCopy := *issue
					results = append(results, &types.IssueWithDependencyMetadata{
						Issue:          issueCopy,
						DependencyType: dep.Type,
					})
				}
				break
			}
		}
	}

	return results, nil
}

// GetDependencyCounts returns dependency and dependent counts for multiple issues
func (m *issueIndex) GetDependencyCounts(ctx context.Context, issueIDs []string) (map[string]*types.DependencyCounts, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*types.DependencyCounts)

	// Initialize all requested IDs with zero counts
	for _, id := range issueIDs {
		result[id] = &types.DependencyCounts{
			DependencyCount: 0,
			DependentCount:  0,
		}
	}

	// Build a set for quick lookup
	idSet := make(map[string]bool)
	for _, id := range issueIDs {
		idSet[id] = true
	}

	// Count dependencies (issues that this issue depends on)
	for _, id := range issueIDs {
		if deps, exists := m.dependencies[id]; exists {
			result[id].DependencyCount = len(deps)
		}
	}

	// Count dependents (issues that depend on this issue)
	for _, deps := range m.dependencies {
		for _, dep := range deps {
			if idSet[dep.DependsOnID] {
				result[dep.DependsOnID].DependentCount++
			}
		}
	}

	return result, nil
}

// GetDependencyRecords gets dependency records for an issue
func (m *issueIndex) GetDependencyRecords(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return copyDependencies(m.dependencies[issueID]), nil
}

// GetAllDependencyRecords gets all dependency records
func (m *issueIndex) GetAllDependencyRecords(ctx context.Context) (map[string][]*types.Dependency, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy
	result := make(map[string][]*types.Dependency)
	for k, v := range m.dependencies {
		result[k] = copyDependencies(v)
	}

	return result, nil
}

// GetDependencyTree gets the dependency tree for an issue
func (m *issueIndex) GetDependencyTree(ctx context.Context, issueID string, maxDepth int, showAllPaths bool, reverse bool) ([]*types.TreeNode, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	root, exists := m.issues[issueID]
	if !exists {
		return nil, nil
	}

	rootNode := treeNodeFromIssue(root, 0, issueID, false)
	nodes := []*types.TreeNode{rootNode}
	if maxDepth <= 0 {
		rootNode.Truncated = len(m.treeChildIDs(issueID, reverse)) > 0
		return nodes, nil
	}

	seen := map[string]bool{}
	if !showAllPaths {
		seen[issueID] = true
	}
	path := map[string]bool{issueID: true}
	m.appendDependencyTreeChildren(&nodes, issueID, 1, maxDepth, reverse, showAllPaths, seen, path)

	return nodes, nil
}

// DetectCycles detects dependency cycles
func (m *issueIndex) DetectCycles(ctx context.Context) ([][]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	const (
		unvisited = iota
		visiting
		visited
	)
	state := make(map[string]int, len(m.issues))
	path := make([]string, 0, len(m.issues))
	pathIndex := make(map[string]int, len(m.issues))
	seenCycles := make(map[string]bool)
	var cycles [][]*types.Issue

	var visit func(string)
	visit = func(id string) {
		state[id] = visiting
		pathIndex[id] = len(path)
		path = append(path, id)
		for _, dep := range m.dependencies[id] {
			nextID := dep.DependsOnID
			if _, exists := m.issues[nextID]; !exists {
				continue
			}
			switch state[nextID] {
			case unvisited:
				visit(nextID)
			case visiting:
				start := pathIndex[nextID]
				cycleIDs := append([]string(nil), path[start:]...)
				key := canonicalCycleKey(cycleIDs)
				if seenCycles[key] {
					continue
				}
				seenCycles[key] = true
				cycle := make([]*types.Issue, 0, len(cycleIDs))
				for _, cycleID := range cycleIDs {
					issueCopy := *m.issues[cycleID]
					cycle = append(cycle, &issueCopy)
				}
				cycles = append(cycles, cycle)
			}
		}
		path = path[:len(path)-1]
		delete(pathIndex, id)
		state[id] = visited
	}

	ids := make([]string, 0, len(m.issues))
	for id := range m.issues {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if state[id] == unvisited {
			visit(id)
		}
	}
	if len(cycles) == 0 {
		return nil, nil
	}
	return cycles, nil
}

func treeNodeFromIssue(issue *types.Issue, depth int, parentID string, truncated bool) *types.TreeNode {
	issueCopy := *issue
	return &types.TreeNode{
		Issue:     issueCopy,
		Depth:     depth,
		ParentID:  parentID,
		Truncated: truncated,
	}
}

func externalDependencyIssue(id string) *types.Issue {
	return &types.Issue{
		ID:        id,
		Title:     id,
		Status:    types.StatusOpen,
		Priority:  4,
		IssueType: types.TypeTask,
	}
}

type issueCopyOptions struct {
	dependencies bool
	labels       bool
	comments     bool
}

func (m *issueIndex) copyIssueLocked(issue *types.Issue, options issueCopyOptions) *types.Issue {
	issueCopy := *issue
	issueCopy.AgentWF = copyRawMessage(issue.AgentWF)
	if options.dependencies {
		if deps, ok := m.dependencies[issue.ID]; ok {
			issueCopy.Dependencies = copyDependencies(deps)
		}
	}
	if options.labels {
		if labels, ok := m.labels[issue.ID]; ok {
			issueCopy.Labels = copyStrings(labels)
		}
	}
	if options.comments {
		if comments, ok := m.comments[issue.ID]; ok {
			issueCopy.Comments = copyComments(comments)
		}
	}
	return &issueCopy
}

func copyDependencies(deps []*types.Dependency) []*types.Dependency {
	if len(deps) == 0 {
		return nil
	}
	copied := make([]*types.Dependency, 0, len(deps))
	for _, dep := range deps {
		if dep == nil {
			copied = append(copied, nil)
			continue
		}
		depCopy := *dep
		copied = append(copied, &depCopy)
	}
	return copied
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func copyComments(comments []*types.Comment) []*types.Comment {
	if len(comments) == 0 {
		return nil
	}
	copied := make([]*types.Comment, 0, len(comments))
	for _, comment := range comments {
		if comment == nil {
			copied = append(copied, nil)
			continue
		}
		commentCopy := *comment
		copied = append(copied, &commentCopy)
	}
	return copied
}

func copyRawMessage(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}

func copyEvents(events []*types.Event) []*types.Event {
	if len(events) == 0 {
		return nil
	}
	copied := make([]*types.Event, 0, len(events))
	for _, event := range events {
		if event == nil {
			copied = append(copied, nil)
			continue
		}
		eventCopy := *event
		copied = append(copied, &eventCopy)
	}
	return copied
}

func (m *issueIndex) appendDependencyTreeChildren(nodes *[]*types.TreeNode, parentID string, depth, maxDepth int, reverse, showAllPaths bool, seen, path map[string]bool) {
	childIDs := m.treeChildIDs(parentID, reverse)
	for _, childID := range childIDs {
		issue := m.issues[childID]
		if issue == nil && isExternalDependencyID(childID) {
			issue = externalDependencyIssue(childID)
		}
		if issue == nil {
			continue
		}

		hasChildren := len(m.treeChildIDs(childID, reverse)) > 0
		truncated := depth >= maxDepth && hasChildren
		if path[childID] {
			truncated = true
		}
		node := treeNodeFromIssue(issue, depth, parentID, truncated)
		*nodes = append(*nodes, node)

		if !showAllPaths && seen[childID] {
			continue
		}
		if !showAllPaths {
			seen[childID] = true
		}
		if depth >= maxDepth || path[childID] || isExternalDependencyID(childID) {
			continue
		}

		path[childID] = true
		m.appendDependencyTreeChildren(nodes, childID, depth+1, maxDepth, reverse, showAllPaths, seen, path)
		delete(path, childID)
	}
}

func (m *issueIndex) treeChildIDs(issueID string, reverse bool) []string {
	ids := make([]string, 0)
	if reverse {
		for dependentID, deps := range m.dependencies {
			for _, dep := range deps {
				if dep.DependsOnID == issueID {
					ids = append(ids, dependentID)
					break
				}
			}
		}
	} else {
		for _, dep := range m.dependencies[issueID] {
			if _, exists := m.issues[dep.DependsOnID]; exists || isExternalDependencyID(dep.DependsOnID) {
				ids = append(ids, dep.DependsOnID)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func canonicalCycleKey(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	minIndex := 0
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[minIndex] {
			minIndex = i
		}
	}
	rotated := make([]string, 0, len(ids))
	rotated = append(rotated, ids[minIndex:]...)
	rotated = append(rotated, ids[:minIndex]...)
	return strings.Join(rotated, "\x00")
}

// Add label methods
func (m *issueIndex) AddLabel(ctx context.Context, issueID, label, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if issue exists
	if _, exists := m.issues[issueID]; !exists {
		return fmt.Errorf("issue %s not found", issueID)
	}

	// Check for duplicate
	for _, l := range m.labels[issueID] {
		if l == label {
			return nil // Already exists
		}
	}

	m.labels[issueID] = append(m.labels[issueID], label)

	return nil
}

func (m *issueIndex) RemoveLabel(ctx context.Context, issueID, label, actor string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	labels := m.labels[issueID]
	newLabels := make([]string, 0)

	for _, l := range labels {
		if l != label {
			newLabels = append(newLabels, l)
		}
	}

	m.labels[issueID] = newLabels

	return nil
}

func (m *issueIndex) GetLabels(ctx context.Context, issueID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return copyStrings(m.labels[issueID]), nil
}

func (m *issueIndex) GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]string)
	for _, issueID := range issueIDs {
		if labels, exists := m.labels[issueID]; exists {
			result[issueID] = copyStrings(labels)
		}
	}
	return result, nil
}

func (m *issueIndex) GetIssuesByLabel(ctx context.Context, label string) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*types.Issue
	for issueID, labels := range m.labels {
		for _, l := range labels {
			if l == label {
				if issue, exists := m.issues[issueID]; exists {
					issueCopy := *issue
					results = append(results, &issueCopy)
				}
				break
			}
		}
	}

	return results, nil
}

// GetReadyWork returns issues that are ready to work on (no open blockers)
func (m *issueIndex) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*types.Issue, 0)
	for _, issue := range m.issues {
		if m.matchesReadyWorkLocked(issue, filter) {
			results = append(results, m.copyIssueLocked(issue, issueCopyOptions{
				dependencies: true,
				labels:       true,
				comments:     true,
			}))
		}
	}

	sortReadyWorkResults(results, filter.SortPolicy)
	return limitIssueResults(results, filter.Limit), nil
}

func (m *issueIndex) matchesReadyWorkLocked(issue *types.Issue, filter types.WorkFilter) bool {
	if issue.Pinned {
		return false
	}
	if !matchesReadyStatus(issue, filter.Status) {
		return false
	}
	if filter.Priority != nil && issue.Priority != *filter.Priority {
		return false
	}
	if !matchesReadyType(issue, filter.Type) {
		return false
	}
	if !matchesReadyAssignee(issue, filter) {
		return false
	}
	if !m.matchesReadyLabelsLocked(issue.ID, filter) {
		return false
	}
	return len(m.getOpenBlockers(issue.ID)) == 0
}

func matchesReadyStatus(issue *types.Issue, status types.Status) bool {
	if status != "" {
		return issue.Status == status
	}
	return issue.Status == types.StatusOpen || issue.Status == types.StatusInProgress
}

func matchesReadyType(issue *types.Issue, issueType string) bool {
	if issueType != "" {
		return string(issue.IssueType) == issueType
	}

	switch issue.IssueType {
	case types.TypeFeatureRequest, types.TypeFeature, types.TypeEpic, types.TypeTask, types.TypeBug, types.TypeChore:
		return true
	default:
		return false
	}
}

func matchesReadyAssignee(issue *types.Issue, filter types.WorkFilter) bool {
	if filter.Unassigned {
		return issue.Assignee == ""
	}
	if filter.Assignee != nil {
		return issue.Assignee == *filter.Assignee
	}
	return true
}

func (m *issueIndex) matchesReadyLabelsLocked(issueID string, filter types.WorkFilter) bool {
	issueLabels := m.labels[issueID]
	if len(filter.Labels) > 0 && !hasAllLabels(issueLabels, filter.Labels) {
		return false
	}
	if len(filter.LabelsAny) > 0 && !hasAnyLabel(issueLabels, filter.LabelsAny) {
		return false
	}
	return true
}

func sortReadyWorkResults(results []*types.Issue, sortPolicy types.SortPolicy) {
	if sortPolicy == "" {
		sortPolicy = types.SortPolicyHybrid
	}

	switch sortPolicy {
	case types.SortPolicyOldest:
		sort.Slice(results, func(i, j int) bool {
			return results[i].CreatedAt.Before(results[j].CreatedAt)
		})
	case types.SortPolicyPriority:
		sort.Slice(results, func(i, j int) bool {
			if results[i].Priority != results[j].Priority {
				return results[i].Priority < results[j].Priority
			}
			return results[i].CreatedAt.Before(results[j].CreatedAt)
		})
	case types.SortPolicyHybrid:
		fallthrough
	default:
		cutoff := time.Now().Add(-48 * time.Hour)
		sort.Slice(results, func(i, j int) bool {
			iRecent := results[i].CreatedAt.After(cutoff)
			jRecent := results[j].CreatedAt.After(cutoff)
			if iRecent != jRecent {
				return iRecent // recent first
			}
			if iRecent {
				if results[i].Priority != results[j].Priority {
					return results[i].Priority < results[j].Priority
				}
			}
			return results[i].CreatedAt.Before(results[j].CreatedAt)
		})
	}
}

// getOpenBlockers returns the IDs of blockers that are not complete yet.
// The caller must hold at least a read lock.
func (m *issueIndex) getOpenBlockers(issueID string) []string {
	deps := m.dependencies[issueID]
	if len(deps) == 0 {
		return nil
	}

	blockers := make([]string, 0)
	for _, dep := range deps {
		if dep.Type != types.DepBlocks {
			continue
		}
		blocker, ok := m.issues[dep.DependsOnID]
		if !ok {
			// If the blocker is missing, treat it as still blocking (data is incomplete)
			blockers = append(blockers, dep.DependsOnID)
			continue
		}
		switch blocker.Status {
		case types.StatusOpen,
			types.StatusInProgress,
			types.StatusInReview,
			types.StatusHumanReview,
			types.StatusBlocked,
			types.StatusDeferred:
			blockers = append(blockers, blocker.ID)
		}
	}

	sort.Strings(blockers)
	return blockers
}

// GetBlockedIssues returns issues that are blocked by other issues
// Note: Pinned issues are excluded from the output (taskledger-ei4)
func (m *issueIndex) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build set of descendant IDs if parent filter is specified
	var descendantIDs map[string]bool
	if filter.ParentID != nil {
		descendantIDs = m.getAllDescendants(*filter.ParentID)
	}

	var results []*types.BlockedIssue

	for _, issue := range m.issues {
		// Only consider non-closed, non-tombstone issues
		if issue.Status == types.StatusClosed || issue.Status == types.StatusTombstone {
			continue
		}

		// Exclude pinned issues (taskledger-ei4)
		if issue.Pinned {
			continue
		}

		// Parent filtering: only include descendants of specified parent
		if descendantIDs != nil && !descendantIDs[issue.ID] {
			continue
		}

		blockers := m.getOpenBlockers(issue.ID)
		// Issue is "blocked" if: status is blocked, status is deferred, or has open blockers
		if issue.Status != types.StatusBlocked && issue.Status != types.StatusDeferred && len(blockers) == 0 {
			continue
		}

		issueCopy := *issue
		if deps, ok := m.dependencies[issue.ID]; ok {
			issueCopy.Dependencies = copyDependencies(deps)
		}
		if labels, ok := m.labels[issue.ID]; ok {
			issueCopy.Labels = copyStrings(labels)
		}
		if comments, ok := m.comments[issue.ID]; ok {
			issueCopy.Comments = copyComments(comments)
		}

		results = append(results, &types.BlockedIssue{
			Issue:          issueCopy,
			BlockedByCount: len(blockers),
			BlockedBy:      blockers,
		})
	}

	// Order by priority ascending so P0 appears before P1, P2, and lower priorities.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Priority != results[j].Priority {
			return results[i].Priority < results[j].Priority
		}
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})

	return results, nil
}

// IsBlocked checks if an issue is blocked by open dependencies (GH#962).
// Returns true if the issue has open blockers, along with the list of blocker IDs.
func (m *issueIndex) IsBlocked(ctx context.Context, issueID string) (bool, []string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	blockers := m.getOpenBlockers(issueID)
	if len(blockers) == 0 {
		return false, nil, nil
	}
	return true, blockers, nil
}

// getAllDescendants returns all descendant IDs of a parent issue recursively
func (m *issueIndex) getAllDescendants(parentID string) map[string]bool {
	descendants := make(map[string]bool)
	m.collectDescendants(parentID, descendants)
	return descendants
}

// collectDescendants recursively collects all descendants of a parent
func (m *issueIndex) collectDescendants(parentID string, descendants map[string]bool) {
	for issueID, deps := range m.dependencies {
		for _, dep := range deps {
			if dep.Type == types.DepParentChild && dep.DependsOnID == parentID {
				if !descendants[issueID] {
					descendants[issueID] = true
					m.collectDescendants(issueID, descendants)
				}
			}
		}
	}
}

func (m *issueIndex) GetEpicsEligibleForClosure(ctx context.Context) ([]*types.EpicStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.epicStatusesLocked(), nil
}

func (m *issueIndex) GetStaleIssues(ctx context.Context, filter types.StaleFilter) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -filter.Days)
	var stale []*types.Issue

	for _, issue := range m.issues {
		if issue.Status == types.StatusClosed {
			continue
		}
		if filter.Status != "" && string(issue.Status) != filter.Status {
			continue
		}
		if issue.UpdatedAt.Before(cutoff) {
			issueCopy := *issue
			stale = append(stale, &issueCopy)
		}
	}

	// Sort by updated_at ascending (oldest first)
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].UpdatedAt.Before(stale[j].UpdatedAt)
	})

	if filter.Limit > 0 && len(stale) > filter.Limit {
		stale = stale[:filter.Limit]
	}

	return stale, nil
}

// GetNewlyUnblockedByClose returns issues that became unblocked when the given issue was closed.
// This is used by the --suggest-next flag on tl close (GH#679).
func (m *issueIndex) GetNewlyUnblockedByClose(ctx context.Context, closedIssueID string) ([]*types.Issue, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var unblocked []*types.Issue

	// Find issues that depend on the closed issue
	for issueID, deps := range m.dependencies {
		issue, exists := m.issues[issueID]
		if !exists {
			continue
		}

		// Only consider open/in_progress, non-pinned issues
		if issue.Status != types.StatusOpen && issue.Status != types.StatusInProgress {
			continue
		}
		if issue.Pinned {
			continue
		}

		// Check if this issue depended on the closed issue
		dependedOnClosed := false
		for _, dep := range deps {
			if dep.DependsOnID == closedIssueID && dep.Type == types.DepBlocks {
				dependedOnClosed = true
				break
			}
		}

		if !dependedOnClosed {
			continue
		}

		// Check if now unblocked (no remaining open blockers)
		blockers := m.getOpenBlockers(issueID)
		if len(blockers) == 0 {
			issueCopy := *issue
			unblocked = append(unblocked, &issueCopy)
		}
	}

	// Sort by priority ascending
	sort.Slice(unblocked, func(i, j int) bool {
		return unblocked[i].Priority < unblocked[j].Priority
	})

	return unblocked, nil
}

func (m *issueIndex) AddComment(ctx context.Context, issueID, actor, comment string) error {
	_, err := m.AddIssueComment(ctx, issueID, actor, comment)
	return err
}

func (m *issueIndex) GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := m.events[issueID]
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}

	return copyEvents(events), nil
}

func (m *issueIndex) AddIssueComment(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.issues[issueID]; !exists {
		return nil, fmt.Errorf("issue %s not found", issueID)
	}
	now := time.Now()
	comment := &types.Comment{
		ID:        int64(len(m.comments[issueID]) + 1),
		IssueID:   issueID,
		Author:    author,
		Text:      text,
		CreatedAt: now,
	}

	m.comments[issueID] = append(m.comments[issueID], comment)
	event := &types.Event{
		IssueID:   issueID,
		EventType: types.EventCommented,
		Actor:     author,
		Comment:   &text,
		CreatedAt: now,
	}
	m.events[issueID] = append(m.events[issueID], event)

	return comment, nil
}

func (m *issueIndex) GetIssueComments(ctx context.Context, issueID string) ([]*types.Comment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return copyComments(m.comments[issueID]), nil
}

func (m *issueIndex) GetCommentsForIssues(ctx context.Context, issueIDs []string) (map[string][]*types.Comment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*types.Comment)
	for _, issueID := range issueIDs {
		if comments, exists := m.comments[issueID]; exists {
			result[issueID] = copyComments(comments)
		}
	}
	return result, nil
}

func (m *issueIndex) GetStatistics(ctx context.Context) (*types.Statistics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &types.Statistics{}

	// First pass: count by status
	for _, issue := range m.issues {
		switch issue.Status {
		case types.StatusOpen:
			stats.OpenIssues++
		case types.StatusInProgress:
			stats.InProgressIssues++
		case types.StatusInReview:
			stats.InReviewIssues++
		case types.StatusHumanReview:
			stats.HumanReviewIssues++
		case types.StatusClosed:
			stats.ClosedIssues++
		case types.StatusDeferred:
			stats.DeferredIssues++
		case types.StatusTombstone:
			stats.TombstoneIssues++
		case types.StatusPinned:
			stats.PinnedIssues++
		}
	}

	// TotalIssues excludes tombstones.
	stats.TotalIssues = stats.OpenIssues +
		stats.InProgressIssues +
		stats.InReviewIssues +
		stats.HumanReviewIssues +
		stats.ClosedIssues +
		stats.DeferredIssues +
		stats.PinnedIssues

	// Second pass: calculate blocked and ready issues based on dependencies
	// An issue is blocked if it has open blockers (uses same logic as GetBlockedIssues)
	for id, issue := range m.issues {
		// Only consider non-closed, non-tombstone issues for blocking
		if issue.Status == types.StatusClosed || issue.Status == types.StatusTombstone {
			continue
		}

		blockers := m.getOpenBlockers(id)
		if len(blockers) > 0 {
			stats.BlockedIssues++
		} else if issue.Status == types.StatusOpen {
			// Ready = open issues with no open blockers
			stats.ReadyIssues++
		}
	}

	// Calculate average lead time (hours from created to closed)
	var totalLeadTime float64
	var closedCount int
	for _, issue := range m.issues {
		if issue.Status == types.StatusClosed && issue.ClosedAt != nil {
			leadTime := issue.ClosedAt.Sub(issue.CreatedAt).Hours()
			totalLeadTime += leadTime
			closedCount++
		}
	}
	if closedCount > 0 {
		stats.AverageLeadTime = totalLeadTime / float64(closedCount)
	}

	stats.EpicsEligibleForClosure = m.countEpicsEligibleForClosure()

	return stats, nil
}

// countEpicsEligibleForClosure returns the count of non-closed epics where all children are closed
func (m *issueIndex) countEpicsEligibleForClosure() int {
	count := 0
	for _, status := range m.epicStatusesLocked() {
		if status.EligibleForClose {
			count++
		}
	}
	return count
}

func (m *issueIndex) epicStatusesLocked() []*types.EpicStatus {
	epicChildren := make(map[string][]string)
	for _, deps := range m.dependencies {
		for _, dep := range deps {
			if dep.Type == types.DepParentChild {
				epicChildren[dep.DependsOnID] = append(epicChildren[dep.DependsOnID], dep.IssueID)
			}
		}
	}

	epicIDs := make([]string, 0)
	for id, issue := range m.issues {
		if issue.IssueType == types.TypeEpic && issue.Status != types.StatusClosed && issue.Status != types.StatusTombstone {
			epicIDs = append(epicIDs, id)
		}
	}
	sort.Strings(epicIDs)

	statuses := make([]*types.EpicStatus, 0, len(epicIDs))
	for _, epicID := range epicIDs {
		epicCopy := *m.issues[epicID]
		children := epicChildren[epicID]
		sort.Strings(children)
		closedChildren := 0
		for _, childID := range children {
			child := m.issues[childID]
			if child != nil && child.Status == types.StatusClosed {
				closedChildren++
			}
		}
		statuses = append(statuses, &types.EpicStatus{
			Epic:             &epicCopy,
			TotalChildren:    len(children),
			ClosedChildren:   closedChildren,
			EligibleForClose: len(children) > 0 && closedChildren == len(children),
		})
	}
	return statuses
}

// ID Generation
func (m *issueIndex) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate parent exists
	if _, exists := m.issues[parentID]; !exists {
		return "", fmt.Errorf("parent issue %s does not exist", parentID)
	}

	// Check hierarchy depth limit (GH#995)
	if err := types.CheckHierarchyDepth(parentID, config.GetInt("hierarchy.max-depth")); err != nil {
		return "", err
	}

	// Get or initialize counter for this parent
	counter := m.counters[parentID]
	counter++
	m.counters[parentID] = counter

	// Format as parentID.counter
	childID := fmt.Sprintf("%s.%d", parentID, counter)
	return childID, nil
}

// Config
func (m *issueIndex) SetConfig(ctx context.Context, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config[key] = value
	return nil
}

func (m *issueIndex) GetConfig(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config[key], nil
}

func (m *issueIndex) DeleteConfig(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.config, key)
	return nil
}

func (m *issueIndex) GetAllConfig(ctx context.Context) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid mutations
	result := make(map[string]string)
	for k, v := range m.config {
		result[k] = v
	}

	return result, nil
}

// GetCustomStatuses retrieves the list of custom status states from config.
func (m *issueIndex) GetCustomStatuses(ctx context.Context) ([]string, error) {
	value, err := m.GetConfig(ctx, "status.custom")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	return parseCustomStatuses(value), nil
}

// parseCustomStatuses splits a comma-separated string into a slice of trimmed status names.
func parseCustomStatuses(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetCustomTypes retrieves the list of custom issue types from config.
func (m *issueIndex) GetCustomTypes(ctx context.Context) ([]string, error) {
	value, err := m.GetConfig(ctx, "types.custom")
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	return parseCustomStatuses(value), nil
}

// Lifecycle
func (m *issueIndex) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	return nil
}

func (m *issueIndex) Path() string {
	return ""
}
