package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
)

const Version = "0.1.0"

// main is a thin starter: all logic lives in run() so tests can call it.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches one CLI invocation and returns the process exit code.
// It never calls os.Exit itself (ADR-0011).
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stdout)
		return 0
	}

	command := args[0]

	switch command {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "vecto version %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return 0

	case "help", "--help", "-h":
		printUsage(stdout)
		return 0

	case "init":
		return handleInit(stdout, stderr)

	case "clean":
		return handleClean(args[1:], stdout, stderr)

	case "list":
		return handleList(stdout, stderr)

	case "graph":
		return handleGraph(args[1:], stdout, stderr)

	case "run":
		return handleRun(args[1:], stdout, stderr)

	default:
		// If first argument is not a known command, assume it's a task name to run.
		return handleRun(args, stdout, stderr)
	}
}
