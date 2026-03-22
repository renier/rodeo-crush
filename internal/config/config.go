package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName = "rodeo-crush"
	AppName       = "Rodeo\U0001f920Crush\U0001f496"
	SessionPrefix = "rodeo"
)

// ConfigDir returns the path to $HOME/.config/rodeo-crush/.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".config", ConfigDirName), nil
}

// DefaultTeam returns the default team configuration.
func DefaultTeam() *TeamConfig {
	return &TeamConfig{
		Roles: []RoleDef{
			DefaultArchitect(),
			DefaultDeveloper(),
			DefaultReviewer(),
			DefaultTester(),
		},
	}
}

type TeamConfig struct {
	Roles []RoleDef `yaml:"roles"`
}

type RoleDef struct {
	Name           string     `yaml:"name"`
	Count          int        `yaml:"count"`
	Label          string     `yaml:"label"`
	Prompt         string     `yaml:"prompt,omitempty"`
	PromptFile     string     `yaml:"prompt_file,omitempty"`
	Filter         RoleFilter `yaml:"filter"`
	Worktree       bool       `yaml:"worktree,omitempty"`
	SendPromptOnce bool       `yaml:"send_prompt_once,omitempty"`
}

type RoleFilter struct {
	Label  string `yaml:"label"`
	Status string `yaml:"status,omitempty"`
	Ready  bool   `yaml:"ready,omitempty"`
}

// FilterCommand returns the bd list command for this role's filter.
func (r *RoleDef) FilterCommand() string {
	var parts []string
	parts = append(parts, "bd list")
	if r.Filter.Label != "" {
		parts = append(parts, "--label", r.Filter.Label)
	}
	if r.Filter.Ready {
		parts = append(parts, "--ready")
	} else if r.Filter.Status != "" {
		parts = append(parts, "--status", r.Filter.Status)
	}
	parts = append(parts, "--json")
	return strings.Join(parts, " ")
}

// Load reads a team configuration from a YAML file.
// If path is empty, it returns the default configuration.
func Load(path string) (*TeamConfig, error) {
	if path == "" {
		return DefaultTeam(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading team config: %w", err)
	}

	cfg := &TeamConfig{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing team config: %w", err)
	}

	return cfg, nil
}

// Validate checks that the configuration is sensible.
func (c *TeamConfig) Validate() error {
	if len(c.Roles) == 0 {
		return fmt.Errorf("at least one role must be defined")
	}
	total := 0
	seen := make(map[string]bool)
	for i, r := range c.Roles {
		if r.Name == "" {
			return fmt.Errorf("role %d: name cannot be empty", i)
		}
		if seen[r.Name] {
			return fmt.Errorf("duplicate role name: %q", r.Name)
		}
		seen[r.Name] = true
		if r.Count < 0 {
			return fmt.Errorf("role %q: count cannot be negative", r.Name)
		}
		if r.Label == "" {
			return fmt.Errorf("role %q: label cannot be empty", r.Name)
		}
		if r.Filter.Label == "" {
			return fmt.Errorf("role %q: filter.label cannot be empty", r.Name)
		}
		if r.Prompt == "" && r.PromptFile == "" {
			return fmt.Errorf("role %q: must have either prompt or prompt_file", r.Name)
		}
		total += r.Count
	}
	if total == 0 {
		return fmt.Errorf("at least one role must have count > 0")
	}
	return nil
}

// ResolvePrompts loads prompt text from prompt_file references, resolving
// paths relative to baseDir. After this call, every RoleDef has Prompt set.
func (c *TeamConfig) ResolvePrompts(baseDir string) error {
	for i := range c.Roles {
		r := &c.Roles[i]
		if r.Prompt != "" {
			continue
		}
		if r.PromptFile == "" {
			return fmt.Errorf("role %q: no prompt or prompt_file", r.Name)
		}
		path := r.PromptFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("role %q: reading prompt file %s: %w", r.Name, path, err)
		}
		r.Prompt = string(data)
	}
	return nil
}

// Agents returns a flat list of Agent descriptors for every pane to launch.
func (c *TeamConfig) Agents() []Agent {
	var agents []Agent
	for ri, role := range c.Roles {
		for i := 1; i <= role.Count; i++ {
			name := role.Name
			if role.Count > 1 {
				name = fmt.Sprintf("%s %d", role.Name, i)
			}
			agents = append(agents, Agent{
				RoleIndex: ri,
				RoleDef:   &c.Roles[ri],
				Name:      name,
				Index:     i,
			})
		}
	}
	return agents
}

type Agent struct {
	RoleIndex int
	RoleDef   *RoleDef
	Name      string
	Index     int
}

// SocketPath returns the unix socket path for this agent.
func (a Agent) SocketPath(base string) string {
	safe := strings.ReplaceAll(a.RoleDef.Label, ":", "-")
	return fmt.Sprintf("%s/crush-%s-%d.sock", base, safe, a.Index)
}

// PaneName returns the tmux pane title for this agent.
func (a Agent) PaneName() string {
	return a.Name
}
