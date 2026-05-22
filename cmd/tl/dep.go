// Package main implements the tl CLI dependency management commands.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"task-ledger/internal/storage"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
	"task-ledger/internal/utils"
)

// isChildOf returns true if childID is a hierarchical child of parentID.
// For example, "tl-abc.1" is a child of "tl-abc", and "tl-abc.1.2" is a child of "tl-abc.1".
func isChildOf(childID, parentID string) bool {
	// A child ID has the format "parentID.N" or "parentID.N.M" etc.
	// Use ParseHierarchicalID to get the actual parent
	_, actualParentID, depth := types.ParseHierarchicalID(childID)
	if depth == 0 {
		return false // Not a hierarchical ID
	}
	// Check if the immediate parent matches
	if actualParentID == parentID {
		return true
	}
	// Also check if parentID is an ancestor (e.g., "tl-abc" is parent of "tl-abc.1.2")
	return strings.HasPrefix(childID, parentID+".")
}

// warnIfCyclesExist checks for dependency cycles and prints a warning if found.
func warnIfCyclesExist(s storage.Storage) {
	if s == nil {
		return
	}
	cycles, err := s.DetectCycles(rootCtx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to check for cycles: %v\n", err)
		return
	}
	if len(cycles) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\n%s Warning: Dependency cycle detected!\n", ui.RenderWarn("⚠"))
	fmt.Fprintf(os.Stderr, "This can hide issues from the ready work list and cause confusion.\n\n")
	fmt.Fprintf(os.Stderr, "Cycle path:\n")
	for _, cycle := range cycles {
		for j, issue := range cycle {
			if j == 0 {
				fmt.Fprintf(os.Stderr, "  %s", issue.ID)
			} else {
				fmt.Fprintf(os.Stderr, " → %s", issue.ID)
			}
		}
		if len(cycle) > 0 {
			fmt.Fprintf(os.Stderr, " → %s", cycle[0].ID)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
	fmt.Fprintf(os.Stderr, "\nRun 'tl dep cycles' for detailed analysis.\n\n")
}

var depCmd = &cobra.Command{
	Use:     "dep [issue-id]",
	GroupID: "deps",
	Short:   "Manage dependencies",
	Long: `Manage dependencies between issues.

When called with an issue ID and --blocks flag, creates a blocking dependency:
  tl dep <blocker-id> --blocks <blocked-id>

This is equivalent to:
  tl dep add <blocked-id> <blocker-id>

Examples:
  tl dep tl-xyz --blocks tl-abc    # tl-xyz blocks tl-abc
  tl dep add tl-abc tl-xyz         # Same as above (tl-abc depends on tl-xyz)`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		blocksID, _ := cmd.Flags().GetString("blocks")

		// If no args and no flags, show help
		if len(args) == 0 && blocksID == "" {
			_ = cmd.Help()
			return
		}

		// If --blocks flag is provided, create a blocking dependency
		if blocksID != "" {
			if len(args) != 1 {
				FatalErrorRespectJSON("--blocks requires exactly one issue ID argument")
			}
			blockerID := args[0]

			CheckReadonly("dep --blocks")

			ctx := rootCtx
			depType := "blocks"

			fromID, err := utils.ResolvePartialID(ctx, store, blocksID)
			if err != nil {
				FatalErrorRespectJSON("resolving issue ID %s: %v", blocksID, err)
			}

			toID, err := utils.ResolvePartialID(ctx, store, blockerID)
			if err != nil {
				FatalErrorRespectJSON("resolving issue ID %s: %v", blockerID, err)
			}

			// Check for child→parent dependency anti-pattern
			if isChildOf(fromID, toID) {
				FatalErrorRespectJSON("cannot add dependency: %s is already a child of %s. Children inherit dependency on parent completion via hierarchy. Adding an explicit dependency would create a deadlock", fromID, toID)
			}

			dep := &types.Dependency{
				IssueID:     fromID,
				DependsOnID: toID,
				Type:        types.DependencyType(depType),
			}
			if err := store.AddDependency(ctx, dep, actor); err != nil {
				FatalErrorRespectJSON("%v", err)
			}

			// Check for cycles after adding dependency.
			warnIfCyclesExist(store)

			if jsonOutput {
				outputJSON(map[string]interface{}{
					"status":     "added",
					"blocker_id": toID,
					"blocked_id": fromID,
					"type":       depType,
				})
				return
			}

			fmt.Printf("%s Added dependency: %s blocks %s\n",
				ui.RenderPass("✓"), toID, fromID)
			return
		}

		// If we have an arg but no --blocks flag, show help
		_ = cmd.Help()
	},
}

var depAddCmd = &cobra.Command{
	Use:   "add [issue-id] [depends-on-id]",
	Short: "Add a dependency",
	Long: `Add a dependency between two issues.

The depends-on-id can be provided as:
  - A positional argument: tl dep add issue-123 issue-456
  - A flag: tl dep add issue-123 --blocked-by issue-456
  - A flag: tl dep add issue-123 --depends-on issue-456

The --blocked-by and --depends-on flags are aliases and both mean "issue-123
depends on (is blocked by) the specified issue."

The depends-on-id can be:
  - A local issue ID (e.g., tl-xyz)
  - An external reference: external:<project>:<capability>

External references are stored as-is for auditability and manual coordination.

Examples:
  tl dep add tl-42 tl-41                              # Positional args
  tl dep add tl-42 --blocked-by tl-41                 # Flag syntax (same effect)
  tl dep add tl-42 --depends-on tl-41                 # Alias (same effect)
  tl dep add tl-42 external:api:auth-ready             # External dependency marker`,
	Args: func(cmd *cobra.Command, args []string) error {
		blockedBy, _ := cmd.Flags().GetString("blocked-by")
		dependsOn, _ := cmd.Flags().GetString("depends-on")
		hasFlag := blockedBy != "" || dependsOn != ""

		if hasFlag {
			// If a flag is provided, we only need 1 positional arg (the dependent issue)
			if len(args) < 1 {
				return fmt.Errorf("requires at least 1 arg(s), only received %d", len(args))
			}
			if len(args) > 1 {
				return fmt.Errorf("cannot use both positional depends-on-id and --blocked-by/--depends-on flag")
			}
			return nil
		}
		// No flag provided, need exactly 2 positional args
		if len(args) != 2 {
			return fmt.Errorf("requires 2 arg(s), only received %d (or use --blocked-by/--depends-on flag)", len(args))
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("dep add")
		depType, _ := cmd.Flags().GetString("type")

		// Get the dependency target from flag or positional arg
		blockedBy, _ := cmd.Flags().GetString("blocked-by")
		dependsOn, _ := cmd.Flags().GetString("depends-on")

		var dependsOnArg string
		if blockedBy != "" {
			dependsOnArg = blockedBy
		} else if dependsOn != "" {
			dependsOnArg = dependsOn
		} else {
			dependsOnArg = args[1]
		}

		ctx := rootCtx

		// Resolve partial IDs first
		var fromID, toID string

		// Check if toID is an external reference (don't resolve it)
		isExternalRef := strings.HasPrefix(dependsOnArg, "external:")

		var err error
		fromID, err = utils.ResolvePartialID(ctx, store, args[0])
		if err != nil {
			FatalErrorRespectJSON("resolving issue ID %s: %v", args[0], err)
		}

		if isExternalRef {
			toID = dependsOnArg
			if err := validateExternalRef(toID); err != nil {
				FatalErrorRespectJSON("%v", err)
			}
		} else {
			toID, err = utils.ResolvePartialID(ctx, store, dependsOnArg)
			if err != nil {
				FatalErrorRespectJSON("resolving dependency ID %s: %v", dependsOnArg, err)
			}
		}

		// Check for child→parent dependency anti-pattern
		// This creates a deadlock: child can't start (parent open), parent can't close (children not done)
		if isChildOf(fromID, toID) {
			FatalErrorRespectJSON("cannot add dependency: %s is already a child of %s. Children inherit dependency on parent completion via hierarchy. Adding an explicit dependency would create a deadlock", fromID, toID)
		}

		dep := &types.Dependency{
			IssueID:     fromID,
			DependsOnID: toID,
			Type:        types.DependencyType(depType),
		}

		if err := store.AddDependency(ctx, dep, actor); err != nil {
			FatalErrorRespectJSON("%v", err)
		}

		// Check for cycles after adding dependency.
		warnIfCyclesExist(store)

		if jsonOutput {
			outputJSON(map[string]interface{}{
				"status":        "added",
				"issue_id":      fromID,
				"depends_on_id": toID,
				"type":          depType,
			})
			return
		}

		fmt.Printf("%s Added dependency: %s depends on %s (%s)\n",
			ui.RenderPass("✓"), fromID, toID, depType)
	},
}

var depListCmd = &cobra.Command{
	Use:   "list [issue-id]",
	Short: "List dependencies or dependents of an issue",
	Long: `List dependencies or dependents of an issue with optional type filtering.

By default shows dependencies (what this issue depends on). Use --direction to control:
  - down: Show dependencies (what this issue depends on) - default
  - up:   Show dependents (what depends on this issue)

Use --type to filter by dependency type (e.g., tracks, blocks, parent-child).

Examples:
  tl dep list gt-abc                     # Show what gt-abc depends on
  tl dep list gt-abc --direction=up      # Show what depends on gt-abc
  tl dep list gt-abc --direction=up -t tracks  # Show what tracks gt-abc (convoy tracking)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := rootCtx

		fullID, err := utils.ResolvePartialID(ctx, store, args[0])
		if err != nil {
			FatalErrorRespectJSON("resolving %s: %v", args[0], err)
		}

		direction, _ := cmd.Flags().GetString("direction")
		typeFilter, _ := cmd.Flags().GetString("type")

		if direction == "" {
			direction = "down"
		}

		var issues []*types.IssueWithDependencyMetadata

		if direction == "up" {
			issues, err = store.GetDependentsWithMetadata(ctx, fullID)
		} else {
			issues, err = store.GetDependenciesWithMetadata(ctx, fullID)
		}
		if err != nil {
			FatalErrorRespectJSON("%v", err)
		}

		// Apply type filter if specified
		if typeFilter != "" {
			var filtered []*types.IssueWithDependencyMetadata
			for _, iss := range issues {
				if string(iss.DependencyType) == typeFilter {
					filtered = append(filtered, iss)
				}
			}
			issues = filtered
		}

		if jsonOutput {
			if issues == nil {
				issues = []*types.IssueWithDependencyMetadata{}
			}
			outputJSON(issues)
			return
		}

		if len(issues) == 0 {
			if typeFilter != "" {
				if direction == "up" {
					fmt.Printf("\nNo issues depend on %s with type '%s'\n", fullID, typeFilter)
				} else {
					fmt.Printf("\n%s has no dependencies of type '%s'\n", fullID, typeFilter)
				}
			} else {
				if direction == "up" {
					fmt.Printf("\nNo issues depend on %s\n", fullID)
				} else {
					fmt.Printf("\n%s has no dependencies\n", fullID)
				}
			}
			return
		}

		if direction == "up" {
			fmt.Printf("\n%s Issues that depend on %s:\n\n", ui.RenderAccent("📋"), fullID)
		} else {
			fmt.Printf("\n%s %s depends on:\n\n", ui.RenderAccent("📋"), fullID)
		}

		for _, iss := range issues {
			// Color the ID based on status
			var idStr string
			switch iss.Status {
			case types.StatusOpen:
				idStr = ui.StatusOpenStyle.Render(iss.ID)
			case types.StatusInProgress, types.StatusInReview, types.StatusHumanReview:
				idStr = ui.StatusInProgressStyle.Render(iss.ID)
			case types.StatusBlocked:
				idStr = ui.StatusBlockedStyle.Render(iss.ID)
			case types.StatusClosed:
				idStr = ui.StatusClosedStyle.Render(iss.ID)
			default:
				idStr = iss.ID
			}

			fmt.Printf("  %s: %s [P%d] (%s) via %s\n",
				idStr, iss.Title, iss.Priority, iss.Status, iss.DependencyType)
		}
		fmt.Println()
	},
}

var depRemoveCmd = &cobra.Command{
	Use:     "remove [issue-id] [depends-on-id]",
	Aliases: []string{"rm"},
	Short:   "Remove a dependency",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("dep remove")
		ctx := rootCtx
		fromID, err := utils.ResolvePartialID(ctx, store, args[0])
		if err != nil {
			FatalErrorRespectJSON("resolving issue ID %s: %v", args[0], err)
		}

		toID, err := utils.ResolvePartialID(ctx, store, args[1])
		if err != nil {
			FatalErrorRespectJSON("resolving dependency ID %s: %v", args[1], err)
		}

		fullFromID := fromID
		fullToID := toID

		if err := store.RemoveDependency(ctx, fullFromID, fullToID, actor); err != nil {
			FatalErrorRespectJSON("%v", err)
		}

		if jsonOutput {
			outputJSON(map[string]interface{}{
				"status":        "removed",
				"issue_id":      fullFromID,
				"depends_on_id": fullToID,
			})
			return
		}

		fmt.Printf("%s Removed dependency: %s no longer depends on %s\n",
			ui.RenderPass("✓"), fullFromID, fullToID)
	},
}

var depTreeCmd = &cobra.Command{
	Use:   "tree [issue-id]",
	Short: "Show dependency tree",
	Long: `Show dependency tree rooted at the given issue.

By default, shows dependencies (what blocks this issue). Use --direction to control:
  - down: Show dependencies (what blocks this issue) - default
  - up:   Show dependents (what this issue blocks)
  - both: Show full graph in both directions

Examples:
  tl dep tree gt-0iqq                    # Show what blocks gt-0iqq
  tl dep tree gt-0iqq --direction=up     # Show what gt-0iqq blocks
  tl dep tree gt-0iqq --status=open      # Only show open issues
  tl dep tree gt-0iqq --depth=3          # Limit to 3 levels deep`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := rootCtx

		fullID, err := utils.ResolvePartialID(ctx, store, args[0])
		if err != nil {
			FatalErrorRespectJSON("resolving %s: %v", args[0], err)
		}

		showAllPaths, _ := cmd.Flags().GetBool("show-all-paths")
		maxDepth, _ := cmd.Flags().GetInt("max-depth")
		reverse, _ := cmd.Flags().GetBool("reverse")
		direction, _ := cmd.Flags().GetString("direction")
		statusFilter, _ := cmd.Flags().GetString("status")
		formatStr, _ := cmd.Flags().GetString("format")

		// Handle --direction flag (takes precedence over the --reverse alias)
		if direction == "" && reverse {
			direction = "up"
		} else if direction == "" {
			direction = "down"
		}

		// Validate direction
		if direction != "down" && direction != "up" && direction != "both" {
			FatalErrorRespectJSON("--direction must be 'down', 'up', or 'both'")
		}

		if maxDepth < 1 {
			FatalErrorRespectJSON("--max-depth must be >= 1")
		}

		// For "both" direction, we need to fetch both trees and merge them
		var tree []*types.TreeNode

		if direction == "both" {
			// Get dependencies (down) - what blocks this issue
			downTree, err := store.GetDependencyTree(ctx, fullID, maxDepth, showAllPaths, false)
			if err != nil {
				FatalErrorRespectJSON("%v", err)
			}

			// Get dependents (up) - what this issue blocks
			upTree, err := store.GetDependencyTree(ctx, fullID, maxDepth, showAllPaths, true)
			if err != nil {
				FatalErrorRespectJSON("%v", err)
			}

			// Merge: root appears once, dependencies below, dependents above
			// We'll show dependents first (with negative-like positioning conceptually),
			// then root, then dependencies
			tree = mergeBidirectionalTrees(downTree, upTree, fullID)
		} else {
			tree, err = store.GetDependencyTree(ctx, fullID, maxDepth, showAllPaths, direction == "up")
			if err != nil {
				FatalErrorRespectJSON("%v", err)
			}
		}

		// Apply status filter if specified
		if statusFilter != "" {
			tree = filterTreeByStatus(tree, types.Status(statusFilter))
		}

		// Handle mermaid format
		if formatStr == "mermaid" {
			outputMermaidTree(tree, args[0])
			return
		}

		if jsonOutput {
			// Always output array, even if empty
			if tree == nil {
				tree = []*types.TreeNode{}
			}
			outputJSON(tree)
			return
		}

		if len(tree) == 0 {
			switch direction {
			case "up":
				fmt.Printf("\n%s has no dependents\n", fullID)
			case "both":
				fmt.Printf("\n%s has no dependencies or dependents\n", fullID)
			default:
				fmt.Printf("\n%s has no dependencies\n", fullID)
			}
			return
		}

		switch direction {
		case "up":
			fmt.Printf("\n%s Dependent tree for %s:\n\n", ui.RenderAccent("🌲"), fullID)
		case "both":
			fmt.Printf("\n%s Full dependency graph for %s:\n\n", ui.RenderAccent("🌲"), fullID)
		default:
			fmt.Printf("\n%s Dependency tree for %s:\n\n", ui.RenderAccent("🌲"), fullID)
		}

		// Render tree with proper connectors
		renderTree(tree, maxDepth, direction)
		fmt.Println()
	},
}

var depCyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Detect dependency cycles",
	Run: func(cmd *cobra.Command, args []string) {
		ctx := rootCtx
		cycles, err := store.DetectCycles(ctx)
		if err != nil {
			FatalErrorRespectJSON("%v", err)
		}

		if jsonOutput {
			// Always output array, even if empty
			if cycles == nil {
				cycles = [][]*types.Issue{}
			}
			outputJSON(cycles)
			return
		}

		if len(cycles) == 0 {
			fmt.Printf("\n%s No dependency cycles detected\n\n", ui.RenderPass("✓"))
			return
		}

		fmt.Printf("\n%s Found %d dependency cycles:\n\n", ui.RenderFail("⚠"), len(cycles))
		for i, cycle := range cycles {
			fmt.Printf("%d. Cycle involving:\n", i+1)
			for _, issue := range cycle {
				fmt.Printf("   - %s: %s\n", issue.ID, issue.Title)
			}
			fmt.Println()
		}
	},
}

