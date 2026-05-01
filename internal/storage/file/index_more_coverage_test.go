package file

import (
	"context"
	"testing"
	"time"

	"task-ledger/internal/types"
)

func timePtr(t time.Time) *time.Time { return &t }

func boolPtr(b bool) *bool { return &b }

func TestIssueIndex_LoadFromIssues_IndexesAndCounters(t *testing.T) {
	store := newIssueIndex()
	defer store.Close()

	extRef := "ext-1"
	issues := []*types.Issue{
		nil,
		{
			ID:          "tl-10",
			Title:       "Ten",
			Status:      types.StatusOpen,
			Priority:    1,
			IssueType:   types.TypeTask,
			ExternalRef: &extRef,
			Dependencies: []*types.Dependency{{
				IssueID:     "tl-10",
				DependsOnID: "tl-2",
				Type:        types.DepBlocks,
			}},
			Labels:   []string{"l1"},
			Comments: []*types.Comment{{ID: 1, IssueID: "tl-10", Author: "a", Text: "c"}},
		},
		{ID: "tl-2", Title: "Two", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask},
		{ID: "tl-a3f8e9", Title: "Parent", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask},
		{ID: "tl-a3f8e9.3", Title: "Child", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask},
	}

	if err := store.LoadFromIssues(issues); err != nil {
		t.Fatalf("LoadFromIssues: %v", err)
	}

	ctx := context.Background()

	got, err := store.GetIssueByExternalRef(ctx, "ext-1")
	if err != nil {
		t.Fatalf("GetIssueByExternalRef: %v", err)
	}
	if got == nil || got.ID != "tl-10" {
		t.Fatalf("GetIssueByExternalRef got=%v", got)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0].DependsOnID != "tl-2" {
		t.Fatalf("expected deps attached")
	}
	if len(got.Labels) != 1 || got.Labels[0] != "l1" {
		t.Fatalf("expected labels attached")
	}

	// Exercise CreateIssue ID generation based on the loaded counter (tl-10 => next should be tl-11).
	if err := store.SetConfig(ctx, "issue_prefix", "tl"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	newIssue := &types.Issue{Title: "New", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, newIssue, "actor"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if newIssue.ID != "tl-11" {
		t.Fatalf("expected generated id tl-11, got %q", newIssue.ID)
	}

	// Hierarchical counter for parent extracted from tl-a3f8e9.3.
	childID, err := store.GetNextChildID(ctx, "tl-a3f8e9")
	if err != nil {
		t.Fatalf("GetNextChildID: %v", err)
	}
	if childID != "tl-a3f8e9.4" {
		t.Fatalf("expected tl-a3f8e9.4, got %q", childID)
	}
}

func TestIssueIndex_GetAllIssues_SortsAndCopies(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	// Create out-of-order IDs.
	a := &types.Issue{ID: "tl-2", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	b := &types.Issue{ID: "tl-1", Title: "B", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, a, "actor"); err != nil {
		t.Fatalf("CreateIssue a: %v", err)
	}
	if err := store.CreateIssue(ctx, b, "actor"); err != nil {
		t.Fatalf("CreateIssue b: %v", err)
	}

	if err := store.AddLabel(ctx, a.ID, "l1", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	all := store.GetAllIssues()
	if len(all) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(all))
	}
	if all[0].ID != "tl-1" || all[1].ID != "tl-2" {
		t.Fatalf("expected sorted by ID, got %q then %q", all[0].ID, all[1].ID)
	}

	// Returned issues must be copies (mutating should not affect stored issue struct).
	all[1].Title = "mutated"
	got, err := store.GetIssue(ctx, "tl-2")
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if got.Title != "A" {
		t.Fatalf("expected stored title unchanged, got %q", got.Title)
	}
}

func TestIssueIndex_CreateIssues_DefaultPrefix_DuplicateExisting_ExternalRef(t *testing.T) {
	store := newIssueIndex()
	defer store.Close()
	ctx := context.Background()

	// Default prefix should be "tl" when unset.
	issues := []*types.Issue{{Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}}
	if err := store.CreateIssues(ctx, issues, "actor"); err != nil {
		t.Fatalf("CreateIssues: %v", err)
	}
	if issues[0].ID != "tl-1" {
		t.Fatalf("expected tl-1, got %q", issues[0].ID)
	}

	ext := "ext"
	batch := []*types.Issue{{ID: "tl-x", Title: "B", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask, ExternalRef: &ext}}
	if err := store.CreateIssues(ctx, batch, "actor"); err != nil {
		t.Fatalf("CreateIssues: %v", err)
	}
	if got, _ := store.GetIssueByExternalRef(ctx, "ext"); got == nil || got.ID != "tl-x" {
		t.Fatalf("expected external ref indexed")
	}

	// Duplicate existing issue ID branch.
	dup := []*types.Issue{{ID: "tl-x", Title: "Dup", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}}
	if err := store.CreateIssues(ctx, dup, "actor"); err == nil {
		t.Fatalf("expected duplicate existing issue error")
	}
}

func TestIssueIndex_GetIssueByExternalRef_IndexPointsToMissingIssue(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	store.mu.Lock()
	store.externalRefToID["dangling"] = "tl-nope"
	store.mu.Unlock()

	got, err := store.GetIssueByExternalRef(ctx, "dangling")
	if err != nil {
		t.Fatalf("GetIssueByExternalRef: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for dangling ref")
	}
}

func TestIssueIndex_DependencyCounts_Records_Tree_Cycles(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	a := &types.Issue{ID: "tl-1", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	b := &types.Issue{ID: "tl-2", Title: "B", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	c := &types.Issue{ID: "tl-3", Title: "C", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	d := &types.Issue{ID: "tl-4", Title: "D", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	for _, iss := range []*types.Issue{a, b, c, d} {
		if err := store.CreateIssue(ctx, iss, "actor"); err != nil {
			t.Fatalf("CreateIssue %s: %v", iss.ID, err)
		}
	}

	if err := store.AddDependency(ctx, &types.Dependency{IssueID: a.ID, DependsOnID: b.ID, Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: a.ID, DependsOnID: c.ID, Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: d.ID, DependsOnID: b.ID, Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	counts, err := store.GetDependencyCounts(ctx, []string{a.ID, b.ID, "tl-missing"})
	if err != nil {
		t.Fatalf("GetDependencyCounts: %v", err)
	}
	if counts[a.ID].DependencyCount != 2 || counts[a.ID].DependentCount != 0 {
		t.Fatalf("unexpected counts for A: %+v", counts[a.ID])
	}
	if counts[b.ID].DependencyCount != 0 || counts[b.ID].DependentCount != 2 {
		t.Fatalf("unexpected counts for B: %+v", counts[b.ID])
	}
	if counts["tl-missing"].DependencyCount != 0 || counts["tl-missing"].DependentCount != 0 {
		t.Fatalf("unexpected counts for missing: %+v", counts["tl-missing"])
	}

	deps, err := store.GetDependencyRecords(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetDependencyRecords: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}

	allDeps, err := store.GetAllDependencyRecords(ctx)
	if err != nil {
		t.Fatalf("GetAllDependencyRecords: %v", err)
	}
	if len(allDeps[a.ID]) != 2 {
		t.Fatalf("expected all deps for A")
	}

	nodes, err := store.GetDependencyTree(ctx, a.ID, 3, false, false)
	if err != nil {
		t.Fatalf("GetDependencyTree: %v", err)
	}
	// Expect 3 nodes: root (A) at depth 0, plus 2 dependencies (B, C) at depth 1
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (root + 2 deps), got %d", len(nodes))
	}
	if nodes[0].ID != a.ID || nodes[0].Depth != 0 {
		t.Fatalf("expected root node %s at depth 0, got %s at depth %d", a.ID, nodes[0].ID, nodes[0].Depth)
	}

	cycles, err := store.DetectCycles(ctx)
	if err != nil {
		t.Fatalf("DetectCycles: %v", err)
	}
	if cycles != nil {
		t.Fatalf("expected nil cycles, got %+v", cycles)
	}

	if err := store.AddDependency(ctx, &types.Dependency{IssueID: b.ID, DependsOnID: d.ID, Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency recursive: %v", err)
	}
	nodes, err = store.GetDependencyTree(ctx, a.ID, 3, false, false)
	if err != nil {
		t.Fatalf("GetDependencyTree recursive: %v", err)
	}
	if len(nodes) != 5 || nodes[2].ID != d.ID || nodes[2].Depth != 2 || !nodes[3].Truncated {
		t.Fatalf("expected recursive dependency at depth 2, got %+v", nodes)
	}
	reverse, err := store.GetDependencyTree(ctx, b.ID, 3, false, true)
	if err != nil {
		t.Fatalf("GetDependencyTree reverse: %v", err)
	}
	if len(reverse) < 3 || reverse[1].ID != a.ID || reverse[2].ID != d.ID {
		t.Fatalf("unexpected reverse dependency tree: %+v", reverse)
	}

	if err := store.AddDependency(ctx, &types.Dependency{IssueID: a.ID, DependsOnID: "external:payments:auth", Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency external: %v", err)
	}
	depsWithMetadata, err := store.GetDependenciesWithMetadata(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetDependenciesWithMetadata external: %v", err)
	}
	foundExternal := false
	for _, dep := range depsWithMetadata {
		if dep.ID == "external:payments:auth" {
			foundExternal = true
		}
	}
	if !foundExternal {
		t.Fatalf("expected external dependency in metadata: %+v", depsWithMetadata)
	}

	cycles, err = store.DetectCycles(ctx)
	if err != nil {
		t.Fatalf("DetectCycles after cycle: %v", err)
	}
	if len(cycles) != 1 || len(cycles[0]) != 2 {
		t.Fatalf("expected one 2-node cycle, got %+v", cycles)
	}
}

func TestIssueIndex_LabelsAndCommentsHelpers(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	a := &types.Issue{ID: "tl-1", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	b := &types.Issue{ID: "tl-2", Title: "B", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, a, "actor"); err != nil {
		t.Fatalf("CreateIssue a: %v", err)
	}
	if err := store.CreateIssue(ctx, b, "actor"); err != nil {
		t.Fatalf("CreateIssue b: %v", err)
	}

	if err := store.AddLabel(ctx, a.ID, "l1", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	if err := store.AddLabel(ctx, b.ID, "l2", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	labels, err := store.GetLabelsForIssues(ctx, []string{a.ID, b.ID, "tl-missing"})
	if err != nil {
		t.Fatalf("GetLabelsForIssues: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(labels))
	}
	if labels[a.ID][0] != "l1" {
		t.Fatalf("unexpected labels for A: %+v", labels[a.ID])
	}

	issues, err := store.GetIssuesByLabel(ctx, "l1")
	if err != nil {
		t.Fatalf("GetIssuesByLabel: %v", err)
	}
	if len(issues) != 1 || issues[0].ID != a.ID {
		t.Fatalf("unexpected issues: %+v", issues)
	}

	if _, err := store.AddIssueComment(ctx, a.ID, "author", "text"); err != nil {
		t.Fatalf("AddIssueComment: %v", err)
	}
	comments, err := store.GetCommentsForIssues(ctx, []string{a.ID, b.ID})
	if err != nil {
		t.Fatalf("GetCommentsForIssues: %v", err)
	}
	if len(comments[a.ID]) != 1 {
		t.Fatalf("expected comments for A")
	}
}

func TestIssueIndex_StaleEventsCustomStatusAndLifecycleHelpers(t *testing.T) {
	store := newIssueIndex()
	defer store.Close()
	ctx := context.Background()

	if err := store.SetConfig(ctx, "issue_prefix", "tl"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	a := &types.Issue{ID: "tl-1", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, a, "actor"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	// Force updated_at into the past for stale detection.
	store.mu.Lock()
	a.UpdatedAt = time.Now().Add(-10 * 24 * time.Hour)
	store.mu.Unlock()

	stale, err := store.GetStaleIssues(ctx, types.StaleFilter{Days: 7, Limit: 10})
	if err != nil {
		t.Fatalf("GetStaleIssues: %v", err)
	}
	if len(stale) != 1 || stale[0].ID != a.ID {
		t.Fatalf("unexpected stale: %+v", stale)
	}

	if err := store.AddComment(ctx, a.ID, "actor", "c"); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	comments, err := store.GetIssueComments(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetIssueComments: %v", err)
	}
	if len(comments) != 1 || comments[0].Text != "c" {
		t.Fatalf("AddComment did not persist comment: %+v", comments)
	}
	if err := store.AddComment(ctx, "tl-missing", "actor", "missing"); err == nil {
		t.Fatalf("expected AddComment on missing issue to fail")
	}

	// Generate multiple events and ensure limiting returns the last N.
	if err := store.UpdateIssue(ctx, a.ID, map[string]interface{}{"title": "t1"}, "actor"); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if err := store.UpdateIssue(ctx, a.ID, map[string]interface{}{"title": "t2"}, "actor"); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	evs, err := store.GetEvents(ctx, a.ID, 2)
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("expected 2 events, got %d", len(evs))
	}

	if err := store.SetConfig(ctx, "status.custom", " triage,  blocked , ,done "); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	statuses, err := store.GetCustomStatuses(ctx)
	if err != nil {
		t.Fatalf("GetCustomStatuses: %v", err)
	}
	if len(statuses) != 3 || statuses[0] != "triage" || statuses[1] != "blocked" || statuses[2] != "done" {
		t.Fatalf("unexpected statuses: %+v", statuses)
	}
	if got := parseCustomStatuses(""); got != nil {
		t.Fatalf("expected nil for empty parseCustomStatuses")
	}

	// Empty custom statuses.
	if err := store.DeleteConfig(ctx, "status.custom"); err != nil {
		t.Fatalf("DeleteConfig: %v", err)
	}
	statuses, err = store.GetCustomStatuses(ctx)
	if err != nil {
		t.Fatalf("GetCustomStatuses(empty): %v", err)
	}
	if statuses != nil {
		t.Fatalf("expected nil statuses when unset, got %+v", statuses)
	}

	if _, err := store.GetEpicsEligibleForClosure(ctx); err != nil {
		t.Fatalf("GetEpicsEligibleForClosure: %v", err)
	}

}

func TestIssueIndex_SearchIssues_AllRetainedFilters(t *testing.T) {
	store := newIssueIndex()
	defer store.Close()
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	closedAt := now.Add(-30 * time.Minute)
	pastDue := now.Add(-1 * time.Hour)
	futureDue := now.Add(3 * time.Hour)
	futureDefer := now.Add(2 * time.Hour)

	issues := []*types.Issue{
		{
			ID:          "tl-alpha",
			Title:       "Alpha login",
			Description: "backend auth",
			Notes:       "needs migration",
			Status:      types.StatusOpen,
			Priority:    1,
			IssueType:   types.TypeTask,
			CreatedAt:   now.Add(-48 * time.Hour),
			UpdatedAt:   now.Add(-2 * time.Hour),
			DueAt:       &pastDue,
			Labels:      []string{"backend"},
		},
		{
			ID:        "tl-beta",
			Title:     "Beta docs",
			Status:    types.StatusClosed,
			Priority:  3,
			IssueType: types.TypeChore,
			Assignee:  "bob",
			CreatedAt: now.Add(-24 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
			ClosedAt:  &closedAt,
			Pinned:    true,
		},
		{
			ID:          "tl-gamma",
			Title:       "Gamma release",
			Description: "frontend",
			Status:      types.StatusOpen,
			Priority:    4,
			IssueType:   types.TypeTask,
			CreatedAt:   now.Add(-1 * time.Hour),
			UpdatedAt:   now.Add(-30 * time.Minute),
			DeferUntil:  &futureDefer,
			DueAt:       &futureDue,
		},
	}
	if err := store.LoadFromIssues(issues); err != nil {
		t.Fatalf("LoadFromIssues: %v", err)
	}

	assertIDs := func(name string, filter types.IssueFilter, want ...string) {
		t.Helper()
		got, err := store.SearchIssues(ctx, "", filter)
		if err != nil {
			t.Fatalf("%s SearchIssues: %v", name, err)
		}
		gotIDs := make(map[string]bool, len(got))
		for _, issue := range got {
			gotIDs[issue.ID] = true
		}
		if len(gotIDs) != len(want) {
			t.Fatalf("%s got %v, want %v", name, gotIDs, want)
		}
		for _, id := range want {
			if !gotIDs[id] {
				t.Fatalf("%s missing %s from %v", name, id, gotIDs)
			}
		}
	}

	assertIDs("title search", types.IssueFilter{TitleSearch: "beta"}, "tl-beta")
	assertIDs("title contains", types.IssueFilter{TitleContains: "login"}, "tl-alpha")
	assertIDs("description contains", types.IssueFilter{DescriptionContains: "auth"}, "tl-alpha")
	assertIDs("notes contains", types.IssueFilter{NotesContains: "migration"}, "tl-alpha")
	assertIDs("empty description", types.IssueFilter{EmptyDescription: true}, "tl-beta")
	assertIDs("no assignee", types.IssueFilter{NoAssignee: true}, "tl-alpha", "tl-gamma")
	assertIDs("no labels", types.IssueFilter{NoLabels: true}, "tl-beta", "tl-gamma")
	assertIDs("created after", types.IssueFilter{CreatedAfter: timePtr(now.Add(-36 * time.Hour))}, "tl-beta", "tl-gamma")
	assertIDs("updated before", types.IssueFilter{UpdatedBefore: timePtr(now.Add(-45 * time.Minute))}, "tl-alpha", "tl-beta")
	assertIDs("closed after", types.IssueFilter{ClosedAfter: timePtr(now.Add(-1 * time.Hour))}, "tl-beta")
	assertIDs("exclude status", types.IssueFilter{ExcludeStatus: []types.Status{types.StatusClosed}}, "tl-alpha", "tl-gamma")
	assertIDs("exclude type", types.IssueFilter{ExcludeTypes: []types.IssueType{types.TypeChore}}, "tl-alpha", "tl-gamma")
	assertIDs("pinned", types.IssueFilter{Pinned: boolPtr(true)}, "tl-beta")
	assertIDs("no pinned", types.IssueFilter{Pinned: boolPtr(false)}, "tl-alpha", "tl-gamma")
	assertIDs("deferred", types.IssueFilter{Deferred: true}, "tl-gamma")
	assertIDs("defer before", types.IssueFilter{DeferBefore: timePtr(now.Add(3 * time.Hour))}, "tl-gamma")
	assertIDs("due after", types.IssueFilter{DueAfter: timePtr(now)}, "tl-gamma")
	assertIDs("overdue", types.IssueFilter{Overdue: true}, "tl-alpha")
}

func TestIssueIndex_GetEpicsEligibleForClosureReportsProgress(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	epic := &types.Issue{ID: "tl-epic", Title: "Epic", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeEpic}
	closedChild := &types.Issue{ID: "tl-child-1", Title: "Closed child", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	openChild := &types.Issue{ID: "tl-child-2", Title: "Open child", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	for _, issue := range []*types.Issue{epic, closedChild, openChild} {
		if err := store.CreateIssue(ctx, issue, "actor"); err != nil {
			t.Fatalf("CreateIssue %s: %v", issue.ID, err)
		}
	}
	if err := store.CloseIssue(ctx, closedChild.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue child: %v", err)
	}
	for _, child := range []*types.Issue{closedChild, openChild} {
		if err := store.AddDependency(ctx, &types.Dependency{IssueID: child.ID, DependsOnID: epic.ID, Type: types.DepParentChild}, "actor"); err != nil {
			t.Fatalf("AddDependency child: %v", err)
		}
	}

	statuses, err := store.GetEpicsEligibleForClosure(ctx)
	if err != nil {
		t.Fatalf("GetEpicsEligibleForClosure: %v", err)
	}
	if len(statuses) != 1 || statuses[0].TotalChildren != 2 || statuses[0].ClosedChildren != 1 || statuses[0].EligibleForClose {
		t.Fatalf("unexpected epic status before close: %+v", statuses)
	}
	if err := store.CloseIssue(ctx, openChild.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue open child: %v", err)
	}
	statuses, err = store.GetEpicsEligibleForClosure(ctx)
	if err != nil {
		t.Fatalf("GetEpicsEligibleForClosure after close: %v", err)
	}
	if len(statuses) != 1 || !statuses[0].EligibleForClose || statuses[0].ClosedChildren != 2 {
		t.Fatalf("unexpected epic status after close: %+v", statuses)
	}
}

func TestIssueIndex_AddLabelAndAddDependency_ErrorPaths(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	issue := &types.Issue{ID: "tl-1", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, issue, "actor"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	if err := store.AddLabel(ctx, "tl-missing", "l", "actor"); err == nil {
		t.Fatalf("expected AddLabel error for missing issue")
	}
	if err := store.AddLabel(ctx, issue.ID, "l", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	// Duplicate label is a no-op.
	if err := store.AddLabel(ctx, issue.ID, "l", "actor"); err != nil {
		t.Fatalf("AddLabel duplicate: %v", err)
	}

	// AddDependency error paths.
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: "tl-missing", DependsOnID: issue.ID, Type: types.DepBlocks}, "actor"); err == nil {
		t.Fatalf("expected AddDependency error for missing IssueID")
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: issue.ID, DependsOnID: "tl-missing", Type: types.DepBlocks}, "actor"); err == nil {
		t.Fatalf("expected AddDependency error for missing DependsOnID")
	}
}

func TestIssueIndex_GetNextChildID_Errors(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	if _, err := store.GetNextChildID(ctx, "tl-missing"); err == nil {
		t.Fatalf("expected error for missing parent")
	}

	deep := &types.Issue{ID: "tl-1.1.1.1", Title: "Deep", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, deep, "actor"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if _, err := store.GetNextChildID(ctx, deep.ID); err == nil {
		t.Fatalf("expected max depth error")
	}
}

func TestIssueIndex_GetAllIssues_AttachesDependenciesAndComments(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	a := &types.Issue{ID: "tl-1", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	b := &types.Issue{ID: "tl-2", Title: "B", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, a, "actor"); err != nil {
		t.Fatalf("CreateIssue a: %v", err)
	}
	if err := store.CreateIssue(ctx, b, "actor"); err != nil {
		t.Fatalf("CreateIssue b: %v", err)
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: a.ID, DependsOnID: b.ID, Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if _, err := store.AddIssueComment(ctx, a.ID, "author", "text"); err != nil {
		t.Fatalf("AddIssueComment: %v", err)
	}

	all := store.GetAllIssues()
	var gotA *types.Issue
	for _, iss := range all {
		if iss.ID == a.ID {
			gotA = iss
			break
		}
	}
	if gotA == nil {
		t.Fatalf("expected to find issue A")
	}
	if len(gotA.Dependencies) != 1 || gotA.Dependencies[0].DependsOnID != b.ID {
		t.Fatalf("expected deps attached")
	}
	if len(gotA.Comments) != 1 || gotA.Comments[0].Text != "text" {
		t.Fatalf("expected comments attached")
	}
}

func TestIssueIndex_GetStaleIssues_FilteringAndLimit(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	old := &types.Issue{ID: "tl-1", Title: "Old", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	newer := &types.Issue{ID: "tl-2", Title: "Newer", Status: types.StatusInProgress, Priority: 1, IssueType: types.TypeTask}
	closed := &types.Issue{ID: "tl-3", Title: "Closed", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	for _, iss := range []*types.Issue{old, newer, closed} {
		if err := store.CreateIssue(ctx, iss, "actor"); err != nil {
			t.Fatalf("CreateIssue %s: %v", iss.ID, err)
		}
	}
	if err := store.CloseIssue(ctx, closed.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue: %v", err)
	}

	store.mu.Lock()
	store.issues[old.ID].UpdatedAt = time.Now().Add(-20 * 24 * time.Hour)
	store.issues[newer.ID].UpdatedAt = time.Now().Add(-10 * 24 * time.Hour)
	store.issues[closed.ID].UpdatedAt = time.Now().Add(-30 * 24 * time.Hour)
	store.mu.Unlock()

	stale, err := store.GetStaleIssues(ctx, types.StaleFilter{Days: 7, Status: "in_progress"})
	if err != nil {
		t.Fatalf("GetStaleIssues: %v", err)
	}
	if len(stale) != 1 || stale[0].ID != newer.ID {
		t.Fatalf("unexpected stale filtered: %+v", stale)
	}

	stale, err = store.GetStaleIssues(ctx, types.StaleFilter{Days: 7, Limit: 1})
	if err != nil {
		t.Fatalf("GetStaleIssues: %v", err)
	}
	if len(stale) != 1 || stale[0].ID != old.ID {
		t.Fatalf("expected oldest stale first, got %+v", stale)
	}
}

func TestIssueIndex_Statistics_EpicsEligibleForClosure_Counting(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	ep := &types.Issue{ID: "tl-1", Title: "Epic", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeEpic}
	c1 := &types.Issue{ID: "tl-2", Title: "Child1", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	c2 := &types.Issue{ID: "tl-3", Title: "Child2", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	for _, iss := range []*types.Issue{ep, c1, c2} {
		if err := store.CreateIssue(ctx, iss, "actor"); err != nil {
			t.Fatalf("CreateIssue %s: %v", iss.ID, err)
		}
	}
	if err := store.CloseIssue(ctx, c1.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue c1: %v", err)
	}
	if err := store.CloseIssue(ctx, c2.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue c2: %v", err)
	}
	// Parent-child deps: child -> epic.
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: c1.ID, DependsOnID: ep.ID, Type: types.DepParentChild}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: c2.ID, DependsOnID: ep.ID, Type: types.DepParentChild}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}

	stats, err := store.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics: %v", err)
	}
	if stats.EpicsEligibleForClosure != 1 {
		t.Fatalf("expected 1 epic eligible, got %d", stats.EpicsEligibleForClosure)
	}
}

func TestIssueIndex_UpdateIssue_SearchIssues_ReadyWork_BlockedIssues(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	now := time.Now()
	assignee := "alice"

	parent := &types.Issue{ID: "tl-1", Title: "Parent", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeEpic}
	child := &types.Issue{ID: "tl-2", Title: "Child", Status: types.StatusOpen, Priority: 2, IssueType: types.TypeTask, Assignee: assignee}
	blocker := &types.Issue{ID: "tl-3", Title: "Blocker", Status: types.StatusOpen, Priority: 3, IssueType: types.TypeTask}
	pinned := &types.Issue{ID: "tl-4", Title: "Pinned", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask, Pinned: true}
	workflow := &types.Issue{ID: "tl-5", Title: "Workflow", Status: types.StatusOpen, Priority: 1, IssueType: types.IssueType("merge-request")}
	for _, iss := range []*types.Issue{parent, child, blocker, pinned} {
		if err := store.CreateIssue(ctx, iss, "actor"); err != nil {
			t.Fatalf("CreateIssue %s: %v", iss.ID, err)
		}
	}
	if err := store.LoadFromIssues([]*types.Issue{workflow}); err != nil {
		t.Fatalf("LoadFromIssues workflow: %v", err)
	}

	// Make created_at deterministic for sorting.
	store.mu.Lock()
	store.issues[parent.ID].CreatedAt = now.Add(-100 * time.Hour)
	store.issues[child.ID].CreatedAt = now.Add(-1 * time.Hour)
	store.issues[blocker.ID].CreatedAt = now.Add(-2 * time.Hour)
	store.issues[pinned.ID].CreatedAt = now.Add(-3 * time.Hour)
	store.issues[workflow.ID].CreatedAt = now.Add(-4 * time.Hour)
	store.mu.Unlock()

	// Dependencies: child is a child of parent; child is blocked by blocker.
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: child.ID, DependsOnID: parent.ID, Type: types.DepParentChild}, "actor"); err != nil {
		t.Fatalf("AddDependency parent-child: %v", err)
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: child.ID, DependsOnID: blocker.ID, Type: types.DepBlocks}, "actor"); err != nil {
		t.Fatalf("AddDependency blocks: %v", err)
	}

	// AddDependency duplicate error path.
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: child.ID, DependsOnID: blocker.ID, Type: types.DepBlocks}, "actor"); err == nil {
		t.Fatalf("expected duplicate dependency error")
	}

	// UpdateIssue: exercise assignee nil, external_ref update+clear, and closed_at behavior.
	ext := "old-ext"
	store.mu.Lock()
	store.issues[child.ID].ExternalRef = &ext
	store.externalRefToID[ext] = child.ID
	store.mu.Unlock()

	if err := store.UpdateIssue(ctx, child.ID, map[string]interface{}{"assignee": nil, "external_ref": "new-ext"}, "actor"); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if got, _ := store.GetIssueByExternalRef(ctx, "old-ext"); got != nil {
		t.Fatalf("expected old-ext removed")
	}
	if got, _ := store.GetIssueByExternalRef(ctx, "new-ext"); got == nil || got.ID != child.ID {
		t.Fatalf("expected new-ext mapping")
	}

	if err := store.UpdateIssue(ctx, child.ID, map[string]interface{}{"status": string(types.StatusClosed)}, "actor"); err != nil {
		t.Fatalf("UpdateIssue close: %v", err)
	}
	closed, _ := store.GetIssue(ctx, child.ID)
	if closed.ClosedAt == nil {
		t.Fatalf("expected ClosedAt set")
	}
	if err := store.UpdateIssue(ctx, child.ID, map[string]interface{}{"status": string(types.StatusOpen), "external_ref": nil}, "actor"); err != nil {
		t.Fatalf("UpdateIssue reopen: %v", err)
	}
	reopened, _ := store.GetIssue(ctx, child.ID)
	if reopened.ClosedAt != nil {
		t.Fatalf("expected ClosedAt cleared")
	}
	if got, _ := store.GetIssueByExternalRef(ctx, "new-ext"); got != nil {
		t.Fatalf("expected new-ext cleared")
	}

	// SearchIssues: query, label AND/OR, IDs filter, ParentID filter, limit.
	if err := store.AddLabel(ctx, parent.ID, "l1", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	if err := store.AddLabel(ctx, child.ID, "l1", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	if err := store.AddLabel(ctx, child.ID, "l2", "actor"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}

	st := types.StatusOpen
	res, err := store.SearchIssues(ctx, "parent", types.IssueFilter{Status: &st})
	if err != nil {
		t.Fatalf("SearchIssues: %v", err)
	}
	if len(res) != 1 || res[0].ID != parent.ID {
		t.Fatalf("unexpected SearchIssues results: %+v", res)
	}

	res, err = store.SearchIssues(ctx, "", types.IssueFilter{Labels: []string{"l1", "l2"}})
	if err != nil {
		t.Fatalf("SearchIssues labels AND: %v", err)
	}
	if len(res) != 1 || res[0].ID != child.ID {
		t.Fatalf("unexpected labels AND results: %+v", res)
	}

	res, err = store.SearchIssues(ctx, "", types.IssueFilter{IDs: []string{child.ID}})
	if err != nil {
		t.Fatalf("SearchIssues IDs: %v", err)
	}
	if len(res) != 1 || res[0].ID != child.ID {
		t.Fatalf("unexpected IDs results: %+v", res)
	}

	res, err = store.SearchIssues(ctx, "", types.IssueFilter{ParentID: &parent.ID})
	if err != nil {
		t.Fatalf("SearchIssues ParentID: %v", err)
	}
	if len(res) != 1 || res[0].ID != child.ID {
		t.Fatalf("unexpected ParentID results: %+v", res)
	}

	res, err = store.SearchIssues(ctx, "", types.IssueFilter{LabelsAny: []string{"l2", "missing"}, Limit: 1})
	if err != nil {
		t.Fatalf("SearchIssues labels OR: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected limit 1")
	}

	// Ready work: child is blocked, pinned excluded, workflow excluded by default.
	ready, err := store.GetReadyWork(ctx, types.WorkFilter{})
	if err != nil {
		t.Fatalf("GetReadyWork: %v", err)
	}
	if len(ready) != 2 { // parent + blocker
		t.Fatalf("expected 2 ready issues, got %d: %+v", len(ready), ready)
	}

	// Filter by workflow type explicitly.
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{Type: "merge-request"})
	if err != nil {
		t.Fatalf("GetReadyWork type: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != workflow.ID {
		t.Fatalf("expected only workflow issue, got %+v", ready)
	}

	// Status + priority filters.
	prio := 3
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{Status: types.StatusOpen, Priority: &prio})
	if err != nil {
		t.Fatalf("GetReadyWork status+priority: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != blocker.ID {
		t.Fatalf("expected blocker only, got %+v", ready)
	}

	// Label filters.
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{Labels: []string{"l1"}})
	if err != nil {
		t.Fatalf("GetReadyWork labels AND: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != parent.ID {
		t.Fatalf("expected parent only, got %+v", ready)
	}
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{LabelsAny: []string{"l2"}})
	if err != nil {
		t.Fatalf("GetReadyWork labels OR: %v", err)
	}
	if len(ready) != 0 {
		t.Fatalf("expected 0 because only l2 issue is blocked")
	}

	// Assignee filter vs Unassigned precedence.
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{Assignee: &assignee})
	if err != nil {
		t.Fatalf("GetReadyWork assignee: %v", err)
	}
	if len(ready) != 0 {
		t.Fatalf("expected 0 due to child being blocked")
	}
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{Unassigned: true})
	if err != nil {
		t.Fatalf("GetReadyWork unassigned: %v", err)
	}
	for _, iss := range ready {
		if iss.Assignee != "" {
			t.Fatalf("expected unassigned only")
		}
	}

	// Sort policies + limit.
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{SortPolicy: types.SortPolicyOldest, Limit: 1})
	if err != nil {
		t.Fatalf("GetReadyWork oldest: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != parent.ID {
		t.Fatalf("expected oldest=parent, got %+v", ready)
	}
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{SortPolicy: types.SortPolicyPriority})
	if err != nil {
		t.Fatalf("GetReadyWork priority: %v", err)
	}
	if len(ready) < 2 || ready[0].Priority > ready[1].Priority {
		t.Fatalf("expected priority sort")
	}
	// Hybrid: recent issues first.
	ready, err = store.GetReadyWork(ctx, types.WorkFilter{SortPolicy: types.SortPolicyHybrid})
	if err != nil {
		t.Fatalf("GetReadyWork hybrid: %v", err)
	}
	if len(ready) != 2 || ready[0].ID != blocker.ID {
		t.Fatalf("expected recent (blocker) first in hybrid, got %+v", ready)
	}

	// Blocked issues: child is blocked by an open blocker.
	blocked, err := store.GetBlockedIssues(ctx, types.WorkFilter{})
	if err != nil {
		t.Fatalf("GetBlockedIssues: %v", err)
	}
	if len(blocked) != 1 || blocked[0].ID != child.ID || blocked[0].BlockedByCount != 1 {
		t.Fatalf("unexpected blocked issues: %+v", blocked)
	}

	// Cover getOpenBlockers missing-blocker branch.
	missing := &types.Issue{ID: "tl-6", Title: "Missing blocker dep", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, missing, "actor"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	// Bypass AddDependency validation to cover the missing-blocker branch in getOpenBlockers.
	store.mu.Lock()
	store.dependencies[missing.ID] = append(store.dependencies[missing.ID], &types.Dependency{IssueID: missing.ID, DependsOnID: "tl-does-not-exist", Type: types.DepBlocks})
	store.mu.Unlock()
	blocked, err = store.GetBlockedIssues(ctx, types.WorkFilter{})
	if err != nil {
		t.Fatalf("GetBlockedIssues: %v", err)
	}
	if len(blocked) != 2 {
		t.Fatalf("expected 2 blocked issues, got %d", len(blocked))
	}
}

func TestIssueIndex_UpdateIssue_CoversMoreFields(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	iss := &types.Issue{ID: "tl-1", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	if err := store.CreateIssue(ctx, iss, "actor"); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}

	if err := store.UpdateIssue(ctx, iss.ID, map[string]interface{}{
		"description":         "d",
		"design":              "design",
		"acceptance_criteria": "ac",
		"notes":               "n",
		"priority":            2,
		"issue_type":          string(types.TypeBug),
		"assignee":            "bob",
		"status":              string(types.StatusInProgress),
	}, "actor"); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}

	got, _ := store.GetIssue(ctx, iss.ID)
	if got.Description != "d" || got.Design != "design" || got.AcceptanceCriteria != "ac" || got.Notes != "n" {
		t.Fatalf("expected text fields updated")
	}
	if got.Priority != 2 || got.IssueType != types.TypeBug || got.Assignee != "bob" || got.Status != types.StatusInProgress {
		t.Fatalf("expected fields updated")
	}

	// Status closed when already closed should not clear ClosedAt.
	if err := store.CloseIssue(ctx, iss.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue: %v", err)
	}
	closedOnce, _ := store.GetIssue(ctx, iss.ID)
	if closedOnce.ClosedAt == nil {
		t.Fatalf("expected ClosedAt")
	}
	if err := store.UpdateIssue(ctx, iss.ID, map[string]interface{}{"status": string(types.StatusClosed)}, "actor"); err != nil {
		t.Fatalf("UpdateIssue closed->closed: %v", err)
	}
	closedTwice, _ := store.GetIssue(ctx, iss.ID)
	if closedTwice.ClosedAt == nil {
		t.Fatalf("expected ClosedAt preserved")
	}
}

func TestIssueIndex_CountEpicsEligibleForClosure_CoversBranches(t *testing.T) {
	store := setupTestIssueIndex(t)
	defer store.Close()
	ctx := context.Background()

	ep1 := &types.Issue{ID: "tl-1", Title: "Epic1", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeEpic}
	epClosed := &types.Issue{ID: "tl-2", Title: "EpicClosed", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeEpic}
	nonEpic := &types.Issue{ID: "tl-3", Title: "NotEpic", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	c := &types.Issue{ID: "tl-4", Title: "Child", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask}
	for _, iss := range []*types.Issue{ep1, epClosed, nonEpic, c} {
		if err := store.CreateIssue(ctx, iss, "actor"); err != nil {
			t.Fatalf("CreateIssue %s: %v", iss.ID, err)
		}
	}
	if err := store.CloseIssue(ctx, epClosed.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue: %v", err)
	}
	// Child -> ep1 (eligible once child is closed).
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: c.ID, DependsOnID: ep1.ID, Type: types.DepParentChild}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	// Child -> nonEpic should not count.
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: c.ID, DependsOnID: nonEpic.ID, Type: types.DepParentChild}, "actor"); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	// Child -> missing epic should not count.
	store.mu.Lock()
	store.dependencies[c.ID] = append(store.dependencies[c.ID], &types.Dependency{IssueID: c.ID, DependsOnID: "tl-missing", Type: types.DepParentChild})
	store.mu.Unlock()

	// Close child to make ep1 eligible.
	if err := store.CloseIssue(ctx, c.ID, "done", "actor", ""); err != nil {
		t.Fatalf("CloseIssue child: %v", err)
	}

	stats, err := store.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics: %v", err)
	}
	if stats.EpicsEligibleForClosure != 1 {
		t.Fatalf("expected 1 eligible epic, got %d", stats.EpicsEligibleForClosure)
	}
}

func TestExtractParentAndChildNumber_CoversFailures(t *testing.T) {
	if _, _, ok := extractParentAndChildNumber("no-dot"); ok {
		t.Fatalf("expected ok=false")
	}
	if _, _, ok := extractParentAndChildNumber("parent.bad"); ok {
		t.Fatalf("expected ok=false")
	}
}
