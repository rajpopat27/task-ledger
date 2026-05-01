package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	filestorage "task-ledger/internal/storage/file"
	"task-ledger/internal/types"
)

func resetCommandStateForTest(t *testing.T) {
	t.Helper()

	resetCommandFlags(rootCmd)
	actor = ""
	jsonOutput = false
	readonlyMode = false
	verboseFlag = false
	quietFlag = false
	rootCtx = context.Background()
	rootCancel = nil
	hookRunner = nil

	if store != nil {
		_ = store.Close()
	}
	store = nil
	storeActive = false

	t.Setenv("TL_ACTOR", "coverage-tester")
	t.Setenv("TASK_LEDGER_ACTOR", "")
	t.Setenv("TASK_LEDGER_DIR", "")
}

func resetCommandFlags(cmd *cobra.Command) {
	resetFlagSet := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if sliceValue, ok := flag.Value.(pflag.SliceValue); ok && flag.DefValue == "[]" {
				_ = sliceValue.Replace([]string{})
			} else {
				_ = flag.Value.Set(flag.DefValue)
			}
			flag.Changed = false
		})
	}
	resetFlagSet(cmd.Flags())
	resetFlagSet(cmd.PersistentFlags())
	for _, child := range cmd.Commands() {
		resetCommandFlags(child)
	}
}

func runTLInProcess(t *testing.T, repo string, args ...string) string {
	t.Helper()
	resetCommandStateForTest(t)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	return captureStdout(t, func() {
		rootCmd.SetArgs(args)
		rootCmd.SetOut(os.Stdout)
		rootCmd.SetErr(os.Stderr)
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("tl %s failed: %v", strings.Join(args, " "), err)
		}
	})
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	restored := false
	defer func() {
		if !restored {
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			_ = stdoutW.Close()
			_ = stderrW.Close()
		}
	}()
	fn()
	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	restored = true

	out, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.ReadAll(stderrR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestFileBackendInProcessRetainedCommandCoverage(t *testing.T) {
	repo := t.TempDir()

	runTLInProcess(t, repo, "init", "--quiet")
	fr := runTLInProcess(t, repo, "create", "FR", "Coverage feature", "--description", "Feature body", "--silent")
	epic := runTLInProcess(t, repo, "create", "epic", "Coverage epic", "--parent", fr, "--silent")
	task := runTLInProcess(t, repo, "create", "task", "Coverage task", "--parent", epic, "--description", "Task body", "--silent")
	blocker := runTLInProcess(t, repo, "create", "bug", "Coverage blocker", "--parent", epic, "--silent")

	for _, id := range []string{fr, epic, task, blocker} {
		if id == "" || strings.Contains(id, "\n") {
			t.Fatalf("expected a single issue id, got %q", id)
		}
	}

	runTLInProcess(t, repo, "list", "--all")
	runTLInProcess(t, repo, "show", task)
	runTLInProcess(t, repo, "update", task, "--status", "in_progress", "--priority", "1", "--assignee", "tester", "--notes", "covered")
	runTLInProcess(t, repo, "label", "add", task, "covered")
	runTLInProcess(t, repo, "label", "list", task)
	runTLInProcess(t, repo, "comments", "add", task, "covered comment")
	runTLInProcess(t, repo, "comments", task)
	runTLInProcess(t, repo, "dep", "add", task, blocker)
	runTLInProcess(t, repo, "blocked")
	runTLInProcess(t, repo, "dep", "list", task)
	runTLInProcess(t, repo, "dep", "tree", task)
	runTLInProcess(t, repo, "dep", "cycles")
	runTLInProcess(t, repo, "ready", "--include-deferred")
	runTLInProcess(t, repo, "count", "--by-status")
	runTLInProcess(t, repo, "search", "Coverage")
	runTLInProcess(t, repo, "close", blocker, "--reason", "fixed")
	runTLInProcess(t, repo, "reopen", blocker, "--reason", "more work")
	runTLInProcess(t, repo, "epic", "status")
	runTLInProcess(t, repo, "epic", "close-eligible", "--dry-run")
	runTLInProcess(t, repo, "delete", blocker, "--force")
	runTLInProcess(t, repo, "doctor")
	runTLInProcess(t, repo, "version")
}

func TestFileBackendInProcessProductionReadinessCoverage(t *testing.T) {
	repo := t.TempDir()

	runTLInProcess(t, repo, "init", "--quiet")
	createOut := runTLInProcess(t, repo, "--json", "create", "task", "In-process JSON", "--description", "body", "--priority", "P1")
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("create json: %v\n%s", err, createOut)
	}
	id := created["id"].(string)

	runTLInProcess(t, repo, "--json", "show", id)
	runTLInProcess(t, repo, "--json", "list", "--all", "--priority-min", "P0", "--priority-max", "P2")
	runTLInProcess(t, repo, "--json", "search", "body", "--priority-min", "P0", "--priority-max", "P2")
	runTLInProcess(t, repo, "--json", "count", "--priority-min", "P0", "--priority-max", "P2")
	runTLInProcess(t, repo, "update", id, "--type", "FR", "--external-ref", "EXT-INPROC", "--notes", "note")
	runTLInProcess(t, repo, "--json", "close", id, "--reason", "done")
	runTLInProcess(t, repo, "--json", "reopen", id, "--reason", "again")
	runTLInProcess(t, repo, "--json", "delete", id, "--force", "--reason", "cleanup")
	runTLInProcess(t, repo, "--json", "doctor")

	legacyPath := filepath.Join(repo, "legacy.jsonl")
	legacy := `{"id":"tk-legacy-inproc","title":"Legacy","issue_type":"task","status":"open","priority":2,"external_ref":"EXT-LEGACY"}` + "\n"
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	runTLInProcess(t, repo, "import-jsonl", legacyPath)
}

