# CLI Reference

This reference covers the JSON file backend fork. Commands not listed here are legacy or unsupported.

## Global Flags

Global flags must appear before the subcommand:

```bash
tl --json list --all
tl --actor raj update tk-b841aa --claim
tl --readonly list --all
tl --verbose ready
```

| Flag | Purpose |
| --- | --- |
| `--json` | JSON output when supported |
| `--actor <name>` | Actor for audit metadata |
| `--readonly` | Block write commands |
| `--verbose`, `-v` | Debug output |

Unsupported legacy globals such as `--db`, `--no-daemon`, `--no-auto-flush`, `--sandbox`, `--allow-stale`, `--lock-timeout`, and `--profile` are hidden.

## Init

```bash
tl init
tl init --quiet
tl init --force --quiet
```

Creates `.task-ledger/issues`. `--force` allows reinitializing when issue files already exist.

Unsupported legacy init flags include `--prefix`, `--branch`, `--contributor`, `--team`, `--stealth`, `--setup-exclude`, `--skip-hooks`, `--skip-merge-driver`, and `--from-jsonl`.

## Create

```bash
tl create FR "Feature request title"
tl create epic "Epic title" --parent fr-a3f2dd
tl create task "Task title" --parent ep-91c0af
tl create bug "Bug title" --parent ep-91c0af
tl create chore "Chore title" --parent ep-91c0af
```

Useful flags:

```bash
tl create task "Title" --parent ep-91c0af --priority P1
tl create task "Title" --description "Long body"
tl create task "Title" --description-file desc.md
tl create task "Title" --body-file desc.md
echo "body" | tl create task "Title" --description -
tl create task "Title" --design "Design notes"
tl create task "Title" --acceptance "Acceptance criteria"
tl create task "Title" --notes "Implementation notes"
tl create task "Title" --external-ref GH-123
tl create task "Title" --labels backend,urgent
tl create task "Title" --estimate 60
tl create task "Title" --due +2h
tl create task "Title" --defer tomorrow
tl create task "Title" --deps bug-e24d91
tl create task "Title" --dry-run
tl create task "Title" --json
tl create task "Title" --silent
```

Supported types: `FR`, `feature_request`, `feature`, `epic`, `task`, `bug`, `chore`.

Unsupported legacy create flags include `--file`, `--id`, `--waits-for`, `--waits-for-gate`, `--force`, `--repo`, `--rig`, `--prefix`, `--ephemeral`, `--mol-type`, `--role-type`, `--agent-rig`, and event flags.

## List, Search, Count

```bash
tl list
tl list --all
tl list --all --json
tl list --all --status open
tl list --all --priority P0
tl list --all --assignee raj
tl list --all --type task
tl list --all --label backend
tl list --all --label-any frontend,backend
tl list --all --title "card"
tl list --all --id tk-a,tk-b
tl list --all --limit 20
tl list --all --long
tl list --all --sort created --reverse
tl list --all --parent ep-91c0af
tl list --all --filter-parent ep-91c0af
tl list --all --tree
tl list --all --ready
```

Text and date filters:

```bash
tl list --all --title-contains auth
tl list --all --desc-contains token
tl list --all --notes-contains TODO
tl list --all --created-after 2026-01-01
tl list --all --created-before 2026-12-31
tl list --all --updated-after 2026-01-01
tl list --all --closed-after 2026-01-01
tl list --all --empty-description
tl list --all --no-assignee
tl list --all --no-labels
tl list --all --priority-min P0 --priority-max P2
tl list --all --deferred
tl list --all --overdue
```

Search:

```bash
tl search card
tl search --query card --status open --type task --label backend --limit 5
```

Count:

```bash
tl count
tl count --status open
tl count --by-status
tl count --by-priority
tl count --by-type
tl count --by-assignee
tl count --by-label
tl count --priority-min P0 --priority-max P2
```

Only one `--by-*` grouping flag may be used at a time.

## Show

```bash
tl show tk-b841aa
tl show tk-b841aa tk-2d91fa
tl show --short tk-b841aa
tl show --json tk-b841aa
```

Missing IDs exit non-zero. Legacy `--thread` and `--refs` are unsupported by the JSON file backend.

