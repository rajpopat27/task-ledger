package main

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
	"task-ledger/internal/config"
	"task-ledger/internal/storage"
	"task-ledger/internal/timeparsing"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/util"
	"task-ledger/internal/validation"
)

// parseTimeFlag parses time strings using the layered time parsing architecture.
// Supports compact durations (+6h, -1d), natural language (tomorrow, next monday),
// and absolute formats (2006-01-02, RFC3339).
func parseTimeFlag(s string) (time.Time, error) {
	return timeparsing.ParseRelativeTime(s, time.Now())
}

// pinIndicator returns a pushpin emoji prefix for pinned issues
func pinIndicator(issue *types.Issue) string {
	if issue.Pinned {
		return "📌 "
	}
	return ""
}

// Priority tags for pretty output - simple text, semantic colors applied via ui package
// Design principle: only P0/P1 get color for attention, P2-P4 are neutral
func renderPriorityTag(priority int) string {
	return ui.RenderPriority(priority)
}

// renderStatusIcon returns the status icon with semantic coloring applied
// Delegates to the shared ui.RenderStatusIcon for consistency across commands
func renderStatusIcon(status types.Status) string {
	return ui.RenderStatusIcon(string(status))
}

// formatPrettyIssue formats a single issue for pretty output
// Uses semantic colors: status icon colored, priority P0/P1 colored, rest neutral
func formatPrettyIssue(issue *types.Issue) string {
	// Use shared helpers from ui package
	statusIcon := ui.RenderStatusIcon(string(issue.Status))
	priorityTag := renderPriorityTag(issue.Priority)

	// Type badge - only show for notable types
	typeBadge := ""
	switch issue.IssueType {
	case "epic":
		typeBadge = ui.TypeEpicStyle.Render("[epic]") + " "
	case "bug":
		typeBadge = ui.TypeBugStyle.Render("[bug]") + " "
	}

	// Format: STATUS_ICON ID PRIORITY [Type] Title
	// Priority uses ● icon with color, no brackets needed
	// Closed issues: entire line is muted
	if issue.Status == types.StatusClosed {
		return fmt.Sprintf("%s %s %s %s%s",
			statusIcon,
			ui.RenderMuted(issue.ID),
			ui.RenderMuted(fmt.Sprintf("● P%d", issue.Priority)),
			ui.RenderMuted(string(issue.IssueType)),
			ui.RenderMuted(" "+issue.Title))
	}

	return fmt.Sprintf("%s %s %s %s%s", statusIcon, issue.ID, priorityTag, typeBadge, issue.Title)
}

// buildIssueTreeWithDeps builds parent-child tree using dependency records
// If allDeps is nil, falls back to dotted ID hierarchy (e.g., "parent.1")
// Treats any dependency on an epic as a parent-child relationship
func buildIssueTreeWithDeps(issues []*types.Issue, allDeps map[string][]*types.Dependency) (roots []*types.Issue, childrenMap map[string][]*types.Issue) {
	issueMap := make(map[string]*types.Issue)
	childrenMap = make(map[string][]*types.Issue)
	isChild := make(map[string]bool)

	// Build issue map and identify epics
	epicIDs := make(map[string]bool)
	for _, issue := range issues {
		issueMap[issue.ID] = issue
		if issue.IssueType == "epic" {
			epicIDs[issue.ID] = true
		}
	}

	// Use dependency records to find parent-child relationships.
	for issueID, deps := range allDeps {
		for _, dep := range deps {
			parentID := dep.DependsOnID
			child, childOk := issueMap[issueID]
			_, parentOk := issueMap[parentID]
			if !childOk || !parentOk {
				continue
			}

			if dep.Type == types.DepParentChild || epicIDs[parentID] {
				childrenMap[parentID] = append(childrenMap[parentID], child)
				isChild[issueID] = true
			}
		}
	}

	// Fallback: check for hierarchical subtask IDs (e.g., "parent.1")
	for _, issue := range issues {
		if isChild[issue.ID] {
			continue // Already a child via dependency
		}
		if strings.Contains(issue.ID, ".") {
			parts := strings.Split(issue.ID, ".")
			parentID := strings.Join(parts[:len(parts)-1], ".")
			if _, exists := issueMap[parentID]; exists {
				childrenMap[parentID] = append(childrenMap[parentID], issue)
				isChild[issue.ID] = true
				continue
			}
		}
	}

	// Roots are issues that aren't children of any other issue
	for _, issue := range issues {
		if !isChild[issue.ID] {
			roots = append(roots, issue)
		}
	}

	return roots, childrenMap
}

