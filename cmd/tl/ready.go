package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"task-ledger/internal/config"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/util"
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show ready work (no blockers)",
	Long:  `Show ready work: issues with no open blockers. By default, this includes open or in_progress issues.`,
	Run: func(cmd *cobra.Command, args []string) {
		limit, _ := cmd.Flags().GetInt("limit")
		status, _ := cmd.Flags().GetString("status")
		assignee, _ := cmd.Flags().GetString("assignee")
		unassigned, _ := cmd.Flags().GetBool("unassigned")
		sortPolicy, _ := cmd.Flags().GetString("sort")
		labels, _ := cmd.Flags().GetStringSlice("label")
		labelsAny, _ := cmd.Flags().GetStringSlice("label-any")
		issueType, _ := cmd.Flags().GetString("type")
		issueType = util.NormalizeIssueType(issueType)
		parentID, _ := cmd.Flags().GetString("parent")
		prettyFormat, _ := cmd.Flags().GetBool("pretty")
		includeDeferred, _ := cmd.Flags().GetBool("include-deferred")
		// Use global jsonOutput set by PersistentPreRun (respects config.yaml + env vars)

		// Normalize labels: trim, dedupe, remove empty
		labels = util.NormalizeLabels(labels)
		labelsAny = util.NormalizeLabels(labelsAny)

		// Apply directory-aware label scoping if no labels explicitly provided (GH#541)
		if len(labels) == 0 && len(labelsAny) == 0 {
			if dirLabels := config.GetDirectoryLabels(); len(dirLabels) > 0 {
				labelsAny = dirLabels
			}
		}

		filter := types.WorkFilter{
			Type:            issueType,
			Limit:           limit,
			Unassigned:      unassigned,
			SortPolicy:      types.SortPolicy(sortPolicy),
			Labels:          labels,
			LabelsAny:       labelsAny,
			IncludeDeferred: includeDeferred, // GH#820: respect --include-deferred flag
		}
		if status != "" {
			readyStatus := types.Status(status)
			if readyStatus != types.StatusOpen && readyStatus != types.StatusInProgress {
				fmt.Fprintf(os.Stderr, "Error: ready --status only supports open or in_progress\n")
				os.Exit(1)
			}
			filter.Status = readyStatus
		}
		// Use Changed() to properly handle P0 (priority=0)
		if cmd.Flags().Changed("priority") {
			priority, _ := cmd.Flags().GetInt("priority")
			filter.Priority = &priority
		}
		if assignee != "" && !unassigned {
			filter.Assignee = &assignee
		}
		if parentID != "" {
			filter.ParentID = &parentID
		}
		// Validate sort policy
		if !filter.SortPolicy.IsValid() {
			fmt.Fprintf(os.Stderr, "Error: invalid sort policy '%s'. Valid values: hybrid, priority, oldest\n", sortPolicy)
			os.Exit(1)
		}
		ctx := rootCtx

		issues, err := store.GetReadyWork(ctx, filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if jsonOutput {
			// Always output array, even if empty
			if issues == nil {
				issues = []*types.Issue{}
			}
			outputJSON(issues)
			return
		}

		if len(issues) == 0 {
			// Check if there are any open issues at all
			hasOpenIssues := false
			if stats, statsErr := store.GetStatistics(ctx); statsErr == nil {
				hasOpenIssues = stats.OpenIssues > 0 || stats.InProgressIssues > 0
			}
			if hasOpenIssues {
				fmt.Printf("\n%s No ready work found (all issues have blocking dependencies)\n\n",
					ui.RenderWarn("✨"))
			} else {
				fmt.Printf("\n%s No open issues\n\n", ui.RenderPass("✨"))
			}
			return
		}
		if prettyFormat {
			displayPrettyList(issues, false)
		} else {
			fmt.Printf("\n%s Ready work (%d issues with no blockers):\n\n", ui.RenderAccent("📋"), len(issues))
			for i, issue := range issues {
				fmt.Printf("%d. [%s] [%s] %s: %s\n", i+1,
					ui.RenderPriority(issue.Priority),
					ui.RenderType(string(issue.IssueType)),
					ui.RenderID(issue.ID), issue.Title)
				if issue.EstimatedMinutes != nil {
					fmt.Printf("   Estimate: %d min\n", *issue.EstimatedMinutes)
				}
				if issue.Assignee != "" {
					fmt.Printf("   Assignee: %s\n", issue.Assignee)
				}
			}
			fmt.Println()
		}
	},
}
var blockedCmd = &cobra.Command{
	Use:   "blocked",
	Short: "Show blocked issues",
	Run: func(cmd *cobra.Command, args []string) {
		// Use global jsonOutput set by PersistentPreRun (respects config.yaml + env vars)
		ctx := rootCtx
		parentID, _ := cmd.Flags().GetString("parent")
		var blockedFilter types.WorkFilter
		if parentID != "" {
			blockedFilter.ParentID = &parentID
		}
		blocked, err := store.GetBlockedIssues(ctx, blockedFilter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if jsonOutput {
			// Always output array, even if empty
			if blocked == nil {
				blocked = []*types.BlockedIssue{}
			}
			outputJSON(blocked)
			return
		}
		if len(blocked) == 0 {
			fmt.Printf("\n%s No blocked issues\n\n", ui.RenderPass("✨"))
			return
		}
		fmt.Printf("\n%s Blocked issues (%d):\n\n", ui.RenderFail("🚫"), len(blocked))
		for _, issue := range blocked {
			fmt.Printf("[%s] %s: %s\n",
				ui.RenderPriority(issue.Priority),
				ui.RenderID(issue.ID), issue.Title)
			blockedBy := issue.BlockedBy
			if blockedBy == nil {
				blockedBy = []string{}
			}
			fmt.Printf("  Blocked by %d open dependencies: %v\n",
				issue.BlockedByCount, blockedBy)
			fmt.Println()
		}
	},
}

func init() {
	readyCmd.Flags().IntP("limit", "n", 10, "Maximum issues to show")
	readyCmd.Flags().String(
		"status",
		"",
		"Filter by ready status (open, in_progress)",
	)
	readyCmd.Flags().IntP("priority", "p", 0, "Filter by priority")
	readyCmd.Flags().StringP("assignee", "a", "", "Filter by assignee")
	readyCmd.Flags().BoolP("unassigned", "u", false, "Show only unassigned issues")
	readyCmd.Flags().StringP("sort", "s", "hybrid", "Sort policy: hybrid (default), priority, oldest")
	readyCmd.Flags().StringSliceP("label", "l", []string{}, "Filter by labels (AND: must have ALL). Can combine with --label-any")
	readyCmd.Flags().StringSlice("label-any", []string{}, "Filter by labels (OR: must have AT LEAST ONE). Can combine with --label")
	readyCmd.Flags().StringP("type", "t", "", "Filter by issue type (task, bug, feature, feature_request, epic, chore). Aliases: feat→feature, fr→feature_request")
	readyCmd.Flags().String("parent", "", "Filter to descendants of this issue/epic")
	readyCmd.Flags().Bool("pretty", false, "Display issues in a tree format with status/priority symbols")
	readyCmd.Flags().Bool("include-deferred", false, "Include issues with future defer_until timestamps")
	rootCmd.AddCommand(readyCmd)
	blockedCmd.Flags().String("parent", "", "Filter to descendants of this issue/epic")
	rootCmd.AddCommand(blockedCmd)
}
