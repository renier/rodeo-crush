package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/renier/rodeo-crush/internal/config"
	"github.com/renier/rodeo-crush/internal/tmux"
)

type mockCrushRunner struct {
	mu      sync.Mutex
	calls   int
	prompts []string
	sendErr error
}

func (m *mockCrushRunner) SendPrompt(_ context.Context, _, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.prompts = append(m.prompts, prompt)
	return "ok", m.sendErr
}

func (m *mockCrushRunner) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type fakeRunner struct {
	tmux.Runner
	calls       [][]string
	captureResp string
}

func (f *fakeRunner) Run(args ...string) (string, error) {
	f.calls = append(f.calls, args)
	if len(args) > 0 && args[0] == "capture-pane" {
		return f.captureResp, nil
	}
	if len(args) > 0 && args[0] == "has-session" {
		return "", nil
	}
	return "", nil
}

func (f *fakeRunner) called(substr string) bool {
	for _, c := range f.calls {
		if strings.Contains(strings.Join(c, " "), substr) {
			return true
		}
	}
	return false
}

func testTeamConfig() *config.TeamConfig {
	return &config.TeamConfig{
		Roles: []config.RoleDef{
			{
				Name: "Developer", Count: 1, Label: "role:developer",
				Prompt: "You are a dev.",
				Filter: config.RoleFilter{Label: "role:developer", Ready: true},
			},
		},
	}
}

func makeHandles(cfg *config.TeamConfig, socketDir string) []agentHandle {
	agents := cfg.Agents()
	handles := make([]agentHandle, len(agents))
	for i, agent := range agents {
		socketPath := agent.SocketPath(socketDir)
		crushCmd := fmt.Sprintf("crush --listen %s --cwd /tmp", socketPath)
		promptFile := filepath.Join(socketDir, "prompt.md")
		os.WriteFile(promptFile, []byte("test prompt"), 0644)
		handles[i] = agentHandle{
			agent:      agent,
			windowIdx:  i,
			socketPath: socketPath,
			crushCmd:   crushCmd,
			promptFile: promptFile,
		}
	}
	return handles
}

func TestNew(t *testing.T) {
	cfg := config.DefaultTeam()
	o := New(cfg, "/tmp/project")

	if o.StallTimeout != DefaultStallTimeout {
		t.Errorf("unexpected stall timeout: %v", o.StallTimeout)
	}
	if o.PollInterval != DefaultPollInterval {
		t.Errorf("unexpected poll interval: %v", o.PollInterval)
	}
	if o.SocketTimeout != DefaultSocketTimeout {
		t.Errorf("unexpected socket timeout: %v", o.SocketTimeout)
	}
	if o.Config != cfg {
		t.Error("config mismatch")
	}
}

func TestWritePromptFile(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "crush-test.sock")

	path, err := writePromptFile(socketPath, "hello world")
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("unexpected content: %s", data)
	}
}

func TestWaitForSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	go func() {
		time.Sleep(100 * time.Millisecond)
		os.WriteFile(sockPath, []byte{}, 0644)
	}()

	ctx := context.Background()
	if err := waitForSocket(ctx, sockPath, 2*time.Second); err != nil {
		t.Errorf("expected socket to appear: %v", err)
	}
}

func TestWaitForSocketTimeout(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "never.sock")

	ctx := context.Background()
	err := waitForSocket(ctx, sockPath, 300*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestWaitForSocketCancelled(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "never.sock")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForSocket(ctx, sockPath, 5*time.Second)
	if err == nil {
		t.Error("expected context cancelled error")
	}
}

// respawnFakeRunner simulates tmux behavior: when respawn-window is called,
// it re-creates the socket file after a short delay to simulate crush starting.
type respawnFakeRunner struct {
	fakeRunner
	socketPaths []string
}

func (r *respawnFakeRunner) Run(args ...string) (string, error) {
	r.calls = append(r.calls, args)
	if len(args) > 0 && args[0] == "respawn-window" {
		go func() {
			time.Sleep(50 * time.Millisecond)
			for _, p := range r.socketPaths {
				os.WriteFile(p, []byte{}, 0644)
			}
		}()
	}
	if len(args) > 0 && args[0] == "capture-pane" {
		return r.captureResp, nil
	}
	return "", nil
}

