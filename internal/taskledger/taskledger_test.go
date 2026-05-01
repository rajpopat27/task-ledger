package taskledger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"task-ledger/internal/types"
)

func withCleanDiscoveryEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TASK_LEDGER_DIR", "")
}

func makeProject(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	taskLedgerDir := filepath.Join(root, ".task-ledger")
	if err := os.MkdirAll(filepath.Join(taskLedgerDir, "issues"), 0o755); err != nil {
		t.Fatalf("create issues dir: %v", err)
	}
	return root, taskLedgerDir
}

func TestFindTaskLedgerDirFindsIssueProject(t *testing.T) {
	withCleanDiscoveryEnv(t)
	root, taskLedgerDir := makeProject(t)
	nested := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	t.Chdir(nested)

	if got := FindTaskLedgerDir(); got != taskLedgerDir {
		t.Fatalf("FindTaskLedgerDir() = %q, want %q", got, taskLedgerDir)
	}
}

func TestFindTaskLedgerDirRejectsUnsupportedMarkerOnlyProjects(t *testing.T) {
	tests := []struct {
		name string
		file string
	}{
		{name: "unsupported local database marker only", file: "unsupported.db"},
		{name: "jsonl only", file: "issues.jsonl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withCleanDiscoveryEnv(t)
			root := t.TempDir()
			taskLedgerDir := filepath.Join(root, ".task-ledger")
			if err := os.MkdirAll(taskLedgerDir, 0o755); err != nil {
				t.Fatalf("create .task-ledger dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(taskLedgerDir, tt.file), []byte("{}"), 0o644); err != nil {
				t.Fatalf("write unsupported project marker: %v", err)
			}
			t.Chdir(root)

			if got := FindTaskLedgerDir(); got != "" {
				t.Fatalf("FindTaskLedgerDir() = %q, want empty", got)
			}
		})
	}
}

func TestFindTaskLedgerDirValidatesTaskLedgerDirEnv(t *testing.T) {
	withCleanDiscoveryEnv(t)
	_, taskLedgerDir := makeProject(t)
	t.Setenv("TASK_LEDGER_DIR", taskLedgerDir)

	if got := FindTaskLedgerDir(); got != taskLedgerDir {
		t.Fatalf("FindTaskLedgerDir() = %q, want %q", got, taskLedgerDir)
	}

	unsupportedRoot := t.TempDir()
	unsupportedDir := filepath.Join(unsupportedRoot, ".task-ledger")
	if err := os.MkdirAll(unsupportedDir, 0o755); err != nil {
		t.Fatalf("create unsupported .task-ledger dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unsupportedDir, "unsupported.db"), nil, 0o644); err != nil {
		t.Fatalf("write unsupported project marker: %v", err)
	}
	t.Setenv("TASK_LEDGER_DIR", unsupportedDir)

	if got := FindTaskLedgerDir(); got != "" {
		t.Fatalf("FindTaskLedgerDir() with unsupported TASK_LEDGER_DIR = %q, want empty", got)
	}
}

func TestFindTaskLedgerDirWithRedirect(t *testing.T) {
	withCleanDiscoveryEnv(t)
	localRoot := t.TempDir()
	localTaskLedger := filepath.Join(localRoot, ".task-ledger")
	if err := os.MkdirAll(localTaskLedger, 0o755); err != nil {
		t.Fatalf("create local .task-ledger: %v", err)
	}

	_, targetTaskLedger := makeProject(t)
	if err := os.WriteFile(filepath.Join(localTaskLedger, RedirectFileName), []byte(targetTaskLedger), 0o644); err != nil {
		t.Fatalf("write redirect: %v", err)
	}
	t.Chdir(localRoot)

	if got := FindTaskLedgerDir(); got != targetTaskLedger {
		t.Fatalf("FindTaskLedgerDir() = %q, want redirect target %q", got, targetTaskLedger)
	}
}

func TestFindAllProjectsReturnsClosestProjectWithIssueCount(t *testing.T) {
	withCleanDiscoveryEnv(t)
	root, taskLedgerDir := makeProject(t)
	store, err := OpenStorage(taskLedgerDir)
	if err != nil {
		t.Fatalf("OpenStorage failed: %v", err)
	}
	issue := &types.Issue{Title: "One", IssueType: types.TypeTask, Status: types.StatusOpen}
	if err := store.CreateIssue(context.Background(), issue, "test"); err != nil {
		t.Fatalf("CreateIssue failed: %v", err)
	}
	_ = store.Close()

	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	t.Chdir(nested)

	projects := FindAllProjects()
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