// outputMermaidTree outputs a dependency tree in Mermaid.js flowchart format
func outputMermaidTree(tree []*types.TreeNode, rootID string) {
	if len(tree) == 0 {
		fmt.Println("flowchart TD")
		fmt.Printf("  %s[\"No dependencies\"]\n", rootID)
		return
	}

	fmt.Println("flowchart TD")

	// Output nodes
	nodesSeen := make(map[string]bool)
	for _, node := range tree {
		if !nodesSeen[node.ID] {
			emoji := getStatusEmoji(node.Status)
			label := fmt.Sprintf("%s %s: %s", emoji, node.ID, node.Title)
			// Escape quotes and backslashes in label
			label = strings.ReplaceAll(label, "\\", "\\\\")
			label = strings.ReplaceAll(label, "\"", "\\\"")
			fmt.Printf("  %s[\"%s\"]\n", node.ID, label)

			nodesSeen[node.ID] = true
		}
	}

	fmt.Println()

	// Output edges - use explicit parent relationships from ParentID
	for _, node := range tree {
		if node.ParentID != "" && node.ParentID != node.ID {
			fmt.Printf("  %s --> %s\n", node.ParentID, node.ID)
		}
	}
}

// getStatusEmoji returns a symbol indicator for a given status
func getStatusEmoji(status types.Status) string {
	switch status {
	case types.StatusOpen:
		return "☐" // U+2610 Ballot Box
	case types.StatusInProgress, types.StatusInReview, types.StatusHumanReview:
		return "◧" // U+25E7 Square Left Half Black
	case types.StatusBlocked:
		return "⚠" // U+26A0 Warning Sign
	case types.StatusDeferred:
		return "❄" // U+2744 Snowflake (on ice)
	case types.StatusClosed:
		return "☑" // U+2611 Ballot Box with Check
	default:
		return "?"
	}
}

