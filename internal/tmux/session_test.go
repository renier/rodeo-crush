package tmux

import (
	"fmt"
	"strings"
	"testing"
)

type mockRunner struct {
	calls [][]string
	errs  map[string]error
}

func newMockRunner() *mockRunner {
	return &mockRunner{errs: make(map[string]error)}
}

func (m *mockRunner) Run(args ...string) (string, error) {
	m.calls = append(m.calls, args)
	key := strings.Join(args, " ")
	for pattern, err := range m.errs {
		if strings.Contains(key, pattern) {
			return "", err
		}
	}
	return "", nil
}

func (m *mockRunner) called(substr string) bool {
	for _, c := range m.calls {
		if strings.Contains(strings.Join(c, " "), substr) {
			return true
		}
	}
	return false
}

func (m *mockRunner) countCalls(command string) int {
	n := 0
	for _, c := range m.calls {
		if len(c) > 0 && c[0] == command {
			n++
		}
	}
	return n
}

func TestCreateSession(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test-rodeo", r)

	windows := []WindowSpec{
		{Name: "Project Manager", Command: "crush --listen /tmp/pm.sock"},
		{Name: "Developer 1", Command: "crush --listen /tmp/dev1.sock"},
		{Name: "Developer 2", Command: "crush --listen /tmp/dev2.sock"},
	}

	if err := s.Create(windows); err != nil {
		t.Fatal(err)
	}

	if !r.called("new-session") {
		t.Error("expected new-session call")
	}

	newWindowCalls := r.countCalls("new-window")
	if newWindowCalls != 2 {
		t.Errorf("expected 2 new-window calls, got %d", newWindowCalls)
	}

	if r.called("split-window") {
		t.Error("should not use split-window")
	}
	if r.called("select-layout") {
		t.Error("should not use select-layout")
	}
	if !r.called("select-window") {
		t.Error("expected select-window to return to first window")
	}
}

func TestCreateSessionWindowNames(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	windows := []WindowSpec{
		{Name: "PM"},
		{Name: "Arch"},
		{Name: "Dev"},
	}
	if err := s.Create(windows); err != nil {
		t.Fatal(err)
	}

	if !r.called("-n PM") {
		t.Error("first window should be named PM")
	}
	if !r.called("-n Arch") {
		t.Error("second window should be named Arch")
	}
	if !r.called("-n Dev") {
		t.Error("third window should be named Dev")
	}
}

func TestCreateSessionCommandInFirstWindow(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	windows := []WindowSpec{
		{Name: "A", Command: "crush --listen /tmp/a.sock"},
	}
	if err := s.Create(windows); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, c := range r.calls {
		if c[0] == "new-session" {
			for _, arg := range c {
				if strings.Contains(arg, "crush --listen") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("expected crush command in new-session call")
	}
}

func TestCreateSessionNoWindows(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	err := s.Create(nil)
	if err == nil {
		t.Error("expected error for empty windows")
	}
}

func TestExists(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	if !s.Exists() {
		t.Error("expected Exists to return true when no error")
	}

	r.errs["has-session"] = fmt.Errorf("no session")
	if s.Exists() {
		t.Error("expected Exists to return false on error")
	}
}

func TestKill(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	if err := s.Kill(); err != nil {
		t.Fatal(err)
	}
	if !r.called("kill-session") {
		t.Error("expected kill-session call")
	}
}

func TestSendKeys(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	if err := s.SendKeys(0, "ls"); err != nil {
		t.Fatal(err)
	}
	if !r.called("send-keys") {
		t.Error("expected send-keys call")
	}
	if !r.called("test:0") {
		t.Error("expected window target test:0")
	}
}

func TestCapturePane(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	_, err := s.CapturePane(2)
	if err != nil {
		t.Fatal(err)
	}
	if !r.called("capture-pane") {
		t.Error("expected capture-pane call")
	}
	if !r.called("test:2") {
		t.Error("expected window target test:2")
	}
}

func TestRespawnWindow(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	err := s.RespawnWindow(1, "crush --listen /tmp/new.sock", "")
	if err != nil {
		t.Fatal(err)
	}
	if !r.called("respawn-window") {
		t.Error("expected respawn-window call")
	}
	if !r.called("-k") {
		t.Error("expected -k flag to kill existing process")
	}
	if !r.called("test:1") {
		t.Error("expected window target test:1")
	}
	if !r.called("crush --listen /tmp/new.sock") {
		t.Error("expected new command in respawn-window")
	}
}

func TestRespawnWindowError(t *testing.T) {
	r := newMockRunner()
	r.errs["respawn-window"] = fmt.Errorf("window dead")
	s := NewSession("test", r)

	err := s.RespawnWindow(0, "crush", "")
	if err == nil {
		t.Error("expected error from respawn-window")
	}
}

func TestCreateSessionWithDir(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	windows := []WindowSpec{
		{Name: "A", Command: "crush --listen /tmp/a.sock", Dir: "/project"},
		{Name: "B", Command: "crush --listen /tmp/b.sock", Dir: "/project"},
	}
	if err := s.Create(windows); err != nil {
		t.Fatal(err)
	}

	for _, c := range r.calls {
		joined := strings.Join(c, " ")
		if c[0] == "new-session" || c[0] == "new-window" {
			if !strings.Contains(joined, "-c /project") {
				t.Errorf("expected -c /project in %s call, got: %v", c[0], c)
			}
		}
	}
}

func TestRespawnWindowWithDir(t *testing.T) {
	r := newMockRunner()
	s := NewSession("test", r)

	err := s.RespawnWindow(1, "crush --listen /tmp/new.sock", "/project")
	if err != nil {
		t.Fatal(err)
	}

	if !r.called("-c") {
		t.Error("expected -c flag for directory")
	}
	if !r.called("/project") {
		t.Error("expected /project directory in respawn-window")
	}
}
