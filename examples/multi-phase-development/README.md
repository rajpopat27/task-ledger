# Multi-Phase Development Workflow

This example shows how to model a larger feature request with epics for each phase and tasks under each epic.

## Setup

```bash
tl init

fr=$(tl create FR "Build real-time collaboration system" --priority P1 --silent)

research=$(tl create epic "Phase 1: Research" --parent "$fr" --priority P1 --silent)
mvp=$(tl create epic "Phase 2: MVP" --parent "$fr" --priority P1 --silent)
iteration=$(tl create epic "Phase 3: Iteration" --parent "$fr" --priority P2 --silent)
hardening=$(tl create epic "Phase 4: Production hardening" --parent "$fr" --priority P3 --silent)
```

## Order the Phases

```bash
tl dep add "$mvp" "$research"
tl dep add "$iteration" "$mvp"
tl dep add "$hardening" "$iteration"
```

`tl ready` will now surface the first unblocked phase and its unblocked child tasks.

## Research Tasks

```bash
tl create task "Evaluate Socket.IO vs native WebSockets" --parent "$research" --priority P1
tl create task "Research operational transform vs CRDT" --parent "$research" --priority P1
tl create task "Document technical decisions" --parent "$research" --priority P2

tl ready
```

## Move to MVP

```bash
tl close "$research" --reason "Research complete; chose Socket.IO and CRDT"

tl create task "Set up Socket.IO server" --parent "$mvp" --priority P1
tl create task "Implement basic CRDT for text" --parent "$mvp" --priority P1
tl create task "Build simple UI for testing" --parent "$mvp" --priority P2

tl ready
```

## Track Discovered Work

When implementation uncovers more work, create it under the same epic and link it back to the task that exposed it:

```bash
task=$(tl create task "Add reconnection logic" --parent "$mvp" --priority P1 --silent)
bug=$(tl create bug "Socket disconnects on network change" --parent "$mvp" --priority P0 --silent)

tl dep add "$bug" "$task" --type discovered-from
tl comments add "$bug" "Found while testing reconnection behavior."
```

## Inspect Progress

```bash
tl show "$fr"
tl list --all --parent "$mvp"
tl dep tree "$fr"
tl count --by-status
```

All state is stored as normal JSON files under `.task-ledger/issues`, so Git diffs show exactly which feature request, epic, or task changed.
