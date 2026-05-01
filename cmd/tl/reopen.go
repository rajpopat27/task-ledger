package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/utils"
)

var reopenCmd = &cobra.Command{
	Use:     "reopen [id...]",
	GroupID: "issues",
	Short:   "Reopen one or more closed issues",
	Long: `Reopen closed issues by setting status to 'open' and clearing the closed_at timestamp.
This is more explicit than 'tl update --status open' and emits a Reopened event.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("reopen")
		reason, _ := cmd.Flags().GetString("reason")
		// Use global jsonOutput set by PersistentPreRun
		ctx := rootCtx
		reopenedIssues := []*types.Issue{}
		if store == nil {
			fmt.Fprintln(os.Stderr, "Error: .task-ledger directory not initialized")
			os.Exit(1)
		}
		for _, id := range args {
			fullID, err := utils.ResolvePartialID(ctx, store, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving %s: %v\n", id, err)
				continue
			}
			// UpdateIssue automatically clears closed_at when status changes from closed
			updates := map[string]interface{}{
				"status": string(types.StatusOpen),
			}
			if err := store.UpdateIssue(ctx, fullID, updates, actor); err != nil {
				fmt.Fprintf(os.Stderr, "Error reopening %s: %v\n", fullID, err)
				continue
			}
			// Add reason as a comment if provided
			if reason != "" {
				if err := store.AddComment(ctx, fullID, actor, reason); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to add comment to %s: %v\n", fullID, err)
				}
			}
			if jsonOutput {
				issue, _ := store.GetIssue(ctx, fullID)
				if issue != nil {
					reopenedIssues = append(reopenedIssues, issue)
				}
			} else {
				reasonMsg := ""
				if reason != "" {
					reasonMsg = ": " + reason
				}
				fmt.Printf("%s Reopened %s%s\n", ui.RenderAccent("↻"), fullID, reasonMsg)
			}
		}
		if jsonOutput && len(reopenedIssues) > 0 {
			outputJSON(reopenedIssues)
		}
	},
}

func init() {
	reopenCmd.Flags().StringP("reason", "r", "", "Reason for reopening")
	reopenCmd.ValidArgsFunction = issueIDCompletion
	rootCmd.AddCommand(reopenCmd)
}
