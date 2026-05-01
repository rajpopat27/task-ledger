# Benchmarks

The old SQLite benchmark suite was removed with the SQLite backend.

Current performance work should benchmark the JSON-file backend in `internal/storage/file`. Add benchmarks next to the code they measure and run them with:

```bash
GOFLAGS=-buildvcs=false go test -run '^$' -bench 'FileStorage(Open|List|Search)1000Issues|CLI(List|Search)1000Issues' -benchmem ./internal/storage/file ./cmd/tl
```

## 1000 Issue Baseline

Run on Linux amd64, AMD Ryzen 5 7600X, with `-benchtime=3x`:

```text
BenchmarkFileStorageOpen1000Issues-12       24.13 ms/op
BenchmarkFileStorageList1000Issues-12        0.42 ms/op
BenchmarkFileStorageSearch1000Issues-12      0.08 ms/op
BenchmarkCLIList1000Issues-12               52.37 ms/op
BenchmarkCLISearch1000Issues-12             54.07 ms/op
```

The direct storage path stays comfortably below interactive latency for 1000 issues. End-to-end CLI list/search include process startup and disk scan, and were about 50-55 ms locally.

Useful benchmark targets for this fork:

- Cold load of `.task-ledger/issues/**/issue.json` for 100, 500, and 1000 issues.
- `ready`, `blocked`, `list`, and text search over a loaded issue set.
- Single-file writes for `update`, `comments add`, `label add`, and `dep add`.
- Transaction rollback for multi-file operations.

Keep benchmark fixtures generated in temporary directories so repository task data is never modified.
