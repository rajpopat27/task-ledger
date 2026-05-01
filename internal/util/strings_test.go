package util

import (
	"reflect"
	"testing"
)

func TestNormalizeLabels(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "single label",
			input:    []string{"bug"},
			expected: []string{"bug"},
		},
		{
			name:     "multiple labels",
			input:    []string{"bug", "critical", "frontend"},
			expected: []string{"bug", "critical", "frontend"},
		},
		{
			name:     "labels with whitespace",
			input:    []string{"  bug  ", " critical", "frontend "},
			expected: []string{"bug", "critical", "frontend"},
		},
		{
			name:     "duplicate labels",
			input:    []string{"bug", "bug", "critical"},
			expected: []string{"bug", "critical"},
		},
		{
			name:     "duplicates after trimming",
			input:    []string{"bug", "  bug  ", " bug"},
			expected: []string{"bug"},
		},
		{
			name:     "empty strings",
			input:    []string{"bug", "", "critical"},
			expected: []string{"bug", "critical"},
		},
		{
			name:     "whitespace-only strings",
			input:    []string{"bug", "   ", "critical", "\t", "\n"},
			expected: []string{"bug", "critical"},
		},
		{
			name:     "preserves order",
			input:    []string{"zebra", "apple", "banana"},
			expected: []string{"zebra", "apple", "banana"},
		},
		{
			name:     "complex case with all issues",
			input:    []string{"  bug  ", "", "bug", "critical", "   ", "frontend", "critical", "  frontend  "},
			expected: []string{"bug", "critical", "frontend"},
		},
		{
			name:     "unicode labels",
			input:    []string{"🐛 bug", "  🐛 bug  ", "🚀 feature"},
			expected: []string{"🐛 bug", "🚀 feature"},
		},
		{
			name:     "case-sensitive",
			input:    []string{"Bug", "bug", "BUG"},
			expected: []string{"Bug", "bug", "BUG"},
		},
		{
			name:     "labels with internal spaces",
			input:    []string{"needs review", "  needs review  ", "in progress"},
			expected: []string{"needs review", "in progress"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeLabels(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("NormalizeLabels(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeLabels_PreservesCapacity(t *testing.T) {
	input := []string{"bug", "critical", "frontend"}
	result := NormalizeLabels(input)

	// Result should have reasonable capacity (not excessive allocation)
	if cap(result) > len(input)*2 {
		t.Errorf("NormalizeLabels capacity too large: got %d, input len %d", cap(result), len(input))
	}
}

func TestNormalizeIssueType(t *testing.T) {
	tests := map[string][2]string{
		"feat alias":                   {"feat", "feature"},
		"FEAT uppercase":               {"FEAT", "feature"},
		"fr alias":                     {"fr", "feature_request"},
		"feature request hyphen alias": {"feature-request", "feature_request"},
		"non-alias unchanged":          {"bug", "bug"},
		"full name unchanged":          {"feature_request", "feature_request"},
		"empty string":                 {"", ""},
		"feature unchanged":            {"feature", "feature"},
		"task unchanged":               {"task", "task"},
	}

	for name, tt := range tests {
		assertNormalizeIssueType(t, name, tt[0], tt[1])
	}
}

func assertNormalizeIssueType(t *testing.T, name, input, expected string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if result := NormalizeIssueType(input); result != expected {
			t.Errorf("NormalizeIssueType(%q) = %q, want %q", input, result, expected)
		}
	})
}
