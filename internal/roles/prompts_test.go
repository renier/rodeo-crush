package roles

import (
	"strings"
	"testing"

	"github.com/renier/rodeo-crush/internal/config"
)

func makeAgent(name, assignee, prompt string, worktree bool) config.Agent {
	role := &config.RoleDef{
		Name:     name,
		Count:    1,
		Assignee: assignee,
		Prompt:   prompt,
		Worktree: worktree,
		Filter: config.RoleFilter{
			Assignee: assignee,
			Status:   "open",
		},
	}
	return config.Agent{
		RoleDef: role,
		Name:    name,
		Index:   1,
	}
}

func TestPromptContainsRole(t *testing.T) {
	tests := []struct {
		agent    config.Agent
		contains []string
	}{
		{
			makeAgent("Architect", "architect", "You are the architect", false),
			[]string{"Architect", "architect", "Finding Work", "You are the architect"},
		},
		{
			makeAgent("Developer", "developer", "You are a dev", true),
			[]string{"Developer", "developer", "worktree", "Finding Work", "You are a dev"},
		},
		{
			makeAgent("Reviewer", "reviewer", "You review code", false),
			[]string{"Reviewer", "reviewer", "You review code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.agent.Name, func(t *testing.T) {
			prompt := Prompt(tt.agent, "/tmp/project")
			for _, s := range tt.contains {
				if !strings.Contains(prompt, s) {
					t.Errorf("prompt for %s missing %q", tt.agent.Name, s)
				}
			}
		})
	}
}

func TestPromptNoWorktreeWhenDisabled(t *testing.T) {
	agent := makeAgent("Reviewer", "reviewer", "review stuff", false)
	prompt := Prompt(agent, "/tmp/project")
	if strings.Contains(prompt, "Git Worktree Instructions") {
		t.Error("worktree=false role should not have worktree instructions")
	}
}

func TestPromptHasWorktreeWhenEnabled(t *testing.T) {
	agent := makeAgent("Developer", "developer", "dev stuff", true)
	prompt := Prompt(agent, "/tmp/project")
	if !strings.Contains(prompt, "Git Worktree Instructions") {
		t.Error("worktree=true role should have worktree instructions")
	}
}

func TestWorkLoopUsesFilterCommand(t *testing.T) {
	role := &config.RoleDef{
		Name:     "Custom",
		Assignee: "custom",
		Prompt:   "custom",
		Filter: config.RoleFilter{
			Assignee: "custom",
			Ready:    true,
		},
	}
	agent := config.Agent{RoleDef: role, Name: "Custom", Index: 1}
	prompt := Prompt(agent, "/tmp/project")
	if !strings.Contains(prompt, "bd list --assignee custom --ready --json") {
		t.Error("prompt should contain the filter command from RoleDef")
	}
}

func TestCustomRolePrompt(t *testing.T) {
	role := &config.RoleDef{
		Name:     "Security Auditor",
		Assignee: "security",
		Prompt:   "You audit code for vulnerabilities.\nFocus on injection and auth issues.",
		Filter: config.RoleFilter{
			Assignee: "security",
			Status:   "in_progress",
		},
	}
	agent := config.Agent{RoleDef: role, Name: "Security Auditor", Index: 1}
	prompt := Prompt(agent, "/tmp/project")
	if !strings.Contains(prompt, "You audit code for vulnerabilities") {
		t.Error("custom prompt content missing")
	}
	if !strings.Contains(prompt, "security") {
		t.Error("custom assignee missing from preamble")
	}
}
