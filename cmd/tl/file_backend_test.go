package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func buildFileBackendBinary(t testing.TB) string {
	t.Helper()
	tmpBin := filepath.Join(t.TempDir(), "tl")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", tmpBin, ".")
	build.Env = append(os.Environ(),
		"GOCACHE=/tmp/taskledger-gocache",
		"GOMODCACHE=/tmp/taskledger-gomodcache",
	)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return tmpBin
}

func runFileBackendTL(t testing.TB, bin, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tl %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func runFileBackendTLError(t testing.TB, bin, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("tl %s unexpectedly succeeded:\n%s", strings.Join(args, " "), out)
	}
	return strings.TrimSpace(string(out))
}

func TestFileBackendCLIHierarchy(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)

	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	fr := run("create", "FR", "Checkout revamp", "--silent")
	epic := run("create", "epic", "Payment form", "--parent", fr, "--silent")
	task := run("create", "task", "Add card field", "--parent", epic, "--silent")
	blocker := run("create", "task", "Prerequisite", "--parent", epic, "--silent")
	run("dep", "add", task, blocker)

	paths := []string{
		filepath.Join(repo, ".task-ledger", "issues", fr, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", task, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", blocker, "issue.json"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	blocked := run("blocked")
	if !strings.Contains(blocked, task) || !strings.Contains(blocked, blocker) {
		t.Fatalf("blocked output did not include task and blocker:\n%s", blocked)
	}
}

func TestFileBackendRetainedCommandMatrix(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	run("version")

	for _, unsupported := range []string{"daemon", "sync", "migrate", "export", "formula", "swarm", "gate", "jira", "linear", "config"} {
		out := runFileBackendTLError(t, tmpBin, repo, unsupported)
		if !strings.Contains(out, "unknown command") {
			t.Fatalf("tl %s should be pruned, got:\n%s", unsupported, out)
		}
	}

	fr := run("create", "FR", "Checkout revamp", "--description", "Top level feature", "--silent")
	epic := run("create", "epic", "Payment form", "--parent", fr, "--silent")
	task := run("create", "task", "Add card field", "--parent", epic, "--labels", "backend", "--estimate", "45", "--due", "+24h", "--silent")
	blocker := run("create", "bug", "Fix token endpoint", "--parent", epic, "--silent")
	chore := run("create", "chore", "Update runbook", "--parent", epic, "--silent")
	for id, prefix := range map[string]string{fr: "fr-", epic: "ep-", task: "tk-", blocker: "bug-", chore: "ch-"} {
		if !strings.HasPrefix(id, prefix) {
			t.Fatalf("expected %s to have prefix %s", id, prefix)
		}
	}

	expectedPaths := []string{
		filepath.Join(repo, ".task-ledger", "issues", fr, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", task, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", blocker, "issue.json"),
		filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", chore, "issue.json"),
	}
	for _, path := range expectedPaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	taskJSONPath := filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", task, "issue.json")
	var taskFile map[string]interface{}
	readIssueJSON(t, taskJSONPath, &taskFile)
	if taskFile["feature_request"] != fr || taskFile["epic"] != epic {
		t.Fatalf("task issue.json missing hierarchy fields: %#v", taskFile)
	}
	if _, ok := taskFile["estimated_minutes"]; !ok {
		t.Fatalf("task issue.json missing estimated_minutes: %#v", taskFile)
	}
	if _, ok := taskFile["due_at"]; !ok {
		t.Fatalf("task issue.json missing due_at: %#v", taskFile)
	}

	run("list", "--all")
	run("list", "--all", "--json")
	run("list", "--all", "--pretty")
	run("list", "--parent", epic, "--all")
	run("show", task)
	run("show", "--short", task)
	run("show", "--json", task)

	run("update", task, "--status", "in_progress", "--priority", "1", "--assignee", "raj", "--notes", "implementation note", "--defer", "+1h")
	readIssueJSON(t, taskJSONPath, &taskFile)
	if taskFile["status"] != "in_progress" || taskFile["assignee"] != "raj" {
		t.Fatalf("update did not persist status/assignee: %#v", taskFile)
	}
	if _, ok := taskFile["defer_until"]; !ok {
		t.Fatalf("update did not persist defer_until: %#v", taskFile)
	}

	run("label", "list", task)
	run("label", "add", task, "urgent")
	if labels := run("label", "list", task); !strings.Contains(labels, "urgent") {
		t.Fatalf("label list missing urgent:\n%s", labels)
	}
	if labels := run("label", "list-all"); !strings.Contains(labels, "urgent") {
		t.Fatalf("label list-all missing urgent:\n%s", labels)
	}
	run("label", "remove", task, "urgent")

	run("comments", "add", task, "first comment")
	run("comment", task, "alias comment")
	if comments := run("comments", task); !strings.Contains(comments, "first comment") || !strings.Contains(comments, "alias comment") {
		t.Fatalf("comments output missing added comments:\n%s", comments)
	}
	readIssueJSON(t, taskJSONPath, &taskFile)
	comments, ok := taskFile["comments"].([]interface{})
	if !ok || len(comments) != 2 {
		t.Fatalf("comments were not persisted to issue.json: %#v", taskFile["comments"])
	}

	run("dep", "add", task, blocker)
	run("dep", "list", task)
	run("dep", "tree", task)
	run("dep", "cycles")
	if blocked := run("blocked"); !strings.Contains(blocked, task) || !strings.Contains(blocked, blocker) {
		t.Fatalf("blocked output missing dependency:\n%s", blocked)
	}
	run("dep", "remove", task, blocker)
	run("ready")
	run("ready", "--status", "open")

	run("close", blocker, "--reason", "fixed")
	run("reopen", blocker, "--reason", "needs another pass")
	run("count")
	run("count", "--by-status")
	run("search", "card")
	run("epic", "status")
	run("epic", "close-eligible", "--dry-run")
	run("doctor")
	run("doctor", "--json")

	run("delete", blocker, "--force")
	var deletedFile map[string]interface{}
	readIssueJSON(t, filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", blocker, "issue.json"), &deletedFile)
	if deletedFile["status"] != "tombstone" {
		t.Fatalf("deleted issue should be tombstoned, got: %#v", deletedFile)
	}

	legacyPath := filepath.Join(repo, "legacy.jsonl")
	legacyLine := `{"id":"legacy-1","title":"Imported legacy task","issue_type":"task","status":"open","priority":2,"labels":["imported"],"comments":[{"id":1,"issue_id":"legacy-1","author":"tester","text":"legacy comment","created_at":"2025-01-01T00:00:00Z"}]}` + "\n"
	if err := os.WriteFile(legacyPath, []byte(legacyLine), 0o644); err != nil {
		t.Fatalf("write legacy JSONL: %v", err)
	}
	run("import-jsonl", legacyPath)
	if show := run("show", "legacy-1"); !strings.Contains(show, "Imported legacy task") {
		t.Fatalf("imported issue not shown:\n%s", show)
	}
}

func runFileBackendTLWithInput(t testing.TB, bin, repo, input string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("tl %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func requireOutputContains(t testing.TB, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("expected output to contain %q:\n%s", want, output)
	}
}

func requireOutputNotContains(t testing.TB, output, unwanted string) {
	t.Helper()
	if strings.Contains(output, unwanted) {
		t.Fatalf("expected output not to contain %q:\n%s", unwanted, output)
	}
}

func TestFileBackendExhaustiveRetainedFlagQA(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}
	runInput := func(input string, args ...string) string {
		t.Helper()
		return runFileBackendTLWithInput(t, tmpBin, repo, input, args...)
	}
	runErr := func(want string, args ...string) string {
		t.Helper()
		out := runFileBackendTLError(t, tmpBin, repo, args...)
		requireOutputContains(t, out, want)
		return out
	}

	run("init", "--quiet")
	if out := run("init", "--force", "--quiet"); out != "" {
		t.Fatalf("init --force --quiet should be silent, got %q", out)
	}

	help := run("help")
	for _, unwanted := range []string{"\n  daemon", "\n  sync", "\n  migrate", "\n  export", "\n  config", "--no-daemon", "--db", "--profile", "--lock-timeout"} {
		requireOutputNotContains(t, help, unwanted)
	}
	createHelp := run("create", "--help")
	for _, unwanted := range []string{"--file", "--id", "--rig", "--event-category", "--mol-type", "--waits-for-gate"} {
		requireOutputNotContains(t, createHelp, unwanted)
	}
	initHelp := run("init", "--help")
	for _, unwanted := range []string{"--prefix", "--branch", "--skip-merge-driver", "--from-jsonl"} {
		requireOutputNotContains(t, initHelp, unwanted)
	}

	for _, args := range [][]string{
		{"init", "--prefix", "qa"},
		{"init", "--branch", "taskledger-sync"},
		{"init", "--contributor"},
		{"init", "--team"},
		{"init", "--stealth"},
		{"init", "--setup-exclude"},
		{"init", "--skip-hooks"},
		{"init", "--skip-merge-driver"},
		{"init", "--from-jsonl"},
	} {
		runErr("unknown flag", args...)
	}

	run("--verbose", "list", "--all")
	run("--actor", "qa", "create", "task", "Actor accepted", "--silent")
	run("--json", "list", "--all")
	runErr("read-only", "--readonly", "create", "task", "readonly blocked")

	fr := run("create", "--title", "QA feature", "--type", "feature_request", "--description", "FR body", "--priority", "P1", "--assignee", "lead", "--labels", "frlabel", "--silent")
	epic := run("create", "epic", "QA epic", "--parent", fr, "--silent")
	otherEpic := run("create", "epic", "Other epic", "--parent", fr, "--silent")
	blocker := run("create", "bug", "Blocking bug", "--parent", epic, "--silent")

	descPath := filepath.Join(repo, "desc.md")
	bodyPath := filepath.Join(repo, "body.md")
	updateDescPath := filepath.Join(repo, "update-desc.md")
	commentPath := filepath.Join(repo, "comment.md")
	if err := os.WriteFile(descPath, []byte("description from file"), 0o644); err != nil {
		t.Fatalf("write desc file: %v", err)
	}
	if err := os.WriteFile(bodyPath, []byte("body file description"), 0o644); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	if err := os.WriteFile(updateDescPath, []byte("updated description from file"), 0o644); err != nil {
		t.Fatalf("write update desc file: %v", err)
	}
	if err := os.WriteFile(commentPath, []byte("comment from file"), 0o644); err != nil {
		t.Fatalf("write comment file: %v", err)
	}

	task := run("create", "task", "Card validation", "--parent", epic, "--priority", "P0", "--assignee", "alice", "--description-file", descPath, "--design", "design text", "--acceptance", "accept text", "--notes", "notes text", "--external-ref", "EXT-1", "--labels", "backend,urgent", "--estimate", "30", "--due", "+2h", "--defer", "+1h", "--silent")
	bodyTask := run("create", "--title", "Body file task", "--body-file", bodyPath, "--type", "task", "--parent", epic, "--silent")
	stdinTask := runInput("stdin description", "create", "task", "Stdin task", "--parent", epic, "--description", "-", "--silent")
	depAtCreate := run("create", "task", "Dependency at create", "--parent", epic, "--deps", blocker, "--silent")
	emptyTask := run("create", "task", "Empty description task", "--parent", epic, "--silent")
	overdueTask := run("create", "task", "Overdue task", "--parent", epic, "--due", "2020-01-01", "--silent")
	closedTask := run("create", "task", "Closed task", "--parent", epic, "--silent")
	run("close", closedTask, "--reason", "done", "--session", "session-1")

	dryRun := run("create", "task", "Dry run task", "--dry-run")
	requireOutputContains(t, dryRun, "Would create issue")
	runErr("parent issue missing-parent not found", "create", "task", "Bad parent", "--parent", "missing-parent")
	runErr("unsupported issue type", "create", "--type", "agent", "Agent unsupported")

	for _, args := range [][]string{
		{"create", "task", "unsupported id", "--id", "manual-1"},
		{"create", "--file", descPath},
		{"create", "task", "unsupported waits", "--waits-for", blocker},
		{"create", "task", "unsupported gate", "--waits-for-gate", "any-children"},
		{"create", "task", "unsupported force", "--force"},
		{"create", "task", "unsupported repo", "--repo", repo},
		{"create", "task", "unsupported rig", "--rig", "other"},
		{"create", "task", "unsupported prefix", "--prefix", "other"},
		{"create", "task", "unsupported ephemeral", "--ephemeral"},
		{"create", "task", "unsupported mol", "--mol-type", "work"},
		{"create", "task", "unsupported role", "--role-type", "crew"},
		{"create", "task", "unsupported agent rig", "--agent-rig", "core"},
		{"create", "task", "unsupported event category", "--event-category", "qa.started"},
		{"create", "task", "unsupported event actor", "--event-actor", "human:qa"},
		{"create", "task", "unsupported event target", "--event-target", task},
		{"create", "task", "unsupported event payload", "--event-payload", "{}"},
	} {
		runErr("unknown flag", args...)
	}

	taskPath := filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", task, "issue.json")
	var issueFile map[string]interface{}
	readIssueJSON(t, taskPath, &issueFile)
	for key, want := range map[string]string{
		"title":               "Card validation",
		"description":         "description from file",
		"design":              "design text",
		"acceptance_criteria": "accept text",
		"notes":               "notes text",
		"external_ref":        "EXT-1",
		"assignee":            "alice",
		"feature_request":     fr,
		"epic":                epic,
	} {
		if issueFile[key] != want {
			t.Fatalf("expected %s=%q in issue file, got %#v", key, want, issueFile[key])
		}
	}
	for _, key := range []string{"estimated_minutes", "due_at", "defer_until"} {
		if _, ok := issueFile[key]; !ok {
			t.Fatalf("expected %s in issue file: %#v", key, issueFile)
		}
	}

	for _, args := range [][]string{
		{"list", "--all", "--status", "open"},
		{"list", "--all", "--status", "closed"},
		{"list", "--all", "--priority", "P0"},
		{"list", "--all", "--assignee", "alice"},
		{"list", "--all", "--type", "task"},
		{"list", "--all", "--label", "backend"},
		{"list", "--all", "--label-any", "urgent"},
		{"list", "--all", "--title", "Card"},
		{"list", "--all", "--id", task + "," + bodyTask},
		{"list", "--all", "--limit", "1"},
		{"list", "--all", "--format", "dot"},
		{"list", "--all", "--long"},
		{"list", "--all", "--sort", "created", "--reverse"},
		{"list", "--all", "--title-contains", "Card"},
		{"list", "--all", "--desc-contains", "description"},
		{"list", "--all", "--notes-contains", "notes"},
		{"list", "--all", "--created-after", "2000-01-01"},
		{"list", "--all", "--created-before", "2999-01-01"},
		{"list", "--all", "--updated-after", "2000-01-01"},
		{"list", "--all", "--updated-before", "2999-01-01"},
		{"list", "--all", "--closed-after", "2000-01-01"},
		{"list", "--all", "--closed-before", "2999-01-01"},
		{"list", "--all", "--empty-description"},
		{"list", "--all", "--no-assignee"},
		{"list", "--all", "--no-labels"},
		{"list", "--all", "--priority-min", "0"},
		{"list", "--all", "--priority-max", "4"},
		{"list", "--all", "--no-pinned"},
		{"list", "--all", "--parent", epic},
		{"list", "--all", "--filter-parent", epic},
		{"list", "--all", "--deferred"},
		{"list", "--all", "--defer-after", "2000-01-01"},
		{"list", "--all", "--defer-before", "2999-01-01"},
		{"list", "--all", "--due-after", "2000-01-01"},
		{"list", "--all", "--due-before", "2999-01-01"},
		{"list", "--all", "--overdue"},
		{"list", "--all", "--pretty"},
		{"list", "--all", "--tree"},
		{"list", "--all", "--no-pager"},
		{"list", "--all", "--ready"},
	} {
		run(args...)
	}
	for _, args := range [][]string{
		{"list", "--include-gates"},
		{"list", "--include-templates"},
		{"list", "--mol-type", "work"},
		{"list", "--watch"},
	} {
		runErr("unknown flag", args...)
	}

	for _, args := range [][]string{
		{"search", "card"},
		{"search", "--query", "card", "--status", "open", "--assignee", "alice", "--type", "task", "--label", "backend", "--label-any", "urgent", "--limit", "5", "--long", "--sort", "priority", "--reverse"},
		{"search", "card", "--created-after", "2000-01-01", "--created-before", "2999-01-01", "--updated-after", "2000-01-01", "--updated-before", "2999-01-01"},
		{"search", "closed", "--closed-after", "2000-01-01", "--closed-before", "2999-01-01", "--priority-min", "0", "--priority-max", "4"},
	} {
		run(args...)
	}

	for _, args := range [][]string{
		{"count"},
		{"count", "--status", "open"},
		{"count", "--priority", "0"},
		{"count", "--assignee", "alice"},
		{"count", "--type", "task"},
		{"count", "--label", "backend"},
		{"count", "--label-any", "urgent"},
		{"count", "--title", "Card"},
		{"count", "--id", task},
		{"count", "--title-contains", "Card"},
		{"count", "--desc-contains", "description"},
		{"count", "--notes-contains", "notes"},
		{"count", "--created-after", "2000-01-01"},
		{"count", "--created-before", "2999-01-01"},
		{"count", "--updated-after", "2000-01-01"},
		{"count", "--updated-before", "2999-01-01"},
		{"count", "--closed-after", "2000-01-01"},
		{"count", "--closed-before", "2999-01-01"},
		{"count", "--empty-description"},
		{"count", "--no-assignee"},
		{"count", "--no-labels"},
		{"count", "--priority-min", "0"},
		{"count", "--priority-max", "4"},
		{"count", "--by-status"},
		{"count", "--by-priority"},
		{"count", "--by-type"},
		{"count", "--by-assignee"},
		{"count", "--by-label"},
	} {
		run(args...)
	}
	runErr("only one --by-*", "count", "--by-status", "--by-type")

	run("show", task, bodyTask)
	run("show", "--short", task)
	run("show", "--json", task)
	runErr("no issue found", "show", "missing-id")
	runErr("unknown flag", "show", "--thread", task)
	runErr("unknown flag", "show", "--refs", task)

	run("update", task, "--title", "Updated card validation", "--type", "chore", "--description-file", updateDescPath, "--design", "new design", "--acceptance", "new accept", "--external-ref", "EXT-2", "--estimate", "90", "--due", "+3h", "--defer", "+30m")
	run("update", task, "--add-label", "added", "--remove-label", "urgent")
	run("update", task, "--set-labels", "final,qa")
	run("update", task, "--parent", otherEpic)
	newTaskPath := filepath.Join(repo, ".task-ledger", "issues", fr, "epics", otherEpic, "tasks", task, "issue.json")
	if _, err := os.Stat(newTaskPath); err != nil {
		t.Fatalf("expected reparented issue path %s: %v", newTaskPath, err)
	}
	run("update", task, "--due", "", "--defer", "")
	claimTask := run("create", "task", "Claim task", "--parent", otherEpic, "--silent")
	run("--actor", "qa", "update", claimTask, "--claim")
	runErr("already claimed", "--actor", "other", "update", claimTask, "--claim")
	runErr("unsupported issue type", "update", task, "--type", "gate")
	runErr("unknown flag", "update", task, "--await-id", "123")
	runErr("parent issue missing-parent not found", "update", task, "--parent", "missing-parent")

	blocker2 := run("create", "bug", "Second blocker", "--parent", epic, "--silent")
	related := run("create", "task", "Related task", "--parent", epic, "--silent")
	run("dep", blocker, "--blocks", bodyTask)
	run("dep", "add", stdinTask, "--blocked-by", blocker2)
	run("dep", "add", depAtCreate, "--depends-on", blocker2)
	run("dep", "add", bodyTask, related, "--type", "related")
	run("dep", "list", bodyTask, "--direction", "down")
	run("dep", "list", blocker, "--direction", "up", "--type", "blocks")
	run("dep", "tree", bodyTask, "--show-all-paths", "--max-depth", "5", "--direction", "both", "--status", "open")
	run("dep", "tree", bodyTask, "--format", "mermaid")
	run("dep", "tree", bodyTask, "--reverse")
	run("dep", "cycles")
	run("dep", "remove", bodyTask, blocker)

	run("label", "add", task, bodyTask, "batch")
	run("--json", "label", "list", task)
	run("--json", "label", "list-all")
	run("label", "remove", task, bodyTask, "batch")

	run("comments", "add", task, "--file", commentPath, "--author", "writer")
	run("comment", bodyTask, "--file", commentPath, "--author", "alias-writer")
	run("--json", "comments", task)
	runErr("no issue found", "comments", "missing-id")

	for _, args := range [][]string{
		{"ready", "--limit", "5"},
		{"ready", "--status", "open"},
		{"ready", "--priority", "0"},
		{"ready", "--assignee", "alice"},
		{"ready", "--unassigned"},
		{"ready", "--sort", "priority"},
		{"ready", "--label", "final"},
		{"ready", "--label-any", "qa"},
		{"ready", "--type", "chore"},
		{"ready", "--parent", otherEpic},
		{"ready", "--pretty"},
		{"ready", "--include-deferred"},
		{"blocked"},
		{"blocked", "--parent", epic},
	} {
		run(args...)
	}
	for _, args := range [][]string{
		{"ready", "--gated"},
		{"ready", "--mol", "mol-1"},
		{"ready", "--mol-type", "work"},
	} {
		runErr("unknown flag", args...)
	}

	suggestBlocker := run("create", "task", "Suggest blocker", "--parent", epic, "--silent")
	suggestBlocked := run("create", "task", "Suggest blocked", "--parent", epic, "--deps", suggestBlocker, "--silent")
	run("close", suggestBlocker, "--suggest-next")
	run("close", bodyTask, stdinTask, "--reason", "qa close", "--session", "session-2")
	run("reopen", bodyTask, stdinTask, "--reason", "qa reopen")
	run("close", bodyTask, "--resolution", "qa fixed")
	runErr("unknown flag", "close", bodyTask, "--continue")
	runErr("unknown flag", "close", bodyTask, "--no-auto")
	_ = suggestBlocked

	deleteDryRun := run("delete", emptyTask, "--dry-run")
	requireOutputContains(t, deleteDryRun, "DELETE PREVIEW")
	if _, err := os.Stat(filepath.Join(repo, ".task-ledger", "issues", fr, "epics", epic, "tasks", emptyTask, "issue.json")); err != nil {
		t.Fatalf("dry-run delete removed issue: %v", err)
	}
	deleteListPath := filepath.Join(repo, "delete-ids.txt")
	if err := os.WriteFile(deleteListPath, []byte(emptyTask+"\n"+overdueTask+"\n"), 0o644); err != nil {
		t.Fatalf("write delete file: %v", err)
	}
	run("delete", "--from-file", deleteListPath, "--force", "--reason", "qa cleanup")
	runErr("unknown flag", "delete", depAtCreate, "--cascade")
	runErr("unknown flag", "delete", depAtCreate, "--hard")

	run("epic", "status", "--eligible-only")
	run("epic", "close-eligible", "--dry-run")
	run("doctor")
	run("doctor", "--json")
	for _, args := range [][]string{
		{"doctor", "--fix"},
		{"doctor", "--yes"},
		{"doctor", "--interactive"},
		{"doctor", "--dry-run"},
		{"doctor", "--fix-child-parent"},
		{"doctor", "--force"},
		{"doctor", "--source", "jsonl"},
	} {
		runErr("unknown flag", args...)
	}

	legacyPath := filepath.Join(repo, "legacy-qa.jsonl")
	legacyLine := `{"id":"legacy-qa-1","title":"Legacy QA","issue_type":"task","status":"open","priority":2,"labels":["legacy"],"comments":[{"id":1,"issue_id":"legacy-qa-1","author":"qa","text":"legacy comment","created_at":"2025-01-01T00:00:00Z"}]}` + "\n"
	if err := os.WriteFile(legacyPath, []byte(legacyLine), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}
	run("import-jsonl", legacyPath)
	runErr("already exists", "import-jsonl", legacyPath)
	badLegacyPath := filepath.Join(repo, "bad-legacy.jsonl")
	if err := os.WriteFile(badLegacyPath, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("write bad legacy file: %v", err)
	}
	runErr("line 1", "import-jsonl", badLegacyPath)
	runErr("no such file", "import-jsonl", filepath.Join(repo, "missing.jsonl"))
}

func TestFileBackendDeleteReasonCreatesTombstone(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	id := run("create", "task", "Delete me", "--silent")
	out := run("--json", "delete", id, "--force", "--reason", "cleanup")
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("delete JSON did not parse: %v\n%s", err, out)
	}
	if result["deleted_count"].(float64) != 1 {
		t.Fatalf("deleted_count = %#v, want 1", result["deleted_count"])
	}

	var issue map[string]interface{}
	readIssueJSON(t, filepath.Join(repo, ".task-ledger", "issues", id, "issue.json"), &issue)
	for key, want := range map[string]string{
		"status":        "tombstone",
		"delete_reason": "cleanup",
		"original_type": "task",
	} {
		if issue[key] != want {
			t.Fatalf("%s = %#v, want %q in %#v", key, issue[key], want, issue)
		}
	}
	if issue["deleted_by"] == "" {
		t.Fatalf("deleted_by missing from tombstone: %#v", issue)
	}
	if _, ok := issue["deleted_at"]; !ok {
		t.Fatalf("deleted_at missing from tombstone: %#v", issue)
	}
}

