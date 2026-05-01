package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"task-ledger/internal/storage"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/utils"
)

var deleteCmd = &cobra.Command{
	Use:     "delete <issue-id> [issue-id...]",
	GroupID: "issues",
	Short:   "Delete one or more issues and clean up references",
	Args:    cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("delete")
		fromFile, _ := cmd.Flags().GetString("from-file")
		force, _ := cmd.Flags().GetBool("force")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		reason, _ := cmd.Flags().GetString("reason")

		issueIDs := append([]string{}, args...)
		if fromFile != "" {
			fileIDs, err := readIssueIDsFromFile(fromFile)
			if err != nil {
				FatalErrorRespectJSON("reading %s: %v", fromFile, err)
			}
			issueIDs = append(issueIDs, fileIDs...)
		}
		if len(issueIDs) == 0 {
			FatalErrorRespectJSON("no issue IDs provided")
		}
		issueIDs = uniqueStrings(issueIDs)

		ctx := rootCtx
		resolvedIDs := make([]string, 0, len(issueIDs))
		issues := make(map[string]*types.Issue)
		for _, id := range issueIDs {
			resolvedID, err := utils.ResolvePartialID(ctx, store, id)
			if err != nil {
				FatalErrorRespectJSON("resolving %s: %v", id, err)
			}
			issue, err := store.GetIssue(ctx, resolvedID)
			if err != nil {
				FatalErrorRespectJSON("getting issue %s: %v", resolvedID, err)
			}
			if issue == nil {
				FatalErrorRespectJSON("issue %s not found", resolvedID)
			}
			resolvedIDs = append(resolvedIDs, resolvedID)
			issues[resolvedID] = issue
		}

		if dryRun || !force {
			showDeletionPreview(resolvedIDs, issues, false, nil)
			if dryRun {
				fmt.Println("\n(Dry-run mode - no changes made)")
				return
			}
			fmt.Printf("\n%s\n", ui.RenderWarn("This operation cannot be undone. Re-run with --force to proceed."))
			return
		}

		deleted := make([]string, 0, len(resolvedIDs))
		var depsRemoved int
		var updatedRefs int
		if err := store.RunInTransaction(ctx, func(tx storage.Transaction) error {
			connected, err := collectConnectedIssuesFrom(ctx, tx, resolvedIDs)
			if err != nil {
				return err
			}
			depsRemoved, err = removeDependencyEdgesFrom(ctx, tx, resolvedIDs, actor)
			if err != nil {
				return err
			}
			for _, id := range resolvedIDs {
				if err := tx.DeleteIssueWithReason(ctx, id, actor, reason); err != nil {
					return fmt.Errorf("delete %s: %w", id, err)
				}
				deleted = append(deleted, id)
			}
			updatedRefs, err = updateTextReferencesInIssuesFrom(ctx, tx, deleted, connected, actor)
			return err
		}); err != nil {
			FatalErrorRespectJSON("delete failed: %v", err)
		}

		if jsonOutput {
			outputJSON(map[string]interface{}{
				"deleted":              deleted,
				"deleted_count":        len(deleted),
				"dependencies_removed": depsRemoved,
				"references_updated":   updatedRefs,
			})
			return
		}
		fmt.Printf("%s Deleted %d issue(s)\n", ui.RenderPass("OK"), len(deleted))
		fmt.Printf("  Removed %d dependency link(s)\n", depsRemoved)
		fmt.Printf("  Updated text references in %d issue(s)\n", updatedRefs)
	},
}

func collectConnectedIssues(ctx context.Context, issueIDs []string) map[string]*types.Issue {
	connected, _ := collectConnectedIssuesFrom(ctx, store, issueIDs)
	return connected
}

func collectConnectedIssuesFrom(ctx context.Context, st interface {
	GetDependencies(context.Context, string) ([]*types.Issue, error)
	GetDependents(context.Context, string) ([]*types.Issue, error)
}, issueIDs []string) (map[string]*types.Issue, error) {
	idSet := make(map[string]bool, len(issueIDs))
	for _, id := range issueIDs {
		idSet[id] = true
	}
	connected := make(map[string]*types.Issue)
	for _, id := range issueIDs {
		deps, err := st.GetDependencies(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get dependencies for %s: %w", id, err)
		}
		for _, dep := range deps {
			if !idSet[dep.ID] {
				connected[dep.ID] = dep
			}
		}
		dependents, err := st.GetDependents(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get dependents for %s: %w", id, err)
		}
		for _, dep := range dependents {
			if !idSet[dep.ID] {
				connected[dep.ID] = dep
			}
		}
	}
	return connected, nil
}

func removeDependencyEdges(ctx context.Context, issueIDs []string) int {
	removed, _ := removeDependencyEdgesFrom(ctx, store, issueIDs, actor)
	return removed
}