func TestPromptLoopSendsPrompt(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testTeamConfig()
	handles := makeHandles(cfg, socketDir)

	// Pre-create socket so waitForSocket succeeds immediately
	os.WriteFile(handles[0].socketPath, []byte{}, 0644)

	cr := &mockCrushRunner{}
	o := &Orchestrator{
		Config:             cfg,
		ProjectDir:         "/tmp",
		SocketDir:          socketDir,
		SocketTimeout:      2 * time.Second,
		PromptLoopInterval: 100 * time.Millisecond,
		Logger:             slog.Default(),
		CrushRunner:        cr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	o.promptLoop(ctx, handles[0])

	if cr.callCount() == 0 {
		t.Error("expected at least one SendPrompt call")
	}
}

func TestPromptLoopResendsAfterExit(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testTeamConfig()
	handles := makeHandles(cfg, socketDir)

	// Socket present from the start
	os.WriteFile(handles[0].socketPath, []byte{}, 0644)

	cr := &mockCrushRunner{}
	o := &Orchestrator{
		Config:             cfg,
		ProjectDir:         "/tmp",
		SocketDir:          socketDir,
		SocketTimeout:      2 * time.Second,
		PromptLoopInterval: 100 * time.Millisecond,
		Logger:             slog.Default(),
		CrushRunner:        cr,
	}

	// Run long enough for multiple send cycles (promptLoopInterval=100ms).
	// Each cycle: waitForSocket (~200ms poll tick) + send + 100ms sleep.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	o.promptLoop(ctx, handles[0])

	if cr.callCount() < 2 {
		t.Errorf("expected at least 2 SendPrompt calls (re-send after exit), got %d", cr.callCount())
	}
}

func TestPromptLoopStopsOnCancel(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testTeamConfig()
	handles := makeHandles(cfg, socketDir)

	os.WriteFile(handles[0].socketPath, []byte{}, 0644)

	cr := &mockCrushRunner{}
	o := &Orchestrator{
		Config:             cfg,
		ProjectDir:         "/tmp",
		SocketDir:          socketDir,
		SocketTimeout:      2 * time.Second,
		PromptLoopInterval: 100 * time.Millisecond,
		Logger:             slog.Default(),
		CrushRunner:        cr,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		o.promptLoop(ctx, handles[0])
		close(done)
	}()

	// Let it send at least once
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("promptLoop did not stop after cancel")
	}
}

func TestPromptLoopWaitsForSocket(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testTeamConfig()
	handles := makeHandles(cfg, socketDir)

	cr := &mockCrushRunner{}
	o := &Orchestrator{
		Config:             cfg,
		ProjectDir:         "/tmp",
		SocketDir:          socketDir,
		SocketTimeout:      500 * time.Millisecond,
		PromptLoopInterval: 100 * time.Millisecond,
		Logger:             slog.Default(),
		CrushRunner:        cr,
	}

	// Create socket after a delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		os.WriteFile(handles[0].socketPath, []byte{}, 0644)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		o.promptLoop(ctx, handles[0])
		close(done)
	}()

	// Wait for at least one send
	time.Sleep(1 * time.Second)
	cancel()
	<-done

	if cr.callCount() == 0 {
		t.Error("expected SendPrompt after socket appeared")
	}
}

func testSendOnceConfig() *config.TeamConfig {
	return &config.TeamConfig{
		Roles: []config.RoleDef{
			{
				Name: "Architect", Count: 1, Label: "role:architect",
				Prompt:         "You are the architect.",
				Filter:         config.RoleFilter{Label: "role:architect", Status: "open"},
				SendPromptOnce: true,
			},
		},
	}
}

func TestPromptLoopSendOnceExitsAfterFirstSend(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testSendOnceConfig()
	handles := makeHandles(cfg, socketDir)

	os.WriteFile(handles[0].socketPath, []byte{}, 0644)

	cr := &mockCrushRunner{}
	o := &Orchestrator{
		Config:             cfg,
		ProjectDir:         "/tmp",
		SocketDir:          socketDir,
		SocketTimeout:      2 * time.Second,
		PromptLoopInterval: 100 * time.Millisecond,
		Logger:             slog.Default(),
		CrushRunner:        cr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		o.promptLoop(ctx, handles[0])
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("promptLoop did not exit after single send for SendPromptOnce agent")
	}

	if cr.callCount() != 1 {
		t.Errorf("expected exactly 1 SendPrompt call for SendPromptOnce agent, got %d", cr.callCount())
	}
}

