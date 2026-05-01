package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"task-ledger/internal/config"
	"task-ledger/internal/ui"
)

var initCmd = &cobra.Command{
	Use:     "init",
	GroupID: "setup",
	Short:   "Initialize tl in the current directory",
	Long: `Initialize tl in the current directory by creating a .task-ledger directory.
Issues are stored as JSON files under .task-ledger/issues.`,
	Run: func(cmd *cobra.Command, _ []string) {
		quiet, _ := cmd.Flags().GetBool("quiet")
		force, _ := cmd.Flags().GetBool("force")

		if err := config.Initialize(); err != nil && !quiet {
			fmt.Fprintf(os.Stderr, "Warning: failed to initialize config: %v\n", err)
		}

		cwd, err := os.Getwd()
		if err != nil {
			FatalError("failed to get current directory: %v", err)
		}
		if isInsideTaskLedgerDir(cwd) {
			FatalError("cannot initialize tl inside a .task-ledger directory")
		}

		taskLedgerDir := filepath.Join(cwd, ".task-ledger")
		issuesDir := filepath.Join(taskLedgerDir, "issues")
		if !force {
			if _, err := os.Stat(issuesDir); err == nil {
				FatalError("this workspace is already initialized; use --force to refresh templates")
			}
		}

		if err := initFileBackend(taskLedgerDir); err != nil {
			FatalError("failed to initialize file backend: %v", err)
		}

		if quiet {
			return
		}
		fmt.Printf("\n%s tl initialized successfully\n\n", ui.RenderPass("OK"))
		fmt.Printf("  Storage: %s\n", ui.RenderAccent(filepath.Join(taskLedgerDir, "issues")))
		fmt.Printf("  Start: %s\n\n", ui.RenderAccent("tl create FR \"Your feature\""))
	},
}

func isInsideTaskLedgerDir(path string) bool {
	clean := filepath.Clean(path)
	sepTaskLedger := string(filepath.Separator) + ".task-ledger" + string(filepath.Separator)
	return strings.Contains(clean, sepTaskLedger) || strings.HasSuffix(clean, string(filepath.Separator)+".task-ledger")
}

func initFileBackend(taskLedgerDir string) error {
	if err := os.MkdirAll(filepath.Join(taskLedgerDir, "issues"), 0o750); err != nil {
		return err
	}
	if err := createConfigYaml(taskLedgerDir); err != nil {
		return err
	}
	if err := createReadme(taskLedgerDir); err != nil {
		return err
	}
	return nil
}

func init() {
	initCmd.Flags().BoolP("quiet", "q", false, "Suppress output")
	initCmd.Flags().Bool("force", false, "Force re-initialization of templates")
	rootCmd.AddCommand(initCmd)
}
