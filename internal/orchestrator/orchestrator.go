package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/renier/rodeo-crush/internal/config"
	"github.com/renier/rodeo-crush/internal/roles"
	"github.com/renier/rodeo-crush/internal/tmux"
)

const (
	DefaultStallTimeout  = 15 * time.Minute
	DefaultPollInterval  = 30 * time.Second
	DefaultSocketTimeout = 10 * time.Second
	promptLoopInterval   = 60 * time.Second
	promptRetryDelay     = 2 * time.Second
)

// CrushRunner abstracts sending prompts to Crush TUI instances via their
// unix socket. The TUI itself is started inside tmux; this interface is only
// for the out-of-band prompt delivery.
type CrushRunner interface {
	SendPrompt(ctx context.Context, socketPath, prompt string) (string, error)
}

// ExecCrushRunner shells out to `crush run --socket`.
type ExecCrushRunner struct{}

func (r *ExecCrushRunner) SendPrompt(ctx context.Context, socketPath, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "crush", "run",
		"-o", "stream-json",
		"--socket", socketPath,
		prompt,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("sending prompt via socket %s: %w", socketPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func writePromptFile(socketPath, prompt string) (string, error) {
	dir := filepath.Dir(socketPath)
	base := filepath.Base(socketPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	path := filepath.Join(dir, name+".prompt.md")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(prompt), 0644); err != nil {
		return "", err
	}
	return path, nil
}

// Orchestrator manages the lifecycle of the rodeo team.
type Orchestrator struct {
	Config            *config.TeamConfig
	ProjectDir        string
	SocketDir         string
	StallTimeout      time.Duration
	PollInterval      time.Duration
	SocketTimeout     time.Duration
	PromptLoopInterval time.Duration
	Logger            *slog.Logger
	Session           *tmux.Session
	CrushRunner       CrushRunner
}

// New creates an Orchestrator with sensible defaults.
func New(cfg *config.TeamConfig, projectDir string) *Orchestrator {
	socketDir := filepath.Join(os.TempDir(), "rodeo-crush", cfg.Session)
	return &Orchestrator{
		Config:             cfg,
		ProjectDir:         projectDir,
		SocketDir:          socketDir,
		StallTimeout:       DefaultStallTimeout,
		PollInterval:       DefaultPollInterval,
		SocketTimeout:      DefaultSocketTimeout,
		PromptLoopInterval: promptLoopInterval,
		Logger:             slog.Default(),
		Session:            tmux.NewSession(cfg.Session, nil),
		CrushRunner:        &ExecCrushRunner{},
	}
}

// agentHandle holds everything needed to (re)start an agent.
type agentHandle struct {
	agent      config.Agent
	windowIdx  int
	socketPath string
	crushCmd   string
	promptFile string
}

// Start launches tmux windows running Crush TUIs, starts a prompt loop
// goroutine for each agent, then enters the monitoring loop.
func (o *Orchestrator) Start(ctx context.Context) error {
	if err := os.MkdirAll(o.SocketDir, 0755); err != nil {
		return fmt.Errorf("creating socket dir: %w", err)
	}

	agents := o.Config.Agents()
	if len(agents) == 0 {
		return fmt.Errorf("no agents configured")
	}

	if o.Session.Exists() {
		o.Logger.Info("killing existing session", "session", o.Config.Session)
		_ = o.Session.Kill()
		time.Sleep(500 * time.Millisecond)
	}

	handles := make([]agentHandle, len(agents))
	windows := make([]tmux.WindowSpec, len(agents))
	for i, agent := range agents {
		socketPath := agent.SocketPath(o.SocketDir)
		prompt := roles.Prompt(agent, o.ProjectDir)

		promptFile, err := writePromptFile(socketPath, prompt)
		if err != nil {
			return fmt.Errorf("writing prompt for %s: %w", agent.Name, err)
		}

		crushCmd := fmt.Sprintf("crush --listen %s --cwd %s",
			shellEscape(socketPath),
			shellEscape(o.ProjectDir),
		)

		windows[i] = tmux.WindowSpec{
			Name:    agent.PaneName(),
			Command: crushCmd,
			Dir:     o.ProjectDir,
		}

		handles[i] = agentHandle{
			agent:      agent,
			windowIdx:  i,
			socketPath: socketPath,
			crushCmd:   crushCmd,
			promptFile: promptFile,
		}
	}

	o.Logger.Info("creating tmux session", "session", o.Config.Session, "windows", len(windows))
	if err := o.Session.Create(windows); err != nil {
		return fmt.Errorf("creating tmux session: %w", err)
	}

	for _, h := range handles {
		go o.promptLoop(ctx, h)
	}

	o.Logger.Info("orchestration started",
		"session", o.Config.Session,
		"agents", len(agents),
		"stall_timeout", o.StallTimeout,
	)

	return o.monitor(ctx, handles)
}

// promptLoop continuously sends the prompt to an agent's Crush TUI via its
// socket. When crush run exits (the agent finished or was killed), the loop
// waits for the socket to reappear and re-sends the prompt.
//
// For agents whose role has SendPromptOnce=true, the prompt is sent exactly
// once and the goroutine exits.
func (o *Orchestrator) promptLoop(ctx context.Context, h agentHandle) {
	for {
		if err := waitForSocket(ctx, h.socketPath, o.SocketTimeout); err != nil {
			if ctx.Err() != nil {
				return
			}
			o.Logger.Warn("socket not ready, retrying",
				"agent", h.agent.Name, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(promptRetryDelay):
				continue
			}
		}

		o.Logger.Info("sending prompt", "agent", h.agent.Name)
		_, err := o.CrushRunner.SendPrompt(ctx, h.socketPath, "@"+h.promptFile)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			o.Logger.Warn("prompt exited with error",
				"agent", h.agent.Name, "error", err)
		} else {
			o.Logger.Info("prompt completed",
				"agent", h.agent.Name)
			if h.agent.RoleDef.SendPromptOnce {
				o.Logger.Info("send-once agent done, exiting prompt loop",
					"agent", h.agent.Name)
				return
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(o.PromptLoopInterval):
		}
	}
}

// Stop tears down the tmux session.
func (o *Orchestrator) Stop() error {
	o.Logger.Info("stopping orchestration", "session", o.Config.Session)
	if err := o.Session.Kill(); err != nil {
		return err
	}
	_ = os.RemoveAll(o.SocketDir)
	return nil
}

// waitForSocket polls until the socket file appears on disk or the timeout
// expires. This gives the Crush TUI time to start listening.
func waitForSocket(ctx context.Context, path string, timeout time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out waiting for socket %s", path)
		case <-ticker.C:
			if _, err := os.Stat(path); err == nil {
				return nil
			}
		}
	}
}

