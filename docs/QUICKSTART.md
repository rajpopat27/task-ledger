# Task Ledger JSON File Quickstart

Get a repository running with the file-backed fork in a few minutes.

## Build

```bash
go build -buildvcs=false -o tl ./cmd/tl
./tl --help
```

`-buildvcs=false` is useful in copied or repaired worktrees where Git metadata is incomplete.

## Initialize

```bash
tl init
```

This creates:

```text
.task-ledger/
  issues/
```

There is no SQLite database, daemon, sync branch, merge driver, or monolithic `.task-ledger/issues.jsonl` required for normal operation.

## Create Work

Create a top-level feature request:

```bash
tl create FR "Checkout revamp"
```

Create an epic under that feature request:

```bash
tl create epic "Payment flow" --parent fr-a3f2dd
```

Create tasks under the epic:

```bash
tl create task "Add card validation" --parent ep-91c0af --priority P1 --labels backend
tl create bug "Fix expired card error" --parent ep-91c0af --priority P0
```

IDs are hash-based and typed:

```text
fr-a3f2dd  feature request
ep-91c0af  epic
tk-b841aa  task
bug-e24d91 bug
ch-9910cc  chore
```

## Inspect the Files

The created task is written as:

```text
.task-ledger/issues/fr-a3f2dd/epics/ep-91c0af/tasks/tk-b841aa/issue.json
```

The CLI is the supported way to mutate issues, but the files are ordinary JSON and can be inspected with `jq`, `rg`, or any editor.

## Link Dependencies

```bash
tl dep add tk-b841aa bug-e24d91
tl dep list tk-b841aa
tl dep tree tk-b841aa
```

Use `--type related` for non-blocking relationships:

```bash
tl dep add tk-b841aa tk-other --type related
```

## Find Ready Work

```bash
tl ready
tl ready --json
tl blocked
```

`ready` shows open work with no open blockers. `blocked` shows work that cannot start because dependencies are still open.

## Update Work

```bash
tl update tk-b841aa --status in_progress
tl update tk-b841aa --assignee raj
tl update tk-b841aa --add-label qa
tl update tk-b841aa --due +2h
tl update tk-b841aa --defer tomorrow
```

Claim an unassigned issue atomically:

```bash
tl --actor raj update tk-b841aa --claim
```

If another actor already claimed it, the command exits non-zero.

## Comment and Close

```bash
tl comments add tk-b841aa "Validation implemented"
tl comments tk-b841aa
tl close tk-b841aa --reason "Done"
tl reopen tk-b841aa --reason "Need one more edge case"
```

## Query

```bash
tl list --all
tl list --all --status open --type task
tl list --all --label backend
tl list --all --parent ep-91c0af
tl search "card"
tl count --by-status
tl count --priority-min P0 --priority-max P2
tl show tk-b841aa --json
```

## Import Legacy JSONL

If you have an old `.task-ledger/issues.jsonl` file:

```bash
tl init
tl import-jsonl .task-ledger/issues.jsonl
```

The import writes per-issue JSON files under `.task-ledger/issues`. It validates the full input before writing, so malformed lines or duplicate IDs do not leave a partial import.

## Delete Work

```bash
tl delete tk-b841aa --dry-run
tl delete tk-b841aa --force --reason "Duplicate"
```

Delete keeps a tombstone issue file with deletion metadata. Normal list/search/count output hides tombstones.

## Unsupported Old Workflow

These old entry points are not part of this fork's supported CLI:

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

See [JSON_FILE_BACKEND.md](JSON_FILE_BACKEND.md) for the storage contract and retained command surface.
