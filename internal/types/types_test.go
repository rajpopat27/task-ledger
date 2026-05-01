package types

import (
	"strings"
	"testing"
	"time"
)

func intPtr(v int) *int { return &v }

func validIssue() *Issue {
	return &Issue{
		ID:        "tl-test",
		Title:     "Test issue",
		Status:    StatusOpen,
		Priority:  2,
		IssueType: TypeTask,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestIssueValidation(t *testing.T) {
	closedAt := time.Now()
	deletedAt := time.Now()

	tests := []struct {
		name    string
		issue   *Issue
		wantErr string
	}{
		{"valid", validIssue(), ""},
		{"nil", nil, "issue is nil"},
		{"missing title", func() *Issue { i := validIssue(); i.Title = ""; return i }(), "title is required"},
		{"title too long", func() *Issue { i := validIssue(); i.Title = strings.Repeat("x", 501); return i }(), "title must be 500 characters"},
		{"invalid priority", func() *Issue { i := validIssue(); i.Priority = 5; return i }(), "priority must be between 0 and 4"},
		{"invalid status", func() *Issue { i := validIssue(); i.Status = "bad"; return i }(), "invalid status"},
		{"invalid type", func() *Issue { i := validIssue(); i.IssueType = "bad"; return i }(), "invalid issue type"},
		{"negative estimate", func() *Issue { i := validIssue(); i.EstimatedMinutes = intPtr(-1); return i }(), "estimated_minutes cannot be negative"},
		{"closed missing timestamp", func() *Issue { i := validIssue(); i.Status = StatusClosed; return i }(), "closed issues must have closed_at"},
		{"open has closed_at", func() *Issue { i := validIssue(); i.ClosedAt = &closedAt; return i }(), "non-closed issues cannot have closed_at"},
		{"tombstone missing deleted_at", func() *Issue { i := validIssue(); i.Status = StatusTombstone; return i }(), "tombstone issues must have deleted_at"},
		{"open has deleted_at", func() *Issue { i := validIssue(); i.DeletedAt = &deletedAt; return i }(), "non-tombstone issues cannot have deleted_at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.issue.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWithCustom(t *testing.T) {
	issue := validIssue()
	issue.Status = "triage"
	issue.IssueType = "research"

	if err := issue.ValidateWithCustom(nil, nil); err == nil {
		t.Fatal("expected custom status/type to fail without configuration")
	}
	if err := issue.ValidateWithCustom([]string{"triage"}, []string{"research"}); err != nil {
		t.Fatalf("ValidateWithCustom returned error: %v", err)
	}
}

func TestTombstoneExpiry(t *testing.T) {
	deleted := time.Now().Add(-31 * 24 * time.Hour)
	issue := validIssue()
	issue.Status = StatusTombstone
	issue.DeletedAt = &deleted

	if !issue.IsTombstone() {
		t.Fatal("expected tombstone")
	}
	if !issue.IsExpired(0) {
		t.Fatal("expected tombstone older than default TTL to be expired")
	}

	live := validIssue()
	if live.IsExpired(-1) {
		t.Fatal("non-tombstone issue should never expire")
	}
}

func TestSetDefaults(t *testing.T) {
	issue := &Issue{Title: "missing defaults", Priority: 2}
	issue.SetDefaults()
	if issue.Status != StatusOpen {
		t.Fatalf("Status = %s, want %s", issue.Status, StatusOpen)
	}
	if issue.IssueType != TypeTask {
		t.Fatalf("IssueType = %s, want %s", issue.IssueType, TypeTask)
	}
}

func TestStatusAndIssueTypeValidity(t *testing.T) {
	for _, status := range []Status{StatusOpen, StatusInProgress, StatusBlocked, StatusDeferred, StatusClosed, StatusTombstone, StatusPinned} {
		if !status.IsValid() {
			t.Fatalf("status %q should be valid", status)
		}
	}
	if Status("custom").IsValid() {
		t.Fatal("custom status should not be built-in")
	}
	if !Status("custom").IsValidWithCustom([]string{"custom"}) {
		t.Fatal("custom status should be valid when configured")
	}

	for _, issueType := range []IssueType{TypeBug, TypeFeature, TypeFeatureRequest, TypeTask, TypeEpic, TypeChore} {
		if !issueType.IsValid() || !issueType.IsBuiltIn() {
			t.Fatalf("issue type %q should be valid built-in", issueType)
		}
	}
	if IssueType("agent").IsValid() {
		t.Fatal("removed legacy issue type should not be built-in")
	}
	if !IssueType("research").IsValidWithCustom([]string{"research"}) {
		t.Fatal("custom issue type should be valid when configured")
	}
}

func TestRequiredSections(t *testing.T) {
	if got := TypeBug.RequiredSections(); len(got) != 2 {
		t.Fatalf("bug sections = %d, want 2", len(got))
	}
	if got := TypeFeatureRequest.RequiredSections(); len(got) != 1 {
		t.Fatalf("feature_request sections = %d, want 1", len(got))
	}
	if got := TypeChore.RequiredSections(); len(got) != 0 {
		t.Fatalf("chore sections = %d, want 0", len(got))
	}
}

func TestDependencyTypes(t *testing.T) {
	for _, depType := range []DependencyType{
		DepBlocks, DepParentChild, DepConditionalBlocks, DepRelated, DepDiscoveredFrom,
		DepRepliesTo, DepRelatesTo, DepDuplicates, DepSupersedes, DepTracks,
		DepUntil, DepCausedBy, DepValidates,
	} {
		if !depType.IsValid() || !depType.IsWellKnown() {
			t.Fatalf("dependency type %q should be valid and well-known", depType)
		}
	}
	if !DepBlocks.AffectsReadyWork() || !DepParentChild.AffectsReadyWork() || !DepConditionalBlocks.AffectsReadyWork() {
		t.Fatal("blocking dependency types should affect ready work")
	}
	if DepRelated.AffectsReadyWork() {
		t.Fatal("related dependency should not affect ready work")
	}
	if DependencyType("").IsValid() {
		t.Fatal("empty dependency type should be invalid")
	}
}

func TestIsFailureClose(t *testing.T) {
	for _, reason := range []string{"failed tests", "Rejected by reviewer", "timeout waiting for CI"} {
		if !IsFailureClose(reason) {
			t.Fatalf("reason %q should be treated as failure", reason)
		}
	}
	if IsFailureClose("done") {
		t.Fatal("successful close reason should not be failure")
	}
}

func TestEmbeddedViewTypes(t *testing.T) {
	blocked := BlockedIssue{Issue: Issue{ID: "tl-1"}, BlockedByCount: 2, BlockedBy: []string{"tl-2", "tl-3"}}
	if blocked.ID != "tl-1" || blocked.BlockedByCount != 2 {
		t.Fatalf("blocked issue embedding failed: %#v", blocked)
	}

	node := TreeNode{Issue: Issue{ID: "tl-1"}, Depth: 1, ParentID: "tl-root"}
	if node.ID != "tl-1" || node.Depth != 1 || node.ParentID != "tl-root" {
		t.Fatalf("tree node embedding failed: %#v", node)
	}
}

func TestSortPolicyIsValid(t *testing.T) {
	for _, policy := range []SortPolicy{SortPolicyHybrid, SortPolicyPriority, SortPolicyOldest, ""} {
		if !policy.IsValid() {
			t.Fatalf("sort policy %q should be valid", policy)
		}
	}
	if SortPolicy("random").IsValid() {
		t.Fatal("unknown sort policy should be invalid")
	}
}
