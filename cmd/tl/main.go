package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"task-ledger/internal/config"
	"task-ledger/internal/debug"
	"task-ledger/internal/hooks"
	"task-ledger/internal/storage"
	filestorage "task-ledger/internal/storage/file"
	"task-ledger/internal/taskledger"
	"task-ledger/internal/utils"
)

var (
	actor      string
	store      storage.Storage
	jsonOutput bool

	rootCtx    context.Context
	rootCancel context.CancelFunc

	hookRunner *hooks.Runner

	readonlyMode bool
	verboseFlag  bool
	quietFlag    bool
	storeMutex   sync.Mutex
	storeActive  bool
)

func getActorWithGit() string {
	if actor != "" {
		return actor
	}
	if tlActor := os.Getenv("TL_ACTOR"); tlActor != "" {
		return tlActor
	}
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		if gitUser := strings.TrimSpace(string(out)); gitUser != "" {
			return gitUser
		}
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "unknown"
}

func getOwner() string {
	if authorEmail := os.Getenv("GIT_AUTHOR_EMAIL"); authorEmail != "" {
		return authorEmail
	}
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		if gitEmail := strings.TrimSpace(string(out)); gitEmail != "" {
			return gitEmail
		}
	}
	return ""
}

func init() {
	if err := config.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize config: %v\n", err)
	}

	rootCmd.PersistentFlags().StringVar(&actor, "actor", "", "Actor name for audit trail (default: $TL_ACTOR, git user.name, $USER)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&readonlyMode, "readonly", false, "Read-only mode: block write operations")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "Enable verbose/debug output")
	rootCmd.PersistentFlags().BoolVarP(&quietFlag, "quiet", "q", false, "Suppress non-essential output (errors only)")

	rootCmd.Flags().BoolP("version", "V", false, "Print version information")

	rootCmd.AddGroup(&cobra.Group{ID: "issues", Title: "Working With Issues:"})
	rootCmd.AddGroup(&cobra.Group{ID: "views", Title: "Views & Reports:"})
	rootCmd.AddGroup(&cobra.Group{ID: "deps", Title: "Dependencies & Structure:"})
	rootCmd.AddGroup(&cobra.Group{ID: "setup", Title: "Setup & Configuration:"})
	rootCmd.AddGroup(&cobra.Group{ID: "maint", Title: "Maintenance:"})

	rootCmd.SetHelpFunc(colorizedHelpFunc)
}

var rootCmd = &cobra.Command{
	Use:   "tl",
	Short: "tl - Dependency-aware issue tracker",
	Long:  `Task Ledger tracks work as JSON files with first-class dependency support.`,
	Run: func(cmd *cobra.Command, args []string) {
		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Printf("tl version %s (%s)\n", Version, Build)
			return
		}
		_ = cmd.Help()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		rootCtx, rootCancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		debug.SetVerbose(verboseFlag)
		debug.SetQuiet(quietFlag)

		if !cmd.Flags().Changed("json") {
			jsonOutput = config.GetBool("json")
		}
		if !cmd.Flags().Changed("readonly") {
			readonlyMode = config.GetBool("readonly")
		}
		if !cmd.Flags().Changed("actor") && actor == "" {
			actor = config.GetString("actor")
		}

		if shouldSkipStoreInit(cmd) {
			actor = getActorWithGit()
			return
		}

		if err := initializeFileBackendForCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		storeMutex.Lock()
		storeActive = false
		storeMutex.Unlock()

		if store != nil {
			_ = store.Close()
			store = nil
		}
		if rootCancel != nil {
			rootCancel()
		}
	},
}

func shouldSkipStoreInit(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}
	if cmd.Parent() == nil && cmd.Name() == "tl" {
		return true
	}
	if v, _ := cmd.Flags().GetBool("version"); v {
		return true
	}
	noStoreCommands := map[string]bool{
		"completion": true,
		"doctor":     true,
		"fish":       true,
		"help":       true,
		"init":       true,
		"version":    true,
		"zsh":        true,
		"bash":       true,
		"powershell": true,
	}
	if noStoreCommands[cmd.Name()] {
		return true
	}
	if cmd.Parent() != nil && noStoreCommands[cmd.Parent().Name()] {
		return true
	}
	return false
}

func initializeFileBackendForCommand() error {
	taskLedgerDir := ""
	if envDir := os.Getenv("TASK_LEDGER_DIR"); envDir != "" {
		taskLedgerDir = utils.CanonicalizePath(envDir)
	} else {
		taskLedgerDir = taskledger.FindTaskLedgerDir()
	}
	if taskLedgerDir == "" {
		return fmt.Errorf("no .task-ledger directory found; run 'tl init' first or set TASK_LEDGER_DIR")
	}

	fileStore, err := filestorage.Open(taskLedgerDir)
	if err != nil {
		return fmt.Errorf("open file storage: %w", err)
	}

	actor = getActorWithGit()
	store = fileStore

	storeMutex.Lock()
	storeActive = true
	storeMutex.Unlock()

	hookRunner = hooks.NewRunner(filepath.Join(taskLedgerDir, "hooks"))
	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
