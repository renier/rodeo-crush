# Example: GitLab MR Review Team

A Rodeo Crush configuration that creates an autonomous MR review pipeline. A
Manager agent discovers merge requests from GitLab, and a pool of Reviewer agents
reviews them — all coordinated through beads.

## How it works

```
┌─────────────┐       beads       ┌─────────────┐
│   Manager   │ ──── creates ────▶│  Reviewer   │
│  (1 agent)  │   MR review tasks │  (3 agents) │
└──────┬──────┘                   └──────┬──────┘
       │                                 │
       │  glab mr list                   │  glab mr diff / checkout
       │  (discovers MRs)                │  (reviews code)
       ▼                                 ▼
   ┌────────┐                        ┌────────┐
   │ GitLab │                        │ GitLab │
   └────────┘                        └────────┘
```

| Role | Count | What it does |
|---|---|---|
| **Manager** | 1 | Polls `glab mr list` for MRs needing review, creates a bead per MR |
| **Reviewer** | 3 | Picks up beads, checks out the MR branch, performs a thorough code review, posts comments |

The Manager runs on a loop (re-prompted every 60s by the orchestrator), checking
for new MRs each cycle. It deduplicates against existing beads so the same MR is
never queued twice.

Reviewers pick up beads in priority order. For each MR, the reviewer gathers context
(`glab mr view`, `glab mr diff`, existing comments), checks out the branch, and
performs a deep code review covering correctness, error handling, concurrency safety,
API design, performance, style, tests, and security. It then presents findings to the
user and **waits for explicit confirmation** before posting inline comments to the MR.
Once comments are posted, the bead is closed.

To review an MR again, you can re-open its corresponding bead (within the Manager window or from another terminal), and the reviewer will pick it right up again.

## Prerequisites

- [glab](https://gitlab.com/gitlab-org/cli) CLI installed and authenticated
- [tmux](https://github.com/tmux/tmux)
- [Crush](https://github.com/renier/crush/tree/headless-prompting) (headless-prompting branch)
- [bd (beads)](https://github.com/bead-code/beads) issue tracker
- A beads database initialized in your project (`bd init`)

Verify glab authentication:

```bash
glab auth status
```

## Usage

From the root of a GitLab-tracked repository:

```bash
rodeo -t /path/to/rodeo-crush/examples/gitlab-mr-review/team.yaml
```

Or specify the project directory explicitly:

```bash
rodeo -t /path/to/rodeo-crush/examples/gitlab-mr-review/team.yaml -d /path/to/your/repo
```

Then attach to the tmux session to watch the agents work:

```bash
tmux attach -t rodeo
```

Navigate between agent windows with `Ctrl-b n` (next) and `Ctrl-b p` (previous).

## Customization

### Scaling reviewers

Change the `count` field in `team.yaml` to add or remove reviewer agents:

```yaml
  - name: Reviewer
    count: 5  # more agents for a larger MR queue
```

### MR discovery filters

Edit `prompts/manager.md` to change which MRs are discovered. For example, to
review MRs across multiple projects:

```bash
glab mr list --reviewer=@me --group=my-org --json id,iid,title,webUrl,...
```

Or to filter by label:

```bash
glab mr list --label=needs-ai-review --json id,iid,title,webUrl,...
```

### Review depth

Edit `prompts/reviewer.md` to adjust the review dimensions. The default prompt
covers correctness, error handling, concurrency, API design, performance, style,
tests, and security. Remove or add sections based on your needs.

### Human-in-the-loop

The reviewer follows the original `gitlab-mr-review` skill workflow: it presents
findings to the user and **stops to wait for confirmation** before posting anything
to GitLab. This means each reviewer agent will block until a human approves, adjusts,
or rejects the findings via the Crush TUI. Scale the reviewer `count` based on how
many MRs you can triage in parallel.

## File structure

```
examples/gitlab-mr-review/
├── README.md                  # This file
├── team.yaml                  # Team configuration (2 roles)
└── prompts/
    ├── manager.md             # MR discovery prompt
    └── reviewer.md            # Code review prompt
```
