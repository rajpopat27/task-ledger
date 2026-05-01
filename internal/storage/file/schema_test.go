package file

import (
	"path/filepath"
	"testing"
	"time"

	"task-ledger/internal/types"
)

func TestFileIssueRoundTrip(t *testing.T) {
	now := time.Date(2026, 4, 30, 10, 11, 12, 0, time.UTC)
	externalRef := "gh-42"
	issue := &types.Issue{
		ID:                 "tk-44de10",
		Title:              "Add card field",
		Description:        "Collect card number",
		Design:             "Use the existing form stack",
		AcceptanceCriteria: "Card field validates input",
		Notes:              "Keep the first pass small",
		Status:             types.StatusOpen,
		Priority:           2,
		IssueType:          types.TypeTask,
		Assignee:           "raj",
		Owner:              "raj@example.com",
		ExternalRef:        &externalRef,
		CreatedAt:          now,
		UpdatedAt:          now,
		Labels:             []string{"payments", "ui"},
		Dependencies: []*types.Dependency{
			{IssueID: "tk-44de10", DependsOnID: "tk-11aa22", Type: types.DepBlocks},
			{IssueID: "tk-44de10", DependsOnID: "ep-7c91aa", Type: types.DepParentChild},
		},
		Comments: []*types.Comment{
			{ID: 1, IssueID: "tk-44de10", Author: "agent", Text: "Looks scoped", CreatedAt: now},
		},
	}

	fileIssue := FromTypesIssue(issue)
	if fileIssue.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", fileIssue.SchemaVersion)
	}
	if fileIssue.Type != "task" {
		t.Fatalf("Type = %q, want task", fileIssue.Type)
	}
	if fileIssue.Parent != "ep-7c91aa" {
		t.Fatalf("Parent = %q, want ep-7c91aa", fileIssue.Parent)
	}
	if len(fileIssue.Deps) != 1 || fileIssue.Deps[0] != "tk-11aa22" {
		t.Fatalf("Deps = %#v, want blocking dep", fileIssue.Deps)
	}

	roundTrip, err := fileIssue.ToTypesIssue()
	if err != nil {
		t.Fatalf("ToTypesIssue returned error: %v", err)
	}
	if roundTrip.ID != issue.ID || roundTrip.Title != issue.Title || roundTrip.IssueType != issue.IssueType {
		t.Fatalf("round trip mismatch: got %#v", roundTrip)
	}
	if len(roundTrip.Dependencies) != 2 {
		t.Fatalf("round trip dependencies = %d, want 2", len(roundTrip.Dependencies))
	}
}

func TestPathForIssue(t *testing.T) {
	store := &FileStorage{
		taskLedgerDir: filepath.Join(t.TempDir(), ".task-ledger"),
		pathByID: map[string]string{
			"fr-adb132": filepath.Join("issues", "fr-adb132", "issue.json"),
			"ep-7c91aa": filepath.Join("issues", "fr-adb132", "epics", "ep-7c91aa", "issue.json"),
		},
		issuesByID: map[string]*types.Issue{
			"fr-adb132": {ID: "fr-adb132", IssueType: types.TypeFeatureRequest},
			"ep-7c91aa": {ID: "ep-7c91aa", IssueType: types.TypeEpic},
		},
	}

	feature := &types.Issue{ID: "fr-new001", IssueType: types.TypeFeatureRequest}
	if got := filepath.ToSlash(store.pathForIssue(feature)); got != "issues/fr-new001/issue.json" {
		t.Fatalf("feature path = %s", got)
	}

	epic := &types.Issue{
		ID:        "ep-new001",
		IssueType: types.TypeEpic,
		Dependencies: []*types.Dependency{
			{IssueID: "ep-new001", DependsOnID: "fr-adb132", Type: types.DepParentChild},
		},
	}
	if got := filepath.ToSlash(store.pathForIssue(epic)); got != "issues/fr-adb132/epics/ep-new001/issue.json" {
		t.Fatalf("epic path = %s", got)
	}

	task := &types.Issue{
		ID:        "tk-new001",
		IssueType: types.TypeTask,
		Dependencies: []*types.Dependency{
			{IssueID: "tk-new001", DependsOnID: "ep-7c91aa", Type: types.DepParentChild},
		},
	}
	if got := filepath.ToSlash(store.pathForIssue(task)); got != "issues/fr-adb132/epics/ep-7c91aa/tasks/tk-new001/issue.json" {
		t.Fatalf("task path = %s", got)
	}
}