// printPrettyTree recursively prints the issue tree
// Children are sorted by priority (P0 first) for intuitive reading
func printPrettyTree(childrenMap map[string][]*types.Issue, parentID string, prefix string) {
	children := childrenMap[parentID]

	// Sort children by priority (ascending: P0 before P1 before P2...)
	slices.SortFunc(children, func(a, b *types.Issue) int {
		return cmp.Compare(a.Priority, b.Priority)
	})

	for i, child := range children {
		isLast := i == len(children)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}
		fmt.Printf("%s%s%s\n", prefix, connector, formatPrettyIssue(child))

		extension := "│   "
		if isLast {
			extension = "    "
		}
		printPrettyTree(childrenMap, child.ID, prefix+extension)
	}
}

// displayPrettyList displays issues in pretty tree format.
func displayPrettyList(issues []*types.Issue, showHeader bool) {
	displayPrettyListWithDeps(issues, showHeader, nil)
}

// displayPrettyListWithDeps displays issues in tree format using dependency data
func displayPrettyListWithDeps(issues []*types.Issue, showHeader bool, allDeps map[string][]*types.Dependency) {
	if showHeader {
		// Clear screen and show header
		fmt.Print("\033[2J\033[H")
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Task Ledger - Open & In Progress (%s)\n", time.Now().Format("15:04:05"))
		fmt.Println(strings.Repeat("=", 80))
		fmt.Println()
	}

	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	roots, childrenMap := buildIssueTreeWithDeps(issues, allDeps)

	for _, issue := range roots {
		fmt.Println(formatPrettyIssue(issue))
		printPrettyTree(childrenMap, issue.ID, "")
	}

	// Summary
	fmt.Println()
	fmt.Println(strings.Repeat("-", 80))
	openCount := 0
	inProgressCount := 0
	reviewCount := 0
	for _, issue := range issues {
		switch issue.Status {
		case "open":
			openCount++
		case "in_progress":
			inProgressCount++
		case "in_review", "human_review":
			reviewCount++
		}
	}
	fmt.Printf(
		"Total: %d issues (%d open, %d in progress, %d in review)\n",
		len(issues),
		openCount,
		inProgressCount,
		reviewCount,
	)
	fmt.Println()
	fmt.Println("Status: ○ open  ◐ in_progress/in_review/human_review  ● blocked  ✓ closed  ❄ deferred")
}

// sortIssues sorts a slice of issues by the specified field and direction
func sortIssues(issues []*types.Issue, sortBy string, reverse bool) {
	if sortBy == "" {
		return
	}

	slices.SortFunc(issues, func(a, b *types.Issue) int {
		var result int

		switch sortBy {
		case "priority":
			// Lower priority numbers come first (P0 > P1 > P2 > P3 > P4)
			result = cmp.Compare(a.Priority, b.Priority)
		case "created":
			// Default: newest first (descending)
			result = b.CreatedAt.Compare(a.CreatedAt)
		case "updated":
			// Default: newest first (descending)
			result = b.UpdatedAt.Compare(a.UpdatedAt)
		case "closed":
			// Default: newest first (descending)
			// Handle nil ClosedAt values
			if a.ClosedAt == nil && b.ClosedAt == nil {
				result = 0
			} else if a.ClosedAt == nil {
				result = 1 // nil sorts last
			} else if b.ClosedAt == nil {
				result = -1 // non-nil sorts before nil
			} else {
				result = b.ClosedAt.Compare(*a.ClosedAt)
			}
		case "status":
			result = cmp.Compare(a.Status, b.Status)
		case "id":
			result = cmp.Compare(a.ID, b.ID)
		case "title":
			result = cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
		case "type":
			result = cmp.Compare(a.IssueType, b.IssueType)
		case "assignee":
			result = cmp.Compare(a.Assignee, b.Assignee)
		default:
			// Unknown sort field, no sorting
			result = 0
		}

		if reverse {
			return -result
		}
		return result
	})
}

