package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Bead represents a single issue from bd list --json.
type Bead struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Status          string   `json:"status"`
	Priority        int      `json:"priority"`
	IssueType       string   `json:"issue_type"`
	Owner           string   `json:"owner"`
	Assignee        string   `json:"assignee"`
	Labels          []string `json:"labels"`
	DependencyCount int      `json:"dependency_count"`
	DependentCount  int      `json:"dependent_count"`
}

// FetchBeads runs bd list --json and parses the output.
func FetchBeads() ([]Bead, error) {
	cmd := exec.Command("bd", "list", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running bd list: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}

	var beads []Bead
	if err := json.Unmarshal([]byte(trimmed), &beads); err != nil {
		return nil, fmt.Errorf("parsing bd list output: %w", err)
	}
	return beads, nil
}
