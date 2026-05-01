package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	filestorage "task-ledger/internal/storage/file"
	"task-ledger/internal/types"
	"task-ledger/internal/ui"
)

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorProblem struct {
	Path     string `json:"path"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type doctorResult struct {
	Path      string          `json:"path"`
	Checks    []doctorCheck   `json:"checks"`
	OverallOK bool            `json:"overall_ok"`
	Issues    int             `json:"issues"`
	Problems  []doctorProblem `json:"problems,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:     "doctor [path]",
	GroupID: "maint",
	Short:   "Check JSON file backend health",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) == 1 {
			path = args[0]
		}

		result := runFileDoctor(path)
		if jsonOutput {
			outputJSON(result)
			return
		}

		if result.OverallOK {
			fmt.Printf("%s JSON file backend healthy\n", ui.RenderPass("OK"))
		} else {
			fmt.Printf("%s JSON file backend has issues\n", ui.RenderFail("FAIL"))
		}
		for _, check := range result.Checks {
			fmt.Printf("  [%s] %s: %s\n", check.Status, check.Name, check.Message)
		}
		for _, problem := range result.Problems {
			fmt.Printf("  [%s] %s: %s (%s)\n", problem.Severity, problem.Code, problem.Message, problem.Path)
		}
		fmt.Printf("  Issues: %d\n", result.Issues)

		if !result.OverallOK {
			os.Exit(1)
		}
	},
}

func runFileDoctor(path string) doctorResult {
	root, err := filepath.Abs(path)
	if err != nil {
		root = path
	}
	taskLedgerDir := filepath.Join(root, ".task-ledger")
	if found := findTaskLedgerDirFrom(root); found != "" {
		taskLedgerDir = found
	}

	result := doctorResult{Path: taskLedgerDir, OverallOK: true}
	addCheck := func(name, status, message string) {
		if status != "ok" {
			result.OverallOK = false
		}
		result.Checks = append(result.Checks, doctorCheck{Name: name, Status: status, Message: message})
	}
	addProblem := func(path, code, message, severity string) {
		result.OverallOK = false
		result.Problems = append(result.Problems, doctorProblem{
			Path:     path,
			Code:     code,
			Message:  message,
			Severity: severity,
		})
	}

	issuesDir := filepath.Join(taskLedgerDir, "issues")
	if info, err := os.Stat(issuesDir); err != nil || !info.IsDir() {
		addCheck("issues-dir", "error", ".task-ledger/issues does not exist; run tl init")
		return result
	}
	addCheck("issues-dir", "ok", issuesDir)

	files, err := filestorage.FindIssueFiles(taskLedgerDir)
	if err != nil {
		addCheck("scan", "error", err.Error())
		return result
	}
	result.Issues = len(files)
	addCheck("scan", "ok", fmt.Sprintf("%d issue file(s)", len(files)))

	seenIDs := make(map[string]string)
	validIssues := 0
	for _, path := range files {
		issue, err := doctorReadIssueFile(path)
		if err != nil {
			addProblem(path, "parse", err.Error(), "error")
			continue
		}
		validIssues++
		if firstPath, exists := seenIDs[issue.ID]; exists {
			addProblem(path, "duplicate-id", fmt.Sprintf("duplicate issue id %s in %s and %s", issue.ID, firstPath, path), "error")
			continue
		}
		seenIDs[issue.ID] = path
	}
	if len(result.Problems) > 0 {
		result.Issues = validIssues
		addCheck("validate", "error", fmt.Sprintf("%d problem(s)", len(result.Problems)))
		return result
	}

	store, err := filestorage.Open(taskLedgerDir)
	if err != nil {
		addCheck("load", "error", err.Error())
		return result
	}
	defer func() { _ = store.Close() }()

	issues, err := store.SearchIssues(rootCtx, "", types.IssueFilter{IncludeTombstones: true})
	if err != nil {
		addCheck("query", "error", err.Error())
		return result
	}
	result.Issues = len(issues)
	addCheck("query", "ok", "all issue files loaded")
	return result
}

func doctorReadIssueFile(path string) (*types.Issue, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from FindIssueFiles under the configured .task-ledger directory.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var fileIssue filestorage.FileIssue
	if err := json.Unmarshal(data, &fileIssue); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	issue, err := fileIssue.ToTypesIssue()
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return issue, nil
}

func findTaskLedgerDirFrom(start string) string {
	dir := filepath.Clean(start)
	for {
		candidate := filepath.Join(dir, ".task-ledger")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