func TestImportJSONLJSONLValidationCoverage(t *testing.T) {
	ctx := context.Background()
	actor := "coverage"

	runCase := func(name, content, want string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
			if err := initFileBackend(taskLedgerDir); err != nil {
				t.Fatalf("init backend: %v", err)
			}
			st, err := filestorage.Open(taskLedgerDir)
			if err != nil {
				t.Fatalf("open storage: %v", err)
			}
			t.Cleanup(func() { _ = st.Close() })
			path := filepath.Join(t.TempDir(), "legacy.jsonl")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("write legacy: %v", err)
			}
			if _, err := importJSONL(ctx, st, path, actor); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("importJSONL error = %v, want %q", err, want)
			}
			files, err := filestorage.FindIssueFiles(taskLedgerDir)
			if err != nil {
				t.Fatalf("FindIssueFiles: %v", err)
			}
			if len(files) != 0 {
				t.Fatalf("validation failure wrote files: %v", files)
			}
		})
	}

	runCase("malformed", `{"id":"tk-good","title":"Good","issue_type":"task","status":"open","priority":2}`+"\nnot json\n", "line 2")
	runCase("duplicate id", `{"id":"tk-dupe","title":"A","issue_type":"task","status":"open","priority":2}`+"\n"+`{"id":"tk-dupe","title":"B","issue_type":"task","status":"open","priority":2}`+"\n", "duplicate ID")
	runCase("duplicate external ref", `{"id":"tk-a","title":"A","issue_type":"task","status":"open","priority":2,"external_ref":"EXT"}`+"\n"+`{"id":"tk-b","title":"B","issue_type":"task","status":"open","priority":2,"external_ref":"EXT"}`+"\n", "external_ref")
	runCase("unsafe id", `{"id":"../bad","title":"Bad","issue_type":"task","status":"open","priority":2}`+"\n", "unsafe issue id")
}

