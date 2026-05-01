package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"task-ledger/internal/config"
	"task-ledger/internal/debug"
	"task-ledger/internal/hooks"
	"task-ledger/internal/timeparsing"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/validation"
)

var createCmd = &cobra.Command{
	Use:     "create [title]",
	GroupID: "issues",
	Aliases: []string{"new"},
	Short:   "Create a new JSON-backed issue",
	Args:    cobra.MinimumNArgs(0), // Changed to allow no args when using -f
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("create")

		// Original single-issue creation logic
		// File backend shorthand:
		//   tl create FR "Feature title"
		//   tl create epic "Epic title" --parent fr-...
		//   tl create task "Task title" --parent ep-...
		shorthandType := ""
		if len(args) > 0 {
			switch strings.ToLower(args[0]) {
			case "fr", "feature-request", "feature_request":
				shorthandType = string(types.TypeFeatureRequest)
				args = args[1:]
			case "epic":
				shorthandType = string(types.TypeEpic)
				args = args[1:]
			case "task":
				shorthandType = string(types.TypeTask)
				args = args[1:]
			case "bug":
				shorthandType = string(types.TypeBug)
				args = args[1:]
			case "chore":
				shorthandType = string(types.TypeChore)
				args = args[1:]
			}
		}

		// Get title from flag or positional argument
		titleFlag, _ := cmd.Flags().GetString("title")
		var title string

		if len(args) > 0 && titleFlag != "" {
			// Both provided - check if they match
			if args[0] != titleFlag {
				FatalError("cannot specify different titles as both positional argument and --title flag\n  Positional: %q\n  --title:    %q", args[0], titleFlag)
			}
			title = args[0] // They're the same, use either
		} else if len(args) > 0 {
			title = args[0]
		} else if titleFlag != "" {
			title = titleFlag
		} else {
			FatalError("title required")
		}

		// Get silent flag
		silent, _ := cmd.Flags().GetBool("silent")

		// Warn if creating a test issue in the current repository (unless silent mode)
		if strings.HasPrefix(strings.ToLower(title), "test") && !silent && !debug.IsQuiet() {
			fmt.Fprintf(os.Stderr, "%s Creating issue with 'Test' prefix in the current repository.\n", ui.RenderWarn("⚠"))
			fmt.Fprintf(os.Stderr, "  For testing, create a temporary directory and run tl init there first.\n")
		}

		// Get field values
		description, _ := getDescriptionFlag(cmd)

		// Check if description is required by config
		if description == "" && !strings.Contains(strings.ToLower(title), "test") {
			if config.GetBool("create.require-description") {
				FatalError("description is required (set create.require-description: false in config.yaml to disable)")
			}
			// Warn if creating an issue without a description (unless silent mode)
			if !silent && !debug.IsQuiet() {
				fmt.Fprintf(os.Stderr, "%s Creating issue without description.\n", ui.RenderWarn("⚠"))
				fmt.Fprintf(os.Stderr, "  Issues without descriptions lack context for future work.\n")
				fmt.Fprintf(os.Stderr, "  Consider adding --description=\"Why this issue exists and what needs to be done\"\n")
			}
		}

		design, _ := cmd.Flags().GetString("design")
		acceptance, _ := cmd.Flags().GetString("acceptance")
		notes, _ := cmd.Flags().GetString("notes")

		// Parse priority (supports both "1" and "P1" formats)
		priorityStr, _ := cmd.Flags().GetString("priority")
		priority, err := validation.ValidatePriority(priorityStr)
		if err != nil {
			FatalError("%v", err)
		}

		issueType, _ := cmd.Flags().GetString("type")
		if shorthandType != "" && !cmd.Flags().Changed("type") {
			issueType = shorthandType
		}
		if !isFileBackendIssueType(types.IssueType(issueType)) {
			FatalError("unsupported issue type %q for JSON file backend. Valid types: %s", issueType, fileBackendIssueTypesHelp())
		}
		assignee, _ := cmd.Flags().GetString("assignee")

		labels, _ := cmd.Flags().GetStringSlice("labels")
		labelAlias, _ := cmd.Flags().GetStringSlice("label")
		if len(labelAlias) > 0 {
			labels = append(labels, labelAlias...)
		}

		parentID, _ := cmd.Flags().GetString("parent")
		externalRef, _ := cmd.Flags().GetString("external-ref")
		deps, _ := cmd.Flags().GetStringSlice("deps")

		// Parse --due flag (GH#820)
		// Uses layered parsing: compact duration → NLP → date-only → RFC3339
		var dueAt *time.Time
		dueStr, _ := cmd.Flags().GetString("due")
		if dueStr != "" {
			t, err := timeparsing.ParseRelativeTime(dueStr, time.Now())
			if err != nil {
				FatalError("invalid --due format %q. Examples: +6h, tomorrow, next monday, 2025-01-15", dueStr)
			}
			dueAt = &t
		}

		// Parse --defer flag (GH#820)
		var deferUntil *time.Time
		deferStr, _ := cmd.Flags().GetString("defer")
		if deferStr != "" {
			t, err := timeparsing.ParseRelativeTime(deferStr, time.Now())
			if err != nil {
				FatalError("invalid --defer format %q. Examples: +1h, tomorrow, next monday, 2025-01-15", deferStr)
			}
			// Warn if defer date is in the past (user probably meant future)
			if t.Before(time.Now()) && !silent && !debug.IsQuiet() {
				fmt.Fprintf(os.Stderr, "%s Defer date %q is in the past. Issue will appear in tl ready immediately.\n",
					ui.RenderWarn("!"), t.Format("2006-01-02 15:04"))
				fmt.Fprintf(os.Stderr, "  Did you mean a future date? Use --defer=+1h or --defer=tomorrow\n")
			}
			deferUntil = &t
		}

		// Handle --dry-run flag (before --rig to ensure it works with cross-rig creation)
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			// Build preview issue
			var externalRefPtr *string
			if externalRef != "" {
				externalRefPtr = &externalRef
			}
			previewIssue := &types.Issue{
				Title:              title,
				Description:        description,
				Design:             design,
				AcceptanceCriteria: acceptance,
				Notes:              notes,
				Status:             types.StatusOpen,
				Priority:           priority,
				IssueType:          types.IssueType(issueType),
				Assignee:           assignee,
				ExternalRef:        externalRefPtr,
				CreatedBy:          getActorWithGit(),
				Owner:              getOwner(),
				DueAt:              dueAt,
				DeferUntil:         deferUntil,
			}

			if jsonOutput {
				outputJSON(previewIssue)
			} else {
				idDisplay := previewIssue.ID
				if idDisplay == "" {
					idDisplay = "(will be generated)"
				}
				fmt.Printf("%s [DRY RUN] Would create issue:\n", ui.RenderWarn("⚠"))
				fmt.Printf("  ID: %s\n", idDisplay)
				fmt.Printf("  Title: %s\n", previewIssue.Title)
				fmt.Printf("  Type: %s\n", previewIssue.IssueType)
				fmt.Printf("  Priority: P%d\n", previewIssue.Priority)
				fmt.Printf("  Status: %s\n", previewIssue.Status)
				if previewIssue.Assignee != "" {
					fmt.Printf("  Assignee: %s\n", previewIssue.Assignee)
				}
				if previewIssue.Description != "" {
					fmt.Printf("  Description: %s\n", previewIssue.Description)
				}
				if len(labels) > 0 {
					fmt.Printf("  Labels: %s\n", strings.Join(labels, ", "))
				}
				if len(deps) > 0 {
					fmt.Printf("  Dependencies: %s\n", strings.Join(deps, ", "))
				}
			}
			return
		}

		// Get estimate if provided
		var estimatedMinutes *int
		if cmd.Flags().Changed("estimate") {
			est, _ := cmd.Flags().GetInt("estimate")
			if est < 0 {
				FatalError("estimate must be a non-negative number of minutes")
			}
			estimatedMinutes = &est
		}

		// Validate template based on --validate flag or config
		validateTemplate, _ := cmd.Flags().GetBool("validate")
		if validateTemplate {
			// Explicit --validate flag: fail on error
			if err := validation.ValidateTemplate(types.IssueType(issueType), description); err != nil {
				FatalError("%v", err)
			}
		} else {
			// Check validation.on-create config (tl-t7jq)
			validationMode := config.GetString("validation.on-create")
			if validationMode == "error" || validationMode == "warn" {
				if err := validation.ValidateTemplate(types.IssueType(issueType), description); err != nil {
					if validationMode == "error" {
						FatalError("%v", err)
					} else {
						// warn mode: print warning but proceed
						fmt.Fprintf(os.Stderr, "%s %v\n", ui.RenderWarn("⚠"), err)
					}
				}
			}
		}

		// Use global jsonOutput set by PersistentPreRun

		// Validate parent before creating. FileStorage generates IDs from the
		// requested issue type, then moves the file after the parent-child
		// dependency is added below.
		if parentID != "" {
			ctx := rootCtx
			parentIssue, err := store.GetIssue(ctx, parentID)
			if err != nil {
				FatalError("failed to check parent issue: %v", err)
			}
			if parentIssue == nil {
				FatalError("parent issue %s not found", parentID)
			}
		}

		var externalRefPtr *string
		if externalRef != "" {
			externalRefPtr = &externalRef
		}

		issue := &types.Issue{
			Title:              title,
			Description:        description,
			Design:             design,
			AcceptanceCriteria: acceptance,
			Notes:              notes,
			Status:             types.StatusOpen,
			Priority:           priority,
			IssueType:          types.IssueType(issueType),
			Assignee:           assignee,
			ExternalRef:        externalRefPtr,
			EstimatedMinutes:   estimatedMinutes,
			CreatedBy:          getActorWithGit(),
			Owner:              getOwner(),
			DueAt:              dueAt,
			DeferUntil:         deferUntil,
		}

		ctx := rootCtx

		if err := store.CreateIssue(ctx, issue, actor); err != nil {
			FatalError("%v", err)
		}

		// If parent was specified, add parent-child dependency
		if parentID != "" {
			dep := &types.Dependency{
				IssueID:     issue.ID,
				DependsOnID: parentID,
				Type:        types.DepParentChild,
			}
			if err := store.AddDependency(ctx, dep, actor); err != nil {
				WarnError("failed to add parent-child dependency %s -> %s: %v", issue.ID, parentID, err)
			}
		}

		// Add labels if specified
		for _, label := range labels {
			if err := store.AddLabel(ctx, issue.ID, label, actor); err != nil {
				WarnError("failed to add label %s: %v", label, err)
			}
		}

		// Add dependencies if specified (format: type:id or just id for default "blocks" type)
		for _, depSpec := range deps {
			// Skip empty specs (e.g., from trailing commas)
			depSpec = strings.TrimSpace(depSpec)
			if depSpec == "" {
				continue
			}

			var depType types.DependencyType
			var dependsOnID string

			// Parse format: "type:id" or just "id" (defaults to "blocks")
			if strings.Contains(depSpec, ":") {
				parts := strings.SplitN(depSpec, ":", 2)
				if len(parts) != 2 {
					WarnError("invalid dependency format '%s', expected 'type:id' or 'id'", depSpec)
					continue
				}
				depType = types.DependencyType(strings.TrimSpace(parts[0]))
				dependsOnID = strings.TrimSpace(parts[1])
			} else {
				// Default to "blocks" if no type specified
				depType = types.DepBlocks
				dependsOnID = depSpec
			}

			// Validate dependency type
			if !depType.IsValid() {
				WarnError("invalid dependency type '%s' (valid: blocks, related, parent-child, discovered-from)", depType)
				continue
			}

			// Add the dependency
			dep := &types.Dependency{
				IssueID:     issue.ID,
				DependsOnID: dependsOnID,
				Type:        depType,
			}
			if err := store.AddDependency(ctx, dep, actor); err != nil {
				WarnError("failed to add dependency %s -> %s: %v", issue.ID, dependsOnID, err)
			}
		}

		// Run create hook
		if hookRunner != nil {
			hookRunner.Run(hooks.EventCreate, issue)
		}

		if jsonOutput {
			outputJSON(issue)
		} else if silent {
			fmt.Println(issue.ID)
		} else {
			fmt.Printf("%s Created issue: %s\n", ui.RenderPass("✓"), issue.ID)
			fmt.Printf("  Title: %s\n", issue.Title)
			fmt.Printf("  Priority: P%d\n", issue.Priority)
			fmt.Printf("  Status: %s\n", issue.Status)
		}

		// Track as last touched issue
		SetLastTouchedID(issue.ID)
	},
}

