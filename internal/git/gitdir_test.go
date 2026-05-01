package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()

	repoPath := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.email", "test@example.com")
	runGit(t, repoPath, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, repoPath, "add", ".")
	runGit(t, repoPath, "commit", "-m", "initial")

	return repoPath
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	ResetCaches()
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
		ResetCaches()
	})
}

func requireSamePath(t *testing.T, got, want string) {
	t.Helper()

	resolvedGot, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("resolve got path %s: %v", got, err)
	}
	resolvedWant, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("resolve want path %s: %v", want, err)
	}
	if resolvedGot != resolvedWant {
		t.Fatalf("path = %s, want %s", resolvedGot, resolvedWant)
	}
}

func createLinkedWorktree(t *testing.T, repoPath string) string {
	t.Helper()

	worktreePath := filepath.Join(t.TempDir(), "linked-worktree")
	runGit(t, repoPath, "worktree", "add", "-b", "feature", worktreePath)
	return worktreePath
}

func TestGetMainRepoRootRegularRepo(t *testing.T) {
	repoPath := setupGitRepo(t)
	chdir(t, repoPath)

	root, err := GetMainRepoRoot()
	if err != nil {
		t.Fatalf("GetMainRepoRoot: %v", err)
	}
	requireSamePath(t, root, repoPath)
}

func TestGetMainRepoRootLinkedWorktree(t *testing.T) {
	repoPath := setupGitRepo(t)
	worktreePath := createLinkedWorktree(t, repoPath)
	nestedDir := filepath.Join(worktreePath, "nested", "dir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("create nested worktree dir: %v", err)
	}
	chdir(t, nestedDir)

	root, err := GetMainRepoRoot()
	if err != nil {
		t.Fatalf("GetMainRepoRoot: %v", err)
	}
	requireSamePath(t, root, repoPath)
}

func TestIsWorktree(t *testing.T) {
	t.Run("regular repo", func(t *testing.T) {
		repoPath := setupGitRepo(t)
		chdir(t, repoPath)

		if IsWorktree() {
			t.Fatal("IsWorktree() = true, want false")
		}
	})

	t.Run("linked worktree", func(t *testing.T) {
		repoPath := setupGitRepo(t)
		worktreePath := createLinkedWorktree(t, repoPath)
		chdir(t, worktreePath)

		if !IsWorktree() {
			t.Fatal("IsWorktree() = false, want true")
		}
	})
}

func TestGetRepoRoot(t *testing.T) {
	repoPath := setupGitRepo(t)
	nestedDir := filepath.Join(repoPath, "nested", "dir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("create nested repo dir: %v", err)
	}
	chdir(t, nestedDir)

	requireSamePath(t, GetRepoRoot(), repoPath)
}

func TestGetRepoRootOutsideGit(t *testing.T) {
	chdir(t, t.TempDir())

	if got := GetRepoRoot(); got != "" {
		t.Fatalf("GetRepoRoot() = %q, want empty", got)
	}
}
