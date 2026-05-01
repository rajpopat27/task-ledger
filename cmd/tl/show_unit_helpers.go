package main

import (
	"context"

	"task-ledger/internal/storage"
	"task-ledger/internal/types"
	"task-ledger/internal/validation"
)

// validateIssueUpdatable checks if an issue can be updated.
// Uses the centralized validation package for consistency.
func validateIssueUpdatable(id string, issue *types.Issue) error {
	return validation.ForUpdate()(id, issue)
}

// validateIssueClosable checks if an issue can be closed.
// Uses the centralized validation package for consistency.
func validateIssueClosable(id string, issue *types.Issue, force bool) error {
	return validation.Chain(
		validation.NotPinned(force),
	)(id, issue)
}

func applyLabelUpdates(ctx context.Context, st storage.Storage, issueID, actor string, setLabels, addLabels, removeLabels []string) error {
	// Set labels (replaces all existing labels)
	if len(setLabels) > 0 {
		currentLabels, err := st.GetLabels(ctx, issueID)
		if err != nil {
			return err
		}
		for _, label := range currentLabels {
			if err := st.RemoveLabel(ctx, issueID, label, actor); err != nil {
				return err
			}
		}
		for _, label := range setLabels {
			if err := st.AddLabel(ctx, issueID, label, actor); err != nil {
				return err
			}
		}
	}

	// Add labels
	for _, label := range addLabels {
		if err := st.AddLabel(ctx, issueID, label, actor); err != nil {
			return err
		}
	}

	// Remove labels
	for _, label := range removeLabels {
		if err := st.RemoveLabel(ctx, issueID, label, actor); err != nil {
			return err
		}
	}

	return nil
}
