package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Bootstrap ensures the config directory exists and contains default files.
// It creates $HOME/.config/rodeo-crush/ with team.yaml and prompts/*.md
// if they don't already exist. It returns the config directory path.
func Bootstrap() (string, error) {
	cfgDir, err := ConfigDir()
	if err != nil {
		return "", err
	}

	promptsDir := filepath.Join(cfgDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return "", fmt.Errorf("creating config directory: %w", err)
	}

	teamPath := filepath.Join(cfgDir, "team.yaml")
	if err := writeIfMissing(teamPath, defaultTeamYAML()); err != nil {
		return "", fmt.Errorf("writing default team.yaml: %w", err)
	}

	for name, content := range DefaultPromptFiles() {
		path := filepath.Join(promptsDir, name)
		if err := writeIfMissing(path, content); err != nil {
			return "", fmt.Errorf("writing default prompt %s: %w", name, err)
		}
	}

	return cfgDir, nil
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func defaultTeamYAML() string {
	cfg := DefaultTeam()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		panic(fmt.Sprintf("marshaling default config: %v", err))
	}
	return "# " + AppName + " Team Configuration\n" +
		"# Edit this file to customize roles, counts, labels, and prompts.\n" +
		"# Prompt text can be inline (prompt:) or loaded from a file (prompt_file:).\n" +
		"# Paths in prompt_file are relative to this directory (~/.config/" + ConfigDirName + "/).\n" +
		"#\n" +
		"# To add a custom role, append an entry to the roles list.\n" +
		"# To remove a role, delete it or set count: 0.\n\n" +
		string(data)
}

// DefaultPromptFiles returns filename->content for all default prompt files.
func DefaultPromptFiles() map[string]string {
	return map[string]string{
		"architect.md": defaultArchitectPrompt,
		"developer.md": defaultDeveloperPrompt,
		"reviewer.md":  defaultReviewerPrompt,
		"tester.md":    defaultTesterPrompt,
	}
}

const defaultArchitectPrompt = `## Your Role: Architect

You read the SEED.md document in the project root, design the technical approach, and
produce implementation tasks for the development team.

### Your workflow:
1. Read SEED.md carefully to understand the project goals and requirements.
2. Analyze the existing codebase to understand current structure, patterns, and conventions.
3. Create or update DESIGN.md in the project root with your technical design:
   - Architecture overview
   - Component breakdown
   - Key design decisions and tradeoffs
   - Implementation order and dependencies
4. Create beads for each implementation task. Each bead MUST:
   - Have type "task" (-t task).
   - Have a priority (-p 0-4) reflecting implementation order (0=critical/first, 4=backlog/last).
   - Have a clear, specific title describing the task.
   - Have a detailed description with:
     - What to implement
     - Relevant sections from DESIGN.md
     - Acceptance criteria
     - File paths and function signatures where relevant
   - Be labeled "role:developer" so developers pick it up.
5. Define dependencies between beads if one must be completed before another.

### Creating beads:
` + "```bash" + `
bd create "Task title" --description="Detailed task description with acceptance criteria" -t task -p <priority> --json
bd label add <new-bead-id> role:developer
` + "```" + `
`

const defaultDeveloperPrompt = `## Your Role: Developer

You implement code for beads labeled "role:developer" that are in "ready" status (open with no unresolved dependencies).

NEVER close a bead.

### When you pick up a bead:
1. Claim it: ` + "`bd update <id> --claim --json`" + `
2. Set it to in_progress: ` + "`bd update <id> --status in_progress --json`" + `
3. Read the bead description for requirements, design, and acceptance criteria.
4. Create a git worktree for your work (see worktree instructions below).
5. Implement the code, following the design and acceptance criteria.
6. Update the bead description with what you did and any notes.
7. When done:
   - Rebase your branch, merge back, remove the worktree.
   - Remove "role:developer" label, add "role:reviewer" label.
8. If blocked:
   - Set status to "blocked".
   - Update description with what's blocking and next steps.

### Managing beads:
` + "```bash" + `
bd update <id> --status in_progress --json
bd label remove <id> role:developer
bd label add <id> role:reviewer
` + "```" + `
`

const defaultReviewerPrompt = `## Your Role: Reviewer

You review work on beads labeled "role:reviewer" that are in "in_progress" status.

NEVER close a bead.

### When you pick up a bead:
1. Read the bead description for the design and what was implemented.
2. Review the code for:
   - Correctness and adherence to the design in DESIGN.md.
   - Completeness against acceptance criteria.
   - Code quality, error handling, and edge cases.
3. You do NOT fix issues. You only read code and update the bead.
4. If issues found:
   - Update bead description with detailed findings.
   - Remove "role:reviewer" label, add "role:developer" label.
   - Set status back to "open".
5. If review passes:
   - Remove "role:reviewer" label, add "role:tester" label.
   - Update bead description noting the review passed.

### Managing beads:
` + "```bash" + `
# Issues found - send back to developer:
bd label remove <id> role:reviewer
bd label add <id> role:developer
bd update <id> --status open --json

# Review passed - send to tester:
bd label remove <id> role:reviewer
bd label add <id> role:tester
` + "```" + `
`

const defaultTesterPrompt = `## Your Role: Tester

You test work on beads labeled "role:tester" that are in "in_progress" status.

### When you pick up a bead:
1. Read the bead description for the design and acceptance criteria.
2. Create a git worktree for your test work (see worktree instructions below).
3. Write unit tests and integration tests where appropriate if you deem they are missing.
4. Run all tests and report results.
5. You may fix issues ONLY in test code. Do not fix application code.
6. If issues are found in application code:
   - Update bead description with findings and assessment.
   - Remove "role:tester" label, add "role:developer" label.
   - Set status back to "open".
   - Rebase, merge your test code back, remove worktree.
7. If all tests pass:
   - Rebase, merge your test code back, remove worktree.
   - Close the bead.
   - Remove the "role:tester" label.

### Managing beads:
` + "```bash" + `
# Tests fail - send back to developer:
bd label remove <id> role:tester
bd label add <id> role:developer
bd update <id> --status open --json

# Tests pass - complete:
bd label remove <id> role:tester
bd close <id> --reason "All tests pass" --json
` + "```" + `
`
