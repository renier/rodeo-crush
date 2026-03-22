package config

func DefaultArchitect() RoleDef {
	return RoleDef{
		Name:           "Architect",
		Count:          1,
		Assignee:       "architect",
		PromptFile:     "prompts/architect.md",
		SendPromptOnce: true,
		Filter: RoleFilter{
			Assignee: "architect",
			Status:   "open",
		},
	}
}

func DefaultDeveloper() RoleDef {
	return RoleDef{
		Name:       "Developer",
		Count:      2,
		Assignee:   "developer",
		PromptFile: "prompts/developer.md",
		Filter: RoleFilter{
			Assignee: "developer",
			Ready:    true,
		},
		Worktree: true,
	}
}

func DefaultReviewer() RoleDef {
	return RoleDef{
		Name:       "Reviewer",
		Count:      1,
		Assignee:   "reviewer",
		PromptFile: "prompts/reviewer.md",
		Filter: RoleFilter{
			Assignee: "reviewer",
			Status:   "in_progress",
		},
	}
}

func DefaultTester() RoleDef {
	return RoleDef{
		Name:       "Tester",
		Count:      1,
		Assignee:   "tester",
		PromptFile: "prompts/tester.md",
		Filter: RoleFilter{
			Assignee: "tester",
			Status:   "in_progress",
		},
		Worktree: true,
	}
}
