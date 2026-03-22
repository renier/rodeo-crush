package roles

import (
	"fmt"
	"strings"

	"github.com/renier/rodeo-crush/internal/config"
)

// Prompt generates the full system prompt for a given agent by combining the
// preamble, user-defined role prompt, work-finding instructions, and optional
// worktree instructions.
func Prompt(agent config.Agent, projectDir string) string {
	role := agent.RoleDef
	var b strings.Builder
	b.WriteString(preamble(agent, projectDir))
	b.WriteString("\n\n")
	b.WriteString(role.Prompt)
	b.WriteString("\n\n")
	b.WriteString(workInstructions(role))
	if role.Worktree {
		b.WriteString("\n\n")
		b.WriteString(worktreeInstructions(projectDir))
	}
	return b.String()
}

func preamble(agent config.Agent, projectDir string) string {
	return fmt.Sprintf(`You are "%s", an AI agent in the `+config.AppName+` orchestration team.
You work in the project at: %s
You track work using the "bd" (beads) CLI issue tracker.
Your role is: %s. This is also your assignee name.

IMPORTANT RULES:
- Work on ONE bead at a time.
- Always use --json flag when calling bd for programmatic output.
- Never modify beads that don't belong to your role.`, agent.Name, projectDir, agent.RoleDef.Assignee)
}

func workInstructions(role *config.RoleDef) string {
	return fmt.Sprintf(`## Finding Work

Find beads assigned to your role using:
`+"```bash"+`
%s
`+"```"+`

Process:
1. Run the query above to find beads assigned to your role.
2. Pick the highest-priority bead (lowest priority number = highest priority).
3. Work on it following your role instructions above.`, role.FilterCommand())
}

func worktreeInstructions(projectDir string) string {
	return fmt.Sprintf(`## Git Worktree Instructions

For each bead you work on, use a separate git worktree to avoid conflicts:

`+"```bash"+`
# Get the current branch
BRANCH=$(git -C %s rev-parse --abbrev-ref HEAD)
BEAD_ID=<bead-id>

# Create worktree
git -C %s worktree add %s-${BRANCH}-${BEAD_ID} -b work/${BEAD_ID}

# Work in the worktree directory
cd %s-${BRANCH}-${BEAD_ID}

# ... do your work ...

# When done, rebase and merge back
git rebase ${BRANCH}
git checkout ${BRANCH}
git merge work/${BEAD_ID}

# Clean up
git -C %s worktree remove %s-${BRANCH}-${BEAD_ID}
git branch -d work/${BEAD_ID}
`+"```"+`

IMPORTANT: Always rebase against the source branch before merging back.
Handle any rebase conflicts yourself.`, projectDir, projectDir, projectDir, projectDir, projectDir, projectDir)
}
