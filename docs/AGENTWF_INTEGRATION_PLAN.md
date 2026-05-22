# Agentwf Integration Plan

## Goal

Support a local deterministic agent workflow orchestrator that uses Task Ledger as
the reviewable JSON task source of truth. Tickets are assumed to be created by an
external process; the orchestrator only discovers, claims, transitions, comments,
and closes existing issues through `tl`.

## Current Fit

Task Ledger already provides the core task-store behavior needed by the
orchestrator:

- JSON issue files under `.task-ledger/issues/**/issue.json`
- `tl ready --json` for dependency-aware ready work
- `tl update <id> --claim --json` for atomic claim and `in_progress`
- `tl update <id> --status ... --json` for state transitions
- `tl comments add <id> ... --json` for human-readable feedback
- `tl close <id> --reason ... --json` for completion
- `tl dep add/remove/list` for dependency management

## Required Task Ledger Changes

### 1. Preserve Orchestrator Metadata

Preserve an optional `agentwf` object in the issue schema:

```json
{
  "agentwf": {
    "worker_agent_id": "go-coder",
    "review_agent_ids": ["go-reviewer", "security-reviewer"],
    "delivery_sha": null,
    "feedback": []
  }
}
```

Purpose:

- `worker_agent_id`: selected worker agent directory.
- `review_agent_ids`: selected reviewer agent directories.
- `delivery_sha`: final delivered commit after human approval.
- `feedback`: structured feedback records for the next worker run.

Task Ledger should round-trip the object but not interpret its internal schema.
Unknown top-level JSON fields are not enough because typed read/write paths can
drop fields when rewriting `issue.json`.

### 2. Add Workflow Statuses

Add first-class statuses:

```text
in_review
human_review
```

The orchestrator mapping is:

```text
open          -> ready for worker pickup
in_progress   -> worker running
in_review     -> AI reviewers running
human_review  -> waiting for human review
closed        -> done
blocked       -> blocked
```

### 3. Tighten Ready Pickup

Expose a way to fetch only unclaimed open work:

```bash
tl ready --status open --unassigned --json
```

The important contract is that worker pickup only starts from `status=open` and
no assignee.

## Deferred Task Ledger Changes

### Compare-And-Set Mutations

For safer orchestration, add optional expected-value flags to mutating commands:

```bash
tl update <id> --status in_review \
  --expect-status in_progress \
  --expect-assignee agentwf:<board_id>:<ticket_id>:<agent_id>
```

Minimum useful expectations:

- `--expect-status`
- `--expect-assignee`
- `--expect-updated-at` or a future issue revision field

This keeps transitions deterministic when users or tools edit the same ticket
between orchestrator reads and writes. It is not required for the first local
adapter because `tl update --claim` already protects initial pickup.

### Safe Claim Clearing

Add a safe way to clear the orchestrator claim after delivery:

```bash
tl update <id> --clear-assignee \
  --expect-assignee agentwf:<board_id>:<ticket_id>:<agent_id>
```

This prevents clearing a human or another agent's claim by accident.

## V1 Adapter Contract

The orchestrator should use only this Task Ledger surface:

```text
tl ready --status open --unassigned --json
tl show <id> --json
tl update <id> --claim --json
tl update <id> --status <status> --json
tl comments add <id> <feedback> --json
tl close <id> --reason <reason> --json
tl dep list <id> --json
```

It should never edit `.task-ledger/issues/**/issue.json` directly.

## Deferred

- Task creation APIs in the orchestrator.
- Multi-project routing inside `tl`.
- Runtime state storage in Task Ledger.
- Agent run history in Task Ledger.
- Docker/container lifecycle metadata in Task Ledger.

Those remain owned by the orchestrator runtime store.
