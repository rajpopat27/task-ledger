package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"task-ledger/internal/storage"
	"task-ledger/internal/types"
)

func TestOpenScansIssueFiles(t *testing.T) {
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	writeIssueFixture(t, taskLedgerDir, filepath.Join("issues", "fr-adb132", "issue.json"), FileIssue{
		SchemaVersion: 1,
		ID:            "fr-adb132",
		Type:          string(types.TypeFeatureRequest),
		Title:         "Checkout revamp",
		Status:        string(types.StatusOpen),
		Priority:      2,
	})
	writeIssueFixture(t, taskLedgerDir, filepath.Join("issues", "fr-adb132", "epics", "ep-7c91aa", "issue.json"), FileIssue{
		SchemaVersion: 1,
		ID:            "ep-7c91aa",
		Type:          string(types.TypeEpic),
		Title:         "Payment form",
		Status:        string(types.StatusOpen),
		Priority:      2,
		Parent:        "fr-adb132",
	})

	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	issue, err := store.GetIssue(context.Background(), "ep-7c91aa")
	if err != nil {
		t.Fatalf("GetIssue returned error: %v", err)
	}
	if issue == nil {
		t.Fatal("GetIssue returned nil")
	}
	if issue.Title != "Payment form" {
		t.Fatalf("Title = %q, want Payment form", issue.Title)
	}
	if len(issue.Dependencies) != 1 || issue.Dependencies[0].DependsOnID != "fr-adb132" {
		t.Fatalf("parent dependency not loaded: %#v", issue.Dependencies)
	}
}

func TestOpenReportsCorruptIssueFilePath(t *testing.T) {
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	path := filepath.Join(taskLedgerDir, "issues", "tk-bad", "issue.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := Open(taskLedgerDir)
	if err == nil {
		t.Fatal("Open succeeded with corrupt issue file")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %v, want corrupt path %s", err, path)
	}
}

func TestCreateIssueWritesSingleIssueFile(t *testing.T) {
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	issue := &types.Issue{
		Title:     "Checkout revamp",
		IssueType: types.TypeFeatureRequest,
		Status:    types.StatusOpen,
		Priority:  2,
	}
	if err := store.CreateIssue(context.Background(), issue, "test"); err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}
	if issue.ID == "" {
		t.Fatal("CreateIssue did not generate an ID")
	}
	if issue.ID[:3] != "fr-" {
		t.Fatalf("generated ID = %q, want fr-*", issue.ID)
	}

	path := filepath.Join(taskLedgerDir, "issues", issue.ID, "issue.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected issue file at %s: %v", path, err)
	}
}

func TestParentChildDependencyMovesIssueUnderParent(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	fr := &types.Issue{Title: "Checkout revamp", IssueType: types.TypeFeatureRequest, Status: types.StatusOpen, Priority: 2}
	if err := store.CreateIssue(ctx, fr, "test"); err != nil {
		t.Fatalf("CreateIssue FR: %v", err)
	}
	epic := &types.Issue{Title: "Payment form", IssueType: types.TypeEpic, Status: types.StatusOpen, Priority: 2}
	if err := store.CreateIssue(ctx, epic, "test"); err != nil {
		t.Fatalf("CreateIssue epic: %v", err)
	}
	if err := store.AddDependency(ctx, &types.Dependency{IssueID: epic.ID, DependsOnID: fr.ID, Type: types.DepParentChild}, "test"); err != nil {
		t.Fatalf("AddDependency parent-child: %v", err)
	}

	want := filepath.Join(taskLedgerDir, "issues", fr.ID, "epics", epic.ID, "issue.json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected nested epic file at %s: %v", want, err)
	}
	old := filepath.Join(taskLedgerDir, "issues", epic.ID, "issue.json")
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("old top-level epic file still exists or stat failed unexpectedly: %v", err)
	}
}