func init() {
	createCmd.Flags().String("title", "", "Issue title (alternative to positional argument)")
	createCmd.Flags().Bool("silent", false, "Output only the issue ID (for scripting)")
	createCmd.Flags().Bool("dry-run", false, "Preview what would be created without actually creating")
	registerPriorityFlag(createCmd, "2")
	createCmd.Flags().StringP("type", "t", "task", "Issue type (feature_request|feature|epic|task|bug|chore)")
	registerCommonIssueFlags(createCmd)
	createCmd.Flags().StringSliceP("labels", "l", []string{}, "Labels (comma-separated)")
	createCmd.Flags().StringSlice("label", []string{}, "Alias for --labels")
	_ = createCmd.Flags().MarkHidden("label") // Only fails if flag missing (caught in tests)
	createCmd.Flags().String("parent", "", "Parent issue ID for hierarchical child (e.g., 'tl-a3f8e9')")
	createCmd.Flags().StringSlice("deps", []string{}, "Dependencies in format 'type:id' or 'id' (e.g., 'discovered-from:tl-20,blocks:tl-15' or 'tl-20')")
	createCmd.Flags().IntP("estimate", "e", 0, "Time estimate in minutes (e.g., 60 for 1 hour)")
	createCmd.Flags().Bool("validate", false, "Validate description contains required sections for issue type")
	// Time-based scheduling flags (GH#820)
	// Examples:
	//   --due=+6h           Due in 6 hours
	//   --due=tomorrow      Due tomorrow
	//   --due="next monday" Due next Monday
	//   --due=2025-01-15    Due on specific date
	//   --defer=+1h         Hidden from tl ready for 1 hour
	//   --defer=tomorrow    Hidden until tomorrow
	createCmd.Flags().String("due", "", "Due date/time. Formats: +6h, +1d, +2w, tomorrow, next monday, 2025-01-15")
	createCmd.Flags().String("defer", "", "Defer until date (issue hidden from tl ready until then). Same formats as --due")
	// Note: --json flag is defined as a persistent flag in main.go, not here
	rootCmd.AddCommand(createCmd)
}
