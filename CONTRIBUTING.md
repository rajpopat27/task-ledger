# Contributing

This fork is a JSON-file-only `tl` CLI. Normal use stores issues under `.task-ledger/issues/**/issue.json`; there is no SQLite backend, daemon mode, sync branch, or generated JSONL index in the retained product surface.

## Setup

Requirements:

- Go 1.25.8 or later
- Git
- `jq` for the Makefile smoke test

Useful commands:

```bash
go build -buildvcs=false -o tl ./cmd/tl
make install
go test ./... -count=1
go test ./... -covermode=atomic -coverprofile=/tmp/task-ledger.coverage.out -count=1
go tool cover -func=/tmp/task-ledger.coverage.out | tail -1
make smoke
```

Use `-buildvcs=false` when working from copied or repaired worktrees where Git metadata may be incomplete.

## Project Layout

```text
cmd/tl/                    CLI commands
internal/storage/file/     JSON-file storage backend and command-local issue index
internal/types/            Core issue, dependency, comment, and filter types
internal/utils/            ID parsing and resolution helpers
docs/                      JSON-file backend docs
```

## Testing Expectations

Changes should be test-driven where practical:

- Add or update focused unit tests for storage and helper behavior.
- Add CLI-level tests in `cmd/tl/file_backend_test.go` for command behavior.
- Run `go test ./... -count=1` before handing off.
- Run an e2e CLI smoke if the change touches command wiring, flags, output, file layout, or storage writes.

The retained CLI should keep working without external services. Tests should create temporary repositories and should not depend on any long-running process.

## Repository Task Data

Do not include unrelated `.task-ledger/issues/**/issue.json` changes in normal code PRs. These files are repository task data. If a test creates task data, it should do so under `t.TempDir()` or another temporary directory.

## Style

- Use `gofmt`.
- Keep changes scoped to the JSON-file backend and retained CLI surface.
- Prefer existing helper APIs and storage interfaces over new abstractions.
- Keep user-facing docs aligned with `tl --help`.
