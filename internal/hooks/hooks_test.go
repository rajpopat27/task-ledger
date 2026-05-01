package hooks

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-ledger/internal/types"
)

func TestNewRunner(t *testing.T) {
	runner := NewRunner("/tmp/hooks")
	if runner == nil {
		t.Fatal("NewRunner returned nil")
	}
	if runner.hooksDir != "/tmp/hooks" {
		t.Errorf("hooksDir = %q, want %q", runner.hooksDir, "/tmp/hooks")
	}
	if runner.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want %v", runner.timeout, 10*time.Second)
	}
}

func TestEventToHook(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{EventCreate, HookOnCreate},
		{EventUpdate, HookOnUpdate},
		{EventClose, HookOnClose},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			result := eventToHook(tt.event)
			if result != tt.expected {
				t.Errorf("eventToHook(%q) = %q, want %q", tt.event, result, tt.expected)
			}
		})
	}
}

func runHookSync(runner *Runner, event string, issue *types.Issue) error {
	hookPath, ok := runner.runnableHook(event)
	if !ok {
		return nil
	}
	return runner.runHook(hookPath, event, issue)
}

func TestRunnableHook_NoHook(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewRunner(tmpDir)

	if _, ok := runner.runnableHook(EventCreate); ok {
		t.Error("runnableHook returned true for non-existent hook")
	}
}

func TestRunnableHook_NotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)

	// Create a non-executable file
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho test"), 0644); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)

	if _, ok := runner.runnableHook(EventCreate); ok {
		t.Error("runnableHook returned true for non-executable hook")
	}
}

func TestRunnableHook_Executable(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)

	// Create an executable file
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)

	got, ok := runner.runnableHook(EventCreate)
	if !ok {
		t.Fatal("runnableHook returned false for executable hook")
	}
	if got != hookPath {
		t.Fatalf("runnableHook path = %q, want %q", got, hookPath)
	}
}

func TestRunnableHook_Directory(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)

	// Create a directory instead of a file
	if err := os.MkdirAll(hookPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	runner := NewRunner(tmpDir)

	if _, ok := runner.runnableHook(EventCreate); ok {
		t.Error("runnableHook returned true for directory")
	}
}

func TestRunHookSync_NoHook(t *testing.T) {
	tmpDir := t.TempDir()
	runner := NewRunner(tmpDir)

	issue := &types.Issue{ID: "tl-test", Title: "Test"}

	// Should not error when hook doesn't exist
	err := runHookSync(runner, EventCreate, issue)
	if err != nil {
		t.Errorf("runHookSync returned error for non-existent hook: %v", err)
	}
}

func TestRunHookSync_NotExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)

	// Create a non-executable file
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho test"), 0644); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)
	issue := &types.Issue{ID: "tl-test", Title: "Test"}

	// Should not error when hook is not executable
	err := runHookSync(runner, EventCreate, issue)
	if err != nil {
		t.Errorf("runHookSync returned error for non-executable hook: %v", err)
	}
}

func TestRunHookSync_Success(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)
	outputFile := filepath.Join(tmpDir, "output.txt")

	// Create a hook that writes to a file
	hookScript := `#!/bin/sh
echo "$1 $2" > ` + outputFile
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)
	issue := &types.Issue{ID: "tl-test", Title: "Test Issue"}

	err := runHookSync(runner, EventCreate, issue)
	if err != nil {
		t.Errorf("runHookSync returned error: %v", err)
	}

	// Verify the hook ran and received correct arguments
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	expected := "tl-test create\n"
	if string(output) != expected {
		t.Errorf("Hook output = %q, want %q", string(output), expected)
	}
}

func TestRunHookSync_ReceivesJSON(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)
	outputFile := filepath.Join(tmpDir, "stdin.txt")

	// Create a hook that captures stdin
	hookScript := `#!/bin/sh
cat > ` + outputFile
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)
	issue := &types.Issue{
		ID:       "tl-test",
		Title:    "Test Issue",
		Assignee: "bob",
	}

	err := runHookSync(runner, EventCreate, issue)
	if err != nil {
		t.Errorf("runHookSync returned error: %v", err)
	}

	// Verify JSON was passed to stdin
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Just check that it contains expected fields
	if len(output) == 0 {
		t.Error("Hook did not receive JSON input")
	}
	if string(output) == "" || output[0] != '{' {
		t.Errorf("Hook input doesn't look like JSON: %s", string(output))
	}
}