// treeRenderer holds state for rendering a tree with proper connectors
type treeRenderer struct {
	// Track which nodes we've already displayed (for "shown above" handling)
	seen map[string]bool
	// Track connector state at each depth level (true = has more siblings)
	activeConnectors []bool
	// Maximum depth reached
	maxDepth int
	// Direction of traversal
	direction string
}

// renderTree renders the tree with proper box-drawing connectors
func renderTree(tree []*types.TreeNode, maxDepth int, direction string) {
	if len(tree) == 0 {
		return
	}

	r := &treeRenderer{
		seen:             make(map[string]bool),
		activeConnectors: make([]bool, maxDepth+1),
		maxDepth:         maxDepth,
		direction:        direction,
	}

	// Build a map of parent -> children for proper sibling tracking
	children := make(map[string][]*types.TreeNode)
	var root *types.TreeNode

	for _, node := range tree {
		if node.Depth == 0 {
			root = node
		} else {
			children[node.ParentID] = append(children[node.ParentID], node)
		}
	}

	if root == nil && len(tree) > 0 {
		root = tree[0]
	}

	// Render recursively from root
	r.renderNode(root, children, 0, true)
}

// renderNode renders a single node and its children
func (r *treeRenderer) renderNode(node *types.TreeNode, children map[string][]*types.TreeNode, depth int, isLast bool) {
	if node == nil {
		return
	}

	// Build the prefix with connectors
	var prefix strings.Builder

	// Add vertical lines for active parent connectors
	for i := 0; i < depth; i++ {
		if r.activeConnectors[i] {
			prefix.WriteString("│   ")
		} else {
			prefix.WriteString("    ")
		}
	}

	// Add the branch connector for non-root nodes
	if depth > 0 {
		if isLast {
			prefix.WriteString("└── ")
		} else {
			prefix.WriteString("├── ")
		}
	}

	// Check if we've seen this node before (diamond dependency)
	if r.seen[node.ID] {
		fmt.Printf("%s%s (shown above)\n", prefix.String(), ui.RenderMuted(node.ID))
		return
	}
	r.seen[node.ID] = true

	// Format the node line
	line := formatTreeNode(node)

	// Add truncation warning if at max depth and has children
	if node.Truncated || (depth == r.maxDepth && len(children[node.ID]) > 0) {
		line += ui.RenderWarn(" …")
	}

	fmt.Printf("%s%s\n", prefix.String(), line)

	// Render children
	nodeChildren := children[node.ID]
	for i, child := range nodeChildren {
		// Update connector state for this depth
		// For depth 0 (root level), never show vertical connector since root has no siblings
		if depth > 0 {
			r.activeConnectors[depth] = (i < len(nodeChildren)-1)
		}
		r.renderNode(child, children, depth+1, i == len(nodeChildren)-1)
	}
}