func TestFileBackendPreservesAgentWFMetadata(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	id := run("create", "task", "Agent task", "--silent")
	issuePath := filepath.Join(repo, ".task-ledger", "issues", id, "issue.json")

	var issue map[string]interface{}
	readIssueJSON(t, issuePath, &issue)
	issue["agentwf"] = map[string]interface{}{
		"worker_agent_id": "coder",
		"review_agent_ids": []interface{}{
			"code-reviewer",
			"security-reviewer",
		},
		"custom": map[string]interface{}{
			"preserve": true,
		},
	}
	data, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		t.Fatalf("marshal issue with agentwf: %v", err)
	}
	if err := os.WriteFile(issuePath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write issue with agentwf: %v", err)
	}

	run("update", id, "--status", "in_review")

	readIssueJSON(t, issuePath, &issue)
	if issue["status"] != "in_review" {
		t.Fatalf("status = %#v, want in_review", issue["status"])
	}
	agentWF, ok := issue["agentwf"].(map[string]interface{})
	if !ok {
		t.Fatalf("agentwf missing after update: %#v", issue)
	}
	if agentWF["worker_agent_id"] != "coder" {
		t.Fatalf("worker_agent_id = %#v, want coder", agentWF["worker_agent_id"])
	}
	reviewers, ok := agentWF["review_agent_ids"].([]interface{})
	if !ok || len(reviewers) != 2 {
		t.Fatalf("review_agent_ids = %#v, want two reviewers", agentWF["review_agent_ids"])
	}
	custom, ok := agentWF["custom"].(map[string]interface{})
	if !ok || custom["preserve"] != true {
		t.Fatalf("custom metadata was not preserved: %#v", agentWF["custom"])
	}
}

