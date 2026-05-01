# tl JSON File Examples

These examples target the JSON-file fork. They use only the retained CLI surface and write normal issue files under `.task-ledger/issues/**/issue.json`.

## Examples

- **[python-agent/](python-agent/)** - Python agent loop that claims ready work, creates follow-ups, and closes tasks.
- **[bash-agent/](bash-agent/)** - Shell-only version of the same agent workflow, using `jq` for JSON parsing.
- **[multi-phase-development/](multi-phase-development/)** - Planning a larger feature request with epics, tasks, and blockers.
- **[multiple-personas/](multiple-personas/)** - Role-based labels and handoffs for architect, implementer, reviewer, and product work.

The old examples for daemon monitoring, Git hook import/export, Jira/Linear/GitHub sync, markdown-to-JSONL import, protected sync branches, and library-level SQLite use were removed because they do not apply to this fork.

## Quick Start

```bash
tl init

fr=$(tl create FR "Example feature request" --silent)
ep=$(tl create epic "Implementation slice" --parent "$fr" --silent)
tl create task "First task" --parent "$ep"

cd examples/python-agent
python agent.py
```

## Creating Your Own Agent

The basic agent workflow:

1. Find ready work: `tl ready --json --limit 1`
2. Claim the task: `tl --actor <agent> update <id> --claim --json`
3. Do the work.
4. Create discovered work: `tl create task "Found issue" --parent <epic-id> --json`
5. Link discoveries: `tl dep add <new-id> <parent-id> --type discovered-from`
6. Complete the task: `tl close <id> --reason "Done" --json`

Use the CLI rather than editing issue files directly. The files stay reviewable JSON, while the CLI preserves validation, audit fields, hierarchy, and dependency updates.
