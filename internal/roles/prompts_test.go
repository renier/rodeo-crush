package roles

import (
	"strings"
	"testing"

	"github.com/renier/rodeo-crush/internal/config"
)

func makeAgent(name, label, prompt string, worktree bool) config.Agent {
	role := &config.RoleDef{
		Name:     name,
		Count:    1,
		Label:    label,
		Prompt:   prompt,
		Worktree: worktree,
		Filter: config.RoleFilter{
			Label:  label,
			Status: "open",
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
			makeAgent("Project Manager", "role:project_manager", "You are the PM", false),
			[]string{"Project Manager", "role:project_manager", "Finding Work", "You are the PM"},
		},
		{
			makeAgent("Developer", "role:developer", "You are a dev", true),
			[]string{"Developer", "role:developer", "worktree", "Finding Work", "You are a dev"},
		},
		{
			makeAgent("Reviewer", "role:reviewer", "You review code", false),
			[]string{"Reviewer", "role:reviewer", "You review code"},
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
	agent := makeAgent("Reviewer", "role:reviewer", "review stuff", false)
	prompt := Prompt(agent, "/tmp/project")
	if strings.Contains(prompt, "Git Worktree Instructions") {
		t.Error("worktree=false role should not have worktree instructions")
	}
}

func TestPromptHasWorktreeWhenEnabled(t *testing.T) {
	agent := makeAgent("Developer", "role:developer", "dev stuff", true)
	prompt := Prompt(agent, "/tmp/project")
	if !strings.Contains(prompt, "Git Worktree Instructions") {
		t.Error("worktree=true role should have worktree instructions")
	}
}

func TestWorkLoopUsesFilterCommand(t *testing.T) {
	role := &config.RoleDef{
		Name:   "Custom",
		Label:  "role:custom",
		Prompt: "custom",
		Filter: config.RoleFilter{
			Label: "role:custom",
			Ready: true,
		},
	}
	agent := config.Agent{RoleDef: role, Name: "Custom", Index: 1}
	prompt := Prompt(agent, "/tmp/project")
	if !strings.Contains(prompt, "bd list --label role:custom --ready --json") {
		t.Error("prompt should contain the filter command from RoleDef")
	}
}

func TestCustomRolePrompt(t *testing.T) {
	role := &config.RoleDef{
		Name:   "Security Auditor",
		Label:  "role:security",
		Prompt: "You audit code for vulnerabilities.\nFocus on injection and auth issues.",
		Filter: config.RoleFilter{
			Label:  "role:security",
			Status: "in_progress",
		},
	}
	agent := config.Agent{RoleDef: role, Name: "Security Auditor", Index: 1}
	prompt := Prompt(agent, "/tmp/project")
	if !strings.Contains(prompt, "You audit code for vulnerabilities") {
		t.Error("custom prompt content missing")
	}
	if !strings.Contains(prompt, "role:security") {
		t.Error("custom label missing from preamble")
	}
}
