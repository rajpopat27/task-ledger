package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	filestorage "task-ledger/internal/storage/file"
	"task-ledger/internal/types"
)

var importJSONLCmd = &cobra.Command{
	Use:   "import-jsonl <issues.jsonl>",
	Short: "Import legacy JSONL snapshots into file storage",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		CheckReadonly("import-jsonl")
		fileStore, ok := store.(*filestorage.FileStorage)
		if !ok {
			FatalError("import-jsonl requires file storage")
		}
		count, err := importJSONL(rootCtx, fileStore, args[0], actor)
		if err != nil {
			FatalError("%v", err)
		}
		fmt.Printf("Imported %d issue(s)\n", count)
	},
}

func importJSONL(ctx context.Context, store *filestorage.FileStorage, path, actor string) (int, error) {
	file, err := os.Open(path) // #nosec G304 -- importing an explicit user-provided JSONL path is the command's purpose.
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNo := 0
	issues := make([]*types.Issue, 0)
	seenIDs := make(map[string]int)
	seenExternalRefs := make(map[string]struct {
		line int
		id   string
	})
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var issue types.Issue
		if err := json.Unmarshal([]byte(line), &issue); err != nil {
			return 0, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if err := filestorage.ValidateFileIssueID(issue.ID); err != nil {
			return 0, fmt.Errorf("line %d issue %s: %w", lineNo, issue.ID, err)
		}
		if firstLine, exists := seenIDs[issue.ID]; exists {
			return 0, fmt.Errorf("line %d issue %s: duplicate ID already seen on line %d", lineNo, issue.ID, firstLine)
		}
		seenIDs[issue.ID] = lineNo
		existing, err := store.GetIssue(ctx, issue.ID)
		if err != nil {
			return 0, fmt.Errorf("line %d issue %s: %w", lineNo, issue.ID, err)
		}
		if existing != nil {
			return 0, fmt.Errorf("line %d issue %s: issue %s already exists", lineNo, issue.ID, issue.ID)
		}
		if issue.ExternalRef != nil && *issue.ExternalRef != "" {
			ref := *issue.ExternalRef
			if first, exists := seenExternalRefs[ref]; exists {
				return 0, fmt.Errorf("line %d issue %s: external_ref %s already belongs to %s from line %d", lineNo, issue.ID, ref, first.id, first.line)
			}
			seenExternalRefs[ref] = struct {
				line int
				id   string
			}{line: lineNo, id: issue.ID}
			existing, err := store.GetIssueByExternalRef(ctx, ref)
			if err != nil {
				return 0, fmt.Errorf("line %d issue %s: %w", lineNo, issue.ID, err)
			}
			if existing != nil {
				return 0, fmt.Errorf("line %d issue %s: external_ref %s already belongs to %s", lineNo, issue.ID, ref, existing.ID)
			}
		}
		issue.SetDefaults()
		if err := issue.Validate(); err != nil {
			return 0, fmt.Errorf("line %d issue %s: %w", lineNo, issue.ID, err)
		}
		issues = append(issues, &issue)
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if err := store.ImportIssues(ctx, issues, actor); err != nil {
		return 0, err
	}
	return len(issues), nil
}

func init() {
	rootCmd.AddCommand(importJSONLCmd)
}
