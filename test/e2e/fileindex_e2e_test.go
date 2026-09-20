package e2e_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildVecto compiles the CLI binary once per test into a temp dir.
func buildVectoFileIndex(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "vecto")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	// The module root is two levels up from test/e2e.
	cmd := exec.Command("go", "build", "-o", bin, "github.com/Sehaan-1/vecto/cmd/vecto")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

func runVectoFileIndex(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("vecto %v failed: %v\nstdout:\n%s\nstderr:\n%s", args, err, out.String(), errb.String())
	}
	return out.String()
}

// TestE2E_FileIndexCLIWorkflow mirrors the CLI default (ADR-0019: the Merkle
// file index is on by default): cold run, hot replay, invalidation, the
// `vecto index` introspection surface, --rebuild, and the --no-file-index
// legacy escape hatch.
func TestE2E_FileIndexCLIWorkflow(t *testing.T) {
	bin := buildVectoFileIndex(t)
	tempDir := t.TempDir()

	manifest := `version: "1"
tasks:
  lint:
    command: "echo linting"
    inputs: ["src/**"]

  build:
    command: "echo building"
    deps: ["lint"]
    inputs: ["src/**"]
    outputs: ["dist/out.txt"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write vecto.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tempDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "src", "a.go"), []byte("package a"), 0644); err != nil {
		t.Fatal(err)
	}

	// Cold run: executes, and creates the file index under .vecto/.
	out := runVectoFileIndex(t, bin, tempDir, "run")
	if !strings.Contains(out, "[✓] lint") || !strings.Contains(out, "[✓] build") {
		t.Fatalf("cold run should execute tasks, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".vecto", "fileindex.json")); err != nil {
		t.Fatalf("file index not created after run: %v", err)
	}

	// Hot run: fully cached via the index.
	out = runVectoFileIndex(t, bin, tempDir, "run")
	if !strings.Contains(out, "[⚡ CACHED] lint") || !strings.Contains(out, "[⚡ CACHED] build") {
		t.Fatalf("hot run should be fully cached, got:\n%s", out)
	}

	// `vecto index` introspection.
	out = runVectoFileIndex(t, bin, tempDir, "index")
	// Two indexed files: src/a.go and vecto.yaml itself. ("dirs indexed:" is
	// padded with two spaces for alignment with "files indexed:").
	for _, want := range []string{"Merkle File Index", "root hash", "files indexed: 2", "dirs indexed:  2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("index output missing %q, got:\n%s", want, out)
		}
	}

	// Invalidation: edit inside the glob.
	if err := os.WriteFile(filepath.Join(tempDir, "src", "a.go"), []byte("package a2"), 0644); err != nil {
		t.Fatal(err)
	}
	out = runVectoFileIndex(t, bin, tempDir, "run")
	if !strings.Contains(out, "[✓] lint") {
		t.Fatalf("edit inside glob should re-execute lint, got:\n%s", out)
	}

	// --rebuild resets the index.
	out = runVectoFileIndex(t, bin, tempDir, "index", "--rebuild")
	if !strings.Contains(out, "Removed file index") {
		t.Fatalf("rebuild output unexpected, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".vecto", "fileindex.json")); !os.IsNotExist(err) {
		t.Fatalf("index file should be gone after --rebuild")
	}

	// --no-file-index runs the legacy path and leaves no index behind.
	out = runVectoFileIndex(t, bin, tempDir, "run", "--no-file-index")
	if !strings.Contains(out, "[✓] lint") {
		t.Fatalf("legacy run should execute (cold namespace), got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tempDir, ".vecto", "fileindex.json")); !os.IsNotExist(err) {
		t.Fatalf("--no-file-index must not create an index file")
	}
}
