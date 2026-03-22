package tmux

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts tmux command execution for testability.
type Runner interface {
	Run(args ...string) (string, error)
}

// ExecRunner runs real tmux commands.
type ExecRunner struct{}

func (r *ExecRunner) Run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Session manages a tmux session with named windows for Crush agents.
type Session struct {
	Name   string
	Runner Runner
}

// NewSession creates a Session manager.
func NewSession(name string, runner Runner) *Session {
	if runner == nil {
		runner = &ExecRunner{}
	}
	return &Session{Name: name, Runner: runner}
}

// Exists checks if the tmux session already exists.
func (s *Session) Exists() bool {
	_, err := s.Runner.Run("has-session", "-t", s.Name)
	return err == nil
}

// Create creates the tmux session with one window per WindowSpec.
// The first window is created with the session itself; subsequent windows
// are added with new-window. Users tab between windows to switch agents.
func (s *Session) Create(windows []WindowSpec) error {
	if len(windows) == 0 {
		return fmt.Errorf("no windows specified")
	}

	first := windows[0]
	args := []string{
		"new-session",
		"-d",
		"-s", s.Name,
		"-n", first.Name,
	}
	if first.Dir != "" {
		args = append(args, "-c", first.Dir)
	}
	if first.Command != "" {
		args = append(args, first.Command)
	}
	if _, err := s.Runner.Run(args...); err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	for i := 1; i < len(windows); i++ {
		w := windows[i]
		newArgs := []string{
			"new-window",
			"-t", s.Name,
			"-n", w.Name,
		}
		if w.Dir != "" {
			newArgs = append(newArgs, "-c", w.Dir)
		}
		if w.Command != "" {
			newArgs = append(newArgs, w.Command)
		}
		if _, err := s.Runner.Run(newArgs...); err != nil {
			return fmt.Errorf("creating window %d (%s): %w", i, w.Name, err)
		}
	}

	if _, err := s.Runner.Run("select-window", "-t", s.Name+":0"); err != nil {
		return fmt.Errorf("selecting first window: %w", err)
	}

	return nil
}

// AddWindow adds a single window to an existing session.
func (s *Session) AddWindow(w WindowSpec) error {
	args := []string{
		"new-window",
		"-t", s.Name,
		"-n", w.Name,
	}
	if w.Dir != "" {
		args = append(args, "-c", w.Dir)
	}
	if w.Command != "" {
		args = append(args, w.Command)
	}
	if _, err := s.Runner.Run(args...); err != nil {
		return fmt.Errorf("adding window %s: %w", w.Name, err)
	}
	return nil
}

// SelectWindow focuses a window by index.
func (s *Session) SelectWindow(index int) error {
	target := fmt.Sprintf("%s:%d", s.Name, index)
	if _, err := s.Runner.Run("select-window", "-t", target); err != nil {
		return fmt.Errorf("selecting window %d: %w", index, err)
	}
	return nil
}

// RespawnWindow kills the running process in a window and starts a new
// command in its place. This is equivalent to `tmux respawn-window -k`.
func (s *Session) RespawnWindow(windowIndex int, command string, dir string) error {
	target := fmt.Sprintf("%s:%d", s.Name, windowIndex)
	args := []string{"respawn-window", "-k", "-t", target}
	if dir != "" {
		args = append(args, "-c", dir)
	}
	if command != "" {
		args = append(args, command)
	}
	if _, err := s.Runner.Run(args...); err != nil {
		return fmt.Errorf("respawning window %d: %w", windowIndex, err)
	}
	return nil
}

// Kill destroys the tmux session.
func (s *Session) Kill() error {
	if _, err := s.Runner.Run("kill-session", "-t", s.Name); err != nil {
		return fmt.Errorf("killing session %s: %w", s.Name, err)
	}
	return nil
}

// SendKeys sends keystrokes to a specific window by index.
func (s *Session) SendKeys(windowIndex int, keys string) error {
	target := fmt.Sprintf("%s:%d", s.Name, windowIndex)
	if _, err := s.Runner.Run("send-keys", "-t", target, keys, "Enter"); err != nil {
		return fmt.Errorf("sending keys to window %d: %w", windowIndex, err)
	}
	return nil
}

// CapturePane captures the visible content of a window (its single pane).
func (s *Session) CapturePane(windowIndex int) (string, error) {
	target := fmt.Sprintf("%s:%d", s.Name, windowIndex)
	out, err := s.Runner.Run("capture-pane", "-t", target, "-p")
	if err != nil {
		return "", fmt.Errorf("capturing window %d: %w", windowIndex, err)
	}
	return out, nil
}

// WindowSpec describes a window to be created in the tmux session.
type WindowSpec struct {
	Name    string
	Command string
	Dir     string
}
