package storage

import (
	"context"
	"testing"

	"task-ledger/internal/types"
)

var (
	_ Storage     = (*mockStorage)(nil)
	_ Transaction = (*mockTransaction)(nil)
)

type mockStorage struct{}

func (m *mockStorage) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	return nil
}
func (m *mockStorage) CreateIssues(ctx context.Context, issues []*types.Issue, actor string) error {
	return nil
}
func (m *mockStorage) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) GetIssueByExternalRef(ctx context.Context, externalRef string) (*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error {
	return nil
}
func (m *mockStorage) CloseIssue(ctx context.Context, id string, reason string, actor string, session string) error {
	return nil
}
func (m *mockStorage) DeleteIssue(ctx context.Context, id string) error {
	return nil
}
func (m *mockStorage) DeleteIssueWithReason(ctx context.Context, id, actor, reason string) error {
	return nil
}
func (m *mockStorage) SearchIssues(ctx context.Context, query string, filter types.IssueFilter) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	return nil
}
func (m *mockStorage) RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error {
	return nil
}
func (m *mockStorage) GetDependencies(ctx context.Context, issueID string) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) GetDependents(ctx context.Context, issueID string) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) GetDependenciesWithMetadata(ctx context.Context, issueID string) ([]*types.IssueWithDependencyMetadata, error) {
	return nil, nil
}
func (m *mockStorage) GetDependentsWithMetadata(ctx context.Context, issueID string) ([]*types.IssueWithDependencyMetadata, error) {
	return nil, nil
}
func (m *mockStorage) GetDependencyRecords(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	return nil, nil
}
func (m *mockStorage) GetAllDependencyRecords(ctx context.Context) (map[string][]*types.Dependency, error) {
	return nil, nil
}
func (m *mockStorage) GetDependencyCounts(ctx context.Context, issueIDs []string) (map[string]*types.DependencyCounts, error) {
	return nil, nil
}
func (m *mockStorage) GetDependencyTree(ctx context.Context, issueID string, maxDepth int, showAllPaths bool, reverse bool) ([]*types.TreeNode, error) {
	return nil, nil
}
func (m *mockStorage) DetectCycles(ctx context.Context) ([][]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) AddLabel(ctx context.Context, issueID, label, actor string) error {
	return nil
}
func (m *mockStorage) RemoveLabel(ctx context.Context, issueID, label, actor string) error {
	return nil
}
func (m *mockStorage) GetLabels(ctx context.Context, issueID string) ([]string, error) {
	return nil, nil
}
func (m *mockStorage) GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error) {
	return nil, nil
}
func (m *mockStorage) GetIssuesByLabel(ctx context.Context, label string) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) GetReadyWork(ctx context.Context, filter types.WorkFilter) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) GetBlockedIssues(ctx context.Context, filter types.WorkFilter) ([]*types.BlockedIssue, error) {
	return nil, nil
}
func (m *mockStorage) IsBlocked(ctx context.Context, issueID string) (bool, []string, error) {
	return false, nil, nil
}
func (m *mockStorage) GetEpicsEligibleForClosure(ctx context.Context) ([]*types.EpicStatus, error) {
	return nil, nil
}
func (m *mockStorage) GetStaleIssues(ctx context.Context, filter types.StaleFilter) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) GetNewlyUnblockedByClose(ctx context.Context, closedIssueID string) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockStorage) AddComment(ctx context.Context, issueID, actor, comment string) error {
	return nil
}
func (m *mockStorage) GetEvents(ctx context.Context, issueID string, limit int) ([]*types.Event, error) {
	return nil, nil
}
func (m *mockStorage) AddIssueComment(ctx context.Context, issueID, author, text string) (*types.Comment, error) {
	return nil, nil
}
func (m *mockStorage) GetIssueComments(ctx context.Context, issueID string) ([]*types.Comment, error) {
	return nil, nil
}
func (m *mockStorage) GetCommentsForIssues(ctx context.Context, issueIDs []string) (map[string][]*types.Comment, error) {
	return nil, nil
}
func (m *mockStorage) GetStatistics(ctx context.Context) (*types.Statistics, error) {
	return nil, nil
}
func (m *mockStorage) GetNextChildID(ctx context.Context, parentID string) (string, error) {
	return "", nil
}
func (m *mockStorage) SetConfig(ctx context.Context, key, value string) error {
	return nil
}
func (m *mockStorage) GetConfig(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockStorage) GetAllConfig(ctx context.Context) (map[string]string, error) {
	return nil, nil
}
func (m *mockStorage) DeleteConfig(ctx context.Context, key string) error {
	return nil
}
func (m *mockStorage) GetCustomStatuses(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockStorage) GetCustomTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (m *mockStorage) RunInTransaction(ctx context.Context, fn func(tx Transaction) error) error {
	return nil
}
func (m *mockStorage) Close() error {
	return nil
}
func (m *mockStorage) Path() string {
	return ""
}