// formatTreeNode formats a single tree node with status, ready indicator, etc.
func formatTreeNode(node *types.TreeNode) string {
	// Handle external dependencies specially
	if IsExternalRef(node.ID) {
		// External deps use their title directly which includes the status indicator
		var idStr string
		switch node.Status {
		case types.StatusClosed:
			idStr = ui.StatusClosedStyle.Render(node.Title)
		case types.StatusBlocked:
			idStr = ui.StatusBlockedStyle.Render(node.Title)
		default:
			idStr = node.Title
		}
		return fmt.Sprintf("%s (external)", idStr)
	}

	// Color the ID based on status
	var idStr string
	switch node.Status {
	case types.StatusOpen:
		idStr = ui.StatusOpenStyle.Render(node.ID)
	case types.StatusInProgress, types.StatusInReview, types.StatusHumanReview:
		idStr = ui.StatusInProgressStyle.Render(node.ID)
	case types.StatusBlocked:
		idStr = ui.StatusBlockedStyle.Render(node.ID)
	case types.StatusClosed:
		idStr = ui.StatusClosedStyle.Render(node.ID)
	default:
		idStr = node.ID
	}

	// Build the line
	line := fmt.Sprintf("%s: %s [P%d] (%s)",
		idStr, node.Title, node.Priority, node.Status)

	// Add READY indicator for open issues (those that could be worked on)
	// An issue is ready if it's open and has no blocking dependencies
	// (In the tree view, depth 0 with status open implies ready in the "down" direction)
	if node.Status == types.StatusOpen && node.Depth == 0 {
		line += " " + ui.PassStyle.Bold(true).Render("[READY]")
	}

	return line
}

