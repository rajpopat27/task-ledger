package validation

import (
	"strings"
	"testing"

	"task-ledger/internal/types"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		// Numeric format
		{"0", 0},
		{"1", 1},
		{"2", 2},
		{"3", 3},
		{"4", 4},

		// P-prefix format (uppercase)
		{"P0", 0},
		{"P1", 1},
		{"P2", 2},
		{"P3", 3},
		{"P4", 4},

		// P-prefix format (lowercase)
		{"p0", 0},
		{"p1", 1},
		{"p2", 2},

		// With whitespace
		{" 1 ", 1},
		{" P1 ", 1},

		// Invalid cases (returns -1)
		{"5", -1},   // Out of range
		{"-1", -1},  // Negative
		{"P5", -1},  // Out of range with prefix
		{"abc", -1}, // Not a number
		{"P", -1},   // Just the prefix
		{"PP1", -1}, // Double prefix
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParsePriority(tt.input)
			if got != tt.expected {
				t.Errorf("ParsePriority(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidatePriority(t *testing.T) {
	tests := []struct {
		input     string
		wantValue int
		wantError bool
	}{
		{"0", 0, false},
		{"2", 2, false},
		{"P1", 1, false},
		{"5", -1, true},
		{"abc", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidatePriority(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidatePriority(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}
			if got != tt.wantValue {
				t.Errorf("ValidatePriority(%q) = %d, want %d", tt.input, got, tt.wantValue)
			}
		})
	}
}

func TestValidateIDFormat(t *testing.T) {
	tests := []struct {
		input      string
		wantPrefix string
		wantError  bool
	}{
		{"", "", false},
		{"tl-a3f8e9", "tl", false},
		{"tl-42", "tl", false},
		{"tl-a3f8e9.1", "tl", false},
		{"foo-bar", "foo", false},
		{"nohyphen", "", true},

		// Hyphenated prefix support
		// These test cases verify that ValidateIDFormat correctly extracts
		// prefixes containing hyphens (e.g., "ledger-core" not just "ledger")
		{"ledger-core-3e9", "ledger-core", false},                     // 3-char hash suffix
		{"ledger-core-3e9.1", "ledger-core", false},                   // hierarchical child
		{"ledger-core-3e9.1.2", "ledger-core", false},                 // deeply nested child
		{"web-app-a3f8e9", "web-app", false},                          // 6-char hash suffix
		{"my-cool-project-1a2b", "my-cool-project", false},            // 4-char hash suffix
		{"document-intelligence-0sa", "document-intelligence", false}, // 3-char hash
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateIDFormat(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateIDFormat(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}
			if got != tt.wantPrefix {
				t.Errorf("ValidateIDFormat(%q) = %q, want %q", tt.input, got, tt.wantPrefix)
			}
		})
	}
}

// TestValidateIDFormat_ParentChildFlow tests the exact scenario that fails with --parent flag:
// When creating a child of a parent with a hyphenated prefix, the generated child ID
// (e.g., "ledger-core-3e9.1") should have its prefix correctly extracted as "ledger-core",
// not "ledger". This test simulates the create.go flow at lines 352-391.
func TestValidateIDFormat_ParentChildFlow(t *testing.T) {
	tests := []struct {
		name          string
		parentID      string
		childSuffix   string
		projectPrefix string
		wantPrefix    string
		shouldMatch   bool
	}{
		{
			name:          "simple prefix - child creation works",
			parentID:      "tl-a3f8e9",
			childSuffix:   ".1",
			projectPrefix: "tl",
			wantPrefix:    "tl",
			shouldMatch:   true,
		},
		{
			name:          "hyphenated prefix - child creation succeeds",
			parentID:      "ledger-core-3e9",
			childSuffix:   ".1",
			projectPrefix: "ledger-core",
			wantPrefix:    "ledger-core",
			shouldMatch:   true,
		},
		{
			name:          "hyphenated prefix - deeply nested child",
			parentID:      "ledger-core-3e9.1",
			childSuffix:   ".2",
			projectPrefix: "ledger-core",
			wantPrefix:    "ledger-core",
			shouldMatch:   true,
		},
		{
			name:          "multi-hyphen prefix - web-app style",
			parentID:      "web-app-abc123",
			childSuffix:   ".1",
			projectPrefix: "web-app",
			wantPrefix:    "web-app",
			shouldMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the child ID generation that happens in create.go:352
			childID := tt.parentID + tt.childSuffix

			// Simulate the validation flow from create.go:361
			extractedPrefix, err := ValidateIDFormat(childID)
			if err != nil {
				t.Fatalf("ValidateIDFormat(%q) unexpected error: %v", childID, err)
			}

			// Check that extracted prefix matches expected
			if extractedPrefix != tt.wantPrefix {
				t.Errorf("ValidateIDFormat(%q) extracted prefix = %q, want %q",
					childID, extractedPrefix, tt.wantPrefix)
			}

			// Simulate the prefix validation from create.go:389
			// This is where the "prefix mismatch" error occurs
			err = ValidatePrefix(extractedPrefix, tt.projectPrefix, false)
			prefixMatches := (err == nil)

			if prefixMatches != tt.shouldMatch {
				if tt.shouldMatch {
					t.Errorf("--parent %s flow: prefix validation failed unexpectedly: %v\n"+
						"  Child ID: %s\n"+
						"  Extracted prefix: %q\n"+
						"  Project prefix: %q",
						tt.parentID, err, childID, extractedPrefix, tt.projectPrefix)
				} else {
					t.Errorf("--parent %s flow: expected prefix mismatch but validation passed", tt.parentID)
				}
			}
		})
	}
}

