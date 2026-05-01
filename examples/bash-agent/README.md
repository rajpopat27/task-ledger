# Bash Agent Example

A shell script demonstrating how an AI agent can use the JSON-file `tl` CLI to manage tasks autonomously.

## Features

- Pure bash implementation (no Python/Node required)
- Colorized terminal output
- Automatic work discovery
- Random issue creation to simulate real agent behavior
- Dependency linking with `discovered-from`
- Status counts display

## Prerequisites

- bash 4.0+
- `tl` on PATH
- jq for JSON parsing: `brew install jq` (macOS) or `apt install jq` (Linux)
- A repository initialized with `tl init`
- At least one open task under an epic

## Usage

```bash
# Make executable
chmod +x agent.sh

# Run with default 10 iterations
./agent.sh

# Run with custom iteration limit
./agent.sh 20
```

## What It Does

The agent runs in a loop:

1. Looks for ready work (no blockers)
2. Claims the task (sets status to `in_progress`)
3. "Works" on it (simulates 1 second of work)
4. 50% chance to discover a follow-up issue
5. If discovered, creates and links the new issue
6. Completes the original task
7. Shows statistics and repeats

## Seed Data

```bash
tl init
fr=$(tl create FR "Agent demo" --silent)
ep=$(tl create epic "Implementation" --parent "$fr" --silent)
tl create task "Implement user authentication" --parent "$ep" --priority P1
```

## Example Output

```
🚀 Task Ledger Agent starting...
   Max iterations: 10

═══════════════════════════════════════════════════
  tl Status Counts
═══════════════════════════════════════════════════
open 5
in_progress 0
closed 2

═══════════════════════════════════════════════════
  Iteration 1/10
═══════════════════════════════════════════════════
ℹ Looking for ready work...
ℹ Claiming task: tl-3
✓ Task claimed
ℹ Working on: Fix authentication bug (tl-3)
  Priority: 1
⚠ Discovered issue while working!
✓ Created issue: tl-8
✓ Linked tk-8abcde <- discovered-from <- tk-3f19ac
ℹ Completing task: tl-3
✓ Task completed: tl-3
```

## Use Cases

**Continuous Integration**
```bash
# Run agent in CI to process testing tasks
./agent.sh 5
```

**Cron Jobs**
```bash
# Run agent every hour
0 * * * * cd /path/to/project && /path/to/agent.sh 3
```

**One-off Task Processing**
```bash
# Process exactly one task and exit
./agent.sh 1
```

## Customization

Edit the script to customize behavior:

```bash
# Change discovery probability (line ~80)
if [[ $((RANDOM % 2)) -eq 0 ]]; then  # 50% chance
# Change to:
if [[ $((RANDOM % 10)) -lt 3 ]]; then  # 30% chance

# Add assignee filtering
tl ready --json --assignee "bot" --limit 1

# Add priority filtering
tl ready --json --priority 1 --limit 1

# Add custom labels
tl create task "New task" --parent "$epic_id" -l "automated,agent-discovered"
```

## Integration with Real Agents

This script is a starting point. To integrate with a real LLM:

1. Replace `do_work()` with calls to your LLM API
2. Parse the LLM's response for tasks to create
3. Use issue IDs to maintain context
4. Track conversation state in issue comments or notes

## See Also

- [../python-agent/](../python-agent/) - Python version with more flexibility
