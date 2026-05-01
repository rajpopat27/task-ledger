package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"task-ledger/internal/hooks"
	"task-ledger/internal/timeparsing"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/util"
	"task-ledger/internal/utils"
	"task-ledger/internal/validation"
)

var updateCmd = &cobra.Command{
	Use:     "update [id...]",
	GroupID: "issues",
	Short:   "Update one or more issues",
	Long: `Update one or more issues.

If no issue ID is provided, updates the last touched issue (from most recent
create, update, show, or close operation).`,
	Args: cobra.MinimumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("update")

		// If no IDs provided, use last touched issue
		if len(args) == 0 {
			lastTouched := GetLastTouchedID()
			if lastTouched == "" {
				FatalErrorRespectJSON("no issue ID provided and no last touched issue")
			}
			args = []string{lastTouched}
		}

		updates := make(map[string]interface{})

		if cmd.Flags().Changed("status") {
			status, _ := cmd.Flags().GetString("status")
			updates["status"] = status

			// If status is being set to closed, include session if provided
			if status == "closed" {
				session, _ := cmd.Flags().GetString("session")
				if session == "" {
					session = os.Getenv("CLAUDE_SESSION_ID")
				}
				if session != "" {
					updates["closed_by_session"] = session
				}
			}
		}
		if cmd.Flags().Changed("priority") {
			priorityStr, _ := cmd.Flags().GetString("priority")
			priority, err := validation.ValidatePriority(priorityStr)
			if err != nil {
				FatalErrorRespectJSON("%v", err)
			}
			updates["priority"] = priority
		}
		if cmd.Flags().Changed("title") {
			title, _ := cmd.Flags().GetString("title")
			updates["title"] = title
		}
		if cmd.Flags().Changed("assignee") {
			assignee, _ := cmd.Flags().GetString("assignee")
			updates["assignee"] = assignee
		}
		description, descChanged := getDescriptionFlag(cmd)
		if descChanged {
			updates["description"] = description
		}
		if cmd.Flags().Changed("design") {
			design, _ := cmd.Flags().GetString("design")
			updates["design"] = design
		}
		if cmd.Flags().Changed("notes") {
			notes, _ := cmd.Flags().GetString("notes")
			updates["notes"] = notes
		}
		if cmd.Flags().Changed("acceptance") || cmd.Flags().Changed("acceptance-criteria") {
			var acceptanceCriteria string
			if cmd.Flags().Changed("acceptance") {
				acceptanceCriteria, _ = cmd.Flags().GetString("acceptance")
			} else {
				acceptanceCriteria, _ = cmd.Flags().GetString("acceptance-criteria")
			}
			updates["acceptance_criteria"] = acceptanceCriteria
		}
		if cmd.Flags().Changed("external-ref") {
			externalRef, _ := cmd.Flags().GetString("external-ref")
			updates["external_ref"] = externalRef
		}
		if cmd.Flags().Changed("estimate") {
			estimate, _ := cmd.Flags().GetInt("estimate")
			if estimate < 0 {
				FatalErrorRespectJSON("estimate must be a non-negative number of minutes")
			}
			updates["estimated_minutes"] = estimate
		}
		if cmd.Flags().Changed("add-label") {
			addLabels, _ := cmd.Flags().GetStringSlice("add-label")
			updates["add_labels"] = addLabels
		}
		if cmd.Flags().Changed("remove-label") {
			removeLabels, _ := cmd.Flags().GetStringSlice("remove-label")
			updates["remove_labels"] = removeLabels
		}
		if cmd.Flags().Changed("set-labels") {
			setLabels, _ := cmd.Flags().GetStringSlice("set-labels")
			updates["set_labels"] = setLabels
		}
		if cmd.Flags().Changed("parent") {
			parent, _ := cmd.Flags().GetString("parent")
			updates["parent"] = parent
		}
		if cmd.Flags().Changed("type") {
			issueType, _ := cmd.Flags().GetString("type")
			issueType = util.NormalizeIssueType(issueType)
			if !isFileBackendIssueType(types.IssueType(issueType)) {
				FatalErrorRespectJSON("unsupported issue type %q for JSON file backend. Valid types: %s", issueType, fileBackendIssueTypesHelp())
			}
			updates["issue_type"] = issueType
		}
		// Time-based scheduling flags (GH#820)
		if cmd.Flags().Changed("due") {
			dueStr, _ := cmd.Flags().GetString("due")
			if dueStr == "" {
				// Empty string clears the due date
				updates["due_at"] = nil
			} else {
				t, err := timeparsing.ParseRelativeTime(dueStr, time.Now())
				if err != nil {
					FatalErrorRespectJSON("invalid --due format %q. Examples: +6h, tomorrow, next monday, 2025-01-15", dueStr)
				}
				updates["due_at"] = t
			}
		}
		if cmd.Flags().Changed("defer") {
			deferStr, _ := cmd.Flags().GetString("defer")
			if deferStr == "" {
				// Empty string clears the defer_until
				updates["defer_until"] = nil
			} else {
				t, err := timeparsing.ParseRelativeTime(deferStr, time.Now())
				if err != nil {
					FatalErrorRespectJSON("invalid --defer format %q. Examples: +1h, tomorrow, next monday, 2025-01-15", deferStr)
				}
				// Warn if defer date is in the past (user probably meant future)
				if t.Before(time.Now()) && !jsonOutput {
					fmt.Fprintf(os.Stderr, "%s Defer date %q is in the past. Issue will appear in tl ready immediately.\n",
						ui.RenderWarn("!"), t.Format("2006-01-02 15:04"))
					fmt.Fprintf(os.Stderr, "  Did you mean a future date? Use --defer=+1h or --defer=tomorrow\n")
				}
				updates["defer_until"] = t
			}
		}

		// Get claim flag
		claimFlag, _ := cmd.Flags().GetBool("claim")

		if len(updates) == 0 && !claimFlag {
			fmt.Println("No updates specified")
			return
		}

		ctx := rootCtx

		updatedIssues := []*types.Issue{}
		var firstUpdatedID string // Track first successful update for last-touched
		hadError := false
		for _, id := range args {
			resolvedID, err := utils.ResolvePartialID(ctx, store, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving %s: %v\n", id, err)
				hadError = true
				continue
			}
			issue, err := store.GetIssue(ctx, resolvedID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting %s: %v\n", resolvedID, err)
				hadError = true
				continue
			}
			if issue == nil {
				fmt.Fprintf(os.Stderr, "Issue %s not found\n", id)
				hadError = true
				continue
			}

			if err := validateIssueUpdatable(id, issue); err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
				hadError = true
				continue
			}

			// Handle claim operation atomically
			if claimFlag {
				// Check if already claimed (has non-empty assignee)
				if issue.Assignee != "" {
					fmt.Fprintf(os.Stderr, "Error claiming %s: already claimed by %s\n", id, issue.Assignee)
					hadError = true
					continue
				}
				// Atomically set assignee and status
				claimUpdates := map[string]interface{}{
					"assignee": actor,
					"status":   "in_progress",
				}
				if err := store.UpdateIssue(ctx, resolvedID, claimUpdates, actor); err != nil {
					fmt.Fprintf(os.Stderr, "Error claiming %s: %v\n", id, err)
					hadError = true
					continue
				}
			}

			// Apply regular field updates if any
			regularUpdates := make(map[string]interface{})
			for k, v := range updates {
				if k != "add_labels" && k != "remove_labels" && k != "set_labels" && k != "parent" {
					regularUpdates[k] = v
				}
			}
			if len(regularUpdates) > 0 {
				if err := store.UpdateIssue(ctx, resolvedID, regularUpdates, actor); err != nil {
					fmt.Fprintf(os.Stderr, "Error updating %s: %v\n", id, err)
					hadError = true
					continue
				}
			}

			// Handle label operations
			var setLabels, addLabels, removeLabels []string
			if v, ok := updates["set_labels"].([]string); ok {
				setLabels = v
			}
			if v, ok := updates["add_labels"].([]string); ok {
				addLabels = v
			}
			if v, ok := updates["remove_labels"].([]string); ok {
				removeLabels = v
			}
			if len(setLabels) > 0 || len(addLabels) > 0 || len(removeLabels) > 0 {
				if err := applyLabelUpdates(ctx, store, resolvedID, actor, setLabels, addLabels, removeLabels); err != nil {
					fmt.Fprintf(os.Stderr, "Error updating labels for %s: %v\n", id, err)
					hadError = true
					continue
				}
			}

			// Handle parent reparenting
			if newParent, ok := updates["parent"].(string); ok {
				// Validate new parent exists (unless empty string to remove parent)
				if newParent != "" {
					parentIssue, err := store.GetIssue(ctx, newParent)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error getting parent %s: %v\n", newParent, err)
						hadError = true
						continue
					}
					if parentIssue == nil {
						fmt.Fprintf(os.Stderr, "Error: parent issue %s not found\n", newParent)
						hadError = true
						continue
					}
				}

				// Find and remove existing parent-child dependency
				deps, err := store.GetDependencyRecords(ctx, resolvedID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error getting dependencies for %s: %v\n", id, err)
					hadError = true
					continue
				}
				for _, dep := range deps {
					if dep.Type == types.DepParentChild {
						if err := store.RemoveDependency(ctx, resolvedID, dep.DependsOnID, actor); err != nil {
							fmt.Fprintf(os.Stderr, "Error removing old parent dependency: %v\n", err)
						}
						break
					}
				}

				// Add new parent-child dependency (if not removing parent)
				if newParent != "" {
					newDep := &types.Dependency{
						IssueID:     resolvedID,
						DependsOnID: newParent,
						Type:        types.DepParentChild,
					}
					if err := store.AddDependency(ctx, newDep, actor); err != nil {
						fmt.Fprintf(os.Stderr, "Error adding parent dependency: %v\n", err)
						hadError = true
						continue
					}
				}
			}

			// Run update hook
			updatedIssue, _ := store.GetIssue(ctx, resolvedID)
			if updatedIssue != nil && hookRunner != nil {
				hookRunner.Run(hooks.EventUpdate, updatedIssue)
			}

			if jsonOutput {
				if updatedIssue != nil {
					updatedIssues = append(updatedIssues, updatedIssue)
				}
			} else {
				fmt.Printf("%s Updated issue: %s\n", ui.RenderPass("✓"), resolvedID)
			}

			// Track first successful update for last-touched
			if firstUpdatedID == "" {
				firstUpdatedID = resolvedID
			}
		}

		// Set last touched after all updates complete
		if firstUpdatedID != "" {
			SetLastTouchedID(firstUpdatedID)
		}

		if jsonOutput && len(updatedIssues) > 0 {
			outputJSON(updatedIssues)
		}
		if hadError {
			os.Exit(1)
		}
	},
}

