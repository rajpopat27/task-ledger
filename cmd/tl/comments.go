package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/utils"
)

var commentsCmd = &cobra.Command{
	Use:     "comments [issue-id]",
	GroupID: "issues",
	Short:   "View or manage comments on an issue",
	Long: `View or manage comments on an issue.

Examples:
  # List all comments on an issue
  tl comments tl-123

  # List comments in JSON format
  tl comments tl-123 --json

  # Add a comment
  tl comments add tl-123 "This is a comment"

  # Add a comment from a file
  tl comments add tl-123 -f notes.txt`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		issueID := args[0]

		if err := ensureStoreActive(); err != nil {
			FatalErrorRespectJSON("getting comments: %v", err)
		}
		ctx := rootCtx
		fullID, err := utils.ResolvePartialID(ctx, store, issueID)
		if err != nil {
			FatalErrorRespectJSON("resolving %s: %v", issueID, err)
		}
		issueID = fullID

		comments, err := store.GetIssueComments(ctx, issueID)
		if err != nil {
			FatalErrorRespectJSON("getting comments: %v", err)
		}

		// Normalize nil to empty slice for consistent JSON output
		if comments == nil {
			comments = make([]*types.Comment, 0)
		}

		if jsonOutput {
			data, err := json.MarshalIndent(comments, "", "  ")
			if err != nil {
				FatalErrorRespectJSON("encoding JSON: %v", err)
			}
			fmt.Println(string(data))
			return
		}

		// Human-readable output
		if len(comments) == 0 {
			fmt.Printf("No comments on %s\n", issueID)
			return
		}

		fmt.Printf("\nComments on %s:\n\n", issueID)
		for _, comment := range comments {
			fmt.Printf("[%s] at %s\n", comment.Author, comment.CreatedAt.Format("2006-01-02 15:04"))
			rendered := ui.RenderMarkdown(comment.Text)
			// TrimRight removes trailing newlines that Glamour adds, preventing extra blank lines
			for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
				fmt.Printf("  %s\n", line)
			}
			fmt.Println()
		}
	},
}

var commentsAddCmd = &cobra.Command{
	Use:   "add [issue-id] [text]",
	Short: "Add a comment to an issue",
	Long: `Add a comment to an issue.

Examples:
  # Add a comment
  tl comments add tl-123 "Working on this now"

  # Add a comment from a file
  tl comments add tl-123 -f notes.txt`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("comment add")
		issueID := args[0]

		// Get comment text from flag or argument
		commentText, _ := cmd.Flags().GetString("file")
		if commentText != "" {
			// Read from file
			data, err := os.ReadFile(commentText) // #nosec G304 - user-provided file path is intentional
			if err != nil {
				FatalErrorRespectJSON("reading file: %v", err)
			}
			commentText = string(data)
		} else if len(args) < 2 {
			FatalErrorRespectJSON("comment text required (use -f to read from file)")
		} else {
			commentText = args[1]
		}

		// Get author from author flag, or use git-aware default
		author, _ := cmd.Flags().GetString("author")
		if author == "" {
			author = getActorWithGit()
		}

		if err := ensureStoreActive(); err != nil {
			FatalErrorRespectJSON("adding comment: %v", err)
		}
		ctx := rootCtx

		fullID, err := utils.ResolvePartialID(ctx, store, issueID)
		if err != nil {
			FatalErrorRespectJSON("resolving %s: %v", issueID, err)
		}
		issueID = fullID

		comment, err := store.AddIssueComment(ctx, issueID, author, commentText)
		if err != nil {
			FatalErrorRespectJSON("adding comment: %v", err)
		}

		if jsonOutput {
			data, err := json.MarshalIndent(comment, "", "  ")
			if err != nil {
				FatalErrorRespectJSON("encoding JSON: %v", err)
			}
			fmt.Println(string(data))
			return
		}

		fmt.Printf("Comment added to %s\n", issueID)
	},
}

// commentCmd is a hidden top-level alias for commentsAddCmd.
var commentCmd = &cobra.Command{
	Use:    "comment [issue-id] [text]",
	Short:  "Add a comment to an issue (alias for 'comments add')",
	Long:   `Add a comment to an issue. This is an alias for 'tl comments add'.`,
	Args:   cobra.MinimumNArgs(1),
	Run:    commentsAddCmd.Run,
	Hidden: true,
}

func init() {
	commentsCmd.AddCommand(commentsAddCmd)
	commentsAddCmd.Flags().StringP("file", "f", "", "Read comment text from file")
	commentsAddCmd.Flags().StringP("author", "a", "", "Add author to comment")

	// Add the same flags to the alias
	commentCmd.Flags().StringP("file", "f", "", "Read comment text from file")
	commentCmd.Flags().StringP("author", "a", "", "Add author to comment")

	// Issue ID completions
	commentsCmd.ValidArgsFunction = issueIDCompletion
	commentsAddCmd.ValidArgsFunction = issueIDCompletion
	commentCmd.ValidArgsFunction = issueIDCompletion

	rootCmd.AddCommand(commentsCmd)
	rootCmd.AddCommand(commentCmd)
}