// filterTreeByStatus filters the tree to only include nodes with the given status
// Note: keeps parent chain to maintain tree structure
func filterTreeByStatus(tree []*types.TreeNode, status types.Status) []*types.TreeNode {
	if len(tree) == 0 {
		return tree
	}

	// First pass: identify which nodes match the status
	matches := make(map[string]bool)
	for _, node := range tree {
		if node.Status == status {
			matches[node.ID] = true
		}
	}

	// If no matches, return empty
	if len(matches) == 0 {
		return []*types.TreeNode{}
	}

	// Second pass: keep matching nodes and their ancestors
	// Build parent map
	parentOf := make(map[string]string)
	for _, node := range tree {
		if node.ParentID != "" && node.ParentID != node.ID {
			parentOf[node.ID] = node.ParentID
		}
	}

	// Mark all ancestors of matching nodes
	keep := make(map[string]bool)
	for id := range matches {
		keep[id] = true
		// Walk up to root
		current := id
		for {
			parent, ok := parentOf[current]
			if !ok || parent == current {
				break
			}
			keep[parent] = true
			current = parent
		}
	}

	// Filter the tree
	var filtered []*types.TreeNode
	for _, node := range tree {
		if keep[node.ID] {
			filtered = append(filtered, node)
		}
	}

	return filtered
}

