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

const Version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "version", "--version", "-v":
		fmt.Printf("vecto version %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return

	case "help", "--help", "-h":
		printUsage()
		return

	case "init":
		handleInit()
		return

	case "clean":
		handleClean(os.Args[2:])
		return

	case "list":
		handleList()
		return

	case "graph":
		handleGraph(os.Args[2:])
		return

	case "run":
		handleRun(os.Args[2:])
		return

	default:
		// If first argument is not a known command, assume it's a task name to run
		handleRun(os.Args[1:])
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func handleInit() {
	target := "vecto.yaml"
	if _, err := os.Stat(target); err == nil {
		fmt.Println("vecto.yaml already exists in current directory.")
		return
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
		fmt.Fprintf(os.Stderr, "Error creating vecto.yaml: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created sample vecto.yaml in current directory.")
}

func handleClean(args []string) {
	cleanFlags := flag.NewFlagSet("clean", flag.ExitOnError)
	maxAgeStr := cleanFlags.String("max-age", "", "Prune cache entries older than duration (e.g. 24h, 168h)")
	cleanFlags.StringVar(maxAgeStr, "a", "", "Short for -max-age")

	if err := cleanFlags.Parse(args); err != nil {
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cacheMgr := cache.New(cwd)

	if *maxAgeStr != "" {
		d, err := time.ParseDuration(*maxAgeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing max-age %q: %v (examples: 24h, 168h)\n", *maxAgeStr, err)
			os.Exit(1)
		}
		pruned, err := cacheMgr.Prune(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error pruning cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Pruned %d cache entries older than %s (%s)\n", pruned, *maxAgeStr, cacheMgr.CacheDir)
		return
	}

	if err := cacheMgr.Clean(); err != nil {
		fmt.Fprintf(os.Stderr, "Error cleaning cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Cleared cache directory (%s)\n", cacheMgr.CacheDir)
}

func handleList() {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Tasks defined in vecto.yaml (%d total):\n\n", len(cfg.Tasks))
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
		fmt.Printf("  • %-16s cmd: %-25s deps: %s\n", n, t.Command, deps)
	}
}

func handleGraph(args []string) {
	graphFlags := flag.NewFlagSet("graph", flag.ExitOnError)
	format := graphFlags.String("format", "mermaid", "Output format: mermaid or dot")
	graphFlags.StringVar(format, "f", "mermaid", "Short for -format")

	if err := graphFlags.Parse(args); err != nil {
		os.Exit(1)
	}

	targets := graphFlags.Args()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	g, err := cfg.BuildGraph()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving task graph: %v\n", err)
		os.Exit(1)
	}

	switch strings.ToLower(*format) {
	case "mermaid", "mmd":
		out, err := g.ToMermaid(targets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Mermaid graph: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
	case "dot", "graphviz":
		out, err := g.ToDOT(targets)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating DOT graph: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
	default:
		fmt.Fprintf(os.Stderr, "Unknown format %q (supported: mermaid, dot)\n", *format)
		os.Exit(1)
	}
}

func handleRun(args []string) {
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

	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	concurrency := runFlags.Int("concurrency", runtime.NumCPU(), "Max concurrent worker goroutines")
	runFlags.IntVar(concurrency, "c", runtime.NumCPU(), "Short for -concurrency")
	keepGoing := runFlags.Bool("keep-going", false, "Continue independent tasks on error")
	runFlags.BoolVar(keepGoing, "k", false, "Short for -keep-going")
	force := runFlags.Bool("force", false, "Force re-execution, ignoring cache")
	runFlags.BoolVar(force, "f", false, "Short for -force")
	verbose := runFlags.Bool("verbose", false, "Surface command output for successful and cached tasks")
	runFlags.BoolVar(verbose, "v", false, "Short for -verbose")
	dryRun := runFlags.Bool("dry-run", false, "Preview execution plan and cache status without executing commands")
	runFlags.BoolVar(dryRun, "d", false, "Short for -dry-run")
	jsonOutput := runFlags.Bool("json", false, "Output execution summary in machine-readable JSON format for CI")
	logLevel := runFlags.String("log-level", "info", "Log level: debug, info, warn, error")
	logFormat := runFlags.String("log-format", "text", "Log format: text, json")

	if err := runFlags.Parse(runnerArgs); err != nil {
		os.Exit(1)
	}

	logger.Setup(os.Stderr, *logLevel, *logFormat)

	targets := runFlags.Args()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	cfg, err := config.LoadConfig(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	g, err := cfg.BuildGraph()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving task graph: %v\n", err)
		os.Exit(1)
	}

	cacheMgr := cache.New(cwd)

	if *dryRun {
		handleDryRun(cfg, g, cacheMgr, targets, cwd)
		return
	}

	var reporterWriter io.Writer = os.Stdout
	if *jsonOutput {
		reporterWriter = io.Discard
	}
	reporter := ui.NewReporter(reporterWriter, !*jsonOutput && isTerminal(os.Stdout))

	r := runner.New(cfg, g, cacheMgr, reporter, cwd, runner.Options{
		Concurrency:     *concurrency,
		KeepGoing:       *keepGoing,
		Force:           *force,
		Verbose:         *verbose,
		PassthroughArgs: passthroughArgs,
	})

	ctx := context.Background()
	startRun := time.Now()
	runErr := r.Run(ctx, targets)
	elapsed := time.Since(startRun)

	if *jsonOutput {
		jsonBytes, err := reporter.JSONSummary(elapsed)
		if err == nil {
			fmt.Println(string(jsonBytes))
		}
	}

	if runErr != nil {
		os.Exit(1)
	}
}

func handleDryRun(cfg *config.Config, g *dag.Graph, cacheMgr *cache.Manager, targets []string, cwd string) {
	topo, err := g.TopologicalSort()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving task graph: %v\n", err)
		os.Exit(1)
	}

	needed := g.NeededTasks(targets)
	depFingerprints := make(map[string]string)

	var plannedTasks []string
	for _, taskName := range topo {
		if needed[taskName] {
			plannedTasks = append(plannedTasks, taskName)
		}
	}

	fmt.Printf("Dry-run execution plan (%d tasks):\n\n", len(plannedTasks))
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
			fmt.Fprintf(os.Stderr, "Error computing fingerprint for %s: %v\n", taskName, err)
			os.Exit(1)
		}
		depFingerprints[taskName] = fp

		shortFP := fp
		if len(shortFP) > 12 {
			shortFP = shortFP[:12]
		}

		if cacheMgr.Has(fp) {
			fmt.Printf("  [⚡ CACHED]       %-16s (hash: %s)\n", taskName, shortFP)
			cachedCount++
		} else {
			fmt.Printf("  [WILL EXECUTE]   %-16s (hash: %s)\n", taskName, shortFP)
			execCount++
		}
	}

	fmt.Printf("\nPlan Summary: %d total (%d cached, %d will execute)\n", len(plannedTasks), cachedCount, execCount)
}

func printUsage() {
	fmt.Printf(`Vecto - High-Performance DAG Task & Build Caching Engine (v%s)

Usage:
  vecto <command> [flags] [targets...] [-- [args...]]

Commands:
  run [targets...]                 Run task(s) and their dependencies concurrently
  graph [targets...]               Export task graph as Mermaid or Graphviz DOT diagram
  list                             List all tasks defined in vecto.yaml
  init                             Create a sample vecto.yaml in current directory
  clean [-a, --max-age <duration>] Clear or prune cached task outputs
  version                          Print version information

Flags for 'run':
  -c, --concurrency N              Max parallel tasks (default: %d)
  -k, --keep-going                 Continue independent tasks on failure
  -f, --force                      Bypass cache and force rerun
  -v, --verbose                    Print command output for all tasks
  -d, --dry-run                    Preview execution plan and cache status without running commands
  --json                           Output machine-readable execution summary in JSON format
  --log-level <level>              Log level: debug, info, warn, error (default: info)
  --log-format <format>            Log format: text, json (default: text)
  -h, --help                       Show help

Flags for 'graph':
  -f, --format <format>            Output format: mermaid (default) or dot

Flags for 'clean':
  -a, --max-age <duration>         Prune entries older than duration (e.g. 24h, 168h)

Argument Passthrough:
  Pass flags directly to underlying task commands using the '--' separator:
    vecto run test -- -v -run TestSingle
`, Version, runtime.NumCPU())
}
