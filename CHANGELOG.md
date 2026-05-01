# Changelog

## JSON-File Fork

- Replaced the legacy SQLite/daemon/sync-branch product path with direct per-issue JSON files under `.task-ledger/issues/**/issue.json`.
- Retained the core CLI workflow: `init`, `create`, `list`, `show`, `update`, `label`, `comments`, `dep`, `ready`, `blocked`, `close`, `reopen`, `count`, `search`, `epic`, `doctor`, `delete`, `import-jsonl`, and `version`.
- Removed legacy command packages and internal packages for daemon/RPC, SQLite, sync branches, import/export loops, Linear sync, auto-import, formulas, molecules, routing, and compacting.
- The file backend rebuilds a command-local issue index from `.task-ledger/issues/**/issue.json` for each CLI invocation.
- Removed SQLite driver dependencies from `go.mod` and `go.sum`.
- Updated tests and CLI smoke coverage for the JSON-file command surface, including typed IDs for parented feature requests, epics, tasks, bugs, and chores.

Historical upstream release notes are intentionally not carried in this fork's changelog because they describe removed backends and commands.