func TestRunHookSync_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)

	// Create a hook that sleeps for longer than timeout
	hookScript := `#!/bin/sh
sleep 60`
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := &Runner{
		hooksDir: tmpDir,
		timeout:  500 * time.Millisecond, // Short timeout
	}
	issue := &types.Issue{ID: "tl-test", Title: "Test"}

	start := time.Now()
	err := runHookSync(runner, EventCreate, issue)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("runHookSync should have returned error for timeout")
	}

	// Should have returned within timeout + some buffer
	if elapsed > 5*time.Second {
		t.Errorf("runHookSync took too long: %v", elapsed)
	}
}

func TestRunHookSync_KillsDescendants(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("TestRunHookSync_KillsDescendants requires Linux /proc")
	}

	if testing.Short() {
		t.Skip("Skipping long-running descendant kill test in short mode")
	}

	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnCreate)
	pidFile := filepath.Join(tmpDir, "child.pid")

	// Hook starts a background sleep, writes its pid, and waits for it.
	// Parent will remain alive until the child exits, so killing the
	// process group should terminate both.
	hookScript := `#!/bin/sh
(sleep 60 & echo $! > ` + pidFile + ` ; wait)`
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := &Runner{
		hooksDir: tmpDir,
		timeout:  500 * time.Millisecond,
	}
	issue := &types.Issue{ID: "tl-test", Title: "Test"}

	err := runHookSync(runner, EventCreate, issue)
	if err == nil {
		t.Fatal("expected runHookSync to return an error on timeout")
	}

	// Read the child PID and ensure it's not running anymore.
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("Failed to read pid file: %v", err)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("Invalid pid in pid file: %v", err)
	}

	// Check /proc/<pid> does not exist - retry a few times in case of timing
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid))); err != nil {
			// Process is gone, test passed
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	// If we get here, the process is still running
	t.Fatalf("Child process %d still exists after timeout", pid)
}

func TestRunHookSync_HookFailure(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnUpdate)

	// Create a hook that exits with error
	hookScript := `#!/bin/sh
exit 1`
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)
	issue := &types.Issue{ID: "tl-test", Title: "Test"}

	err := runHookSync(runner, EventUpdate, issue)
	if err == nil {
		t.Error("runHookSync should have returned error for failed hook")
	}
}

func TestRun_Async(t *testing.T) {
	tmpDir := t.TempDir()
	hookPath := filepath.Join(tmpDir, HookOnClose)
	outputFile := filepath.Join(tmpDir, "async_output.txt")

	// Create a hook that writes to a file
	hookScript := "#!/bin/sh\n" +
		"echo \"async\" > \"" + outputFile + "\"\n"
	if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
		t.Fatalf("Failed to create hook file: %v", err)
	}

	runner := NewRunner(tmpDir)
	issue := &types.Issue{ID: "tl-test", Title: "Test"}

	// Run should return immediately
	runner.Run(EventClose, issue)

	// Wait for the async hook to complete with retries.
	// Under high test load the goroutine scheduling + exec can be delayed.
	var output []byte
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		output, err = os.ReadFile(outputFile)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to read output file after retries: %v", err)
	}

	expected := "async\n"
	if string(output) != expected {
		t.Errorf("Hook output = %q, want %q", string(output), expected)
	}
}

func TestAllHookEvents(t *testing.T) {
	// Verify all event constants have corresponding hook names
	events := []struct {
		event string
		hook  string
	}{
		{EventCreate, HookOnCreate},
		{EventUpdate, HookOnUpdate},
		{EventClose, HookOnClose},
	}

	for _, e := range events {
		t.Run(e.event, func(t *testing.T) {
			result := eventToHook(e.event)
			if result != e.hook {
				t.Errorf("eventToHook(%q) = %q, want %q", e.event, result, e.hook)
			}
		})
	}
}