// formatIssueLong formats a single issue in long format to a buffer
func formatIssueLong(buf *strings.Builder, issue *types.Issue, labels []string) {
	status := string(issue.Status)
	if status == "closed" {
		line := fmt.Sprintf("%s%s [P%d] [%s] %s\n  %s",
			pinIndicator(issue), issue.ID, issue.Priority,
			issue.IssueType, status, issue.Title)
		buf.WriteString(ui.RenderClosedLine(line))
		buf.WriteString("\n")
	} else {
		buf.WriteString(fmt.Sprintf("%s%s [%s] [%s] %s\n",
			pinIndicator(issue),
			ui.RenderID(issue.ID),
			ui.RenderPriority(issue.Priority),
			ui.RenderType(string(issue.IssueType)),
			ui.RenderStatus(status)))
		buf.WriteString(fmt.Sprintf("  %s\n", issue.Title))
	}
	if issue.Assignee != "" {
		buf.WriteString(fmt.Sprintf("  Assignee: %s\n", issue.Assignee))
	}
	if len(labels) > 0 {
		buf.WriteString(fmt.Sprintf("  Labels: %v\n", labels))
	}
	buf.WriteString("\n")
}

// formatAgentIssue formats a single issue in ultra-compact agent mode format
// Output: just "ID: Title" - no colors, no emojis, no brackets
func formatAgentIssue(buf *strings.Builder, issue *types.Issue) {
	buf.WriteString(fmt.Sprintf("%s: %s\n", issue.ID, issue.Title))
}

// formatIssueCompact formats a single issue in compact format to a buffer
// Uses status icons for better scanability - consistent with tl graph
// Format: [icon] [pin] ID [Priority] [Type] @assignee [labels] - Title
func formatIssueCompact(buf *strings.Builder, issue *types.Issue, labels []string) {
	labelsStr := ""
	if len(labels) > 0 {
		labelsStr = fmt.Sprintf(" %v", labels)
	}
	assigneeStr := ""
	if issue.Assignee != "" {
		assigneeStr = fmt.Sprintf(" @%s", issue.Assignee)
	}

	// Get styled status icon
	statusIcon := renderStatusIcon(issue.Status)

	if issue.Status == types.StatusClosed {
		// Closed issues: entire line muted (fades visually)
		line := fmt.Sprintf("%s %s%s [P%d] [%s]%s%s - %s",
			statusIcon, pinIndicator(issue), issue.ID, issue.Priority,
			issue.IssueType, assigneeStr, labelsStr, issue.Title)
		buf.WriteString(ui.RenderClosedLine(line))
		buf.WriteString("\n")
	} else {
		// Active issues: status icon + semantic colors for priority/type
		buf.WriteString(fmt.Sprintf("%s %s%s [%s] [%s]%s%s - %s\n",
			statusIcon,
			pinIndicator(issue),
			ui.RenderID(issue.ID),
			ui.RenderPriority(issue.Priority),
			ui.RenderType(string(issue.IssueType)),
			assigneeStr, labelsStr, issue.Title))
	}
}

