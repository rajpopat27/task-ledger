package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"task-ledger/internal/hooks"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/utils"
)

var closeCmd = &cobra.Command{
	Use:     "close [id...]",
	GroupID: "issues",
	Short:   "Close one or more issues",
	Long: `Close one or more issues.

If no issue ID is provided, closes the last touched issue (from most recent
create, update, show, or close operation).`,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("close")

		// If no IDs provided, use last touched issue
		if len(args) == 0 {
			lastTouched := GetLastTouchedID()
			if lastTouched == "" {
				FatalErrorRespectJSON("no issue ID provided and no last touched issue")
			}
			args = []string{lastTouched}
		}
		reason, _ := cmd.Flags().GetString("reason")
		if reason == "" {
			// Check --resolution alias (Jira CLI convention)
			reason, _ = cmd.Flags().GetString("resolution")
		}
		if reason == "" {
			reason = "Closed"
		}
		force, _ := cmd.Flags().GetBool("force")
		suggestNext, _ := cmd.Flags().GetBool("suggest-next")

		// Get session ID from flag or environment variable
		session, _ := cmd.Flags().GetString("session")
		if session == "" {
			session = os.Getenv("CLAUDE_SESSION_ID")
		}

		ctx := rootCtx

		// --suggest-next only works with a single issue
		if suggestNext && len(args) > 1 {
			FatalErrorRespectJSON("--suggest-next only works when closing a single issue")
		}

		var resolvedIDs []string
		for _, id := range args {
			resolved, err := utils.ResolvePartialID(ctx, store, id)
			if err != nil {
				FatalErrorRespectJSON("resolving ID %s: %v", id, err)
			}
			resolvedIDs = append(resolvedIDs, resolved)
		}

		closedIssues := []*types.Issue{}
		closedCount := 0

		// Handle local IDs
		for _, id := range resolvedIDs {
			// Get issue for checks
			issue, _ := store.GetIssue(ctx, id)

			if err := validateIssueClosable(id, issue, force); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				continue
			}

			// Check if issue has open blockers (GH#962)
			if !force {
				blocked, blockers, err := store.IsBlocked(ctx, id)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error checking blockers for %s: %v\n", id, err)
					continue
				}
				if blocked && len(blockers) > 0 {
					fmt.Fprintf(os.Stderr, "cannot close %s: blocked by open issues %v (use --force to override)\n", id, blockers)
					continue
				}
			}

			if err := store.CloseIssue(ctx, id, reason, actor, session); err != nil {
				fmt.Fprintf(os.Stderr, "Error closing %s: %v\n", id, err)
				continue
			}

			closedCount++

			// Run close hook
			closedIssue, _ := store.GetIssue(ctx, id)
			if closedIssue != nil && hookRunner != nil {
				hookRunner.Run(hooks.EventClose, closedIssue)
			}

			if jsonOutput {
				if closedIssue != nil {
					closedIssues = append(closedIssues, closedIssue)
				}
			} else {
				fmt.Printf("%s Closed %s: %s\n", ui.RenderPass("✓"), id, reason)
			}
		}

		// Handle --suggest-next flag in direct mode
		if suggestNext && len(resolvedIDs) == 1 && closedCount > 0 {
			unblocked, err := store.GetNewlyUnblockedByClose(ctx, resolvedIDs[0])
			if err == nil && len(unblocked) > 0 {
				if jsonOutput {
					outputJSON(map[string]interface{}{
						"closed":    closedIssues,
						"unblocked": unblocked,
					})
					return
				}
				fmt.Printf("\nNewly unblocked:\n")
				for _, issue := range unblocked {
					fmt.Printf("  • %s %q (P%d)\n", issue.ID, issue.Title, issue.Priority)
				}
			}
		}

		if jsonOutput && len(closedIssues) > 0 {
			outputJSON(closedIssues)
		}
	},
}

func init() {
	closeCmd.Flags().StringP("reason", "r", "", "Reason for closing")
	closeCmd.Flags().String("resolution", "", "Alias for --reason (Jira CLI convention)")
	_ = closeCmd.Flags().MarkHidden("resolution") // Hidden alias for agent/CLI ergonomics
	closeCmd.Flags().BoolP("force", "f", false, "Force close pinned issues")
	closeCmd.Flags().Bool("suggest-next", false, "Show newly unblocked issues after closing")
	closeCmd.Flags().String("session", "", "Claude Code session ID (or set CLAUDE_SESSION_ID env var)")
	closeCmd.ValidArgsFunction = issueIDCompletion
	rootCmd.AddCommand(closeCmd)
}