func TestFileBackendReadyStatusFilter(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	openID := run("create", "task", "Open task", "--silent")
	inProgressID := run("create", "task", "In progress task", "--silent")
	run("update", inProgressID, "--status", "in_progress")

	out := run("--json", "ready", "--status", "open", "--limit", "50")
	var issues []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("ready JSON did not parse: %v\n%s", err, out)
	}

	seen := map[string]bool{}
	for _, issue := range issues {
		if id, ok := issue["id"].(string); ok {
			seen[id] = true
		}
	}
	if !seen[openID] {
		t.Fatalf("ready --status open did not include open issue %s: %#v", openID, issues)
	}
	if seen[inProgressID] {
		t.Fatalf("ready --status open included in_progress issue %s: %#v", inProgressID, issues)
	}
}

func TestFileBackendDeleteFromFileIsTransactional(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	a := run("create", "task", "Delete A", "--silent")
	b := run("create", "task", "Delete B", "--silent")
	c := run("create", "task", "Connected", "--description", fmt.Sprintf("mentions %s and %s", a, b), "--notes", fmt.Sprintf("note %s %s", a, b), "--silent")
	run("dep", "add", c, a)
	run("dep", "add", c, b)

	idsPath := filepath.Join(repo, "delete-ids.txt")
	if err := os.WriteFile(idsPath, []byte(a+"\n"+b+"\n"), 0o644); err != nil {
		t.Fatalf("write delete ids: %v", err)
	}
	out := run("--json", "delete", "--from-file", idsPath, "--force", "--reason", "cleanup")
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("delete JSON did not parse: %v\n%s", err, out)
	}
	if result["deleted_count"].(float64) != 2 || result["dependencies_removed"].(float64) != 2 {
		t.Fatalf("unexpected delete result: %#v", result)
	}

	show := run("--json", "show", c)
	var connected map[string]interface{}
	if err := json.Unmarshal([]byte(show), &connected); err != nil {
		t.Fatalf("show JSON did not parse: %v\n%s", err, show)
	}
	if deps, ok := connected["dependencies"].([]interface{}); ok && len(deps) != 0 {
		t.Fatalf("dependencies were not removed: %#v", connected["dependencies"])
	}
	desc := connected["description"].(string)
	notes := connected["notes"].(string)
	for _, id := range []string{a, b} {
		if !strings.Contains(desc, "[deleted:"+id+"]") || !strings.Contains(notes, "[deleted:"+id+"]") {
			t.Fatalf("text refs were not updated for %s: desc=%q notes=%q", id, desc, notes)
		}
	}
}

