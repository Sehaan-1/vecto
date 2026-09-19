package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
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

func handleRun(args []string) {
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	concurrency := runFlags.Int("concurrency", runtime.NumCPU(), "Max concurrent worker goroutines")
	runFlags.IntVar(concurrency, "c", runtime.NumCPU(), "Short for -concurrency")
	keepGoing := runFlags.Bool("keep-going", false, "Continue independent tasks on error")
	runFlags.BoolVar(keepGoing, "k", false, "Short for -keep-going")
	force := runFlags.Bool("force", false, "Force re-execution, ignoring cache")
	runFlags.BoolVar(force, "f", false, "Short for -force")
	verbose := runFlags.Bool("verbose", false, "Surface command output for successful and cached tasks")
	runFlags.BoolVar(verbose, "v", false, "Short for -verbose")

	if err := runFlags.Parse(args); err != nil {
		os.Exit(1)
	}

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
	reporter := ui.NewReporter(os.Stdout, isTerminal(os.Stdout))

	r := runner.New(cfg, g, cacheMgr, reporter, cwd, runner.Options{
		Concurrency: *concurrency,
		KeepGoing:   *keepGoing,
		Force:       *force,
		Verbose:     *verbose,
	})

	ctx := context.Background()
	if err := r.Run(ctx, targets); err != nil {
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`Vecto - High-Performance DAG Task & Build Caching Engine (v%s)

Usage:
  vecto <command> [flags] [targets...]

Commands:
  run [targets...]                 Run task(s) and their dependencies concurrently
  list                             List all tasks defined in vecto.yaml
  init                             Create a sample vecto.yaml in current directory
  clean [-a, --max-age <duration>] Clear or prune cached task outputs
  version                          Print version information

Flags for 'run':
  -c, --concurrency N              Max parallel tasks (default: %d)
  -k, --keep-going                 Continue independent tasks on failure
  -f, --force                      Bypass cache and force rerun
  -v, --verbose                    Print command output for all tasks
  -h, --help                       Show help

Flags for 'clean':
  -a, --max-age <duration>         Prune entries older than duration (e.g. 24h, 168h)
`, Version, runtime.NumCPU())
}