## Update

```bash
tl update tk-b841aa --title "New title"
tl update tk-b841aa --type chore
tl update tk-b841aa --status in_progress
tl update tk-b841aa --priority P1
tl update tk-b841aa --assignee raj
tl update tk-b841aa --description-file desc.md
tl update tk-b841aa --design "New design"
tl update tk-b841aa --acceptance "Acceptance criteria"
tl update tk-b841aa --notes "Notes"
tl update tk-b841aa --external-ref GH-123
tl update tk-b841aa --estimate 90
tl update tk-b841aa --due +3h
tl update tk-b841aa --due ""
tl update tk-b841aa --defer +30m
tl update tk-b841aa --defer ""
tl update tk-b841aa --add-label qa
tl update tk-b841aa --remove-label urgent
tl update tk-b841aa --set-labels final,backend
tl update tk-b841aa --parent ep-other
tl --actor raj update tk-b841aa --claim
```

`--claim` sets the assignee to the actor and moves the issue to `in_progress`. If the issue is already claimed, the command exits non-zero.

## Dependencies

```bash
tl dep add tk-b841aa bug-e24d91
tl dep tk-b841aa --blocked-by bug-e24d91
tl dep bug-e24d91 --blocks tk-b841aa
tl dep add tk-b841aa tk-related --type related
tl dep list tk-b841aa
tl dep list tk-b841aa --direction down
tl dep list bug-e24d91 --direction up --type blocks
tl dep tree tk-b841aa
tl dep tree tk-b841aa --direction both --max-depth 5
tl dep tree tk-b841aa --format mermaid
tl dep cycles
tl dep remove tk-b841aa bug-e24d91
```

## Labels

```bash
tl label add tk-b841aa tk-other backend
tl label remove tk-b841aa tk-other backend
tl label list tk-b841aa
tl --json label list tk-b841aa
tl --json label list-all
```

## Comments

```bash
tl comments add tk-b841aa "Comment text"
tl comments add tk-b841aa --file comment.md --author raj
tl comment tk-b841aa "Comment text"
tl comments tk-b841aa
tl --json comments tk-b841aa
```

`comment` is an alias for adding a comment.

## Ready and Blocked

```bash
tl ready
tl ready --limit 10
tl ready --priority 0
tl ready --assignee raj
tl ready --unassigned
tl ready --sort priority
tl ready --label backend
tl ready --label-any backend,frontend
tl ready --type task
tl ready --parent ep-91c0af
tl ready --include-deferred
tl blocked
tl blocked --parent ep-91c0af
```

Legacy molecule/gate filters on `ready` are unsupported.

## Close, Reopen, Delete

```bash
tl close tk-b841aa --reason "Done"
tl close tk-a tk-b --reason "Done"
tl close tk-b841aa --resolution "Fixed"
tl close tk-b841aa --session session-1
tl close tk-b841aa --suggest-next
tl reopen tk-b841aa --reason "More work needed"
tl delete tk-b841aa --dry-run
tl delete tk-b841aa --force --reason "Duplicate"
tl delete --from-file delete-ids.txt --force --reason "Cleanup"
```

Delete is a soft delete. It writes a tombstone with `deleted_at`, `deleted_by`, `delete_reason`, and `original_type`, then hides the tombstone from normal list/search/count output.

Legacy `close --continue`, `close --no-auto`, `delete --cascade`, and `delete --hard` are unsupported.

## Epic

```bash
tl epic status
tl epic status --eligible-only
tl epic close-eligible --dry-run
```

## Doctor

```bash
tl doctor
tl doctor --json
```

Legacy doctor repair flags such as `--fix`, `--yes`, `--interactive`, `--dry-run`, `--fix-child-parent`, `--force`, and `--source` are unsupported by the retained JSON-file CLI.

## Import Legacy JSONL

```bash
tl import-jsonl .task-ledger/issues.jsonl
```

This converts legacy JSONL records into per-issue `.task-ledger/issues/**/issue.json` files. The importer is all-or-nothing and rejects malformed JSON, unsafe IDs, duplicate IDs, duplicate `external_ref` values, and conflicts with existing issue files.