func TestFileBackendDuplicateExternalRefRejected(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	run("create", "task", "A", "--external-ref", "EXT-1", "--silent")
	out := runFileBackendTLError(t, tmpBin, repo, "create", "task", "B", "--external-ref", "EXT-1", "--silent")
	requireOutputContains(t, out, "external_ref EXT-1 already belongs to")
}

func TestImportJSONLRejectsMalformedJSONWithoutPartialImport(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	path := filepath.Join(repo, "legacy.jsonl")
	content := `{"id":"tk-good","title":"Good","issue_type":"task","status":"open","priority":2}` + "\nnot json\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	out := runFileBackendTLError(t, tmpBin, repo, "import-jsonl", path)
	requireOutputContains(t, out, "line 2")
	if _, err := os.Stat(filepath.Join(repo, ".task-ledger", "issues", "tk-good", "issue.json")); !os.IsNotExist(err) {
		t.Fatalf("malformed import wrote partial issue or stat failed unexpectedly: %v", err)
	}
}

func TestImportJSONLRejectsDuplicateIDWithoutPartialImport(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	path := filepath.Join(repo, "legacy.jsonl")
	line := `{"id":"tk-dupe","title":"Dupe","issue_type":"task","status":"open","priority":2}`
	if err := os.WriteFile(path, []byte(line+"\n"+line+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	out := runFileBackendTLError(t, tmpBin, repo, "import-jsonl", path)
	requireOutputContains(t, out, "line 2")
	if _, err := os.Stat(filepath.Join(repo, ".task-ledger", "issues", "tk-dupe", "issue.json")); !os.IsNotExist(err) {
		t.Fatalf("duplicate import wrote partial issue or stat failed unexpectedly: %v", err)
	}
}

func TestImportJSONLRejectsDuplicateExternalRefWithoutPartialImport(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	path := filepath.Join(repo, "legacy.jsonl")
	content := strings.Join([]string{
		`{"id":"tk-a","title":"A","issue_type":"task","status":"open","priority":2,"external_ref":"EXT-1"}`,
		`{"id":"tk-b","title":"B","issue_type":"task","status":"open","priority":2,"external_ref":"EXT-1"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}
	out := runFileBackendTLError(t, tmpBin, repo, "import-jsonl", path)
	requireOutputContains(t, out, "line 2")
	if _, err := os.Stat(filepath.Join(repo, ".task-ledger", "issues", "tk-a", "issue.json")); !os.IsNotExist(err) {
		t.Fatalf("duplicate external_ref import wrote partial issue or stat failed unexpectedly: %v", err)
	}
}

func TestImportJSONLRejectsUnsafeIDs(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	path := filepath.Join(repo, "unsafe.jsonl")
	content := `{"id":"tk/good","title":"Unsafe","issue_type":"task","status":"open","priority":2}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write unsafe legacy: %v", err)
	}
	out := runFileBackendTLError(t, tmpBin, repo, "import-jsonl", path)
	requireOutputContains(t, out, "line 1")
	requireOutputContains(t, out, "unsafe issue id")
	if _, err := os.Stat(filepath.Join(repo, ".task-ledger", "issues", "tk", "good", "issue.json")); !os.IsNotExist(err) {
		t.Fatalf("unsafe ID wrote nested file or stat failed unexpectedly: %v", err)
	}
}

func TestFileBackendCountPriorityRangeAcceptsPNotation(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	run("create", "task", "P0", "--priority", "P0", "--silent")
	run("create", "task", "P1", "--priority", "P1", "--silent")
	run("create", "task", "P3", "--priority", "P3", "--silent")
	out := run("--json", "count", "--priority-min", "P0", "--priority-max", "P1")
	var result struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("count JSON did not parse: %v\n%s", err, out)
	}
	if result.Count != 2 {
		t.Fatalf("count = %d, want 2", result.Count)
	}
}

func TestFileBackendCountPriorityRangeRejectsInvalidValue(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")
	out := runFileBackendTLError(t, tmpBin, repo, "count", "--priority-min", "high")
	requireOutputContains(t, out, "invalid priority")
}

func TestFileBackendUpdateTypeNormalizesAliases(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	id := run("create", "task", "Alias type", "--silent")
	run("update", id, "--type", "FR")
	var issue map[string]interface{}
	readIssueJSON(t, filepath.Join(repo, ".task-ledger", "issues", id, "issue.json"), &issue)
	if issue["type"] != "feature_request" {
		t.Fatalf("type = %#v, want feature_request in %#v", issue["type"], issue)
	}
}

func TestDoctorJSONReportsCorruptIssueFile(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	badPath := filepath.Join(repo, ".task-ledger", "issues", "tk-bad", "issue.json")
	if err := os.MkdirAll(filepath.Dir(badPath), 0o755); err != nil {
		t.Fatalf("mkdir bad issue dir: %v", err)
	}
	if err := os.WriteFile(badPath, []byte("{not json\n"), 0o644); err != nil {
		t.Fatalf("write bad issue: %v", err)
	}

	out := runFileBackendTL(t, tmpBin, repo, "--json", "doctor")
	var result struct {
		OverallOK bool `json:"overall_ok"`
		Problems  []struct {
			Path     string `json:"path"`
			Code     string `json:"code"`
			Message  string `json:"message"`
			Severity string `json:"severity"`
		} `json:"problems"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("doctor JSON did not parse: %v\n%s", err, out)
	}
	if result.OverallOK {
		t.Fatalf("doctor reported ok for corrupt file: %#v", result)
	}
	if len(result.Problems) != 1 {
		t.Fatalf("problem count = %d, want 1 in %#v", len(result.Problems), result)
	}
	problem := result.Problems[0]
	if problem.Path != badPath || problem.Code != "parse" || problem.Severity != "error" || !strings.Contains(problem.Message, "parse") {
		t.Fatalf("unexpected problem: %#v", problem)
	}
}

func TestDoctorReportsDuplicateIDs(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	issue := map[string]interface{}{
		"schema_version": 1,
		"id":             "tk-dupe",
		"type":           "task",
		"title":          "Duplicate",
		"status":         "open",
		"priority":       2,
		"deps":           []string{},
		"labels":         []string{},
		"comments":       []string{},
	}
	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("marshal issue: %v", err)
	}
	first := filepath.Join(repo, ".task-ledger", "issues", "first", "issue.json")
	second := filepath.Join(repo, ".task-ledger", "issues", "second", "issue.json")
	for _, path := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	out := runFileBackendTLError(t, tmpBin, repo, "doctor")
	requireOutputContains(t, out, "duplicate-id")
	requireOutputContains(t, out, first)
	requireOutputContains(t, out, second)
}

func TestFileBackendJSONOutputContracts(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	createOut := run("--json", "create", "task", "JSON contract", "--description", "term", "--priority", "P1")
	var created map[string]interface{}
	if err := json.Unmarshal([]byte(createOut), &created); err != nil {
		t.Fatalf("create JSON did not parse: %v\n%s", err, createOut)
	}
	id, ok := created["id"].(string)
	if !ok || !strings.HasPrefix(id, "tk-") || created["title"] != "JSON contract" {
		t.Fatalf("unexpected create JSON: %#v", created)
	}

	assertJSONObject := func(name, out string, requiredKeys ...string) {
		t.Helper()
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(out), &obj); err != nil {
			t.Fatalf("%s JSON object did not parse: %v\n%s", name, err, out)
		}
		for _, key := range requiredKeys {
			if _, ok := obj[key]; !ok {
				t.Fatalf("%s JSON missing key %q: %#v", name, key, obj)
			}
		}
	}
	assertJSONArray := func(name, out string) []interface{} {
		t.Helper()
		var arr []interface{}
		if err := json.Unmarshal([]byte(out), &arr); err != nil {
			t.Fatalf("%s JSON array did not parse: %v\n%s", name, err, out)
		}
		if arr == nil {
			t.Fatalf("%s JSON array is nil", name)
		}
		return arr
	}

	assertJSONObject("show", run("--json", "show", id), "id", "title", "status", "priority")
	if arr := assertJSONArray("list", run("--json", "list", "--all")); len(arr) != 1 {
		t.Fatalf("list length = %d, want 1", len(arr))
	}
	if arr := assertJSONArray("search", run("--json", "search", "term")); len(arr) != 1 {
		t.Fatalf("search length = %d, want 1", len(arr))
	}
	assertJSONArray("ready", run("--json", "ready"))
	assertJSONArray("blocked", run("--json", "blocked"))
	assertJSONObject("count", run("--json", "count"), "count")
	if arr := assertJSONArray("close", run("--json", "close", id, "--reason", "done")); len(arr) != 1 {
		t.Fatalf("close length = %d, want 1", len(arr))
	}
	if arr := assertJSONArray("reopen", run("--json", "reopen", id, "--reason", "retry")); len(arr) != 1 {
		t.Fatalf("reopen length = %d, want 1", len(arr))
	}
	assertJSONObject("delete", run("--json", "delete", id, "--force", "--reason", "cleanup"), "deleted", "deleted_count", "dependencies_removed", "references_updated")
}

func TestFileBackendConcurrentCreatesDoNotCorruptStorage(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	runFileBackendTL(t, tmpBin, repo, "init", "--quiet")

	const workers = 10
	type result struct {
		id  string
		out string
		err error
	}
	results := make(chan result, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmd := exec.Command(tmpBin, "--quiet", "--json", "create", "task", fmt.Sprintf("Concurrent %02d", i), "--description", "body")
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if err != nil {
				results <- result{out: string(out), err: fmt.Errorf("%w", err)}
				return
			}
			var created map[string]interface{}
			if err := json.Unmarshal(out, &created); err != nil {
				results <- result{out: string(out), err: err}
				return
			}
			id, _ := created["id"].(string)
			results <- result{id: id, out: string(out)}
		}(i)
	}
	wg.Wait()
	close(results)

	ids := make(map[string]bool)
	for res := range results {
		if res.err != nil {
			t.Fatalf("concurrent create failed: %v\n%s", res.err, res.out)
		}
		if res.id == "" {
			t.Fatalf("concurrent create returned empty id:\n%s", res.out)
		}
		if ids[res.id] {
			t.Fatalf("duplicate id returned by concurrent create: %s", res.id)
		}
		ids[res.id] = true
	}
	if len(ids) != workers {
		t.Fatalf("created %d ids, want %d", len(ids), workers)
	}

	out := runFileBackendTL(t, tmpBin, repo, "--json", "list", "--all")
	var issues []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatalf("list JSON did not parse: %v\n%s", err, out)
	}
	if len(issues) != workers {
		t.Fatalf("list returned %d issues, want %d", len(issues), workers)
	}
	assertAllIssueFilesParse(t, filepath.Join(repo, ".task-ledger"))
}

func TestFileBackendConcurrentCreateAndUpdate(t *testing.T) {
	tmpBin := buildFileBackendBinary(t)
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		return runFileBackendTL(t, tmpBin, repo, args...)
	}

	run("init", "--quiet")
	id := run("create", "task", "Shared issue", "--description", "body", "--silent")

	var wg sync.WaitGroup
	errs := make(chan string, 15)
	runConcurrent := func(args ...string) {
		defer wg.Done()
		cmd := exec.Command(tmpBin, args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			errs <- fmt.Sprintf("tl %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	for i := 0; i < 5; i++ {
		wg.Add(3)
		go runConcurrent("comments", "add", id, fmt.Sprintf("comment-%d", i))
		go runConcurrent("label", "add", id, fmt.Sprintf("label-%d", i))
		go runConcurrent("update", id, "--notes", fmt.Sprintf("note-%d", i))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	var issue map[string]interface{}
	readIssueJSON(t, filepath.Join(repo, ".task-ledger", "issues", id, "issue.json"), &issue)
	if comments, ok := issue["comments"].([]interface{}); !ok || len(comments) != 5 {
		t.Fatalf("comments = %#v, want 5 comments", issue["comments"])
	}
	if labels, ok := issue["labels"].([]interface{}); !ok || len(labels) != 5 {
		t.Fatalf("labels = %#v, want 5 labels", issue["labels"])
	}
	assertAllIssueFilesParse(t, filepath.Join(repo, ".task-ledger"))
}

func assertAllIssueFilesParse(t *testing.T, taskLedgerDir string) {
	t.Helper()
	err := filepath.WalkDir(filepath.Join(taskLedgerDir, "issues"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "issue.json" {
			return nil
		}
		var issue map[string]interface{}
		readIssueJSON(t, path, &issue)
		return nil
	})
	if err != nil {
		t.Fatalf("walk issue files: %v", err)
	}
}

func BenchmarkCLIList1000Issues(b *testing.B) {
	tmpBin := buildFileBackendBinary(b)
	repo := seedCLIFileBackendBenchmarkRepo(b, tmpBin, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(tmpBin, "--json", "list", "--all")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("tl list failed: %v\n%s", err, out)
		}
	}
}

func BenchmarkCLISearch1000Issues(b *testing.B) {
	tmpBin := buildFileBackendBinary(b)
	repo := seedCLIFileBackendBenchmarkRepo(b, tmpBin, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command(tmpBin, "--json", "search", "Issue 0999")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("tl search failed: %v\n%s", err, out)
		}
	}
}

func seedCLIFileBackendBenchmarkRepo(tb testing.TB, bin string, count int) string {
	tb.Helper()
	repo := tb.TempDir()
	runFileBackendTL(tb, bin, repo, "init", "--quiet")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("tk-%04d", i)
		path := filepath.Join(repo, ".task-ledger", "issues", id, "issue.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatalf("mkdir issue dir: %v", err)
		}
		issue := fmt.Sprintf(`{
  "schema_version": 1,
  "id": %q,
  "type": "task",
  "title": %q,
  "status": "open",
  "priority": %d,
  "deps": [],
  "labels": [],
  "comments": [],
  "created_at": %q,
  "updated_at": %q
}
`, id, fmt.Sprintf("Issue %04d", i), i%5, now, now)
		if err := os.WriteFile(path, []byte(issue), 0o644); err != nil {
			tb.Fatalf("write issue fixture: %v", err)
		}
	}
	return repo
}

func readIssueJSON(t *testing.T, path string, dest interface{}) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, data)
	}
}
