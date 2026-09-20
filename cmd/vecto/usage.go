package main

import (
	"fmt"
	"io"
	"runtime"
)

// printUsage writes help text to w (ADR-0011: help lives here, not in main.go).
func printUsage(w io.Writer) {
	fmt.Fprintf(w, `Vecto - High-Performance DAG Task & Build Caching Engine (v%s)

Usage:
  vecto <command> [flags] [targets...] [-- [args...]]

Commands:
  run [targets...]                 Run task(s) and their dependencies concurrently
  graph [targets...]               Export task graph as Mermaid or Graphviz DOT diagram
  list                             List all tasks defined in vecto.yaml
  index [--rebuild]                Inspect (or reset) the Merkle file index (ADR-0019)
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
  --no-file-index                  Disable the Merkle file index (legacy full-scan hashing)
  --log-level <level>              Log level: debug, info, warn, error (default: info)
  --log-format <format>            Log format: text, json (default: text)
  -h, --help                       Show help

Flags for 'graph':
  -f, --format <format>            Output format: mermaid (default) or dot

Flags for 'index':
  --rebuild                        Delete the file index; the next run rebuilds it

Flags for 'clean':
  -a, --max-age <duration>         Prune entries older than duration (e.g. 24h, 168h)

Argument Passthrough:
  Pass flags directly to underlying task commands using the '--' separator:
    vecto run test -- -v -run TestSingle
`, Version, runtime.NumCPU())
}
