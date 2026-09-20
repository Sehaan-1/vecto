package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/dag"
	"github.com/Sehaan-1/vecto/internal/hash"
	"github.com/Sehaan-1/vecto/internal/logger"
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func handleInit(stdout, stderr io.Writer) int {
	target := "vecto.yaml"
	if _, err := os.Stat(target); err == nil {
		fmt.Fprintln(stdout, "vecto.yaml already exists in current directory.")
		return 0
	}

	sample := `version: "1"

tasks:
  lint:
    command: "go vet ./..."
    inputs: ["**/*.go"]

  test:
    command: "go test ./..."
    deps: ["lint"]
    inputs: ["**/*.go"]

  build:
    command: "go build -o bin/app ."
    deps: ["test"]
    inputs: ["**/*.go"]
    outputs: ["bin/app"]
`
	if err := os.WriteFile(target, []byte(sample), 0644); err != nil {
		fmt.Fprintf(stderr, "Error creating vecto.yaml: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "Created sample vecto.yaml in current directory.")
	return 0
}

func handleClean(args []string, stdout, stderr io.Writer) int {
	cleanFlags := flag.NewFlagSet("clean", flag.ContinueOnError)
	cleanFlags.SetOutput(stderr)
	// Single backing var, one registration per name, same default (ADR-0011).
	var maxAge string
	cleanFlags.StringVar(&maxAge, "max-age", "", "Prune cache entries older than duration (e.g. 24h, 168h)")
	cleanFlags.StringVar(&maxAge, "a", "", "Short for -max-age")

	if err := cleanFlags.Parse(args); err != nil {
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cacheMgr := cache.New(cwd)

	if maxAge != "" {
		d, err := time.ParseDuration(maxAge)
		if err != nil {
			fmt.Fprintf(stderr, "Error parsing max-age %q: %v (examples: 24h, 168h)\n", maxAge, err)
			return 1
		}
		pruned, err := cacheMgr.Prune(d)
		if err != nil {
			fmt.Fprintf(stderr, "Error pruning cache: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Pruned %d cache entries older than %s (%s)\n", pruned, maxAge, cacheMgr.CacheDir)
		return 0
	}

	if err := cacheMgr.Clean(); err != nil {
		fmt.Fprintf(stderr, "Error cleaning cache: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Cleared cache directory (%s)\n", cacheMgr.CacheDir)
	return 0
}

func handleList(stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Tasks defined in vecto.yaml (%d total):\n\n", len(cfg.Tasks))
	names := make([]string, 0, len(cfg.Tasks))
	for n := range cfg.Tasks {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		t := cfg.Tasks[n]
		deps := "(none)"
		if len(t.Dependencies) > 0 {
			deps = fmt.Sprintf("%v", t.Dependencies)
		}
		fmt.Fprintf(stdout, "  • %-16s cmd: %-25s deps: %s\n", n, t.Command, deps)
	}
	return 0
}

func handleGraph(args []string, stdout, stderr io.Writer) int {
	graphFlags := flag.NewFlagSet("graph", flag.ContinueOnError)
	graphFlags.SetOutput(stderr)
	var format string
	graphFlags.StringVar(&format, "format", "mermaid", "Output format: mermaid or dot")
	graphFlags.StringVar(&format, "f", "mermaid", "Short for -format")

	if err := graphFlags.Parse(args); err != nil {
		return 2
	}

	targets := graphFlags.Args()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	g, err := cfg.BuildGraph()
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving task graph: %v\n", err)
		return 1
	}

	switch strings.ToLower(format) {
	case "mermaid", "mmd":
		out, err := g.ToMermaid(targets)
		if err != nil {
			fmt.Fprintf(stderr, "Error generating Mermaid graph: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, out)
		return 0
	case "dot", "graphviz":
		out, err := g.ToDOT(targets)
		if err != nil {
			fmt.Fprintf(stderr, "Error generating DOT graph: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, out)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown format %q (supported: mermaid, dot)\n", format)
		return 1
	}
}

func handleRun(args []string, stdout, stderr io.Writer) int {
	// Separate runner flags from dynamic passthrough args after "--"
	var runnerArgs []string
	var passthroughArgs []string
	for i, arg := range args {
		if arg == "--" {
			runnerArgs = args[:i]
			if i+1 < len(args) {
				passthroughArgs = args[i+1:]
			}
			break
		}
	}
	if passthroughArgs == nil {
		runnerArgs = args
	}

	runFlags := flag.NewFlagSet("run", flag.ContinueOnError)
	runFlags.SetOutput(stderr)
	var concurrency int
	runFlags.IntVar(&concurrency, "concurrency", runtime.NumCPU(), "Max concurrent worker goroutines")
	runFlags.IntVar(&concurrency, "c", runtime.NumCPU(), "Short for -concurrency")
	var keepGoing bool
	runFlags.BoolVar(&keepGoing, "keep-going", false, "Continue independent tasks on error")
	runFlags.BoolVar(&keepGoing, "k", false, "Short for -keep-going")
	var force bool
	runFlags.BoolVar(&force, "force", false, "Force re-execution, ignoring cache")
	runFlags.BoolVar(&force, "f", false, "Short for -force")
	var verbose bool
	runFlags.BoolVar(&verbose, "verbose", false, "Surface command output for successful and cached tasks")
	runFlags.BoolVar(&verbose, "v", false, "Short for -verbose")
	var dryRun bool
	runFlags.BoolVar(&dryRun, "dry-run", false, "Preview execution plan and cache status without executing commands")
	runFlags.BoolVar(&dryRun, "d", false, "Short for -dry-run")
	var jsonOutput bool
	runFlags.BoolVar(&jsonOutput, "json", false, "Output execution summary in machine-readable JSON format for CI")
	var logLevel string
	runFlags.StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	var logFormat string
	runFlags.StringVar(&logFormat, "log-format", "text", "Log format: text, json")

	if err := runFlags.Parse(runnerArgs); err != nil {
		return 2
	}

	logger.Setup(stderr, logLevel, logFormat)

	targets := runFlags.Args()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	g, err := cfg.BuildGraph()
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving task graph: %v\n", err)
		return 1
	}

	cacheMgr := cache.New(cwd)

	if dryRun {
		return handleDryRun(cfg, g, cacheMgr, targets, cwd, stdout, stderr)
	}

	var reporterWriter io.Writer = stdout
	if jsonOutput {
		reporterWriter = io.Discard
	}
	tty := false
	if f, ok := stdout.(*os.File); ok {
		tty = isTerminal(f)
	}
	reporter := ui.NewReporter(reporterWriter, !jsonOutput && tty)

	r := runner.New(cfg, g, cacheMgr, reporter, cwd, runner.Options{
		Concurrency:     concurrency,
		KeepGoing:       keepGoing,
		Force:           force,
		Verbose:         verbose,
		PassthroughArgs: passthroughArgs,
	})

	ctx := context.Background()
	startRun := time.Now()
	runErr := r.Run(ctx, targets)
	elapsed := time.Since(startRun)

	if jsonOutput {
		jsonBytes, err := reporter.JSONSummary(elapsed)
		if err == nil {
			fmt.Fprintln(stdout, string(jsonBytes))
		}
	}

	if runErr != nil {
		return 1
	}
	return 0
}

func handleDryRun(cfg *config.Config, g *dag.Graph, cacheMgr *cache.Manager, targets []string, cwd string, stdout, stderr io.Writer) int {
	topo, err := g.TopologicalSort()
	if err != nil {
		fmt.Fprintf(stderr, "Error resolving task graph: %v\n", err)
		return 1
	}

	needed := g.NeededTasks(targets)
	depFingerprints := make(map[string]string)

	var plannedTasks []string
	for _, taskName := range topo {
		if needed[taskName] {
			plannedTasks = append(plannedTasks, taskName)
		}
	}

	fmt.Fprintf(stdout, "Dry-run execution plan (%d tasks):\n\n", len(plannedTasks))
	cachedCount := 0
	execCount := 0

	for _, taskName := range plannedTasks {
		taskCfg := cfg.Tasks[taskName]

		deps := make(map[string]string, len(taskCfg.Dependencies))
		for _, dep := range taskCfg.Dependencies {
			if fp, ok := depFingerprints[dep]; ok {
				deps[dep] = fp
			}
		}

		fp, err := hash.ComputeTaskFingerprint(cwd, taskCfg.Command, taskCfg.Inputs, taskCfg.Env, deps)
		if err != nil {
			fmt.Fprintf(stderr, "Error computing fingerprint for %s: %v\n", taskName, err)
			return 1
		}
		depFingerprints[taskName] = fp

		shortFP := fp
		if len(shortFP) > 12 {
			shortFP = shortFP[:12]
		}

		if cacheMgr.Has(fp) {
			fmt.Fprintf(stdout, "  [⚡ CACHED]       %-16s (hash: %s)\n", taskName, shortFP)
			cachedCount++
		} else {
			fmt.Fprintf(stdout, "  [WILL EXECUTE]   %-16s (hash: %s)\n", taskName, shortFP)
			execCount++
		}
	}

	fmt.Fprintf(stdout, "\nPlan Summary: %d total (%d cached, %d will execute)\n", len(plannedTasks), cachedCount, execCount)
	return 0
}