func TestPromptLoopSendOnceRetriesOnError(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testSendOnceConfig()
	handles := makeHandles(cfg, socketDir)

	os.WriteFile(handles[0].socketPath, []byte{}, 0644)

	cr := &mockCrushRunner{sendErr: fmt.Errorf("connection refused")}
	o := &Orchestrator{
		Config:             cfg,
		ProjectDir:         "/tmp",
		SocketDir:          socketDir,
		SocketTimeout:      2 * time.Second,
		PromptLoopInterval: 100 * time.Millisecond,
		Logger:             slog.Default(),
		CrushRunner:        cr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	o.promptLoop(ctx, handles[0])

	if cr.callCount() < 2 {
		t.Errorf("expected SendPromptOnce agent to retry on error, got %d calls", cr.callCount())
	}
}

func TestMonitorSkipsSendOnceAgents(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testSendOnceConfig()
	handles := makeHandles(cfg, socketDir)

	fr := &respawnFakeRunner{
		fakeRunner: fakeRunner{captureResp: "same output"},
	}

	o := &Orchestrator{
		Config:       cfg,
		ProjectDir:   "/tmp",
		SocketDir:    socketDir,
		StallTimeout: 100 * time.Millisecond,
		PollInterval: 50 * time.Millisecond,
		Logger:       slog.Default(),
		Session:      tmux.NewSession("test", fr),
		CrushRunner:  &mockCrushRunner{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = o.monitor(ctx, handles)

	if fr.called("respawn-window") {
		t.Error("monitor should NOT respawn SendPromptOnce agents")
	}
}

func TestMonitorDetectsStallAndRespawns(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testTeamConfig()
	handles := makeHandles(cfg, socketDir)

	fr := &respawnFakeRunner{
		fakeRunner: fakeRunner{captureResp: "same output"},
	}
	for _, h := range handles {
		fr.socketPaths = append(fr.socketPaths, h.socketPath)
	}

	o := &Orchestrator{
		Config:        cfg,
		ProjectDir:    "/tmp",
		SocketDir:     socketDir,
		StallTimeout:  100 * time.Millisecond,
		PollInterval:  50 * time.Millisecond,
		SocketTimeout: 2 * time.Second,
		Logger:        slog.Default(),
		Session:       tmux.NewSession("test", fr),
		CrushRunner:   &mockCrushRunner{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_ = o.monitor(ctx, handles)

	if !fr.called("respawn-window") {
		t.Error("expected respawn-window call on stall")
	}
	if !fr.called("-k") {
		t.Error("expected -k flag in respawn-window")
	}
}

func TestMonitorNoStallOnChangingOutput(t *testing.T) {
	counter := 0
	dynamicRunner := &dynamicFakeRunner{
		fn: func(args ...string) (string, error) {
			if len(args) > 0 && args[0] == "capture-pane" {
				counter++
				return fmt.Sprintf("output %d", counter), nil
			}
			return "", nil
		},
	}

	cfg := testTeamConfig()
	socketDir := t.TempDir()
	handles := makeHandles(cfg, socketDir)

	fr := &respawnFakeRunner{
		fakeRunner: fakeRunner{},
	}

	o := &Orchestrator{
		Config:        cfg,
		ProjectDir:    "/tmp",
		SocketDir:     socketDir,
		StallTimeout:  200 * time.Millisecond,
		PollInterval:  50 * time.Millisecond,
		SocketTimeout: 500 * time.Millisecond,
		Logger:        slog.Default(),
		Session:       tmux.NewSession("test", dynamicRunner),
		CrushRunner:   &mockCrushRunner{},
	}
	_ = fr

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	_ = o.monitor(ctx, handles)
}

func TestRestartAgentOnlyRespawns(t *testing.T) {
	socketDir := t.TempDir()
	cfg := testTeamConfig()
	handles := makeHandles(cfg, socketDir)

	fr := &fakeRunner{}
	cr := &mockCrushRunner{}

	o := &Orchestrator{
		Config:      cfg,
		ProjectDir:  "/tmp",
		SocketDir:   socketDir,
		Logger:      slog.Default(),
		Session:     tmux.NewSession("test", fr),
		CrushRunner: cr,
	}

	// Create the socket so Remove has something to delete
	os.WriteFile(handles[0].socketPath, []byte{}, 0644)

	err := o.restartAgent(handles[0])
	if err != nil {
		t.Fatalf("restartAgent failed: %v", err)
	}

	if !fr.called("respawn-window") {
		t.Error("expected respawn-window call")
	}
	if cr.callCount() != 0 {
		t.Error("restartAgent should not send prompts; that's promptLoop's job")
	}

	// Socket should have been removed
	if _, err := os.Stat(handles[0].socketPath); err == nil {
		t.Error("expected stale socket to be removed")
	}
}

type dynamicFakeRunner struct {
	fn func(args ...string) (string, error)
}

func (d *dynamicFakeRunner) Run(args ...string) (string, error) {
	return d.fn(args...)
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"simple", "simple"},
		{"has space", "'has space'"},
		{"/path/to/file", "/path/to/file"},
	}
	for _, tt := range tests {
		got := shellEscape(tt.in)
		if got != tt.want {
			t.Errorf("shellEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
