# Rodeo🤠Crush💖

An orchestration harness that runs a team of [Crush](https://github.com/renier/crush/tree/headless-prompts) AI agents in tmux windows, coordinated through the [beads](https://github.com/bead-code/beads) issue tracker.

**NOTE**: Highly experimental and subject to change. Created as a proof of concept using a fork of crush, that adds additional options to the cli. The core idea is to use Crush agents as specialized workers in a software development pipeline, with tmux for visibility and beads for task management. Use at your own risk!

## How It Works

Rodeo Crush spins up a tmux session with multiple Crush TUI instances, each assigned a specific role in a software development pipeline:

| Role | What it does |
|---|---|
| **Architect** | Reads SEED.md, produces a technical design (DESIGN.md), creates implementation beads |
| **Developer** | Implements code in git worktrees, one bead at a time |
| **Reviewer** | Reviews code for correctness and design adherence |
| **Tester** | Writes and runs tests, closes beads that pass |

Work flows through the pipeline via beads assignees (`developer` -> `reviewer` -> `tester`). The Architect kicks things off by reading `SEED.md` and creating beads assigned to `developer`. Each subsequent agent continuously polls for beads matching its assignee name and processes them.

If no `SEED.md` is present in the project directory, the Architect starts idle with a "stand by for instructions" prompt — you can send it work manually via the Crush TUI.

A background monitor watches for stalled agents and prods them back to work.

## Requirements

- [tmux](https://github.com/tmux/tmux)
- [Crush](https://github.com/renier/crush/tree/headless-prompts) (headless-prompts branch)
- [bd (beads)](https://github.com/bead-code/beads) issue tracker
- Go 1.26+

## Install

```bash
go install github.com/renier/rodeo-crush/cmd/rodeo@latest
```

Or build from source:

```bash
git clone https://github.com/renier/rodeo-crush.git
cd rodeo-crush
make build
```

## Development

A Makefile is provided for common tasks. Run `make help` to see all targets:

```
make build          Build the rodeo binary
make install        Install rodeo to $GOPATH/bin
make clean          Remove build artifacts
make test           Run all tests
make test-verbose   Run all tests with verbose output
make test-race      Run tests with race detector
make cover          Run tests with coverage and open report
make vet            Run go vet
make fmt            Format all Go source files
make fmt-check      Check formatting (fails if files need formatting)
make lint           Run all linters (vet + format check)
make tidy           Tidy and verify go.mod
make ci             Run full CI pipeline (lint, race tests, build)
```

## Usage

```bash
# Start the orchestrator (runs in foreground, Ctrl+C to stop)
rodeo

# Monitor beads progress in another terminal
rodeo beads

# Custom team config
rodeo -t my-team.yaml

# Specify project directory
rodeo -d /path/to/project

# Custom timeouts
rodeo -s 5m -p 1m
```

### Flags

| Flag | Description | Default |
|---|---|---|
| `-t, --team` | Team config YAML file | `~/.config/rodeo-crush/team.yaml` |
| `-d, --dir` | Project directory | Current directory |
| `-s, --stall` | Stall timeout before prodding agents | `15m` |
| `-p, --poll` | Poll interval for stall detection | `30s` |
| `-l, --log` | Log file path | `~/.config/rodeo-crush/rodeo-crush.log` |

## Configuration

On first run, Rodeo Crush creates a config directory at `~/.config/rodeo-crush/` with default files:

```
~/.config/rodeo-crush/
  team.yaml                  # Team configuration
  prompts/
    architect.md             # Architect system prompt
    developer.md             # Developer system prompt
    reviewer.md              # Reviewer system prompt
    tester.md                # Tester system prompt
```

Edit these files to customize the system. Existing files are never overwritten.

### Team Configuration

The `team.yaml` defines your team as a list of roles. Each role specifies its name, how many agents to run, its assignee name, how to find work, and the system prompt to use:

```yaml
roles:
  - name: Architect
    count: 1
    assignee: "architect"
    prompt_file: prompts/architect.md
    send_prompt_once: true
    filter:
      assignee: "architect"
      status: open

  - name: Developer
    count: 2
    assignee: "developer"
    prompt_file: prompts/developer.md
    filter:
      assignee: "developer"
      ready: true           # only pick up beads with resolved deps
    worktree: true           # use git worktrees for isolation

  - name: Reviewer
    count: 1
    assignee: "reviewer"
    prompt_file: prompts/reviewer.md
    filter:
      assignee: "reviewer"
      status: in_progress
```

#### Role fields

| Field | Required | Description |
|---|---|---|
| `name` | yes | Display name (used for tmux window titles) |
| `count` | yes | Number of parallel agents for this role |
| `assignee` | yes | Beads assignee name for this role (e.g. `developer`) |
| `prompt` | * | Inline system prompt text |
| `prompt_file` | * | Path to a prompt markdown file (relative to config dir) |
| `filter.assignee` | yes | Beads assignee name to query for work |
| `filter.status` | no | Status filter (`open`, `in_progress`, `blocked`) |
| `filter.ready` | no | If `true`, only pick up beads with all deps resolved |
| `worktree` | no | If `true`, agent gets git worktree instructions |
| `send_prompt_once` | no | If `true`, prompt is sent once and the agent is not re-prompted |

\* One of `prompt` or `prompt_file` is required. If both are set, `prompt` takes precedence.

### Custom Prompts

Each prompt file is a markdown document that tells the Crush agent how to do its job. Rodeo Crush wraps it with a preamble (identity, project path, rules) and a work loop (the `bd list` command derived from `filter`).

You can edit the default prompts or write entirely new ones. Example custom role:

```yaml
  - name: Security Auditor
    count: 1
    assignee: "security"
    prompt_file: prompts/security_auditor.md
    filter:
      assignee: "security"
      status: in_progress
```

With `prompts/security_auditor.md`:

```markdown
## Your Role: Security Auditor

You audit code for security vulnerabilities on beads assigned to "security".

### When you pick up a bead:
1. Read the bead description for context on what was implemented.
2. Review the code for:
   - SQL injection, XSS, and other injection vulnerabilities.
   - Authentication and authorization issues.
   - Secrets in code or config.
   - Insecure dependencies.
3. Update the bead description with findings.
4. If issues found: reassign to developer and set status open.
5. If clean: close the bead with reason "Security audit passed".
```

### Inline vs File Prompts

For quick experiments you can inline the prompt directly in `team.yaml`:

```yaml
  - name: Linter
    count: 1
    assignee: "linter"
    prompt: |
      You run linting and formatting checks.
      Fix any issues you find, then close the bead.
    filter:
      assignee: "linter"
      status: open
```

For anything substantial, `prompt_file` is recommended for readability.

### Per-Project Overrides

Pass `-t` to use a project-specific team config instead of the global one:

```bash
rodeo -t ./my-project-team.yaml -d /path/to/project
```

Prompt file paths in that config resolve relative to the directory containing the YAML file.

## Architecture

```
cmd/rodeo/              CLI entrypoint (subcommands: rodeo, rodeo beads)
internal/
  config/               Team YAML parsing, validation, config dir bootstrap
  roles/                Prompt assembly (preamble + user prompt + work loop + worktree)
  tmux/                 tmux session/window management
  orchestrator/         Lifecycle management, SEED.md detection, stall detection, agent prodding
  tui/                  Bubble Tea beads dashboard (rodeo beads)
```

Each agent gets its own tmux window running `crush --listen <socket>`, which starts the full Crush TUI. The orchestrator then sends the initial prompt to each agent via `crush run --socket <socket>` from a Go goroutine outside tmux. This two-phase approach lets users see the live TUI in each window -- tab between agents with `Ctrl-b n` / `Ctrl-b p`. The monitor captures window output to detect stalls and prods agents through the same socket mechanism.

## Workflow

1. Write a `SEED.md` in your project root describing what you want built
2. Run `rodeo` to start the orchestrator
3. The beads dashboard runs automatically in the first tmux window
4. Watch the agents collaborate in tmux (`tmux attach -t rodeo`), tab between windows with `Ctrl-b n` / `Ctrl-b p`
5. The Architect reads SEED.md, creates DESIGN.md and implementation beads
6. Beads flow through the pipeline automatically until tests pass and work is closed

## License

MIT
