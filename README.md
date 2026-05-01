# tl - Task Ledger JSON File Fork

**Git-native issue tracking for agents, backed by ordinary JSON files.**

This fork removes the primary SQLite/daemon/sync-branch product path and stores issues directly under `.task-ledger/issues/**/issue.json`. The CLI loads those files for each command, applies the change, and writes stable formatted JSON back to disk. There is no external service and no database required for normal use.

Package identity: the Go module is `task-ledger`; the installed CLI binary is `tl`. The fork version starts at `0.48.0` because this JSON-file product path is intentionally different from its upstream base.

## Quick Start

```bash
# Install from a clone
make install

# Or build a local binary
make build

# Initialize a repository
tl init

# Create a feature request, epic, and task
fr=$(tl create FR "Checkout revamp" --silent)
ep=$(tl create epic "Payment flow" --parent "$fr" --silent)
tk=$(tl create task "Add card validation" --parent "$ep" --priority P1 --silent)

# Query the work graph
tl ready
tl show "$tk"
```

## Storage Layout

`tl init` creates `.task-ledger/issues`. Each issue is a standalone JSON file:

```text
.task-ledger/
  issues/
    fr-a3f2dd/
      issue.json
      epics/
        ep-91c0af/
          issue.json
          tasks/
            tk-b841aa/
              issue.json
```

The issue file includes the issue body, labels, dependencies, comments, scheduling fields, audit metadata, and hierarchy pointers. Git sees normal text files, so review, merge, and agent inspection work without a database dump step.

## Supported Issue Types

| Type | ID Prefix | Role |
| --- | --- | --- |
| `FR`, `feature_request`, `feature` | `fr-` | Top-level feature request |
| `epic` | `ep-` | Work stream under a feature request |
| `task` | `tk-` | Task under an epic |
| `bug` | `bug-` | Bug task |
| `chore` | `ch-` | Maintenance task |

Typical hierarchy:

```bash
tl create FR "Billing migration"
tl create epic "Invoices" --parent fr-a3f2dd
tl create task "Render PDF invoice" --parent ep-91c0af
```

## Retained Commands

The fork keeps the core tracking surface:

| Command | Purpose |
| --- | --- |
| `tl init` | Create `.task-ledger/issues` |
| `tl create` | Create feature requests, epics, tasks, bugs, and chores |
| `tl list`, `tl search`, `tl count` | Query issues |
| `tl show` | Show one or more issues |
| `tl update` | Edit fields, labels, parent, schedule, assignee, status |
| `tl close`, `tl reopen`, `tl delete` | Lifecycle operations |
| `tl dep` | Add, remove, list, and visualize dependencies |
| `tl label` | Batch label operations |
| `tl comments`, `tl comment` | Add and view comments |
| `tl ready`, `tl blocked` | Find ready or blocked work |
| `tl epic` | Epic status and close-eligible helpers |
| `tl doctor` | File-backend health checks |
| `tl import-jsonl` | Import legacy JSONL snapshots into per-issue JSON files |
| `tl version` | Version output |

Legacy commands tied to SQLite, daemon sync, multi-repo routing, molecules, formulas, gates, Jira/Linear sync, config DB state, import/export JSONL sync, and protected sync branches are pruned or hidden from the retained CLI.

## Global Flags

Supported global flags:

```bash
tl --json list --all
tl --actor raj update tk-b841aa --claim
tl --readonly list --all
tl --verbose ready
```

Unsupported legacy flags such as `--db`, `--no-daemon`, `--no-auto-flush`, `--sandbox`, `--allow-stale`, `--lock-timeout`, and `--profile` are hidden.

## Documentation

Start with:

- [Quickstart](docs/QUICKSTART.md)
- [CLI Reference](docs/CLI_REFERENCE.md)
- [JSON File Backend](docs/JSON_FILE_BACKEND.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Agent Instructions](AGENT_INSTRUCTIONS.md)

Legacy docs and automation for removed backend, sync, integration, and publishing workflows have been removed from this fork.
