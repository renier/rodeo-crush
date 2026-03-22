package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/renier/rodeo-crush/internal/config"
	"github.com/renier/rodeo-crush/internal/orchestrator"
	"github.com/renier/rodeo-crush/internal/tui"
)

const usage = `Rodeo Crush - AI Agent Orchestration Harness

Usage:
  rodeo [flags]

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
  rodeo                          # Use defaults
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
	var (
		teamFile     string
		projectDir   string
		logFile      string
		stallTimeout time.Duration
		pollInterval time.Duration
	)

	stallTimeout = orchestrator.DefaultStallTimeout
	pollInterval = orchestrator.DefaultPollInterval

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-h", "--help":
			fmt.Print(usage)
			return nil
		case "-t", "--team":
			if i+1 >= len(args) {
				return fmt.Errorf("--team requires a file path")
			}
			i++
			teamFile = args[i]
		case "-d", "--dir":
			if i+1 >= len(args) {
				return fmt.Errorf("--dir requires a path")
			}
			i++
			projectDir = args[i]
		case "-s", "--stall":
			if i+1 >= len(args) {
				return fmt.Errorf("--stall requires a duration")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("invalid stall duration: %w", err)
			}
			stallTimeout = d
		case "-p", "--poll":
			if i+1 >= len(args) {
				return fmt.Errorf("--poll requires a duration")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return fmt.Errorf("invalid poll duration: %w", err)
			}
			pollInterval = d
		case "-l", "--log":
			if i+1 >= len(args) {
				return fmt.Errorf("--log requires a file path")
			}
			i++
			logFile = args[i]
		default:
			return fmt.Errorf("unknown flag: %s\nRun 'rodeo --help' for usage", args[i])
		}
	}

	cfgDir, err := config.Bootstrap()
	if err != nil {
		return fmt.Errorf("bootstrapping config: %w", err)
	}

	if projectDir == "" {
		projectDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	projectDir, err = filepath.Abs(projectDir)
	if err != nil {
		return fmt.Errorf("resolving project dir: %w", err)
	}

	if teamFile == "" {
		teamFile = filepath.Join(cfgDir, "team.yaml")
	}

	cfg, err := config.Load(teamFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	promptBaseDir := filepath.Dir(teamFile)
	if err := cfg.ResolvePrompts(promptBaseDir); err != nil {
		return fmt.Errorf("resolving prompts: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

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
	o.StallTimeout = stallTimeout
	o.PollInterval = pollInterval
	o.Logger = logger

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := o.Start(ctx); err != nil {
			logger.Error("orchestrator error", "error", err)
		}
	}()

	if err := tui.Run(cfg.Session); err != nil {
		cancel()
		_ = o.Stop()
		return fmt.Errorf("TUI error: %w", err)
	}

	cancel()
	logger.Info("rodeo crush stopped, cleaning up")
	return o.Stop()
}
