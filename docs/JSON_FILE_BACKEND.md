# JSON File Backend

This fork stores Task Ledger data as ordinary JSON files. The file backend is the default and only supported product path for the retained CLI.

## Goals

- No SQLite database for normal operation.
- No daemon, background sync process, or external service.
- No central JSONL index required for correctness.
- A small, inspectable file per issue.
- Git-friendly paths that make feature requests, epics, and tasks easy for humans and agents to find.
- Full CLI behavior for the core issue-tracking workflow.

## Directory Layout

`tl init` creates:

```text
.task-ledger/
  config.yaml
  README.md
  issues/
```

Issues are stored by hierarchy:

```text
.task-ledger/issues/
  fr-a3f2dd/
    issue.json
    epics/
      ep-91c0af/
        issue.json
        tasks/
          tk-b841aa/
            issue.json
          bug-e24d91/
            issue.json
```

Top-level feature requests use `fr-` IDs. Epics live below their feature request. Tasks, bugs, and chores live below their epic. If an issue is created without a parent, it is stored as `.task-ledger/issues/<id>/issue.json`.

## Issue File Format

Each `issue.json` is stable formatted JSON:

```json
{
  "schema_version": 1,
  "id": "tk-b841aa",
  "type": "task",
  "title": "Add card validation",
  "status": "open",
  "priority": 1,
  "parent": "ep-91c0af",
  "feature_request": "fr-a3f2dd",
  "epic": "ep-91c0af",
  "deps": ["bug-e24d91"],
  "dependencies": [],
  "labels": ["backend"],
  "assignee": "raj",
  "estimated_minutes": 60,
  "description": "Validate card details before submit.",
  "comments": [],
  "created_at": "2026-05-01T10:00:00Z",
  "created_by": "raj",
  "updated_at": "2026-05-01T10:00:00Z",
  "closed_at": null
}
```

Important fields:

| Field | Meaning |
| --- | --- |
| `schema_version` | On-disk schema version. Currently `1`. |
| `id` | Stable issue ID. |
| `type` | `feature_request`, `feature`, `epic`, `task`, `bug`, or `chore`. |
| `parent` | Parent issue ID for hierarchy. |
| `feature_request` | Top-level feature request ID for nested work. |
| `epic` | Epic ID for tasks, bugs, and chores. |
| `deps` | Convenience list of blocking issue IDs. |
| `dependencies` | Full dependency records with dependency type and metadata. |
| `labels` | Sorted label list. |
| `comments` | Inline comments for the issue. |
| `due_at`, `defer_until` | Optional scheduling timestamps. |
| `deleted_at`, `deleted_by`, `delete_reason` | Tombstone metadata when imported or represented as deleted. |

## Write Model

Each mutating command:

1. Loads `.task-ledger/issues/**/issue.json`.
2. Acquires `.task-ledger/.lock`.
3. Reloads the command-local representation from disk after the lock is acquired.
4. Applies the command to that command-local representation.
5. Writes touched `issue.json` files with temp-file replacement.
6. Moves files when hierarchy changes, for example after `tl update <id> --parent <epic>`.
7. Rolls back touched files if a transaction fails.

The command-local map is temporary process state only. It is rebuilt from disk for every command and is not a database.

Fresh lock files are respected and cause writers to wait until timeout. Stale lock files older than the backend's stale-lock threshold are removed automatically before the writer retries.

## Manual Editing Contract

Direct edits are supported for inspection and emergency repair, but files must remain valid JSON. Keep these invariants:

- `id` values must be unique.
- `external_ref` values must be unique when present.
- IDs may contain ASCII letters, digits, hyphen, underscore, and dot. They must not contain path separators or traversal segments.
- `type` must be `feature_request`, `feature`, `epic`, `task`, `bug`, or `chore`.
- `status` must be one of the retained statuses, including `tombstone` for soft-deleted issues.
- Parent links and hierarchy paths should agree. Prefer `tl update <id> --parent <parent-id>` instead of moving files manually.
- Same-issue edits on two Git branches can still conflict; resolve the JSON file like normal text.

## Delete Policy

`tl delete --force --reason <text>` soft-deletes issues by converting them to tombstones. Tombstones keep their issue file, original type, deletion actor, deletion timestamp, and reason. Normal list/search/count output hides tombstones; `tl doctor --json` still validates them.

## Supported Commands

The retained CLI surface is:

```text
blocked
close
comment
comments
count
create
delete
dep
doctor
epic
help
import-jsonl
init
label
list
ready
reopen
search
show
update
version
```

Unsupported legacy commands are removed from help and should not be documented for this fork.

## Unsupported Legacy Surface

The following are intentionally not part of the JSON file backend product:

- SQLite database selection (`--db`) for normal CLI use.
- Daemon/RPC mode (`tl daemon`, `--no-daemon`, auto-start daemon).
- Sync branch and protected-branch workflows (`tl init --branch`, `tl sync`).
- Merge driver setup for monolithic JSONL.
- `tl init --stealth`, `--contributor`, `--team`.
- Multi-repo routing flags such as `--repo`, `--rig`, and related daemon commands.
- Molecule/formula/gate/event/template/wisp/swarm command families.

## Importing Old JSONL

Use `import-jsonl` to migrate legacy `.task-ledger/issues.jsonl` data:

```bash
tl init
tl import-jsonl /path/to/issues.jsonl
```

The importer parses the whole JSONL file before writing anything. It rejects malformed lines, unsafe IDs, duplicate IDs, duplicate `external_ref` values, and conflicts with existing issue files. If any record fails validation, no imported issue files are written.

## AI Agent Guidance

Agents should interact through the CLI, not by editing JSON directly. Direct edits are possible, but the CLI keeps dependencies, paths, labels, timestamps, and comments consistent.

Recommended agent commands:

```bash
tl ready --json
tl show <id> --json
tl create task "Title" --parent <epic-id> --description "..." --json
tl update <id> --status in_progress --json
tl comments add <id> "Note" --json
tl close <id> --reason "Done" --json
```

For repository searches, agents can also use normal tools:

```bash
find .task-ledger/issues -name issue.json
jq '.id, .title, .status' .task-ledger/issues/fr-a3f2dd/epics/ep-91c0af/tasks/tk-b841aa/issue.json
```
