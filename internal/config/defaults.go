package config

func DefaultProjectManager() RoleDef {
	return RoleDef{
		Name:           "Project Manager",
		Count:          1,
		Label:          "role:project_manager",
		PromptFile:     "prompts/project_manager.md",
		SendPromptOnce: true,
		Filter: RoleFilter{
			Label:  "role:project_manager",
			Status: "open",
		},
	}
}

func DefaultArchitect() RoleDef {
	return RoleDef{
		Name:       "Architect",
		Count:      1,
		Label:      "role:architect",
		PromptFile: "prompts/architect.md",
		Filter: RoleFilter{
			Label:  "role:architect",
			Status: "open",
		},
	}
}

func DefaultDeveloper() RoleDef {
	return RoleDef{
		Name:       "Developer",
		Count:      2,
		Label:      "role:developer",
		PromptFile: "prompts/developer.md",
		Filter: RoleFilter{
			Label: "role:developer",
			Ready: true,
		},
		Worktree: true,
	}
}

func DefaultReviewer() RoleDef {
	return RoleDef{
		Name:       "Reviewer",
		Count:      1,
		Label:      "role:reviewer",
		PromptFile: "prompts/reviewer.md",
		Filter: RoleFilter{
			Label:  "role:reviewer",
			Status: "in_progress",
		},
	}
}

func DefaultTester() RoleDef {
	return RoleDef{
		Name:       "Tester",
		Count:      1,
		Label:      "role:tester",
		PromptFile: "prompts/tester.md",
		Filter: RoleFilter{
			Label:  "role:tester",
			Status: "in_progress",
		},
		Worktree: true,
	}
}
