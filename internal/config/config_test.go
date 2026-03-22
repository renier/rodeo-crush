package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultTeam(t *testing.T) {
	cfg := DefaultTeam()
	if len(cfg.Roles) != 4 {
		t.Fatalf("expected 4 roles, got %d", len(cfg.Roles))
	}
	devRole := cfg.Roles[1]
	if devRole.Name != "Developer" || devRole.Count != 2 {
		t.Errorf("expected Developer count 2, got %s count %d", devRole.Name, devRole.Count)
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "team.yaml")
	data := []byte(`roles:
  - name: Dev
    count: 3
    label: "role:dev"
    prompt: "You are a dev."
    filter:
      label: "role:dev"
      ready: true
    worktree: true
  - name: QA
    count: 2
    label: "role:qa"
    prompt: "You are QA."
    filter:
      label: "role:qa"
      status: in_progress
`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(cfg.Roles))
	}
	if cfg.Roles[0].Count != 3 {
		t.Errorf("expected Dev count 3, got %d", cfg.Roles[0].Count)
	}
	if cfg.Roles[1].Name != "QA" {
		t.Errorf("expected role 'QA', got %q", cfg.Roles[1].Name)
	}
	if !cfg.Roles[0].Worktree {
		t.Error("expected Dev to have worktree=true")
	}
}

func TestLoadEmpty(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roles) != 4 {
		t.Errorf("expected default 4 roles, got %d", len(cfg.Roles))
	}
}

