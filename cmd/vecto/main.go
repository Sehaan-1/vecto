package main

import (
	"fmt"
	"os"
)

const Version = "0.1.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "--version", "-v":
			fmt.Printf("vecto version %s\n", Version)
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}
	printUsage()
}

func printUsage() {
	fmt.Printf(`Vecto - High-Performance DAG Task & Build Caching Engine (v%s)

Usage:
  vecto <command> [arguments]

Commands:
  run <task...>     Run task(s) and their dependencies concurrently
  list              List all tasks defined in vecto.yaml
  init              Create a sample vecto.yaml in the current directory
  clean             Clear local cached task outputs (.vecto/cache)
  version           Print version information

Flags:
  -h, --help        Show this help message
  -v, --version     Show version
`, Version)
}
