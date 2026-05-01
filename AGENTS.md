# Agent Instructions

See [AGENT_INSTRUCTIONS.md](AGENT_INSTRUCTIONS.md) for current JSON file backend instructions.

This file exists for compatibility with tools that look for AGENTS.md.

## Visual Design Anti-Patterns

**NEVER use emoji-style icons** (🔴🟠🟡🔵⚪) in CLI output. They cause cognitive overload.

**ALWAYS use small Unicode symbols** with semantic colors:
- Status: `○ ◐ ● ✓ ❄`
- Priority: `● P0` (filled circle with color)

See CLAUDE.md "Visual Design System" section for full guidance.

## Session Completion

When ending a work session, make sure code, tests, docs, and `.task-ledger/issues/**/issue.json` task changes are handled like normal repository files.

Recommended workflow:

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Use normal Git**:
   ```bash
   git pull --rebase
   git add <changed files>
   git commit -m "Describe change"
   git push
   git status
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed and pushed
7. **Hand off** - Provide context for next session

Do not run legacy `tl sync`; the JSON file backend writes issue JSON files directly.
