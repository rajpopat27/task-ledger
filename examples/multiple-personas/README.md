# Multiple Personas Workflow

This example uses labels and dependencies to coordinate different roles on the same feature request.

## Setup

```bash
tl init

fr=$(tl create FR "Design new caching layer" --priority P1 --silent)
architecture=$(tl create epic "Architecture" --parent "$fr" --priority P1 --labels architecture --silent)
implementation=$(tl create epic "Implementation" --parent "$fr" --priority P1 --labels implementation --silent)
review=$(tl create epic "Review and hardening" --parent "$fr" --priority P1 --labels review --silent)

tl dep add "$implementation" "$architecture"
tl dep add "$review" "$implementation"
```

## Architect

```bash
adr=$(tl create task "Write ADR: caching layer design" --parent "$architecture" --priority P1 --labels architecture --silent)
tl create task "Evaluate Redis vs Memcached" --parent "$architecture" --priority P1 --labels architecture
tl create task "Design cache invalidation strategy" --parent "$architecture" --priority P1 --labels architecture

tl list --all --label architecture
tl ready --json
```

## Implementer

After the architecture epic is closed, implementation becomes ready:

```bash
tl close "$architecture" --reason "Architecture decisions recorded"

pool=$(tl create task "Implement Redis connection pool" --parent "$implementation" --priority P1 --labels implementation --silent)
middleware=$(tl create task "Add cache middleware to API routes" --parent "$implementation" --priority P1 --labels implementation --silent)

tl dep add "$pool" "$adr" --type related
tl dep add "$middleware" "$adr" --type related

tl update "$pool" --status in_progress
```

## Reviewer

```bash
tl close "$implementation" --reason "Initial implementation complete"

review_task=$(tl create task "Code review: Redis caching layer" --parent "$review" --priority P1 --labels review --silent)
bug=$(tl create bug "Fix connection leak on timeout" --parent "$review" --priority P0 --labels review,bug --silent)

tl dep add "$review_task" "$bug"
tl comments add "$review_task" "Review is blocked until the timeout leak is fixed."

tl blocked
```

## Product Owner

```bash
tl list --all --type feature
tl list --all --priority P0
tl list --all --status in_progress
tl count --by-label
```

This keeps role-specific views lightweight while preserving one source of truth in `.task-ledger/issues/**/issue.json`.
