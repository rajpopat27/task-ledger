package validation

import (
	"fmt"

	"task-ledger/internal/types"
)

// IssueValidator validates an issue and returns an error if validation fails.
// Validators can be composed using Chain() for complex validation logic.
type IssueValidator func(id string, issue *types.Issue) error

// Chain composes multiple validators into a single validator.
// Validators are executed in order and the first error stops the chain.
func Chain(validators ...IssueValidator) IssueValidator {
	return func(id string, issue *types.Issue) error {
		for _, v := range validators {
			if err := v(id, issue); err != nil {
				return err
			}
		}
		return nil
	}
}

// Exists validates that an issue is not nil.
func Exists() IssueValidator {
	return func(id string, issue *types.Issue) error {
		if issue == nil {
			return fmt.Errorf("issue %s not found", id)
		}
		return nil
	}
}

// NotPinned validates that an issue is not pinned.
// Returns an error if the issue is pinned, unless force is true.
func NotPinned(force bool) IssueValidator {
	return func(id string, issue *types.Issue) error {
		if issue == nil {
			return nil // Let Exists() handle nil check if needed
		}
		if !force && issue.Status == types.StatusPinned {
			return fmt.Errorf("cannot modify pinned issue %s (use --force to override)", id)
		}
		return nil
	}
}

// NotClosed validates that an issue is not already closed.
func NotClosed() IssueValidator {
	return func(id string, issue *types.Issue) error {
		if issue == nil {
			return nil
		}
		if issue.Status == types.StatusClosed {
			return fmt.Errorf("issue %s is already closed", id)
		}
		return nil
	}
}

// HasStatus validates that an issue has one of the allowed statuses.
func HasStatus(allowed ...types.Status) IssueValidator {
	return func(id string, issue *types.Issue) error {
		if issue == nil {
			return nil
		}
		for _, status := range allowed {
			if issue.Status == status {
				return nil
			}
		}
		return fmt.Errorf("issue %s has status %s, expected one of: %v", id, issue.Status, allowed)
	}
}

// HasType validates that an issue has one of the allowed types.
func HasType(allowed ...types.IssueType) IssueValidator {
	return func(id string, issue *types.Issue) error {
		if issue == nil {
			return nil
		}
		for _, t := range allowed {
			if issue.IssueType == t {
				return nil
			}
		}
		return fmt.Errorf("issue %s has type %s, expected one of: %v", id, issue.IssueType, allowed)
	}
}

// ForUpdate returns a validator chain for update operations.
func ForUpdate() IssueValidator {
	return Chain(
		Exists(),
	)
}

// ForClose returns a validator chain for close operations.
// Validates: issue exists and is not pinned (unless force).
func ForClose(force bool) IssueValidator {
	return Chain(
		Exists(),
		NotPinned(force),
	)
}

// ForDelete returns a validator chain for delete operations.
// Validates: issue exists.
func ForDelete() IssueValidator {
	return Chain(
		Exists(),
	)
}

// ForReopen returns a validator chain for reopen operations.
// Validates: issue exists and is closed.
func ForReopen() IssueValidator {
	return Chain(
		Exists(),
		HasStatus(types.StatusClosed),
	)
}