func TestValidate(t *testing.T) {
	validRole := RoleDef{
		Name: "Worker", Count: 1, Label: "role:worker",
		Prompt: "work", Filter: RoleFilter{Label: "role:worker", Status: "open"},
	}
	tests := []struct {
		name    string
		cfg     TeamConfig
		wantErr bool
	}{
		{"valid", TeamConfig{Roles: []RoleDef{validRole}}, false},
		{"no roles", TeamConfig{}, true},
		{"negative count", TeamConfig{Roles: []RoleDef{{Name: "A", Count: -1, Label: "role:a", Prompt: "p", Filter: RoleFilter{Label: "role:a"}}}}, true},
		{"no name", TeamConfig{Roles: []RoleDef{{Count: 1, Label: "role:a", Prompt: "p", Filter: RoleFilter{Label: "role:a"}}}}, true},
		{"no label", TeamConfig{Roles: []RoleDef{{Name: "A", Count: 1, Prompt: "p", Filter: RoleFilter{Label: "role:a"}}}}, true},
		{"no filter label", TeamConfig{Roles: []RoleDef{{Name: "A", Count: 1, Label: "role:a", Prompt: "p"}}}, true},
		{"no prompt", TeamConfig{Roles: []RoleDef{{Name: "A", Count: 1, Label: "role:a", Filter: RoleFilter{Label: "role:a"}}}}, true},
		{"prompt_file ok", TeamConfig{Roles: []RoleDef{{Name: "A", Count: 1, Label: "role:a", PromptFile: "a.md", Filter: RoleFilter{Label: "role:a"}}}}, false},
		{"duplicate names", TeamConfig{Roles: []RoleDef{validRole, validRole}}, true},
		{"all zero counts", TeamConfig{Roles: []RoleDef{{Name: "A", Count: 0, Label: "role:a", Prompt: "p", Filter: RoleFilter{Label: "role:a"}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgents(t *testing.T) {
	cfg := &TeamConfig{
		Roles: []RoleDef{
			{Name: "PM", Count: 1, Label: "role:pm", Prompt: "p", Filter: RoleFilter{Label: "role:pm"}},
			{Name: "Dev", Count: 2, Label: "role:dev", Prompt: "p", Filter: RoleFilter{Label: "role:dev"}},
			{Name: "QA", Count: 1, Label: "role:qa", Prompt: "p", Filter: RoleFilter{Label: "role:qa"}},
		},
	}

	agents := cfg.Agents()
	if len(agents) != 4 {
		t.Fatalf("expected 4 agents, got %d", len(agents))
	}

	if agents[0].Name != "PM" {
		t.Errorf("expected 'PM', got %q", agents[0].Name)
	}
	if agents[1].Name != "Dev 1" {
		t.Errorf("expected 'Dev 1', got %q", agents[1].Name)
	}
	if agents[2].Name != "Dev 2" {
		t.Errorf("expected 'Dev 2', got %q", agents[2].Name)
	}
	if agents[3].Name != "QA" {
		t.Errorf("expected 'QA', got %q", agents[3].Name)
	}
}

func TestFilterCommand(t *testing.T) {
	tests := []struct {
		name   string
		role   RoleDef
		expect string
	}{
		{
			"status filter",
			RoleDef{Filter: RoleFilter{Label: "role:pm", Status: "open"}},
			"bd list --label role:pm --status open --json",
		},
		{
			"ready filter",
			RoleDef{Filter: RoleFilter{Label: "role:dev", Ready: true}},
			"bd list --label role:dev --ready --json",
		},
		{
			"ready takes precedence over status",
			RoleDef{Filter: RoleFilter{Label: "role:dev", Ready: true, Status: "open"}},
			"bd list --label role:dev --ready --json",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.role.FilterCommand()
			if got != tt.expect {
				t.Errorf("got %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestResolvePrompts(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "my_role.md")
	if err := os.WriteFile(promptFile, []byte("custom prompt content"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &TeamConfig{
		Roles: []RoleDef{
			{Name: "Inline", Count: 1, Label: "role:a", Prompt: "already set", Filter: RoleFilter{Label: "role:a"}},
			{Name: "FromFile", Count: 1, Label: "role:b", PromptFile: "my_role.md", Filter: RoleFilter{Label: "role:b"}},
		},
	}

	if err := cfg.ResolvePrompts(dir); err != nil {
		t.Fatal(err)
	}

	if cfg.Roles[0].Prompt != "already set" {
		t.Errorf("inline prompt should be unchanged, got %q", cfg.Roles[0].Prompt)
	}
	if cfg.Roles[1].Prompt != "custom prompt content" {
		t.Errorf("expected loaded prompt, got %q", cfg.Roles[1].Prompt)
	}
}

func TestResolvePromptsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "abs.md")
	if err := os.WriteFile(promptFile, []byte("absolute"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &TeamConfig{
		Roles: []RoleDef{
			{Name: "Abs", Count: 1, Label: "role:a", PromptFile: promptFile, Filter: RoleFilter{Label: "role:a"}},
		},
	}

	if err := cfg.ResolvePrompts("/some/other/dir"); err != nil {
		t.Fatal(err)
	}
	if cfg.Roles[0].Prompt != "absolute" {
		t.Errorf("expected 'absolute', got %q", cfg.Roles[0].Prompt)
	}
}

func TestBootstrap(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgDir, err := Bootstrap()
	if err != nil {
		t.Fatal(err)
	}

	expected := filepath.Join(tmpHome, ".config", ConfigDirName)
	if cfgDir != expected {
		t.Errorf("expected %s, got %s", expected, cfgDir)
	}

	teamPath := filepath.Join(cfgDir, "team.yaml")
	if _, err := os.Stat(teamPath); err != nil {
		t.Errorf("team.yaml not created: %v", err)
	}

	for name := range DefaultPromptFiles() {
		path := filepath.Join(cfgDir, "prompts", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("prompt %s not created: %v", name, err)
		}
	}

	cfg, err := Load(teamPath)
	if err != nil {
		t.Fatalf("loading bootstrapped config: %v", err)
	}
	if err := cfg.ResolvePrompts(cfgDir); err != nil {
		t.Fatalf("resolving bootstrapped prompts: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("bootstrapped config should be valid: %v", err)
	}
}

func TestBootstrapIdempotent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfgDir, err := Bootstrap()
	if err != nil {
		t.Fatal(err)
	}

	customContent := "# my custom config\nroles:\n  - name: X\n    count: 1\n    label: \"role:x\"\n    prompt: \"x\"\n    filter:\n      label: \"role:x\"\n      status: open\n"
	teamPath := filepath.Join(cfgDir, "team.yaml")
	if err := os.WriteFile(teamPath, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = Bootstrap()
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(teamPath)
	if string(data) != customContent {
		t.Error("bootstrap overwrote existing team.yaml")
	}
}

func TestSocketPath(t *testing.T) {
	agent := Agent{
		RoleDef: &RoleDef{Label: "role:developer"},
		Index:   2,
	}
	got := agent.SocketPath("/tmp/sockets")
	expected := "/tmp/sockets/crush-role-developer-2.sock"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