// monitor watches each agent window for stalls. When an agent's output has
// not changed for StallTimeout, it respawns the window (killing the old
// Crush process and starting a fresh one). The prompt loop goroutine
// detects the new socket and re-sends the prompt automatically.
// Agents with SendPromptOnce=true are excluded from monitoring.
func (o *Orchestrator) monitor(ctx context.Context, handles []agentHandle) error {
	type windowState struct {
		lastOutput string
		lastChange time.Time
	}

	states := make([]windowState, len(handles))
	now := time.Now()
	for i := range states {
		states[i].lastChange = now
	}

	ticker := time.NewTicker(o.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.Logger.Info("context cancelled, stopping monitor")
			return nil
		case <-ticker.C:
			for i, h := range handles {
				if h.agent.RoleDef.SendPromptOnce {
					continue
				}

				output, err := o.Session.CapturePane(h.windowIdx)
				if err != nil {
					o.Logger.Warn("failed to capture window", "agent", h.agent.Name, "error", err)
					continue
				}

				if output != states[i].lastOutput {
					states[i].lastOutput = output
					states[i].lastChange = time.Now()
					continue
				}

				stalledFor := time.Since(states[i].lastChange)
				if stalledFor >= o.StallTimeout {
					o.Logger.Warn("agent stalled, restarting",
						"agent", h.agent.Name,
						"stalled_for", stalledFor.Round(time.Second),
					)
					if err := o.restartAgent(h); err != nil {
						o.Logger.Error("failed to restart agent",
							"agent", h.agent.Name, "error", err)
					}
					states[i].lastOutput = ""
					states[i].lastChange = time.Now()
				}
			}
		}
	}
}

// restartAgent kills the Crush process in the tmux window via respawn-window
// and removes the stale socket. The prompt loop goroutine will detect the new
// socket and re-send the prompt automatically.
func (o *Orchestrator) restartAgent(h agentHandle) error {
	_ = os.Remove(h.socketPath)

	o.Logger.Info("respawning window", "agent", h.agent.Name, "window", h.windowIdx)
	if err := o.Session.RespawnWindow(h.windowIdx, h.crushCmd, o.ProjectDir); err != nil {
		return fmt.Errorf("respawning window: %w", err)
	}

	return nil
}

func shellEscape(s string) string {
	if !strings.ContainsAny(s, " \t\n'\"\\$`!#&|;(){}[]<>?*~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