func TestWriteFileIssueAtomicUsesUniqueTempAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "issue.json")

	first := FileIssue{
		SchemaVersion: SchemaVersion,
		ID:            "tk-atomic",
		Type:          string(types.TypeTask),
		Title:         "first",
		Status:        string(types.StatusOpen),
		Priority:      2,
	}
	second := first
	second.Title = "second"

	if err := writeFileIssueAtomic(path, first); err != nil {
		t.Fatalf("first writeFileIssueAtomic: %v", err)
	}
	if err := writeFileIssueAtomic(path, second); err != nil {
		t.Fatalf("second writeFileIssueAtomic: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var got FileIssue
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("final issue.json is not valid JSON: %v\n%s", err, data)
	}
	if got.Title != "second" {
		t.Fatalf("final title = %q, want second", got.Title)
	}

	for _, pattern := range []string{
		filepath.Join(dir, "issue.json.tmp.*"),
		filepath.Join(dir, ".issue.json.*.tmp"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("Glob(%q): %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Fatalf("unexpected temp files matching %s: %v", pattern, matches)
		}
	}
}

func TestWithWriteLockTimesOutOnFreshLock(t *testing.T) {
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	store.lockTimeout = 20 * time.Millisecond

	if err := os.MkdirAll(taskLedgerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	lockPath := filepath.Join(taskLedgerDir, ".lock")
	if err := os.WriteFile(lockPath, []byte("locked"), 0o644); err != nil {
		t.Fatalf("WriteFile lock: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(lockPath) })

	err = store.CreateIssue(context.Background(), &types.Issue{
		Title:     "Should wait",
		IssueType: types.TypeTask,
		Status:    types.StatusOpen,
		Priority:  2,
	}, "test")
	if err == nil {
		t.Fatal("CreateIssue succeeded while lock was held")
	}
	if !strings.Contains(err.Error(), "lock") {
		t.Fatalf("error = %v, want lock error", err)
	}
}

func TestWithWriteLockBreaksStaleLock(t *testing.T) {
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	store.lockTimeout = 20 * time.Millisecond

	lockPath := filepath.Join(taskLedgerDir, ".lock")
	if err := os.WriteFile(lockPath, []byte("pid=1\n"), 0o644); err != nil {
		t.Fatalf("WriteFile lock: %v", err)
	}
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("Chtimes lock: %v", err)
	}

	issue := &types.Issue{
		Title:     "Stale lock should be broken",
		IssueType: types.TypeTask,
		Status:    types.StatusOpen,
		Priority:  2,
	}
	if err := store.CreateIssue(context.Background(), issue, "test"); err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}
	if issue.ID == "" {
		t.Fatal("CreateIssue did not create an issue")
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale lock still exists or stat failed unexpectedly: %v", err)
	}
}

func TestFileStorageCreateIssuesWritesAllIssues(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	issues := []*types.Issue{
		{Title: "Batch one", IssueType: types.TypeTask, Status: types.StatusOpen, Priority: 2},
		{Title: "Batch two", IssueType: types.TypeBug, Status: types.StatusOpen, Priority: 1},
	}
	if err := store.CreateIssues(ctx, issues, "test"); err != nil {
		t.Fatalf("CreateIssues returned error: %v", err)
	}

	for _, issue := range issues {
		if issue.ID == "" {
			t.Fatalf("issue %q did not get an ID", issue.Title)
		}
		if _, err := store.GetIssue(ctx, issue.ID); err != nil {
			t.Fatalf("GetIssue(%s): %v", issue.ID, err)
		}
		if _, err := os.Stat(filepath.Join(taskLedgerDir, "issues", issue.ID, "issue.json")); err != nil {
			t.Fatalf("expected issue file for %s: %v", issue.ID, err)
		}
	}
}

func TestFileStorageCreateIssuesRejectsInvalidBatchWithoutFiles(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	issues := []*types.Issue{
		{Title: "Valid", IssueType: types.TypeTask, Status: types.StatusOpen, Priority: 2},
		{IssueType: types.TypeBug, Status: types.StatusOpen, Priority: 1},
	}
	if err := store.CreateIssues(ctx, issues, "test"); err == nil {
		t.Fatal("CreateIssues succeeded for invalid batch")
	}
	files, err := FindIssueFiles(taskLedgerDir)
	if err != nil {
		t.Fatalf("FindIssueFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("CreateIssues wrote files for invalid batch: %v", files)
	}
}

func TestFileStorageCreateIssuesRejectsDuplicateExternalRefsWithoutFiles(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ext := "EXT-1"
	issues := []*types.Issue{
		{Title: "A", IssueType: types.TypeTask, Status: types.StatusOpen, Priority: 2, ExternalRef: &ext},
		{Title: "B", IssueType: types.TypeBug, Status: types.StatusOpen, Priority: 1, ExternalRef: &ext},
	}
	if err := store.CreateIssues(ctx, issues, "test"); err == nil {
		t.Fatal("CreateIssues succeeded with duplicate external refs")
	}
	files, err := FindIssueFiles(taskLedgerDir)
	if err != nil {
		t.Fatalf("FindIssueFiles: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("CreateIssues wrote files for duplicate external refs: %v", files)
	}
}

func TestRunInTransactionRollsBackFilesOnError(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	fr := &types.Issue{Title: "Checkout revamp", IssueType: types.TypeFeatureRequest, Status: types.StatusOpen, Priority: 2}
	if err := store.CreateIssue(ctx, fr, "test"); err != nil {
		t.Fatalf("CreateIssue FR: %v", err)
	}

	errBoom := errors.New("boom")
	err = store.RunInTransaction(ctx, func(tx storage.Transaction) error {
		issue := &types.Issue{Title: "Rolled back", IssueType: types.TypeTask, Status: types.StatusOpen, Priority: 2}
		if err := tx.CreateIssue(ctx, issue, "test"); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("RunInTransaction error = %v, want boom", err)
	}

	files, err := FindIssueFiles(taskLedgerDir)
	if err != nil {
		t.Fatalf("FindIssueFiles: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("issue file count after rollback = %d, want 1 (%v)", len(files), files)
	}
}

func TestDeleteIssueCreatesTombstoneWithReason(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	issue := &types.Issue{ID: "tk-delete", Title: "Delete me", IssueType: types.TypeTask, Status: types.StatusOpen, Priority: 2}
	if err := store.CreateIssue(ctx, issue, "test"); err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}
	if err := store.DeleteIssueWithReason(ctx, issue.ID, "actor", "cleanup"); err != nil {
		t.Fatalf("DeleteIssueWithReason returned error: %v", err)
	}

	got, err := store.GetIssue(ctx, issue.ID)
	if err != nil {
		t.Fatalf("GetIssue returned error: %v", err)
	}
	if got == nil {
		t.Fatal("tombstoned issue was removed")
	}
	if got.Status != types.StatusTombstone || got.DeletedAt == nil || got.DeletedBy != "actor" || got.DeleteReason != "cleanup" || got.OriginalType != string(types.TypeTask) {
		t.Fatalf("unexpected tombstone: %#v", got)
	}
	results, err := store.SearchIssues(ctx, "", types.IssueFilter{})
	if err != nil {
		t.Fatalf("SearchIssues returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("tombstone should be hidden from default search: %#v", results)
	}
}

func TestImportIssuePreservesRelations(t *testing.T) {
	ctx := context.Background()
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	store, err := Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	issue := &types.Issue{
		ID:        "tk-import",
		Title:     "Imported issue",
		IssueType: types.TypeTask,
		Status:    types.StatusOpen,
		Priority:  2,
		Labels:    []string{"imported"},
		Dependencies: []*types.Dependency{
			{IssueID: "tk-import", DependsOnID: "tk-parent", Type: types.DepBlocks},
		},
		Comments: []*types.Comment{
			{ID: 1, IssueID: "tk-import", Author: "agent", Text: "imported comment"},
		},
	}
	if err := store.ImportIssue(ctx, issue, "test"); err != nil {
		t.Fatalf("ImportIssue returned error: %v", err)
	}

	got, err := store.GetIssue(ctx, "tk-import")
	if err != nil {
		t.Fatalf("GetIssue returned error: %v", err)
	}
	if len(got.Labels) != 1 || got.Labels[0] != "imported" {
		t.Fatalf("labels = %#v, want imported", got.Labels)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0].DependsOnID != "tk-parent" {
		t.Fatalf("dependencies = %#v, want tk-parent", got.Dependencies)
	}
	comments, err := store.GetIssueComments(ctx, "tk-import")
	if err != nil {
		t.Fatalf("GetIssueComments returned error: %v", err)
	}
	if len(comments) != 1 || comments[0].Text != "imported comment" {
		t.Fatalf("comments = %#v, want imported comment", comments)
	}
}

func BenchmarkFileStorageOpen1000Issues(b *testing.B) {
	taskLedgerDir := seedFileStorageBenchmarkRepo(b, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store, err := Open(taskLedgerDir)
		if err != nil {
			b.Fatalf("Open: %v", err)
		}
		_ = store.Close()
	}
}

func BenchmarkFileStorageList1000Issues(b *testing.B) {
	taskLedgerDir := seedFileStorageBenchmarkRepo(b, 1000)
	store, err := Open(taskLedgerDir)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	b.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.SearchIssues(ctx, "", types.IssueFilter{}); err != nil {
			b.Fatalf("SearchIssues: %v", err)
		}
	}
}

func BenchmarkFileStorageSearch1000Issues(b *testing.B) {
	taskLedgerDir := seedFileStorageBenchmarkRepo(b, 1000)
	store, err := Open(taskLedgerDir)
	if err != nil {
		b.Fatalf("Open: %v", err)
	}
	b.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.SearchIssues(ctx, "Issue 0999", types.IssueFilter{}); err != nil {
			b.Fatalf("SearchIssues: %v", err)
		}
	}
}

func seedFileStorageBenchmarkRepo(tb testing.TB, count int) string {
	tb.Helper()
	taskLedgerDir := filepath.Join(tb.TempDir(), ".task-ledger")
	now := time.Now().UTC()
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("tk-%04d", i)
		writeIssueFixture(tb, taskLedgerDir, filepath.Join("issues", id, "issue.json"), FileIssue{
			SchemaVersion: SchemaVersion,
			ID:            id,
			Type:          string(types.TypeTask),
			Title:         fmt.Sprintf("Issue %04d", i),
			Status:        string(types.StatusOpen),
			Priority:      i % 5,
			Deps:          []string{},
			Labels:        []string{},
			Comments:      []*types.Comment{},
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	return taskLedgerDir
}

func writeIssueFixture(t testing.TB, taskLedgerDir, rel string, issue FileIssue) {
	t.Helper()
	path := filepath.Join(taskLedgerDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	data, err := MarshalFileIssue(issue)
	if err != nil {
		t.Fatalf("MarshalFileIssue: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}