type mockTransaction struct{}

func (m *mockTransaction) CreateIssue(ctx context.Context, issue *types.Issue, actor string) error {
	return nil
}
func (m *mockTransaction) CreateIssues(ctx context.Context, issues []*types.Issue, actor string) error {
	return nil
}
func (m *mockTransaction) UpdateIssue(ctx context.Context, id string, updates map[string]interface{}, actor string) error {
	return nil
}
func (m *mockTransaction) CloseIssue(ctx context.Context, id string, reason string, actor string, session string) error {
	return nil
}
func (m *mockTransaction) DeleteIssue(ctx context.Context, id string) error {
	return nil
}
func (m *mockTransaction) DeleteIssueWithReason(ctx context.Context, id, actor, reason string) error {
	return nil
}
func (m *mockTransaction) GetIssue(ctx context.Context, id string) (*types.Issue, error) {
	return nil, nil
}
func (m *mockTransaction) SearchIssues(ctx context.Context, query string, filter types.IssueFilter) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockTransaction) AddDependency(ctx context.Context, dep *types.Dependency, actor string) error {
	return nil
}
func (m *mockTransaction) RemoveDependency(ctx context.Context, issueID, dependsOnID string, actor string) error {
	return nil
}
func (m *mockTransaction) GetDependencies(ctx context.Context, issueID string) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockTransaction) GetDependents(ctx context.Context, issueID string) ([]*types.Issue, error) {
	return nil, nil
}
func (m *mockTransaction) GetDependencyRecords(ctx context.Context, issueID string) ([]*types.Dependency, error) {
	return nil, nil
}
func (m *mockTransaction) AddLabel(ctx context.Context, issueID, label, actor string) error {
	return nil
}
func (m *mockTransaction) RemoveLabel(ctx context.Context, issueID, label, actor string) error {
	return nil
}
func (m *mockTransaction) AddComment(ctx context.Context, issueID, actor, comment string) error {
	return nil
}

func TestInterfaceDocumentation(t *testing.T) {
	t.Run("storage interface has retained method groups", func(t *testing.T) {
		var s Storage = &mockStorage{}

		_ = s.CreateIssue
		_ = s.CreateIssues
		_ = s.GetIssue
		_ = s.GetIssueByExternalRef
		_ = s.UpdateIssue
		_ = s.CloseIssue
		_ = s.DeleteIssue
		_ = s.DeleteIssueWithReason
		_ = s.SearchIssues

		_ = s.AddDependency
		_ = s.RemoveDependency
		_ = s.GetDependencies
		_ = s.GetDependents
		_ = s.GetDependencyRecords
		_ = s.GetAllDependencyRecords
		_ = s.GetDependencyCounts
		_ = s.GetDependencyTree
		_ = s.DetectCycles

		_ = s.AddLabel
		_ = s.RemoveLabel
		_ = s.GetLabels
		_ = s.GetLabelsForIssues
		_ = s.GetIssuesByLabel

		_ = s.GetReadyWork
		_ = s.GetBlockedIssues
		_ = s.GetEpicsEligibleForClosure
		_ = s.GetStaleIssues

		_ = s.AddComment
		_ = s.GetEvents
		_ = s.AddIssueComment
		_ = s.GetIssueComments
		_ = s.GetCommentsForIssues

		_ = s.GetStatistics
		_ = s.GetNextChildID
		_ = s.SetConfig
		_ = s.GetConfig
		_ = s.GetAllConfig
		_ = s.DeleteConfig
		_ = s.GetCustomStatuses
		_ = s.GetCustomTypes
		_ = s.RunInTransaction
		_ = s.Close
		_ = s.Path
	})

	t.Run("transaction interface has retained mutation methods", func(t *testing.T) {
		var tx Transaction = &mockTransaction{}

		_ = tx.CreateIssue
		_ = tx.CreateIssues
		_ = tx.UpdateIssue
		_ = tx.CloseIssue
		_ = tx.DeleteIssue
		_ = tx.DeleteIssueWithReason
		_ = tx.GetIssue
		_ = tx.SearchIssues
		_ = tx.AddDependency
		_ = tx.RemoveDependency
		_ = tx.GetDependencies
		_ = tx.GetDependents
		_ = tx.GetDependencyRecords
		_ = tx.AddLabel
		_ = tx.RemoveLabel
		_ = tx.AddComment
	})
}