func TestParseIssueType(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantType      types.IssueType
		wantError     bool
		errorContains string
	}{
		// Valid issue types
		{"bug type", "bug", types.TypeBug, false, ""},
		{"feature type", "feature", types.TypeFeature, false, ""},
		{"feature request type", "feature_request", types.TypeFeatureRequest, false, ""},
		{"task type", "task", types.TypeTask, false, ""},
		{"epic type", "epic", types.TypeEpic, false, ""},
		{"chore type", "chore", types.TypeChore, false, ""},

		// Case sensitivity (function is case-sensitive)
		{"uppercase bug", "BUG", types.TypeTask, true, "invalid issue type"},
		{"mixed case feature", "FeAtUrE", types.TypeTask, true, "invalid issue type"},

		// With whitespace
		{"bug with spaces", "  bug  ", types.TypeBug, false, ""},
		{"feature with tabs", "\tfeature\t", types.TypeFeature, false, ""},

		// Invalid issue types
		{"invalid type", "invalid", types.TypeTask, true, "invalid issue type"},
		{"empty string", "", types.TypeTask, true, "invalid issue type"},
		{"whitespace only", "   ", types.TypeTask, true, "invalid issue type"},
		{"numeric type", "123", types.TypeTask, true, "invalid issue type"},
		{"special chars", "bug!", types.TypeTask, true, "invalid issue type"},
		{"removed message type", "message", types.TypeTask, true, "invalid issue type"},
		{"removed molecule type", "molecule", types.TypeTask, true, "invalid issue type"},
		{"removed gate type", "gate", types.TypeTask, true, "invalid issue type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIssueType(tt.input)

			// Check error conditions
			if (err != nil) != tt.wantError {
				t.Errorf("ParseIssueType(%q) error = %v, wantError %v", tt.input, err, tt.wantError)
				return
			}

			if err != nil && tt.errorContains != "" {
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("ParseIssueType(%q) error message = %q, should contain %q", tt.input, err.Error(), tt.errorContains)
				}
				return
			}

			// Check return value
			if got != tt.wantType {
				t.Errorf("ParseIssueType(%q) = %v, want %v", tt.input, got, tt.wantType)
			}
		})
	}
}

func TestValidatePrefix(t *testing.T) {
	tests := []struct {
		name            string
		requestedPrefix string
		projectPrefix   string
		force           bool
		wantError       bool
	}{
		{"matching prefixes", "tl", "tl", false, false},
		{"empty project prefix", "tl", "", false, false},
		{"mismatched with force", "foo", "tl", true, false},
		{"mismatched without force", "foo", "tl", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrefix(tt.requestedPrefix, tt.projectPrefix, tt.force)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidatePrefix() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidatePrefixWithAllowed(t *testing.T) {
	tests := []struct {
		name            string
		requestedPrefix string
		projectPrefix   string
		allowedPrefixes string
		force           bool
		wantError       bool
	}{
		// Basic cases (same as ValidatePrefix)
		{"matching prefixes", "tl", "tl", "", false, false},
		{"empty project prefix", "tl", "", "", false, false},
		{"mismatched with force", "foo", "tl", "", true, false},
		{"mismatched without force", "foo", "tl", "", false, true},

		// Multi-prefix cases (Gas Town use case)
		{"allowed prefix gt", "gt", "hq", "gt,hmc", false, false},
		{"allowed prefix hmc", "hmc", "hq", "gt,hmc", false, false},
		{"primary prefix still works", "hq", "hq", "gt,hmc", false, false},
		{"prefix not in allowed list", "foo", "hq", "gt,hmc", false, true},

		// Edge cases
		{"allowed with spaces", "gt", "hq", "gt, hmc, foo", false, false},
		{"empty allowed list", "gt", "hq", "", false, true},
		{"single allowed prefix", "gt", "hq", "gt", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrefixWithAllowed(tt.requestedPrefix, tt.projectPrefix, tt.allowedPrefixes, tt.force)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidatePrefixWithAllowed() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
