# Agent Instructions

These instructions apply to the JSON file backend fork.

## Source Of Truth

Use the `tl` CLI for task tracking. The CLI stores issues as ordinary JSON files under:

```text
.task-ledger/issues/**/issue.json
```

There is no SQLite database, daemon, sync branch, or JSONL import/export loop in the supported workflow.

## Common Agent Commands

```bash
tl ready --json
tl blocked --json
tl list --all --json
tl show <id> --json
tl create task "Title" --parent <epic-id> --description "Details" --json
tl update <id> --status in_progress --json
tl update <id> --add-label needs-test --json
tl comments add <id> "Note" --json
tl close <id> --reason "Done" --json
```

Use `--actor <name>` when the actor should be explicit:

```bash
tl --actor codex update <id> --claim --json
```

Use `--readonly` in analysis-only contexts:

```bash
tl --readonly list --all --json
```

## Work Hierarchy

Create work in this shape unless the user asks otherwise:

```bash
fr=$(tl create FR "Feature request" --silent)
ep=$(tl create epic "Implementation slice" --parent "$fr" --silent)
tk=$(tl create task "Concrete task" --parent "$ep" --silent)
```

File layout:

```text
.task-ledger/issues/<fr>/issue.json
.task-ledger/issues/<fr>/epics/<ep>/issue.json
.task-ledger/issues/<fr>/epics/<ep>/tasks/<task>/issue.json
```

## Development Guidelines

- Go version: 1.25.8+
- Add tests for every behavior change.
- Prefer e2e CLI tests for command behavior.
- Prefer storage tests for file layout, schema, and transaction behavior.
- Update docs when user-visible commands, flags, or storage layout changes.
- Do not reintroduce SQLite, daemon, sync-branch, or JSONL-as-primary-storage behavior into the retained CLI.

## File Organization

```text
cmd/tl/                         CLI commands
cmd/tl/zz_file_backend_commands.go
                                retained command allowlist
internal/storage/file/          JSON file backend
internal/types/                 core issue/dependency/comment types
docs/JSON_FILE_BACKEND.md       storage contract
docs/CLI_REFERENCE.md           supported command reference
```

The removed SQLite, daemon, RPC, sync-branch, molecule, formula, routing, and old importer/exporter packages should stay removed.

## Testing Workflow

For focused JSON-file backend changes:

```bash
go test ./internal/storage/file -count=1
go test ./cmd/tl -run 'TestFileBackend' -count=1
```

For command-surface changes:

```bash
go test ./cmd/tl -count=1
```

Before handing off production-ready work:

```bash
go test ./... -count=1
```

If the checkout has broken Git worktree metadata, use:

```bash
GOFLAGS=-buildvcs=false go test ./... -count=1
```

## Manual QA

Run CLI QA in a temporary directory so repository task data is not polluted:

```bash
tmp=$(mktemp -d)
tl_bin=/path/to/tl
cd "$tmp"
"$tl_bin" init --quiet
fr=$("$tl_bin" create FR "Manual QA" --silent)
ep=$("$tl_bin" create epic "CLI coverage" --parent "$fr" --silent)
tk=$("$tl_bin" create task "Exercise commands" --parent "$ep" --silent)
"$tl_bin" update "$tk" --status in_progress
"$tl_bin" comments add "$tk" "Manual QA note"
"$tl_bin" close "$tk" --reason "Verified"
```

## Git Workflow

The task files are normal repository files. After CLI changes, review and commit the changed `.task-ledger/issues/**/issue.json` files like any other source file.

Do not run legacy `tl sync`, `tl daemon`, `tl import`, `tl export`, or `tl migrate` as part of this fork's normal workflow.

## Unsupported Legacy Surface

Agents should not use:

```text
tl daemon
tl sync
tl import
tl export
tl migrate
tl config
tl init --branch
tl init --stealth
tl init --contributor
tl init --team
tl --db
tl --no-daemon
```

Use [docs/CLI_REFERENCE.md](docs/CLI_REFERENCE.md) as the command source of truth.