var listCmd = &cobra.Command{
	Use:     "list",
	GroupID: "issues",
	Short:   "List issues",
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetString("status")
		assignee, _ := cmd.Flags().GetString("assignee")
		issueType, _ := cmd.Flags().GetString("type")
		issueType = util.NormalizeIssueType(issueType)
		limit, _ := cmd.Flags().GetInt("limit")
		allFlag, _ := cmd.Flags().GetBool("all")
		formatStr, _ := cmd.Flags().GetString("format")
		labels, _ := cmd.Flags().GetStringSlice("label")
		labelsAny, _ := cmd.Flags().GetStringSlice("label-any")
		titleSearch, _ := cmd.Flags().GetString("title")
		idFilter, _ := cmd.Flags().GetString("id")
		longFormat, _ := cmd.Flags().GetBool("long")
		sortBy, _ := cmd.Flags().GetString("sort")
		reverse, _ := cmd.Flags().GetBool("reverse")

		// Pattern matching flags
		titleContains, _ := cmd.Flags().GetString("title-contains")
		descContains, _ := cmd.Flags().GetString("desc-contains")
		notesContains, _ := cmd.Flags().GetString("notes-contains")

		// Empty/null check flags
		emptyDesc, _ := cmd.Flags().GetBool("empty-description")
		noAssignee, _ := cmd.Flags().GetBool("no-assignee")
		noLabels, _ := cmd.Flags().GetBool("no-labels")

		// Pinned filtering flags
		pinnedFlag, _ := cmd.Flags().GetBool("pinned")
		noPinnedFlag, _ := cmd.Flags().GetBool("no-pinned")

		// Parent filtering (--filter-parent is alias for --parent)
		parentID, _ := cmd.Flags().GetString("parent")
		if parentID == "" {
			parentID, _ = cmd.Flags().GetString("filter-parent")
		}

		// Time-based scheduling filters (GH#820)
		deferredFlag, _ := cmd.Flags().GetBool("deferred")
		overdueFlag, _ := cmd.Flags().GetBool("overdue")

		// Pretty flags.
		prettyFormat, _ := cmd.Flags().GetBool("pretty")
		treeFormat, _ := cmd.Flags().GetBool("tree")
		prettyFormat = prettyFormat || treeFormat // --tree is alias for --pretty

		// Pager control (tl-jdz3)
		noPager, _ := cmd.Flags().GetBool("no-pager")

		// Ready filter (tl-ihu31)
		readyFlag, _ := cmd.Flags().GetBool("ready")

		// Use global jsonOutput set by PersistentPreRun

		// Normalize labels: trim, dedupe, remove empty
		labels = util.NormalizeLabels(labels)
		labelsAny = util.NormalizeLabels(labelsAny)

		// Apply directory-aware label scoping if no labels explicitly provided (GH#541)
		if len(labels) == 0 && len(labelsAny) == 0 {
			if dirLabels := config.GetDirectoryLabels(); len(dirLabels) > 0 {
				labelsAny = dirLabels
			}
		}

		// Handle limit: --limit 0 means unlimited (explicit override)
		// Otherwise use the value (default 50 or user-specified)
		// Agent mode uses lower default (20) for context efficiency
		effectiveLimit := limit
		if cmd.Flags().Changed("limit") && limit == 0 {
			effectiveLimit = 0 // Explicit unlimited
		} else if !cmd.Flags().Changed("limit") && ui.IsAgentMode() {
			effectiveLimit = 20 // Agent mode default
		}

		filter := types.IssueFilter{
			Limit: effectiveLimit,
		}

		// --ready flag: show only open issues.
		if readyFlag {
			s := types.StatusOpen
			filter.Status = &s
		} else if status != "" && status != "all" {
			s := types.Status(status)
			filter.Status = &s
		}

		// Default to non-closed issues unless --all or explicit --status (GH#788)
		if status == "" && !allFlag && !readyFlag {
			filter.ExcludeStatus = []types.Status{types.StatusClosed}
		}
		// Use Changed() to properly handle P0 (priority=0)
		if cmd.Flags().Changed("priority") {
			priorityStr, _ := cmd.Flags().GetString("priority")
			priority, err := validation.ValidatePriority(priorityStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			filter.Priority = &priority
		}
		if assignee != "" {
			filter.Assignee = &assignee
		}
		if issueType != "" {
			t := types.IssueType(issueType)
			filter.IssueType = &t
		}
		if len(labels) > 0 {
			filter.Labels = labels
		}
		if len(labelsAny) > 0 {
			filter.LabelsAny = labelsAny
		}
		if titleSearch != "" {
			filter.TitleSearch = titleSearch
		}
		if idFilter != "" {
			ids := util.NormalizeLabels(strings.Split(idFilter, ","))
			if len(ids) > 0 {
				filter.IDs = ids
			}
		}

		// Pattern matching
		if titleContains != "" {
			filter.TitleContains = titleContains
		}
		if descContains != "" {
			filter.DescriptionContains = descContains
		}
		if notesContains != "" {
			filter.NotesContains = notesContains
		}

		applyLifecycleDateRangeFlags(cmd, &filter)

		// Empty/null checks
		if emptyDesc {
			filter.EmptyDescription = true
		}
		if noAssignee {
			filter.NoAssignee = true
		}
		if noLabels {
			filter.NoLabels = true
		}

		applyPriorityRangeFlags(cmd, &filter)

		// Pinned filtering: --pinned and --no-pinned are mutually exclusive
		if pinnedFlag && noPinnedFlag {
			fmt.Fprintf(os.Stderr, "Error: --pinned and --no-pinned are mutually exclusive\n")
			os.Exit(1)
		}
		if pinnedFlag {
			pinned := true
			filter.Pinned = &pinned
		} else if noPinnedFlag {
			pinned := false
			filter.Pinned = &pinned
		}

		// Parent filtering: filter children by parent issue
		if parentID != "" {
			filter.ParentID = &parentID
		}

		// Time-based scheduling filters (GH#820)
		if deferredFlag {
			filter.Deferred = true
		}
		applySchedulingDateRangeFlags(cmd, &filter)
		if overdueFlag {
			filter.Overdue = true
		}

		ctx := rootCtx

		issues, err := store.SearchIssues(ctx, "", filter)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Apply sorting
		sortIssues(issues, sortBy, reverse)

		// Handle pretty format (GH#654)
		if prettyFormat {
			// Load dependencies for tree structure
			allDeps, _ := store.GetAllDependencyRecords(ctx)
			displayPrettyListWithDeps(issues, false, allDeps)
			// Show truncation hint if we hit the limit (GH#788)
			if effectiveLimit > 0 && len(issues) == effectiveLimit {
				fmt.Fprintf(os.Stderr, "\nShowing %d issues (use --limit 0 for all)\n", effectiveLimit)
			}
			return
		}

		// Handle format flag
		if formatStr != "" {
			if err := outputFormattedList(ctx, store, issues, formatStr); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		if jsonOutput {
			// Get labels and dependency counts in bulk (single query instead of N queries)
			issueIDs := make([]string, len(issues))
			for i, issue := range issues {
				issueIDs[i] = issue.ID
			}
			labelsMap, _ := store.GetLabelsForIssues(ctx, issueIDs)
			depCounts, _ := store.GetDependencyCounts(ctx, issueIDs)

			// Populate labels for JSON output
			for _, issue := range issues {
				issue.Labels = labelsMap[issue.ID]
			}

			// Build response with counts
			issuesWithCounts := make([]*types.IssueWithCounts, len(issues))
			for i, issue := range issues {
				counts := depCounts[issue.ID]
				if counts == nil {
					counts = &types.DependencyCounts{DependencyCount: 0, DependentCount: 0}
				}
				issuesWithCounts[i] = &types.IssueWithCounts{
					Issue:           issue,
					DependencyCount: counts.DependencyCount,
					DependentCount:  counts.DependentCount,
				}
			}
			outputJSON(issuesWithCounts)
			return
		}

		// Load labels in bulk for display
		issueIDs := make([]string, len(issues))
		for i, issue := range issues {
			issueIDs[i] = issue.ID
		}
		labelsMap, _ := store.GetLabelsForIssues(ctx, issueIDs)

		// Build output in buffer for pager support (tl-jdz3)
		var buf strings.Builder
		if ui.IsAgentMode() {
			// Agent mode: ultra-compact, no colors, no pager
			for _, issue := range issues {
				formatAgentIssue(&buf, issue)
			}
			fmt.Print(buf.String())
			return
		} else if longFormat {
			// Long format: multi-line with details
			buf.WriteString(fmt.Sprintf("\nFound %d issues:\n\n", len(issues)))
			for _, issue := range issues {
				labels := labelsMap[issue.ID]
				formatIssueLong(&buf, issue, labels)
			}
		} else {
			// Compact format: one line per issue
			for _, issue := range issues {
				labels := labelsMap[issue.ID]
				formatIssueCompact(&buf, issue, labels)
			}
		}

		// Output with pager support
		if err := ui.ToPager(buf.String(), ui.PagerOptions{NoPager: noPager}); err != nil {
			if _, writeErr := fmt.Fprint(os.Stdout, buf.String()); writeErr != nil {
				fmt.Fprintf(os.Stderr, "Error writing output: %v\n", writeErr)
			}
		}

		// Show truncation hint if we hit the limit (GH#788)
		if effectiveLimit > 0 && len(issues) == effectiveLimit {
			fmt.Fprintf(os.Stderr, "\nShowing %d issues (use --limit 0 for all)\n", effectiveLimit)
		}

	},
}

func init() {
	listCmd.Flags().StringP(
		"status",
		"s",
		"",
		"Filter by status (open, in_progress, in_review, human_review, blocked, deferred, closed)",
	)
	registerPriorityFlag(listCmd, "")
	listCmd.Flags().StringP("assignee", "a", "", "Filter by assignee")
	listCmd.Flags().StringP("type", "t", "", "Filter by type (feature_request, feature, epic, task, bug, chore)")
	listCmd.Flags().StringSliceP("label", "l", []string{}, "Filter by labels (AND: must have ALL). Can combine with --label-any")
	listCmd.Flags().StringSlice("label-any", []string{}, "Filter by labels (OR: must have AT LEAST ONE). Can combine with --label")
	listCmd.Flags().String("title", "", "Filter by title text (case-insensitive substring match)")
	listCmd.Flags().String("id", "", "Filter by specific issue IDs (comma-separated, e.g., tl-1,tl-5,tl-10)")
	listCmd.Flags().IntP("limit", "n", 50, "Limit results (default 50, use 0 for unlimited)")
	listCmd.Flags().String("format", "", "Output format: 'digraph' (for golang.org/x/tools/cmd/digraph), 'dot' (Graphviz), or Go template")
	listCmd.Flags().Bool("all", false, "Show all issues including closed (overrides default filter)")
	listCmd.Flags().Bool("long", false, "Show detailed multi-line output for each issue")
	listCmd.Flags().String("sort", "", "Sort by field: priority, created, updated, closed, status, id, title, type, assignee")
	listCmd.Flags().BoolP("reverse", "r", false, "Reverse sort order")

	// Pattern matching
	listCmd.Flags().String("title-contains", "", "Filter by title substring (case-insensitive)")
	listCmd.Flags().String("desc-contains", "", "Filter by description substring (case-insensitive)")
	listCmd.Flags().String("notes-contains", "", "Filter by notes substring (case-insensitive)")

	// Date ranges
	listCmd.Flags().String("created-after", "", "Filter issues created after date (YYYY-MM-DD or RFC3339)")
	listCmd.Flags().String("created-before", "", "Filter issues created before date (YYYY-MM-DD or RFC3339)")
	listCmd.Flags().String("updated-after", "", "Filter issues updated after date (YYYY-MM-DD or RFC3339)")
	listCmd.Flags().String("updated-before", "", "Filter issues updated before date (YYYY-MM-DD or RFC3339)")
	listCmd.Flags().String("closed-after", "", "Filter issues closed after date (YYYY-MM-DD or RFC3339)")
	listCmd.Flags().String("closed-before", "", "Filter issues closed before date (YYYY-MM-DD or RFC3339)")

	// Empty/null checks
	listCmd.Flags().Bool("empty-description", false, "Filter issues with empty or missing description")
	listCmd.Flags().Bool("no-assignee", false, "Filter issues with no assignee")
	listCmd.Flags().Bool("no-labels", false, "Filter issues with no labels")

	// Priority ranges
	listCmd.Flags().String("priority-min", "", "Filter by minimum priority (inclusive, 0-4 or P0-P4)")
	listCmd.Flags().String("priority-max", "", "Filter by maximum priority (inclusive, 0-4 or P0-P4)")

	// Pinned filtering
	listCmd.Flags().Bool("pinned", false, "Show only pinned issues")
	listCmd.Flags().Bool("no-pinned", false, "Exclude pinned issues")

	// Parent filtering: filter children by parent issue
	listCmd.Flags().String("parent", "", "Filter by parent issue ID (shows children of specified issue)")
	listCmd.Flags().String("filter-parent", "", "Alias for --parent")

	// Time-based scheduling filters (GH#820)
	listCmd.Flags().Bool("deferred", false, "Show only issues with defer_until set")
	listCmd.Flags().String("defer-after", "", "Filter issues deferred after date (supports relative: +6h, tomorrow)")
	listCmd.Flags().String("defer-before", "", "Filter issues deferred before date (supports relative: +6h, tomorrow)")
	listCmd.Flags().String("due-after", "", "Filter issues due after date (supports relative: +6h, tomorrow)")
	listCmd.Flags().String("due-before", "", "Filter issues due before date (supports relative: +6h, tomorrow)")
	listCmd.Flags().Bool("overdue", false, "Show only issues with due_at in the past (not closed)")

	// Pretty flags.
	listCmd.Flags().Bool("pretty", false, "Display issues in a tree format with status/priority symbols")
	listCmd.Flags().Bool("tree", false, "Alias for --pretty: hierarchical tree format")

	// Pager control (tl-jdz3)
	listCmd.Flags().Bool("no-pager", false, "Disable pager output")

	// Ready filter: show only issues ready to be worked on (tl-ihu31)
	listCmd.Flags().Bool("ready", false, "Show only ready issues (status=open)")

	// Note: --json flag is defined as a persistent flag in main.go, not here
	rootCmd.AddCommand(listCmd)
}

// outputDotFormat outputs issues in Graphviz DOT format
func outputDotFormat(ctx context.Context, store storage.Storage, issues []*types.Issue) error {
	fmt.Println("digraph dependencies {")
	fmt.Println("  rankdir=TB;")
	fmt.Println("  node [shape=box, style=rounded];")
	fmt.Println()

	// Build map of all issues for quick lookup
	issueMap := make(map[string]*types.Issue)
	for _, issue := range issues {
		issueMap[issue.ID] = issue
	}

	// Output nodes with labels including ID, type, priority, and status
	for _, issue := range issues {
		// Build label with ID, type, priority, and title (using actual newlines)
		label := fmt.Sprintf("%s\n[%s P%d]\n%s\n(%s)",
			issue.ID,
			issue.IssueType,
			issue.Priority,
			issue.Title,
			issue.Status)

		// Color by status only - keep it simple
		fillColor := "white"
		fontColor := "black"

		switch issue.Status {
		case "closed":
			fillColor = "lightgray"
			fontColor = "dimgray"
		case "in_progress", "in_review", "human_review":
			fillColor = "lightyellow"
		case "blocked":
			fillColor = "lightcoral"
		}

		fmt.Printf("  %q [label=%q, style=\"rounded,filled\", fillcolor=%q, fontcolor=%q];\n",
			issue.ID, label, fillColor, fontColor)
	}
	fmt.Println()

	// Output edges with labels for dependency type
	for _, issue := range issues {
		deps, err := store.GetDependencyRecords(ctx, issue.ID)
		if err != nil {
			continue
		}
		for _, dep := range deps {
			// Only output edges where both nodes are in the filtered list
			if issueMap[dep.DependsOnID] != nil {
				// Color code by dependency type
				color := "black"
				style := "solid"
				switch dep.Type {
				case "blocks":
					color = "red"
					style = "bold"
				case "parent-child":
					color = "blue"
				case "discovered-from":
					color = "green"
					style = "dashed"
				case "related":
					color = "gray"
					style = "dashed"
				}
				fmt.Printf("  %q -> %q [label=%q, color=%s, style=%s];\n",
					issue.ID, dep.DependsOnID, dep.Type, color, style)
			}
		}
	}

	fmt.Println("}")
	return nil
}

// outputFormattedList outputs issues in a custom format (preset or Go template)
func outputFormattedList(ctx context.Context, store storage.Storage, issues []*types.Issue, formatStr string) error {
	// Handle special 'dot' format (Graphviz output)
	if formatStr == "dot" {
		return outputDotFormat(ctx, store, issues)
	}

	// Built-in format presets
	presets := map[string]string{
		"digraph": "{{.IssueID}} {{.DependsOnID}}",
	}

	// Check if it's a preset
	templateStr, isPreset := presets[formatStr]
	if !isPreset {
		templateStr = formatStr
	}

	// Parse template
	tmpl, err := template.New("format").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("invalid format template: %w", err)
	}

	// Build map of all issues for quick lookup
	issueMap := make(map[string]bool)
	for _, issue := range issues {
		issueMap[issue.ID] = true
	}

	// For each issue, output its dependencies using the template
	for _, issue := range issues {
		deps, err := store.GetDependencyRecords(ctx, issue.ID)
		if err != nil {
			continue
		}
		for _, dep := range deps {
			// Only output edges where both nodes are in the filtered list
			if issueMap[dep.DependsOnID] {
				// Template data includes both issue and dependency info
				data := map[string]interface{}{
					"IssueID":     issue.ID,
					"DependsOnID": dep.DependsOnID,
					"Type":        dep.Type,
					"Issue":       issue,
					"Dependency":  dep,
				}

				var buf bytes.Buffer
				if err := tmpl.Execute(&buf, data); err != nil {
					return fmt.Errorf("template execution error: %w", err)
				}
				fmt.Println(buf.String())
			}
		}
	}

	return nil
}