// mergeBidirectionalTrees merges up and down trees into a single visualization
// The root appears once, with dependencies shown below and dependents shown above
func mergeBidirectionalTrees(downTree, upTree []*types.TreeNode, rootID string) []*types.TreeNode {
	var result []*types.TreeNode

	hasUpNodes := false
	for _, node := range upTree {
		if node.ID != rootID {
			hasUpNodes = true
			break
		}
	}

	if hasUpNodes {
		for _, node := range upTree {
			if node.ID == rootID {
				continue // Skip root, we'll add it once from down tree
			}
			upNode := *node
			result = append(result, &upNode)
		}
	}

	result = append(result, downTree...)

	return result
}

// validateExternalRef validates the format of an external dependency reference.
// Valid format: external:<project>:<capability>
func validateExternalRef(ref string) error {
	if !strings.HasPrefix(ref, "external:") {
		return fmt.Errorf("external reference must start with 'external:'")
	}

	parts := strings.SplitN(ref, ":", 3)
	if len(parts) != 3 {
		return fmt.Errorf("invalid external reference format: expected 'external:<project>:<capability>', got '%s'", ref)
	}

	project := parts[1]
	capability := parts[2]

	if project == "" {
		return fmt.Errorf("external reference missing project name")
	}
	if capability == "" {
		return fmt.Errorf("external reference missing capability name")
	}

	return nil
}

