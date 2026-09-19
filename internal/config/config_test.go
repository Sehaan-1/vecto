package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sehaan-1/vecto/internal/config"
)

func TestConfig_LoadAndBuildGraph(t *testing.T) {
	tempDir := t.TempDir()

	validYAML := `
version: "1"
tasks:
  compile:
    command: "echo compiling"
    inputs: ["src/*.go"]
    outputs: ["bin/"]
  test:
    command: "echo testing"
    deps: ["compile"]
    inputs: ["src/*.go"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(validYAML), 0644); err != nil {
		t.Fatalf("failed to write vecto.yaml: %v", err)
	}

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(cfg.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(cfg.Tasks))
	}

	g, err := cfg.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	layers, err := g.ExecutionLayers()
	if err != nil {
		t.Fatalf("ExecutionLayers failed: %v", err)
	}

	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(layers))
	}
}

func TestConfig_MissingDependencyFails(t *testing.T) {
	tempDir := t.TempDir()

	invalidYAML := `
version: "1"
tasks:
  test:
    command: "echo test"
    deps: ["ghost_task"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write vecto.yaml: %v", err)
	}

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for missing dependency, got nil")
	}
}