func removeDependencyEdgesFrom(ctx context.Context, st interface {
	GetDependencyRecords(context.Context, string) ([]*types.Dependency, error)
	GetDependents(context.Context, string) ([]*types.Issue, error)
	RemoveDependency(context.Context, string, string, string) error
}, issueIDs []string, actor string) (int, error) {
	removed := 0
	for _, id := range issueIDs {
		records, err := st.GetDependencyRecords(ctx, id)
		if err != nil {
			return removed, fmt.Errorf("get dependency records for %s: %w", id, err)
		}
		for _, dep := range records {
			if dep.Type == types.DepParentChild {
				continue
			}
			if err := st.RemoveDependency(ctx, dep.IssueID, dep.DependsOnID, actor); err != nil {
				return removed, fmt.Errorf("remove dependency %s -> %s: %w", dep.IssueID, dep.DependsOnID, err)
			}
			removed++
		}
		dependents, err := st.GetDependents(ctx, id)
		if err != nil {
			return removed, fmt.Errorf("get dependents for %s: %w", id, err)
		}
		for _, dep := range dependents {
			if err := st.RemoveDependency(ctx, dep.ID, id, actor); err != nil {
				return removed, fmt.Errorf("remove dependent %s -> %s: %w", dep.ID, id, err)
			}
			removed++
		}
	}
	return removed, nil
}

func showDeletionPreview(issueIDs []string, issues map[string]*types.Issue, _ bool, _ error) {
	fmt.Printf("\n%s\n", ui.RenderFail("DELETE PREVIEW"))
	fmt.Printf("\nIssues to delete (%d):\n", len(issueIDs))
	for _, id := range issueIDs {
		if issue := issues[id]; issue != nil {
			fmt.Printf("  %s: %s\n", id, issue.Title)
		}
	}
}

func updateTextReferencesInIssues(ctx context.Context, deletedIDs []string, connectedIssues map[string]*types.Issue) int {
	updated, _ := updateTextReferencesInIssuesFrom(ctx, store, deletedIDs, connectedIssues, actor)
	return updated
}

func updateTextReferencesInIssuesFrom(ctx context.Context, st interface {
	GetIssue(context.Context, string) (*types.Issue, error)
	UpdateIssue(context.Context, string, map[string]interface{}, string) error
}, deletedIDs []string, connectedIssues map[string]*types.Issue, actor string) (int, error) {
	updatedCount := 0
	for _, id := range deletedIDs {
		idPattern := `(^|[^A-Za-z0-9_-])(` + regexp.QuoteMeta(id) + `)($|[^A-Za-z0-9_-])`
		re := regexp.MustCompile(idPattern)
		replacementText := `$1[deleted:` + id + `]$3`
		for connID := range connectedIssues {
			connIssue, err := st.GetIssue(ctx, connID)
			if err != nil {
				return updatedCount, fmt.Errorf("get connected issue %s: %w", connID, err)
			}
			if connIssue == nil {
				continue
			}
			updates := make(map[string]interface{})
			if re.MatchString(connIssue.Description) {
				updates["description"] = re.ReplaceAllString(connIssue.Description, replacementText)
			}
			if connIssue.Notes != "" && re.MatchString(connIssue.Notes) {
				updates["notes"] = re.ReplaceAllString(connIssue.Notes, replacementText)
			}
			if connIssue.Design != "" && re.MatchString(connIssue.Design) {
				updates["design"] = re.ReplaceAllString(connIssue.Design, replacementText)
			}
			if connIssue.AcceptanceCriteria != "" && re.MatchString(connIssue.AcceptanceCriteria) {
				updates["acceptance_criteria"] = re.ReplaceAllString(connIssue.AcceptanceCriteria, replacementText)
			}
			if len(updates) == 0 {
				continue
			}
			if err := st.UpdateIssue(ctx, connID, updates, actor); err != nil {
				return updatedCount, fmt.Errorf("update text references in %s: %w", connID, err)
			}
			updatedCount++
		}
	}
	return updatedCount, nil
}

func readIssueIDsFromFile(filename string) ([]string, error) {
	f, err := os.Open(filename) // #nosec G304 -- deleting from an explicit user-provided list file is the command's purpose.
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var ids []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ids = append(ids, line)
	}
	return ids, scanner.Err()
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Actually delete (without this flag, shows preview)")
	deleteCmd.Flags().String("from-file", "", "Read issue IDs from file (one per line)")
	deleteCmd.Flags().Bool("dry-run", false, "Preview what would be deleted without making changes")
	deleteCmd.Flags().String("reason", "", "Reason for deletion")
	deleteCmd.ValidArgsFunction = issueIDCompletion
	rootCmd.AddCommand(deleteCmd)
}
