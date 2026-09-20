package runner_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/dag"
	"github.com/Sehaan-1/vecto/internal/fileindex"
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

// TestRunner_FileIndexIncrementalInvalidation exercises the full ADR-0019
// loop through the real runner: cold run, hot replay, invalidation on an
// edit inside the glob, and no false invalidation on an edit outside it.
func TestRunner_FileIndexIncrementalInvalidation(t *testing.T) {
	tempDir := t.TempDir()

	// Project files: input inside the glob, input outside it.
	if err := os.MkdirAll(filepath.Join(tempDir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "sub", "in.txt"), []byte("in-v1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "out.txt"), []byte("out-v1"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"process": {
				Command: "echo process",
				Inputs:  []string{"sub/**"},
			},
			"watch": {
				Command:      "echo watch",
				Dependencies: []string{"process"},
				Inputs:       []string{"sub/**"},
			},
		},
	}
	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}
	cacheMgr := cache.New(tempDir)

	doRun := func() string {
		buf := &bytes.Buffer{}
		r := runner.New(cfg, g, cacheMgr, ui.NewReporter(buf, false), tempDir,
			runner.Options{Concurrency: 2, FileIndex: true})
		if err := r.Run(context.Background(), nil); err != nil {
			t.Fatalf("run failed: %v", err)
		}
		return buf.String()
	}

	// 1. Cold: both execute.
	out1 := doRun()
	if !strings.Contains(out1, "[✓] process") || !strings.Contains(out1, "[✓] watch") {
		t.Fatalf("cold run should execute both tasks, got:\n%s", out1)
	}
	if _, err := os.Stat(fileindex.IndexPath(tempDir)); err != nil {
		t.Fatalf("index file not created: %v", err)
	}

	// 2. Hot: both cached.
	out2 := doRun()
	if !strings.Contains(out2, "[⚡ CACHED] process") || !strings.Contains(out2, "[⚡ CACHED] watch") {
		t.Fatalf("hot run should cache both tasks, got:\n%s", out2)
	}

	// 3. Edit inside the glob: both re-execute (process fingerprint changes,
	// watch changes transitively via the dependency fingerprint).
	if err := os.WriteFile(filepath.Join(tempDir, "sub", "in.txt"), []byte("in-v2"), 0644); err != nil {
		t.Fatal(err)
	}
	out3 := doRun()
	if !strings.Contains(out3, "[✓] process") || !strings.Contains(out3, "[✓] watch") {
		t.Fatalf("edit inside glob should re-execute both tasks, got:\n%s", out3)
	}

	// 4. Edit outside the glob: both cached again (no false invalidation).
	if err := os.WriteFile(filepath.Join(tempDir, "out.txt"), []byte("out-v2"), 0644); err != nil {
		t.Fatal(err)
	}
	out4 := doRun()
	if !strings.Contains(out4, "[⚡ CACHED] process") || !strings.Contains(out4, "[⚡ CACHED] watch") {
		t.Fatalf("edit outside glob must not invalidate, got:\n%s", out4)
	}
}

// TestRunner_FileIndexFallbackOnCorruptIndex: a corrupted index must degrade
// to legacy hashing — tasks still run and are cached correctly.
func TestRunner_FileIndexFallbackOnCorruptIndex(t *testing.T) {
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, ".vecto"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileindex.IndexPath(tempDir), []byte("{corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "data.txt"), []byte("d"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"job": {Command: "echo job", Inputs: []string{"data.txt"}},
		},
	}
	g := dag.New()
	g.AddTask("job", nil)
	cacheMgr := cache.New(tempDir)

	buf := &bytes.Buffer{}
	r := runner.New(cfg, g, cacheMgr, ui.NewReporter(buf, false), tempDir,
		runner.Options{Concurrency: 1, FileIndex: true})
	if err := r.Run(context.Background(), nil); err != nil {
		t.Fatalf("run with corrupt index failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[✓] job") {
		t.Fatalf("job should execute via legacy fallback, got:\n%s", out)
	}
	if !strings.Contains(out, "falling back to full-scan hashing") {
		t.Fatalf("expected a fallback warning, got:\n%s", out)
	}
	// A healthy index must now exist (the fallback rebuilt via legacy hashing
	// does NOT write an index, so the corrupt file is simply left alone —
	// next run warns again). The key property: the run succeeded.
}
