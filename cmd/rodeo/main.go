package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/renier/rodeo-crush/internal/config"
	"github.com/renier/rodeo-crush/internal/orchestrator"
	"github.com/renier/rodeo-crush/internal/tui"
)

const usage = `Rodeo Crush - AI Agent Orchestration Harness

Usage:
  rodeo [flags]            Start the orchestrator (tmux agents)
  rodeo beads              Show the beads progress dashboard

Flags:
  -t, --team <file>       Team config YAML (default: ~/.config/rodeo-crush/team.yaml)
  -d, --dir <path>        Project directory (default: current directory)
  -s, --stall <duration>  Stall timeout before prodding agents (default: 15m)
  -p, --poll <duration>   Poll interval for monitoring (default: 30s)
  -l, --log <file>        Log file path (default: ~/.config/rodeo-crush/rodeo-crush.log)
  -h, --help              Show this help

Configuration lives in ~/.config/rodeo-crush/:
  team.yaml               Team configuration (roles, counts, labels, filters)
  prompts/*.md            Prompt files for each role

On first run, default configuration files are created automatically.
Edit them to customize roles or add new ones.

Examples:
  rodeo                          # Start orchestrator
  rodeo beads                    # Monitor beads in another terminal
  rodeo -t my-team.yaml          # Custom team config
  rodeo -d /path/to/project      # Specify project dir
  rodeo -s 5m -p 1m              # Custom timeouts
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	// Subcommand must be the first argument (before any flags).
	var subcmd string
	rest := args
	if len(args) > 0 && len(args[0]) > 0 && args[0][0] != '-' {
		subcmd = args[0]
		rest = args[1:]
	}

	switch subcmd {
	case "":
		return runOrchestrator(rest)
	case "beads":
		return runBeads(rest)
	default:
		return fmt.Errorf("unknown command: %s\nRun 'rodeo --help' for usage", subcmd)
	}
}

type flags struct {
	teamFile     string
	projectDir   string
	logFile      string
	stallTimeout time.Duration
	pollInterval time.Duration
}

func newFlagSet(name string) (*flag.FlagSet, *flags) {
	f := &flags{
		stallTimeout: orchestrator.DefaultStallTimeout,
		pollInterval: orchestrator.DefaultPollInterval,
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Print(usage) }

	fs.StringVar(&f.teamFile, "t", "", "")
	fs.StringVar(&f.teamFile, "team", "", "")
	fs.StringVar(&f.projectDir, "d", "", "")
	fs.StringVar(&f.projectDir, "dir", "", "")
	fs.StringVar(&f.logFile, "l", "", "")
	fs.StringVar(&f.logFile, "log", "", "")
	fs.DurationVar(&f.stallTimeout, "s", f.stallTimeout, "")
	fs.DurationVar(&f.stallTimeout, "stall", f.stallTimeout, "")
	fs.DurationVar(&f.pollInterval, "p", f.pollInterval, "")
	fs.DurationVar(&f.pollInterval, "poll", f.pollInterval, "")

	return fs, f
}

// parseFlags parses the argument list into a FlagSet and flags struct.
// Returns flag.ErrHelp cleanly (caller should exit 0).
func parseFlags(name string, args []string) (*flag.FlagSet, *flags, error) {
	fs, f := newFlagSet(name)
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	if fs.NArg() > 0 {
		return nil, nil, fmt.Errorf("unexpected argument: %s\nRun 'rodeo --help' for usage", fs.Arg(0))
	}
	return fs, f, nil
}

func resolveConfig(teamFile, projectDir string) (*config.TeamConfig, string, string, error) {
	cfgDir, err := config.Bootstrap()
	if err != nil {
		return nil, "", "", fmt.Errorf("bootstrapping config: %w", err)
	}

	if projectDir == "" {
		projectDir, err = os.Getwd()
		if err != nil {
			return nil, "", "", fmt.Errorf("getting working directory: %w", err)
		}
	}

	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolving project dir: %w", err)
	}

	if teamFile == "" {
		teamFile = filepath.Join(cfgDir, "team.yaml")
	}

	cfg, err := config.Load(teamFile)
	if err != nil {
		return nil, "", "", fmt.Errorf("loading config: %w", err)
	}

	promptBaseDir := filepath.Dir(teamFile)
	if err := cfg.ResolvePrompts(promptBaseDir); err != nil {
		return nil, "", "", fmt.Errorf("resolving prompts: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, "", "", fmt.Errorf("invalid config: %w", err)
	}

	return cfg, cfgDir, projectDir, nil
}

func runOrchestrator(args []string) error {
	_, f, err := parseFlags("rodeo", args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	cfg, cfgDir, projectDir, err := resolveConfig(f.teamFile, f.projectDir)
	if err != nil {
		return err
	}

	logFile := f.logFile
	if logFile == "" {
		logFile = filepath.Join(cfgDir, "rodeo-crush.log")
	}

	lf, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", logFile, err)
	}
	defer lf.Close()

	logger := slog.New(slog.NewTextHandler(lf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	o := orchestrator.New(cfg, projectDir)
	o.StallTimeout = f.stallTimeout
	o.PollInterval = f.pollInterval
	o.Logger = logger

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := o.Start(ctx); err != nil {
			logger.Error("orchestrator error", "error", err)
		}
	}()

	fmt.Printf("%s running\n", config.AppName)
	fmt.Println("   Run 'rodeo beads' in another terminal to monitor progress.")
	fmt.Println("   Press Ctrl+C to stop.")

	<-ctx.Done()

	fmt.Println("\nStopping...")
	logger.Info("rodeo crush stopped, cleaning up")
	return o.Stop()
}

func runBeads(args []string) error {
	fs := flag.NewFlagSet("rodeo beads", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Print(usage) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected argument: %s\nRun 'rodeo --help' for usage", fs.Arg(0))
	}

	return tui.Run()
}
