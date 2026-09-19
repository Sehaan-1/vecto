package runner_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/dag"
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

func TestRunner_ConcurrentAndCacheHit(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"codegen": {
				Command: "echo codegen",
			},
			"lint": {
				Command: "echo lint",
			},
			"build": {
				Command:      "echo build",
				Dependencies: []string{"codegen"},
			},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)

	// First Run: cold
	buf1 := &bytes.Buffer{}
	reporter1 := ui.NewReporter(buf1, false)
	r1 := runner.New(cfg, g, cacheMgr, reporter1, tempDir, runner.Options{Concurrency: 4})

	err := r1.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	out1 := buf1.String()
	if !strings.Contains(out1, "[✓] codegen") || !strings.Contains(out1, "[✓] build") {
		t.Fatalf("expected completed tasks in first run, got:\n%s", out1)
	}

	// Second Run: hot cache replay
	buf2 := &bytes.Buffer{}
	reporter2 := ui.NewReporter(buf2, false)
	r2 := runner.New(cfg, g, cacheMgr, reporter2, tempDir, runner.Options{Concurrency: 4})

	startHot := time.Now()
	err = r2.Run(context.Background(), nil)
	hotDuration := time.Since(startHot)

	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	out2 := buf2.String()
	if !strings.Contains(out2, "[⚡ CACHED] codegen") || !strings.Contains(out2, "[⚡ CACHED] build") {
		t.Fatalf("expected all tasks CACHED in second run, got:\n%s", out2)
	}

	// Must replay instantly (< 50ms per destination criteria)
	if hotDuration > 200*time.Millisecond {
		t.Errorf("hot cache replay took too long: %v", hotDuration)
	}
}

func TestRunner_FailFastAndKeepGoing(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"fail_task": {
				Command: "non_existent_command_12345",
			},
			"dep_task": {
				Command:      "echo dep",
				Dependencies: []string{"fail_task"},
			},
			"independent_task": {
				Command: "echo independent",
			},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)

	// Keep-going mode
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)
	r := runner.New(cfg, g, cacheMgr, reporter, tempDir, runner.Options{
		Concurrency: 2,
		KeepGoing:   true,
	})

	err := r.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected run error when task fails, got nil")
	}

	out := buf.String()
	if !strings.Contains(out, "[✗ FAILED] fail_task") {
		t.Errorf("expected fail_task to fail, got:\n%s", out)
	}
	if !strings.Contains(out, "[○ SKIPPED] dep_task") {
		t.Errorf("expected dep_task to be skipped, got:\n%s", out)
	}
	if !strings.Contains(out, "[✓] independent_task") {
		t.Errorf("expected independent_task to complete under keep-going, got:\n%s", out)
	}
}