func init() {
	updateCmd.Flags().StringP("status", "s", "", "New status")
	registerPriorityFlag(updateCmd, "")
	updateCmd.Flags().String("title", "", "New title")
	updateCmd.Flags().StringP("type", "t", "", "New type (feature_request|feature|epic|task|bug|chore)")
	registerCommonIssueFlags(updateCmd)
	updateCmd.Flags().String("acceptance-criteria", "", "DEPRECATED: use --acceptance")
	_ = updateCmd.Flags().MarkHidden("acceptance-criteria") // Only fails if flag missing (caught in tests)
	updateCmd.Flags().IntP("estimate", "e", 0, "Time estimate in minutes (e.g., 60 for 1 hour)")
	updateCmd.Flags().StringSlice("add-label", nil, "Add labels (repeatable)")
	updateCmd.Flags().StringSlice("remove-label", nil, "Remove labels (repeatable)")
	updateCmd.Flags().StringSlice("set-labels", nil, "Set labels, replacing all existing (repeatable)")
	updateCmd.Flags().String("parent", "", "New parent issue ID (reparents the issue, use empty string to remove parent)")
	updateCmd.Flags().Bool("claim", false, "Atomically claim the issue (sets assignee to you, status to in_progress; fails if already claimed)")
	updateCmd.Flags().String("session", "", "Claude Code session ID for status=closed (or set CLAUDE_SESSION_ID env var)")
	// Time-based scheduling flags (GH#820)
	// Examples:
	//   --due=+6h           Due in 6 hours
	//   --due=tomorrow      Due tomorrow
	//   --due="next monday" Due next Monday
	//   --due=2025-01-15    Due on specific date
	//   --due=""            Clear due date
	//   --defer=+1h         Hidden from tl ready for 1 hour
	//   --defer=""          Clear defer (show in tl ready immediately)
	updateCmd.Flags().String("due", "", "Due date/time (empty to clear). Formats: +6h, +1d, +2w, tomorrow, next monday, 2025-01-15")
	updateCmd.Flags().String("defer", "", "Defer until date (empty to clear). Issue hidden from tl ready until then")
	updateCmd.ValidArgsFunction = issueIDCompletion
	rootCmd.AddCommand(updateCmd)
}
