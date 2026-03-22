# Agent Instructions

This project uses **bd** (beads) for issue tracking — issues chained together like beads. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work (open + no blocking deps)
bd show <id>          # View issue details
bd update <id> -a <role> --status in_progress  # Claim work
bd close <id>         # Complete work
bd dolt push          # Push beads data to remote
```

## Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i` (interactive) mode on some systems, causing the agent to hang indefinitely waiting for y/n input.

**Use these forms instead:**
```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**Other commands that may prompt:**
- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Dolt-powered version control with native sync
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Creating Issues

```bash
bd create "Fix login bug"
bd create "Add auth" -p 0 -t feature -a developer --json
bd create "Write tests" -d "Unit tests for auth" -a tester --json
bd create "Found bug" -d "Details" -p 1 --deps discovered-from:<parent-id> --json
```

### Viewing Issues

```bash
bd list                  # List all issues
bd list --status open    # List by status
bd list --assignee developer  # List by assignee
bd list --priority 0     # List by priority (0-4, 0=highest)
bd show <id>             # Show issue details
bd ready --json          # Show issues ready to work on
```

Ready = status is `open` AND no blocking dependencies. Use `bd ready` to find work.

### Updating Issues

```bash
bd update <id> -a <role> --status in_progress --json  # Claim work for a role
bd update <id> -a reviewer --json                     # Reassign to another role
bd update <id> --priority 0 --json                    # Change priority
```

**Do NOT use `--claim`** — it sets the assignee from git config, which doesn't match our role-based assignee names. Always use `-a <role>` explicitly.

### Closing Issues

```bash
bd close <id> --reason "Completed" --json
bd close <id1> <id2> --reason "Fixed in PR #42" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Dependencies

```bash
bd dep add <id1> <id2>   # Add dependency (id2 blocks id1)
bd dep tree <id>         # Visualize dependency tree
bd dep cycles            # Detect circular dependencies
```

**Dependency types:**

- `blocks` - Task B must complete before task A
- `related` - Soft connection, doesn't block progress
- `parent-child` - Epic/subtask hierarchical relationship
- `discovered-from` - Auto-created when discovering related work

### Workflow for AI Agents

1. **Check ready work**: `bd ready --json` shows unblocked issues
2. **Claim your task**: `bd update <id> -a <role> --status in_progress --json`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" --description="Details about what was found" -p 1 --deps discovered-from:<parent-id> --json`
5. **Complete**: `bd close <id> --reason "Done" --json`

### Auto-Sync

bd automatically syncs via Dolt:

- Each write auto-commits to Dolt history
- Use `bd dolt push`/`bd dolt pull` for remote sync
- No manual export/import needed

### Database Location

bd automatically discovers the database:

1. `--db /path/to/db.db` flag
2. `$BEADS_DB` environment variable
3. `.beads/*.db` in current directory or ancestors
4. `~/.beads/default.db` as fallback

### Important Rules

- Always use `--json` flag for programmatic use
- Always use `-a <role>` to set assignee (never `--claim`)
- Link discovered work with `discovered-from` dependencies
- Check `bd ready` before asking "what should I work on?"
- Do NOT create markdown TODO lists
- Do NOT use external issue trackers
- Do NOT duplicate tracking systems

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