// IsExternalRef returns true if the dependency reference is an external reference.
func IsExternalRef(ref string) bool {
	return strings.HasPrefix(ref, "external:")
}

// ParseExternalRef parses an external reference into project and capability.
// Returns empty strings if the format is invalid.
func ParseExternalRef(ref string) (project, capability string) {
	if !IsExternalRef(ref) {
		return "", ""
	}
	parts := strings.SplitN(ref, ":", 3)
	if len(parts) != 3 {
		return "", ""
	}
	return parts[1], parts[2]
}

func init() {
	// dep command shorthand flag
	depCmd.Flags().StringP("blocks", "b", "", "Issue ID that this issue blocks (shorthand for: tl dep add <blocked> <blocker>)")

	depAddCmd.Flags().StringP("type", "t", "blocks", "Dependency type (blocks|tracks|related|parent-child|discovered-from|until|caused-by|validates|relates-to|supersedes)")
	depAddCmd.Flags().String("blocked-by", "", "Issue ID that blocks the first issue (alternative to positional arg)")
	depAddCmd.Flags().String("depends-on", "", "Issue ID that the first issue depends on (alias for --blocked-by)")

	depTreeCmd.Flags().Bool("show-all-paths", false, "Show all paths to nodes (no deduplication for diamond dependencies)")
	depTreeCmd.Flags().IntP("max-depth", "d", 50, "Maximum tree depth to display (safety limit)")
	depTreeCmd.Flags().Bool("reverse", false, "Show dependent tree (alias for --direction=up)")
	depTreeCmd.Flags().String("direction", "", "Tree direction: 'down' (dependencies), 'up' (dependents), or 'both'")
	depTreeCmd.Flags().String(
		"status",
		"",
		"Filter to only show issues with this status (open, in_progress, in_review, human_review, blocked, deferred, closed)",
	)
	depTreeCmd.Flags().String("format", "", "Output format: 'mermaid' for Mermaid.js flowchart")
	depTreeCmd.Flags().StringP("type", "t", "", "Filter to only show dependencies of this type (e.g., tracks, blocks, parent-child)")

	depListCmd.Flags().String("direction", "down", "Direction: 'down' (dependencies), 'up' (dependents)")
	depListCmd.Flags().StringP("type", "t", "", "Filter by dependency type (e.g., tracks, blocks, parent-child)")

	// Issue ID completions for dep subcommands
	depAddCmd.ValidArgsFunction = issueIDCompletion
	depRemoveCmd.ValidArgsFunction = issueIDCompletion
	depListCmd.ValidArgsFunction = issueIDCompletion
	depTreeCmd.ValidArgsFunction = issueIDCompletion

	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
	depCmd.AddCommand(depListCmd)
	depCmd.AddCommand(depTreeCmd)
	depCmd.AddCommand(depCyclesCmd)
	rootCmd.AddCommand(depCmd)
}