func TestFileBackendListFormattingCoverage(t *testing.T) {
	now := time.Now()
	closedAt := now.Add(-time.Hour)
	parent := &types.Issue{ID: "ep-parent", Title: "Parent", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeEpic, CreatedAt: now, UpdatedAt: now}
	child := &types.Issue{ID: "tk-child", Title: "Child", Status: types.StatusOpen, Priority: 0, IssueType: types.TypeTask, Assignee: "alice", Pinned: true, CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)}
	closed := &types.Issue{ID: "tk-closed", Title: "Closed", Status: types.StatusClosed, Priority: 3, IssueType: types.TypeTask, ClosedAt: &closedAt, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	issues := []*types.Issue{closed, child, parent}

	for _, sortBy := range []string{"priority", "created", "updated", "closed", "status", "id", "title", "type", "assignee", "unknown"} {
		sortIssues(append([]*types.Issue(nil), issues...), sortBy, false)
		sortIssues(append([]*types.Issue(nil), issues...), sortBy, true)
	}
	if _, err := parseTimeFlag("2026-01-01"); err != nil {
		t.Fatalf("parseTimeFlag: %v", err)
	}
	if pinIndicator(child) == "" || renderPriorityTag(1) == "" {
		t.Fatal("expected pin and priority rendering")
	}

	deps := map[string][]*types.Dependency{
		child.ID: {
			{IssueID: child.ID, DependsOnID: parent.ID, Type: types.DepParentChild},
		},
	}
	roots, children := buildIssueTreeWithDeps([]*types.Issue{parent, child}, deps)
	if len(roots) != 1 || roots[0].ID != parent.ID || len(children[parent.ID]) != 1 {
		t.Fatalf("unexpected tree: roots=%#v children=%#v", roots, children)
	}

	var buf strings.Builder
	formatIssueLong(&buf, child, []string{"backend"})
	formatIssueCompact(&buf, closed, nil)
	formatAgentIssue(&buf, parent)
	if !strings.Contains(buf.String(), "tk-child") || !strings.Contains(buf.String(), "ep-parent") {
		t.Fatalf("format helpers missing issue IDs: %s", buf.String())
	}

	out := captureStdout(t, func() {
		displayPrettyListWithDeps([]*types.Issue{parent, child}, false, deps)
	})
	if !strings.Contains(out, "Parent") || !strings.Contains(out, "Child") {
		t.Fatalf("pretty output missing issues:\n%s", out)
	}

	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	if err := initFileBackend(taskLedgerDir); err != nil {
		t.Fatalf("init backend: %v", err)
	}
	st, err := filestorage.Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := context.Background()
	for _, issue := range []*types.Issue{
		{ID: "tk-a", Title: "A", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask},
		{ID: "tk-b", Title: "B", Status: types.StatusBlocked, Priority: 2, IssueType: types.TypeTask},
	} {
		if err := st.CreateIssue(ctx, issue, "coverage"); err != nil {
			t.Fatalf("create %s: %v", issue.ID, err)
		}
	}
	if err := st.AddDependency(ctx, &types.Dependency{IssueID: "tk-b", DependsOnID: "tk-a", Type: types.DepBlocks}, "coverage"); err != nil {
		t.Fatalf("add dependency: %v", err)
	}
	stored, err := st.SearchIssues(ctx, "", types.IssueFilter{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	captureStdout(t, func() {
		if err := outputDotFormat(ctx, st, stored); err != nil {
			t.Fatalf("outputDotFormat: %v", err)
		}
		if err := outputFormattedList(ctx, st, stored, "digraph"); err != nil {
			t.Fatalf("outputFormattedList preset: %v", err)
		}
		if err := outputFormattedList(ctx, st, stored, "{{.IssueID}} -> {{.DependsOnID}}"); err != nil {
			t.Fatalf("outputFormattedList template: %v", err)
		}
	})
	if err := outputFormattedList(ctx, st, stored, "{{"); err == nil {
		t.Fatal("expected invalid template error")
	}
}

func TestFileBackendCommandHelpers(t *testing.T) {
	if !isInsideTaskLedgerDir(filepath.Join(t.TempDir(), ".task-ledger", "issues")) {
		t.Fatal("expected path inside .task-ledger to be detected")
	}
	if isInsideTaskLedgerDir(filepath.Join(t.TempDir(), "work")) {
		t.Fatal("unexpected .task-ledger detection for regular path")
	}

	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	if err := initFileBackend(taskLedgerDir); err != nil {
		t.Fatalf("init file backend: %v", err)
	}
	for _, path := range []string{
		filepath.Join(taskLedgerDir, "issues"),
		filepath.Join(taskLedgerDir, "config.yaml"),
		filepath.Join(taskLedgerDir, "README.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}

	if !isFileBackendIssueType(types.TypeFeatureRequest) || isFileBackendIssueType("agent") {
		t.Fatal("file backend issue type filter returned an unexpected result")
	}
	if !strings.Contains(fileBackendIssueTypesHelp(), "feature_request") {
		t.Fatalf("issue type help missing feature_request: %s", fileBackendIssueTypesHelp())
	}
	if short := shortCommit("1234567890abcdef"); short != "1234567890ab" {
		t.Fatalf("short commit mismatch: %s", short)
	}
	if short := shortCommit("123"); short != "123" {
		t.Fatalf("short commit should preserve short hashes: %s", short)
	}

	Commit = "commit-from-test"
	Branch = "branch-from-test"
	t.Cleanup(func() {
		Commit = ""
		Branch = ""
	})
	if resolveCommitHash() != "commit-from-test" || resolveBranch() != "branch-from-test" {
		t.Fatal("ldflag version helpers did not prefer explicit values")
	}
}

func TestFileBackendDescriptionAndDeleteHelpers(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(bodyPath, []byte("file body"), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	if body, err := readBodyFile(bodyPath); err != nil || body != "file body" {
		t.Fatalf("read body file = %q, %v", body, err)
	}

	cmd := &cobra.Command{Use: "test"}
	registerCommonIssueFlags(cmd)
	if err := cmd.Flags().Set("description", "inline body"); err != nil {
		t.Fatalf("set description: %v", err)
	}
	if body, changed := getDescriptionFlag(cmd); body != "inline body" || !changed {
		t.Fatalf("description flag = %q, %v", body, changed)
	}

	cmd = &cobra.Command{Use: "test"}
	registerCommonIssueFlags(cmd)
	if err := cmd.Flags().Set("body-file", bodyPath); err != nil {
		t.Fatalf("set body-file: %v", err)
	}
	if body, changed := getDescriptionFlag(cmd); body != "file body" || !changed {
		t.Fatalf("body-file flag = %q, %v", body, changed)
	}

	idsPath := filepath.Join(t.TempDir(), "ids.txt")
	if err := os.WriteFile(idsPath, []byte("tk-1\n\n# ignored\ntk-2\n"), 0o600); err != nil {
		t.Fatalf("write ids file: %v", err)
	}
	ids, err := readIssueIDsFromFile(idsPath)
	if err != nil {
		t.Fatalf("read ids: %v", err)
	}
	if strings.Join(ids, ",") != "tk-1,tk-2" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if got := strings.Join(uniqueStrings([]string{"a", "b", "a"}), ","); got != "a,b" {
		t.Fatalf("unique strings mismatch: %s", got)
	}
}

func TestFileBackendStorageBackedHelpers(t *testing.T) {
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	if err := initFileBackend(taskLedgerDir); err != nil {
		t.Fatalf("init backend: %v", err)
	}
	st, err := filestorage.Open(taskLedgerDir)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	store = st
	storeActive = true
	actor = "coverage-tester"
	rootCtx = context.Background()
	t.Cleanup(func() {
		store = nil
		storeActive = false
	})

	ctx := context.Background()
	for _, issue := range []*types.Issue{
		{ID: "tk-main", Title: "main", Status: types.StatusOpen, Priority: 2, IssueType: types.TypeTask},
		{ID: "tk-blocker", Title: "blocker", Status: types.StatusOpen, Priority: 1, IssueType: types.TypeTask},
		{ID: "tk-related", Title: "mentions tk-main", Description: "depends on tk-main", Notes: "tk-main note", Design: "tk-main design", AcceptanceCriteria: "tk-main acceptance", Status: types.StatusOpen, Priority: 2, IssueType: types.TypeTask},
	} {
		if err := st.CreateIssue(ctx, issue, actor); err != nil {
			t.Fatalf("create %s: %v", issue.ID, err)
		}
	}
	if err := st.AddDependency(ctx, &types.Dependency{IssueID: "tk-main", DependsOnID: "tk-blocker", Type: types.DepBlocks}, actor); err != nil {
		t.Fatalf("add dependency: %v", err)
	}

	connected := collectConnectedIssues(ctx, []string{"tk-blocker"})
	if _, ok := connected["tk-main"]; !ok {
		t.Fatalf("expected tk-main as connected dependent: %#v", connected)
	}
	if removed := removeDependencyEdges(ctx, []string{"tk-blocker"}); removed != 1 {
		t.Fatalf("expected one dependency edge removed, got %d", removed)
	}
	if updated := updateTextReferencesInIssues(ctx, []string{"tk-main"}, map[string]*types.Issue{"tk-related": connectedIssue(t, st, ctx, "tk-related")}); updated != 1 {
		t.Fatalf("expected one text reference update, got %d", updated)
	}
	updated, err := st.GetIssue(ctx, "tk-related")
	if err != nil {
		t.Fatalf("get related: %v", err)
	}
	if !strings.Contains(updated.Description, "[deleted:tk-main]") {
		t.Fatalf("reference was not rewritten: %#v", updated)
	}

	if err := applyLabelUpdates(ctx, st, "tk-main", actor, []string{"set-a", "set-b"}, []string{"add-c"}, []string{"set-a"}); err != nil {
		t.Fatalf("apply label updates: %v", err)
	}
	labels, err := st.GetLabels(ctx, "tk-main")
	if err != nil {
		t.Fatalf("get labels: %v", err)
	}
	labelSet := make(map[string]bool, len(labels))
	for _, label := range labels {
		labelSet[label] = true
	}
	if len(labels) != 2 || !labelSet["set-b"] || !labelSet["add-c"] {
		t.Fatalf("unexpected labels: %v", labels)
	}
}

func connectedIssue(t *testing.T, st interface {
	GetIssue(context.Context, string) (*types.Issue, error)
}, ctx context.Context, id string) *types.Issue {
	t.Helper()
	issue, err := st.GetIssue(ctx, id)
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	return issue
}

func TestFileBackendDependencyHelpers(t *testing.T) {
	if !isChildOf("tl-root.1.2", "tl-root") || isChildOf("tl-root", "tl-root") {
		t.Fatal("hierarchical child detection mismatch")
	}
	if err := validateExternalRef("external:payments:auth"); err != nil {
		t.Fatalf("valid external ref rejected: %v", err)
	}
	if err := validateExternalRef("payments:auth"); err == nil {
		t.Fatal("invalid external ref accepted")
	}
	project, capability := ParseExternalRef("external:payments:auth")
	if project != "payments" || capability != "auth" || IsExternalRef("tk-1") {
		t.Fatalf("external ref parse mismatch: %s %s", project, capability)
	}

	tree := []*types.TreeNode{
		{Issue: types.Issue{ID: "tk-root", Title: "root", Status: types.StatusOpen, Priority: 1}, Depth: 0},
		{Issue: types.Issue{ID: "tk-blocker", Title: "blocker", Status: types.StatusBlocked, Priority: 2}, Depth: 1, ParentID: "tk-root"},
	}
	filtered := filterTreeByStatus(tree, types.StatusBlocked)
	if len(filtered) != 2 {
		t.Fatalf("expected blocked filter to retain ancestor and match, got %d", len(filtered))
	}
	if format := formatTreeNode(tree[0]); !strings.Contains(format, "READY") {
		t.Fatalf("root open node should be marked ready: %s", format)
	}
	if emoji := getStatusEmoji(types.StatusClosed); emoji == "?" {
		t.Fatal("closed status should have a display marker")
	}

	output := captureStdout(t, func() {
		outputMermaidTree(tree, "tk-root")
		renderTree(tree, 4, "down")
	})
	if !strings.Contains(output, "flowchart TD") || !strings.Contains(output, "tk-blocker") {
		t.Fatalf("tree output missing expected content:\n%s", output)
	}
	merged := mergeBidirectionalTrees(tree, []*types.TreeNode{{Issue: types.Issue{ID: "tk-dependent", Title: "dependent"}}}, "tk-root")
	if len(merged) != 3 {
		t.Fatalf("unexpected merged tree length: %d", len(merged))
	}
}

func TestFileBackendMainHelperState(t *testing.T) {
	resetCommandStateForTest(t)
	if !shouldSkipStoreInit(nil) || !shouldSkipStoreInit(rootCmd) || !shouldSkipStoreInit(initCmd) {
		t.Fatal("store initialization skip rules failed")
	}
	if shouldSkipStoreInit(createCmd) {
		t.Fatal("create should require store initialization")
	}
	taskLedgerDir := filepath.Join(t.TempDir(), ".task-ledger")
	if err := initFileBackend(taskLedgerDir); err != nil {
		t.Fatalf("init backend: %v", err)
	}
	t.Setenv("TASK_LEDGER_DIR", taskLedgerDir)
	if err := ensureStoreActive(); err != nil {
		t.Fatalf("ensure store active: %v", err)
	}
	if !storeActive {
		t.Fatal("store should be active after ensureStoreActive")
	}
}
