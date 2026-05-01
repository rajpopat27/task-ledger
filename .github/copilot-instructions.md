# GitHub Copilot Instructions for tl JSON File Fork

## Project Overview

`tl` is a Git-backed issue tracker for AI-supervised coding workflows. This fork stores each issue as a normal JSON file under `.task-ledger/issues/**/issue.json`.

**Key Features:**
- Dependency-aware issue tracking.
- FR -> epic -> task hierarchy.
- AI-optimized CLI with JSON output.
- Direct JSON file storage, without a required database, daemon, sync branch, or JSONL export loop.

## Tech Stack

- **Language**: Go 1.25.8+
- **Storage**: JSON files in `internal/storage/file/`
- **CLI Framework**: Cobra
- **Testing**: Go standard testing + table-driven tests

## Coding Guidelines

### Testing
- Always write tests for new features
- Use temporary directories for CLI/e2e tests.
- Run `GOFLAGS=-buildvcs=false go test ./...` before committing if Git worktree metadata is incomplete.
- Never create test issues in the repository's real `.task-ledger/issues` tree.

### Code Style
- Run `golangci-lint run ./...` before committing
- Follow existing patterns in `cmd/tl/` for new commands
- Add `--json` flag to all commands for programmatic use
- Update docs when changing behavior

### Git Workflow
- Review `.task-ledger/issues/**/issue.json` changes like normal source files.
- Do not run legacy `tl sync`, `tl daemon`, `tl import`, or `tl export`.

## Issue Tracking with tl

**CRITICAL**: This project uses **tl** for ALL task tracking. Do NOT create markdown TODO lists.

### Essential Commands

```bash
# Find work
tl ready --json                    # Unblocked issues
tl blocked --json                  # Blocked issues

# Create and manage (ALWAYS include --description)
fr=$(tl create FR "Feature request" --silent)
ep=$(tl create epic "Implementation slice" --parent "$fr" --silent)
tl create task "Title" --parent "$ep" --description="Detailed context" --priority P1 --json
tl --actor copilot update <id> --claim --json
tl close <id> --reason "Done" --json

# Search
tl list --all --status open --priority P1 --json
tl show <id> --json
```

### Workflow

1. **Check ready work**: `tl ready --json`
2. **Claim task**: `tl --actor copilot update <id> --claim`
3. **Work on it**: Implement, test, document
4. **Discover new work?** `tl create task "Found bug" --parent <epic-id> --description="What was found and why" --priority P1 --json`
5. **Complete**: `tl close <id> --reason "Done" --json`
6. **Review issue files**: include any relevant `.task-ledger/issues/**/issue.json` changes in the commit

**IMPORTANT**: Always include `--description` when creating issues. Issues without descriptions lack context for future work.

### Priorities

- `P0` - Critical (security, data loss, broken builds)
- `P1` - High (major features, important bugs)
- `P2` - Medium (default, nice-to-have)
- `P3` - Low (polish, optimization)
- `P4` - Backlog (future ideas)

## Project Structure

```
beads/
├── cmd/tl/              # CLI commands (add new commands here)
├── internal/
│   ├── types/           # Core data types
│   └── storage/file/    # JSON file backend
├── examples/            # Integration examples
├── docs/                # Documentation
└── .task-ledger/
    └── issues/          # Per-issue JSON files
```

## Available Resources

### Key Documentation
- **AGENTS.md** - Comprehensive AI agent guide (detailed workflows, advanced features)
- **AGENT_INSTRUCTIONS.md** - Development procedures and testing
- **README.md** - User-facing documentation
- **docs/CLI_REFERENCE.md** - Complete command reference

## Important Rules

- ✅ Use tl for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Use temporary directories for test repositories
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT create test issues in the real repo issue tree
- ❌ Do NOT use legacy SQLite/daemon/sync commands

---

**For detailed workflows and advanced features, see [AGENTS.md](../AGENTS.md)**
