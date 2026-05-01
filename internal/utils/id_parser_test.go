package utils

import (
	"context"
	"path/filepath"
	"testing"

	filestorage "task-ledger/internal/storage/file"
	"task-ledger/internal/types"
)

func newResolveTestStore(t *testing.T) *filestorage.FileStorage {
	t.Helper()
	store, err := filestorage.Open(filepath.Join(t.TempDir(), ".task-ledger"))
	if err != nil {
		t.Fatalf("Open file storage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestParseIssueID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{
			name:     "already has prefix",
			input:    "tl-a3f8e9",
			prefix:   "tl-",
			expected: "tl-a3f8e9",
		},
		{
			name:     "missing prefix",
			input:    "a3f8e9",
			prefix:   "tl-",
			expected: "tl-a3f8e9",
		},
		{
			name:     "hierarchical with prefix",
			input:    "tl-a3f8e9.1.2",
			prefix:   "tl-",
			expected: "tl-a3f8e9.1.2",
		},
		{
			name:     "hierarchical without prefix",
			input:    "a3f8e9.1.2",
			prefix:   "tl-",
			expected: "tl-a3f8e9.1.2",
		},
		{
			name:     "custom prefix with ID",
			input:    "ticket-123",
			prefix:   "ticket-",
			expected: "ticket-123",
		},
		{
			name:     "custom prefix without ID",
			input:    "123",
			prefix:   "ticket-",
			expected: "ticket-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseIssueID(tt.input, tt.prefix)
			if result != tt.expected {
				t.Errorf("ParseIssueID(%q, %q) = %q; want %q", tt.input, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestResolvePartialID(t *testing.T) {
	ctx := context.Background()
	store := newResolveTestStore(t)

	// Create test issues with sequential IDs (current implementation)
	// When hash IDs (tl-165) are implemented, these can be hash-based
	issue1 := &types.Issue{
		ID:        "tl-1",
		Title:     "Test Issue 1",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}
	issue2 := &types.Issue{
		ID:        "tl-2",
		Title:     "Test Issue 2",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}
	issue3 := &types.Issue{
		ID:        "tl-10",
		Title:     "Test Issue 3",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}
	// Test hierarchical IDs - parent and child
	parentIssue := &types.Issue{
		ID:        "offlinebrew-3d0",
		Title:     "Parent Epic",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeEpic,
	}
	childIssue := &types.Issue{
		ID:        "offlinebrew-3d0.1",
		Title:     "Child Task",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}

	if err := store.CreateIssue(ctx, issue1, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIssue(ctx, issue2, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIssue(ctx, issue3, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIssue(ctx, parentIssue, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIssue(ctx, childIssue, "test"); err != nil {
		t.Fatal(err)
	}

	// Set config for prefix
	if err := store.SetConfig(ctx, "issue_prefix", "tl-"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		input       string
		expected    string
		shouldError bool
		errorMsg    string
	}{
		{
			name:     "exact match with prefix",
			input:    "tl-1",
			expected: "tl-1",
		},
		{
			name:     "exact match without prefix",
			input:    "1",
			expected: "tl-1",
		},
		{
			name:     "exact match with prefix (two digits)",
			input:    "tl-10",
			expected: "tl-10",
		},
		{
			name:     "exact match without prefix (two digits)",
			input:    "10",
			expected: "tl-10",
		},
		{
			name:        "nonexistent issue",
			input:       "tl-999",
			shouldError: true,
			errorMsg:    "no issue found",
		},
		{
			name:     "partial match - unique substring",
			input:    "tl-1",
			expected: "tl-1",
		},
		{
			name:     "ambiguous partial match",
			input:    "tl-1",
			expected: "tl-1", // Will match exactly, not ambiguously
		},
		{
			name:     "exact match parent ID with hierarchical child - gh-316",
			input:    "offlinebrew-3d0",
			expected: "offlinebrew-3d0", // Should match exactly, not be ambiguous with offlinebrew-3d0.1
		},
		{
			name:     "exact match parent without prefix - gh-316",
			input:    "3d0",
			expected: "offlinebrew-3d0", // Should still prefer exact hash match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolvePartialID(ctx, store, tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ResolvePartialID(%q) expected error containing %q, got nil", tt.input, tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("ResolvePartialID(%q) error = %q; want error containing %q", tt.input, err.Error(), tt.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ResolvePartialID(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("ResolvePartialID(%q) = %q; want %q", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestResolvePartialIDs(t *testing.T) {
	ctx := context.Background()
	store := newResolveTestStore(t)

	// Create test issues
	issue1 := &types.Issue{
		ID:        "tl-1",
		Title:     "Test Issue 1",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}
	issue2 := &types.Issue{
		ID:        "tl-2",
		Title:     "Test Issue 2",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}

	if err := store.CreateIssue(ctx, issue1, "test"); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateIssue(ctx, issue2, "test"); err != nil {
		t.Fatal(err)
	}

	if err := store.SetConfig(ctx, "issue_prefix", "tl-"); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		inputs      []string
		expected    []string
		shouldError bool
	}{
		{
			name:     "resolve multiple IDs without prefix",
			inputs:   []string{"1", "2"},
			expected: []string{"tl-1", "tl-2"},
		},
		{
			name:     "resolve mixed full and partial IDs",
			inputs:   []string{"tl-1", "2"},
			expected: []string{"tl-1", "tl-2"},
		},
		{
			name:        "error on nonexistent ID",
			inputs:      []string{"1", "999"},
			shouldError: true,
		},
		{
			name:     "empty input list",
			inputs:   []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolvePartialIDs(ctx, store, tt.inputs)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ResolvePartialIDs(%v) expected error, got nil", tt.inputs)
				}
			} else {
				if err != nil {
					t.Errorf("ResolvePartialIDs(%v) unexpected error: %v", tt.inputs, err)
				}
				if len(result) != len(tt.expected) {
					t.Errorf("ResolvePartialIDs(%v) returned %d results; want %d", tt.inputs, len(result), len(tt.expected))
				}
				for i := range result {
					if result[i] != tt.expected[i] {
						t.Errorf("ResolvePartialIDs(%v)[%d] = %q; want %q", tt.inputs, i, result[i], tt.expected[i])
					}
				}
			}
		})
	}
}

func TestResolvePartialID_NoConfig(t *testing.T) {
	ctx := context.Background()
	store := newResolveTestStore(t)

	// Create test issue without setting config (test default prefix)
	issue1 := &types.Issue{
		ID:        "tl-1",
		Title:     "Test Issue",
		Status:    types.StatusOpen,
		Priority:  1,
		IssueType: types.TypeTask,
	}

	if err := store.CreateIssue(ctx, issue1, "test"); err != nil {
		t.Fatal(err)
	}

	// Don't set config - should use default "tl" prefix
	result, err := ResolvePartialID(ctx, store, "1")
	if err != nil {
		t.Fatalf("ResolvePartialID failed with default config: %v", err)
	}

	if result != "tl-1" {
		t.Errorf("ResolvePartialID(\"1\") with default config = %q; want \"tl-1\"", result)
	}
}

func TestExtractIssuePrefix(t *testing.T) {
	tests := []struct {
		name     string
		issueID  string
		expected string
	}{
		{
			name:     "standard format",
			issueID:  "tl-a3f8e9",
			expected: "tl",
		},
		{
			name:     "custom prefix",
			issueID:  "ticket-123",
			expected: "ticket",
		},
		{
			name:     "hierarchical ID",
			issueID:  "tl-a3f8e9.1.2",
			expected: "tl",
		},
		{
			name:     "no hyphen",
			issueID:  "invalid",
			expected: "",
		},
		{
			name:     "empty string",
			issueID:  "",
			expected: "",
		},
		{
			name:     "only prefix",
			issueID:  "tl-",
			expected: "tl",
		},
		{
			name:     "multi-part prefix with numeric suffix",
			issueID:  "alpha-beta-1",
			expected: "alpha-beta", // Last hyphen before numeric suffix
		},
		{
			name:     "multi-part non-numeric suffix (word-like)",
			issueID:  "vc-baseline-test",
			expected: "vc", // Word-like suffix (4+ chars, no digit) uses first hyphen (tl-fasa fix)
		},
		{
			name:     "taskledger-vscode style prefix",
			issueID:  "taskledger-vscode-1",
			expected: "taskledger-vscode", // Last hyphen before numeric suffix
		},
		{
			name:     "web-app style prefix",
			issueID:  "web-app-123",
			expected: "web-app", // Should extract "web-app", not "web-"
		},
		{
			name:     "three-part prefix with hash",
			issueID:  "my-cool-app-a3f8e9",
			expected: "my-cool-app", // Hash suffix should use last hyphen logic
		},
		{
			name:     "four-part prefix with 4-char hash",
			issueID:  "super-long-project-name-1a2b",
			expected: "super-long-project-name", // 4-char hash
		},
		{
			name:     "prefix with 5-char hash",
			issueID:  "my-app-1a2b3",
			expected: "my-app", // 5-char hash
		},
		{
			name:     "prefix with 6-char hash",
			issueID:  "web-app-a1b2c3",
			expected: "web-app", // 6-char hash
		},
		{
			name:     "uppercase hash",
			issueID:  "my-app-A3F8E9",
			expected: "my-app", // Uppercase hash should work
		},
		{
			name:     "mixed case hash",
			issueID:  "proj-AbCd12",
			expected: "proj", // Mixed case hash should work
		},
		{
			name:     "3-char hash with hyphenated prefix",
			issueID:  "document-intelligence-0sa",
			expected: "document-intelligence", // 3-char hash (base36) should use last hyphen
		},
		{
			name:     "3-char hash with multi-part prefix",
			issueID:  "my-cool-app-1x7",
			expected: "my-cool-app", // 3-char base36 hash
		},
		{
			name:     "3-char all-letters suffix (now treated as hash, GH #446)",
			issueID:  "test-proj-abc",
			expected: "test-proj", // 3-char all-letter now accepted as hash (GH #446)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractIssuePrefix(tt.issueID)
			if result != tt.expected {
				t.Errorf("ExtractIssuePrefix(%q) = %q; want %q", tt.issueID, result, tt.expected)
			}
		})
	}
}

func TestExtractIssueNumber(t *testing.T) {
	tests := []struct {
		name     string
		issueID  string
		expected int
	}{
		{
			name:     "simple number",
			issueID:  "tl-123",
			expected: 123,
		},
		{
			name:     "hash ID (no number)",
			issueID:  "tl-a3f8e9",
			expected: 0,
		},
		{
			name:     "hierarchical with number",
			issueID:  "tl-42.1.2",
			expected: 42,
		},
		{
			name:     "no hyphen",
			issueID:  "invalid",
			expected: 0,
		},
		{
			name:     "empty string",
			issueID:  "",
			expected: 0,
		},
		{
			name:     "zero",
			issueID:  "tl-0",
			expected: 0,
		},
		{
			name:     "large number",
			issueID:  "tl-999999",
			expected: 999999,
		},
		{
			name:     "number with text after",
			issueID:  "tl-123abc",
			expected: 123,
		},
		{
			name:     "hyphenated prefix with number",
			issueID:  "alpha-beta-7",
			expected: 7,
		},
	}

	for _, tt := range tests {
		assertExtractIssueNumber(t, tt.name, tt.issueID, tt.expected)
	}
}

func assertExtractIssueNumber(t *testing.T, name, issueID string, expected int) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if result := ExtractIssueNumber(issueID); result != expected {
			t.Errorf("ExtractIssueNumber(%q) = %d; want %d", issueID, result, expected)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
