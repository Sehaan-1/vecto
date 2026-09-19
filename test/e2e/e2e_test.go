package e2e_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

func TestE2E_FullWalkAndCacheReplay(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Setup sample project with vecto.yaml
	manifest := `version: "1"
tasks:
  codegen:
    command: "echo generating code"
    inputs: ["schema.json"]

  lint:
    command: "echo linting code"
    inputs: ["src/*.txt"]

  build:
    command: "echo compiling binary"
    deps: ["codegen", "lint"]
    inputs: ["src/*.txt"]
    outputs: ["dist/output.txt"]
`
	if err := os.WriteFile(filepath.Join(tempDir, "vecto.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write vecto.yaml: %v", err)
	}

	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "schema.json"), []byte(`{"version": 1}`), 0644); err != nil {
		t.Fatalf("failed to write schema: %v", err)
	}

	if err := os.WriteFile(filepath.Join(srcDir, "main.txt"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write main.txt: %v", err)
	}

	// 2. Cold Run: walk through execution
	cfg, err := config.LoadConfig(tempDir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	g, err := cfg.BuildGraph()
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	cacheMgr := cache.New(tempDir)

	buf1 := &bytes.Buffer{}
	reporter1 := ui.NewReporter(buf1, false)
	r1 := runner.New(cfg, g, cacheMgr, reporter1, tempDir, runner.Options{Concurrency: 4})

	err = r1.Run(context.Background(), []string{"build"})
	if err != nil {
		t.Fatalf("Cold run failed: %v", err)
	}

	out1 := buf1.String()
	if !strings.Contains(out1, "[✓] codegen") || !strings.Contains(out1, "[✓] lint") || !strings.Contains(out1, "[✓] build") {
		t.Fatalf("Cold run did not execute all tasks. Got:\n%s", out1)
	}

	// 3. Hot Run: immediate re-run without changing files -> MUST BE 100% CACHED
	buf2 := &bytes.Buffer{}
	reporter2 := ui.NewReporter(buf2, false)
	r2 := runner.New(cfg, g, cacheMgr, reporter2, tempDir, runner.Options{Concurrency: 4})

	hotStart := time.Now()
	err = r2.Run(context.Background(), []string{"build"})
	hotElapsed := time.Since(hotStart)

	if err != nil {
		t.Fatalf("Hot run failed: %v", err)
	}

	out2 := buf2.String()
	if !strings.Contains(out2, "[⚡ CACHED] codegen") ||
		!strings.Contains(out2, "[⚡ CACHED] lint") ||
		!strings.Contains(out2, "[⚡ CACHED] build") {
		t.Fatalf("Hot run failed to replay all tasks from cache! Got:\n%s", out2)
	}

	// Destination criteria: replay must be sub-50ms
	if hotElapsed > 100*time.Millisecond {
		t.Errorf("Hot cache replay exceeded performance threshold: %v", hotElapsed)
	}

	// 4. Invalidation: modify schema.json
	if err := os.WriteFile(filepath.Join(tempDir, "schema.json"), []byte(`{"version": 2}`), 0644); err != nil {
		t.Fatalf("failed to update schema: %v", err)
	}

	buf3 := &bytes.Buffer{}
	reporter3 := ui.NewReporter(buf3, false)
	r3 := runner.New(cfg, g, cacheMgr, reporter3, tempDir, runner.Options{Concurrency: 4})

	err = r3.Run(context.Background(), []string{"build"})
	if err != nil {
		t.Fatalf("Invalidation run failed: %v", err)
	}

	out3 := buf3.String()
	// codegen must re-run because schema.json changed
	if !strings.Contains(out3, "[✓] codegen") {
		t.Errorf("expected codegen to re-run after input edit. Got:\n%s", out3)
	}
	// lint had NO changes, so lint must remain CACHED!
	if !strings.Contains(out3, "[⚡ CACHED] lint") {
		t.Errorf("expected lint to remain cached. Got:\n%s", out3)
	}
}
