package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// createConfigYaml creates the config.yaml template in the specified directory
func createConfigYaml(taskLedgerDir string) error {
	configYamlPath := filepath.Join(taskLedgerDir, "config.yaml")

	// Skip if already exists
	if _, err := os.Stat(configYamlPath); err == nil {
		return nil
	}

	configYamlTemplate := `# Task Ledger Configuration File
# This file configures default behavior for all tl commands in this repository
# All settings can also be set via environment variables (TL_* prefix)
# or overridden with command-line flags

# Storage is direct JSON files:
# .task-ledger/issues/<fr-id>/issue.json
# .task-ledger/issues/<fr-id>/epics/<ep-id>/issue.json
# .task-ledger/issues/<fr-id>/epics/<ep-id>/tasks/<task-id>/issue.json

# Enable JSON output by default
# json: false

# Default actor for audit trails (overridden by TL_ACTOR or --actor)
# actor: ""

# Require descriptions on new issues
# create:
#   require-description: false

# Template validation mode for create: none, warn, or error
# validation:
#   on-create: none

# Maximum child nesting depth for hierarchical IDs
# hierarchy:
#   max-depth: 3
`

	if err := os.WriteFile(configYamlPath, []byte(configYamlTemplate), 0600); err != nil {
		return fmt.Errorf("failed to write config.yaml: %w", err)
	}

	return nil
}

// createReadme creates the README.md file in the .task-ledger directory
func createReadme(taskLedgerDir string) error {
	readmePath := filepath.Join(taskLedgerDir, "README.md")

	// Skip if already exists
	if _, err := os.Stat(readmePath); err == nil {
		return nil
	}

	readmeTemplate := `# Task Ledger - AI-Native Issue Tracking

Welcome to Task Ledger! This repository uses **Task Ledger** for issue tracking - a modern, AI-native tool designed to live directly in your codebase alongside your code.

## What is Task Ledger?

Task Ledger is issue tracking that lives in your repo, making it perfect for AI coding agents and developers who want their issues close to their code. No web UI required - everything works through the CLI and integrates seamlessly with git.

**Learn more:** See the repository docs and ` + "`tl --help`" + `.

## Quick Start

### Essential Commands

` + "```bash" + `
# Create new issues
tl create FR "Add user authentication"
tl create epic "Login flow" --parent <fr-id>
tl create task "Build login form" --parent <ep-id>

# View all issues
tl list

# View issue details
tl show <issue-id>

# Update issue status
tl update <issue-id> --status in_progress
tl update <issue-id> --status done

# Validate the issue files
tl doctor
` + "```" + `

### Working with Issues

Issues in Task Ledger are:
- **Git-native**: Stored as ` + "`.task-ledger/issues/**/issue.json`" + ` files and synced like code
- **AI-friendly**: CLI-first design works perfectly with AI coding agents
- **File-first**: Direct JSON issue files are the source of truth

## Why Task Ledger?

✨ **AI-Native Design**
- Built specifically for AI-assisted development workflows
- CLI-first interface works seamlessly with AI coding agents
- No context switching to web UIs

🚀 **Developer Focused**
- Issues live in your repo, right next to your code
- Works offline, syncs when you push
- Fast, lightweight, and stays out of your way

🔧 **Git Integration**
- Issue files are regular JSON files
- Small per-issue writes reduce conflicts
- Git history is the history

## Get Started with Task Ledger

Try Task Ledger in your own projects:

` + "```bash" + `
# Build task-ledger from source
go build -buildvcs=false -o tl ./cmd/tl

# Initialize in your repo
tl init

# Create your first issue
tl create FR "Try out Task Ledger"
` + "```" + `

## Learn More

- **Documentation**: See the repository ` + "`docs/`" + ` directory
- **Quick Start Guide**: Run ` + "`tl quickstart`" + `
- **Examples**: See the repository ` + "`examples/`" + ` directory

---

*Task Ledger: Issue tracking that moves at the speed of thought* ⚡
`

	// Write README.md (0644 is standard for markdown files)
	// #nosec G306 - README needs to be readable
	if err := os.WriteFile(readmePath, []byte(readmeTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write README.md: %w", err)
	}

	return nil
}
