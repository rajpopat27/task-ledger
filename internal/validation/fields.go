package validation

import (
	"fmt"
	"strings"

	"task-ledger/internal/types"
	"task-ledger/internal/utils"
)

// ParsePriority extracts and validates a priority value from content.
// Supports both numeric (0-4) and P-prefix format (P0-P4).
// Returns the parsed priority (0-4) or -1 if invalid.
func ParsePriority(content string) int {
	content = strings.TrimSpace(content)

	// Handle "P1", "P0", etc. format
	if strings.HasPrefix(strings.ToUpper(content), "P") {
		content = content[1:] // Strip the "P" prefix
	}

	var p int
	if _, err := fmt.Sscanf(content, "%d", &p); err == nil && p >= 0 && p <= 4 {
		return p
	}
	return -1 // Invalid
}

// ParseIssueType extracts and validates an issue type from content.
// Returns the validated type or error if invalid.
func ParseIssueType(content string) (types.IssueType, error) {
	issueType := types.IssueType(strings.TrimSpace(content))

	// Use the canonical IsValid() from types package
	if !issueType.IsValid() {
		return types.TypeTask, fmt.Errorf("invalid issue type: %s", content)
	}

	return issueType, nil
}

// ValidatePriority parses and validates a priority string.
// Returns the parsed priority (0-4) or an error if invalid.
// Supports both numeric (0-4) and P-prefix format (P0-P4).
func ValidatePriority(priorityStr string) (int, error) {
	priority := ParsePriority(priorityStr)
	if priority == -1 {
		return -1, fmt.Errorf("invalid priority %q (expected 0-4 or P0-P4, not words like high/medium/low)", priorityStr)
	}
	return priority, nil
}

// ValidateIDFormat validates that an ID has the correct format.
// Supports: prefix-number (tl-42), prefix-hash (tl-a3f8e9), or hierarchical (tl-a3f8e9.1).
// Also supports hyphenated prefixes like "ledger-core-3e9" or "web-app-abc123".
// Returns the prefix part or an error if invalid.
func ValidateIDFormat(id string) (string, error) {
	if id == "" {
		return "", nil
	}

	// Must contain hyphen
	if !strings.Contains(id, "-") {
		return "", fmt.Errorf("invalid ID format '%s' (expected format: prefix-hash or prefix-hash.number, e.g., 'tl-a3f8e9' or 'tl-a3f8e9.1')", id)
	}

	// Use ExtractIssuePrefix which correctly handles hyphenated prefixes
	// by looking at the last hyphen and checking if suffix is hash-like.
	// This fixes the bug where "ledger-core-3e9" was parsed as prefix "ledger"
	// instead of "ledger-core".
	prefix := utils.ExtractIssuePrefix(id)

	return prefix, nil
}

// ValidatePrefix checks that the requested prefix matches the configured project prefix.
// Returns an error if they don't match (unless force is true).
func ValidatePrefix(requestedPrefix, projectPrefix string, force bool) error {
	return ValidatePrefixWithAllowed(requestedPrefix, projectPrefix, "", force)
}

// ValidatePrefixWithAllowed checks that the requested prefix is allowed.
// It matches if:
// - force is true
// - projectPrefix is empty
// - requestedPrefix matches projectPrefix
// - requestedPrefix is in the comma-separated allowedPrefixes list
// Returns an error if none of these conditions are met.
func ValidatePrefixWithAllowed(requestedPrefix, projectPrefix, allowedPrefixes string, force bool) error {
	if force || projectPrefix == "" || projectPrefix == requestedPrefix {
		return nil
	}

	// Check if requestedPrefix is in the allowed list
	if allowedPrefixes != "" {
		for _, allowed := range strings.Split(allowedPrefixes, ",") {
			allowed = strings.TrimSpace(allowed)
			if allowed == requestedPrefix {
				return nil
			}
		}
	}

	// Build helpful error message
	if allowedPrefixes != "" {
		return fmt.Errorf("prefix mismatch: project uses '%s' (allowed: %s) but you specified '%s' (use --force to override)",
			projectPrefix, allowedPrefixes, requestedPrefix)
	}
	return fmt.Errorf("prefix mismatch: project uses '%s' but you specified '%s' (use --force to override)", projectPrefix, requestedPrefix)
}
