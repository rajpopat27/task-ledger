package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/utils"
)

var showCmd = &cobra.Command{
	Use:     "show [id...]",
	GroupID: "issues",
	Short:   "Show issue details",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		short, _ := cmd.Flags().GetBool("short")
		ctx := rootCtx

		details := make([]*types.IssueDetails, 0, len(args))
		for _, arg := range args {
			id, err := utils.ResolvePartialID(ctx, store, arg)
			if err != nil {
				FatalErrorRespectJSON("%v", err)
			}
			issue, err := store.GetIssue(ctx, id)
			if err != nil {
				FatalErrorRespectJSON("getting issue %s: %v", id, err)
			}
			if issue == nil {
				FatalErrorRespectJSON("no issue found for %s", arg)
			}

			detail := issueDetails(ctx, issue)
			details = append(details, detail)
			SetLastTouchedID(id)
		}

		if jsonOutput {
			if len(details) == 1 {
				outputJSON(details[0])
			} else {
				outputJSON(details)
			}
			return
		}

		for i, detail := range details {
			if i > 0 {
				fmt.Println()
			}
			printIssueDetail(detail, short)
		}
	},
}

func issueDetails(ctx context.Context, issue *types.Issue) *types.IssueDetails {
	labels, _ := store.GetLabels(ctx, issue.ID)
	deps, _ := store.GetDependenciesWithMetadata(ctx, issue.ID)
	dependents, _ := store.GetDependentsWithMetadata(ctx, issue.ID)
	comments, _ := store.GetIssueComments(ctx, issue.ID)

	var parent *string
	for _, dep := range issue.Dependencies {
		if dep.Type == types.DepParentChild {
			parentID := dep.DependsOnID
			parent = &parentID
			break
		}
	}

	return &types.IssueDetails{
		Issue:        *issue,
		Labels:       labels,
		Dependencies: deps,
		Dependents:   dependents,
		Comments:     comments,
		Parent:       parent,
	}
}

func printIssueDetail(detail *types.IssueDetails, short bool) {
	issue := &detail.Issue
	if short {
		fmt.Printf("%s %s\n", issue.ID, issue.Title)
		return
	}

	fmt.Printf("%s %s\n", ui.RenderID(issue.ID), issue.Title)
	fmt.Printf("Status: %s\n", issue.Status)
	fmt.Printf("Type: %s\n", issue.IssueType)
	fmt.Printf("Priority: P%d\n", issue.Priority)
	if issue.Assignee != "" {
		fmt.Printf("Assignee: %s\n", issue.Assignee)
	}
	if len(detail.Labels) > 0 {
		fmt.Printf("Labels: %s\n", strings.Join(detail.Labels, ", "))
	}
	if detail.Parent != nil {
		fmt.Printf("Parent: %s\n", *detail.Parent)
	}
	if issue.Description != "" {
		fmt.Printf("\n%s\n", issue.Description)
	}
	if issue.Design != "" {
		fmt.Printf("\nDesign:\n%s\n", issue.Design)
	}
	if issue.AcceptanceCriteria != "" {
		fmt.Printf("\nAcceptance:\n%s\n", issue.AcceptanceCriteria)
	}
	if issue.Notes != "" {
		fmt.Printf("\nNotes:\n%s\n", issue.Notes)
	}
	if len(detail.Dependencies) > 0 {
		fmt.Println("\nBlocks on:")
		for _, dep := range detail.Dependencies {
			fmt.Printf("  %s %s\n", dep.ID, dep.Title)
		}
	}
	if len(detail.Dependents) > 0 {
		fmt.Println("\nBlocks:")
		for _, dep := range detail.Dependents {
			fmt.Printf("  %s %s\n", dep.ID, dep.Title)
		}
	}
	if len(detail.Comments) > 0 {
		fmt.Println("\nComments:")
		for _, comment := range detail.Comments {
			fmt.Printf("  [%s] %s\n", comment.Author, comment.Text)
		}
	}
}

func init() {
	showCmd.Flags().Bool("short", false, "Show a compact one-line summary")
	showCmd.ValidArgsFunction = issueIDCompletion
	rootCmd.AddCommand(showCmd)
}
