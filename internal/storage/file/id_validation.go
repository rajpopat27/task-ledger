package file

import (
	"fmt"
	"strings"
)

// ValidateFileIssueID enforces the subset of issue IDs that are safe as
// directory names below .task-ledger/issues.
func ValidateFileIssueID(id string) error {
	if id == "" {
		return fmt.Errorf("issue id is required")
	}
	if id == "." || id == ".." {
		return fmt.Errorf("unsafe issue id %q", id)
	}
	if strings.HasPrefix(id, ".") {
		return fmt.Errorf("unsafe issue id %q: must not start with dot", id)
	}
	if strings.HasSuffix(id, "-") || strings.HasSuffix(id, ".") {
		return fmt.Errorf("unsafe issue id %q: empty suffix", id)
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return fmt.Errorf("unsafe issue id %q: only ASCII letters, digits, hyphen, underscore, and dot are allowed", id)
		}
	}
	for _, part := range strings.Split(id, ".") {
		if part == "" || part == ".." {
			return fmt.Errorf("unsafe issue id %q: path traversal segments are not allowed", id)
		}
	}
	return nil
}
