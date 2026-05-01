package taskledger_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	taskledger "task-ledger"
)

func TestOpenStorageAndProjectDiscovery(t *testing.T) {
	project := t.TempDir()
	taskLedgerDir := filepath.Join(project, ".task-ledger")
	if err := os.MkdirAll(filepath.Join(taskLedgerDir, "issues"), 0o755); err != nil {
		t.Fatalf("create issues dir: %v", err)
	}
	t.Chdir(project)

	t.Setenv("TASK_LEDGER_DIR", taskLedgerDir)
	store, err := taskledger.OpenStorage(taskLedgerDir)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	issue := &taskledger.Issue{
		Title:     "Public API issue",
		IssueType: taskledger.TypeTask,
		Status:    taskledger.StatusOpen,
	}
	if err := store.CreateIssue(context.Background(), issue, "test"); err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}

	if got := taskledger.FindTaskLedgerDir(); got != taskLedgerDir {
		t.Fatalf("FindTaskLedgerDir() = %q, want %q", got, taskLedgerDir)
	}

	projects := taskledger.FindAllProjects()
	if len(projects) != 1 {
		t.Fatalf("FindAllProjects() returned %d projects, want 1", len(projects))
	}
	if projects[0].Path != taskLedgerDir {
		t.Fatalf("project path = %q, want %q", projects[0].Path, taskLedgerDir)
	}
	if projects[0].IssueCount != 1 {
		t.Fatalf("IssueCount = %d, want 1", projects[0].IssueCount)
	}
}

func TestConstants(t *testing.T) {
	if taskledger.StatusOpen != "open" {
		t.Errorf("StatusOpen = %q, want open", taskledger.StatusOpen)
	}
	if taskledger.TypeTask != "task" {
		t.Errorf("TypeTask = %q, want task", taskledger.TypeTask)
	}
	if taskledger.DepBlocks != "blocks" {
		t.Errorf("DepBlocks = %q, want blocks", taskledger.DepBlocks)
	}
}
