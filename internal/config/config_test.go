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

func TestConfig_UnknownFieldRejected(t *testing.T) {
	tempDir := t.TempDir()

	// Typo: "depend" instead of "deps"
	typoYAML := `
version: "1"
tasks:
  test:
    command: "echo test"
    depend: ["compile"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(typoYAML), 0644); err != nil {
		t.Fatalf("failed to write vecto.yaml: %v", err)
	}

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for unknown field 'depend', but it was silently accepted!")
	}
}

func TestConfig_UnsupportedVersionRejected(t *testing.T) {
	tempDir := t.TempDir()

	invalidYAML := `
version: "99"
tasks:
  test:
    command: "echo test"
`
	if err := os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write vecto.yaml: %v", err)
	}

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for unsupported version '99', got nil")
	}
}

func TestConfig_Diagnostics_TypoSuggestion(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
version: "1"
tasks:
  compile:
    command: "go build"
  test:
    command: "go test"
    deps: ["compyle"]
`
	_ = os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(yamlContent), 0644)

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for typo dependency, got nil")
	}

	errStr := err.Error()
	if !containsSubstring(errStr, "did you mean \"compile\"?") {
		t.Errorf("expected typo suggestion for 'compyle' -> 'compile', got: %s", errStr)
	}
}

func TestConfig_Diagnostics_SelfDependency(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
version: "1"
tasks:
  test:
    command: "go test"
    deps: ["test"]
`
	_ = os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(yamlContent), 0644)

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for self-dependency, got nil")
	}
	if !containsSubstring(err.Error(), "self-dependency detected") {
		t.Errorf("expected self-dependency diagnostic, got: %s", err.Error())
	}
}

func TestConfig_Diagnostics_GroupTaskSupported(t *testing.T) {
	tempDir := t.TempDir()

	// Group task with no command, only deps
	yamlContent := `
version: "1"
tasks:
  lint:
    command: "golangci-lint run"
  test:
    command: "go test ./..."
  all:
    deps: ["lint", "test"]
`
	_ = os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(yamlContent), 0644)

	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("group task without command should be valid, got: %v", err)
	}
	if len(cfg.Tasks["all"].Dependencies) != 2 {
		t.Errorf("expected 2 dependencies for group task 'all'")
	}
}

func TestConfig_Diagnostics_CircularDependency(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
version: "1"
tasks:
  a:
    command: "echo a"
    deps: ["b"]
  b:
    command: "echo b"
    deps: ["a"]
`
	_ = os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(yamlContent), 0644)

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for circular dependency, got nil")
	}
	if !containsSubstring(err.Error(), "cycle detected") {
		t.Errorf("expected cycle detection error, got: %s", err.Error())
	}
}

func TestConfig_Diagnostics_OverlappingInputOutput(t *testing.T) {
	tempDir := t.TempDir()

	yamlContent := `
version: "1"
tasks:
  build:
    command: "go build"
    inputs: ["main.go"]
    outputs: ["main.go"]
`
	_ = os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(yamlContent), 0644)

	_, err := config.LoadConfig(tempDir)
	if err == nil {
		t.Fatal("expected error for overlapping input/output, got nil")
	}
	if !containsSubstring(err.Error(), "both an input and an output") {
		t.Errorf("expected overlapping input/output diagnostic, got: %s", err.Error())
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}


