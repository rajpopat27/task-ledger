# Python Agent Example

A simple Python script demonstrating how an AI agent can use the JSON-file `tl` CLI to manage tasks.

## Features

- Finds ready work using `tl ready --json`
- Claims tasks by updating status
- Simulates discovering new issues during work
- Links discovered issues with `discovered-from` dependency
- Completes tasks and moves to the next one

## Prerequisites

- Python 3.7+
- `tl` on PATH.
- A repository initialized with `tl init`.
- At least one open task under an epic.

## Usage

```bash
# Make the script executable
chmod +x agent.py

# Run the agent
./agent.py
```

## What It Does

1. Queries for ready work (no blocking dependencies)
2. Claims the highest priority task
3. "Works" on the task (simulated)
4. If the task involves implementation, discovers a testing task
5. Creates the new testing task under the same parent epic and links it with `discovered-from`
6. Completes the original task
7. Repeats until no ready work remains

## Example Output

```
🚀 Task Ledger Agent starting...

============================================================
Iteration 1/10
============================================================

📋 Claiming task: tl-1
🤖 Working on: Implement user authentication (tl-1)
   Priority: 1, Type: feature

💡 Discovered: Missing test coverage for this feature
✨ Creating issue: Add tests for Implement user authentication
🔗 Linking tl-2 ← discovered-from ← tl-1
✅ Completing task: tl-1 - Implemented successfully

🔄 New work discovered and linked. Running another cycle...
```

## Seed Data

```bash
tl init
fr=$(tl create FR "Agent demo" --silent)
ep=$(tl create epic "Implementation" --parent "$fr" --silent)
tl create task "Implement user authentication" --parent "$ep" --priority P1
```

## Integration with Real Agents

To integrate with a real LLM-based agent:

1. Replace `simulate_work()` with actual LLM calls
2. Parse the LLM's response for discovered issues/bugs
3. Use the issue ID to track context across conversations
4. Use the issue JSON files in `.task-ledger/issues` as persistent context across sessions

## Advanced Usage

```python
# Create an agent with custom behavior
agent = TaskLedgerAgent()

# Find specific types of work
ready = agent.run_tl("ready", "--priority", "1", "--assignee", "bot")

# Create issues with labels
agent.run_tl("create", "task", "New task", "--parent", epic_id, "-l", "urgent,backend")

# Query dependency tree
tree = agent.run_tl("dep", "tree", task_id)
```

## See Also

- [../bash-agent/](../bash-agent/) - Bash version of this example
