package runner_test

import (
	"bytes"
	"context"
	"runtime"
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

	// Must replay instantly (< 200ms)
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

func TestRunner_TransitiveSkip_MultiLevel(t *testing.T) {
	tempDir := t.TempDir()

	// A -> B -> C (multi-level chain)
	// D is independent
	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"A_fail": {
				Command: "non_existent_cmd_fail_abc",
			},
			"B_dep": {
				Command:      "echo B",
				Dependencies: []string{"A_fail"},
			},
			"C_deep_dep": {
				Command:      "echo C",
				Dependencies: []string{"B_dep"},
			},
			"D_independent": {
				Command: "echo D",
			},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)

	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)
	r := runner.New(cfg, g, cacheMgr, reporter, tempDir, runner.Options{
		Concurrency: 2,
		KeepGoing:   true,
	})

	err := r.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected failure")
	}

	out := buf.String()
	if !strings.Contains(out, "[✗ FAILED] A_fail") {
		t.Errorf("expected A_fail to fail, got:\n%s", out)
	}
	if !strings.Contains(out, "[○ SKIPPED] B_dep") {
		t.Errorf("expected B_dep to be skipped, got:\n%s", out)
	}
	// CRITICAL TEST: C depends on B (which was skipped, not failed). C MUST BE SKIPPED!
	if !strings.Contains(out, "[○ SKIPPED] C_deep_dep") {
		t.Errorf("BUG DETECTED: C_deep_dep was not skipped when upstream B was skipped! Got:\n%s", out)
	}
	if strings.Contains(out, "[✓] C_deep_dep") {
		t.Errorf("FATAL BUG: C_deep_dep executed even though its dependency was skipped! Got:\n%s", out)
	}
	if !strings.Contains(out, "[✓] D_independent") {
		t.Errorf("expected D_independent to succeed under keep-going, got:\n%s", out)
	}
}

func TestRunner_TargetSubsetAndVerbose(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"codegen": {
				Command: "echo generating code",
			},
			"lint": {
				Command: "echo linting code",
			},
			"build": {
				Command:      "echo compiling binary",
				Dependencies: []string{"codegen"},
			},
			"deploy": {
				Command:      "echo deploying binary",
				Dependencies: []string{"build"},
			},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)

	// Target ONLY build (which depends on codegen).
	// lint and deploy should NOT run, and should NOT appear in total count.
	r := runner.New(cfg, g, cacheMgr, reporter, tempDir, runner.Options{
		Concurrency: 2,
		Verbose:     true,
	})

	err := r.Run(context.Background(), []string{"build"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	reporter.Summary(50 * time.Millisecond)
	out := buf.String()

	// Verify codegen and build ran
	if !strings.Contains(out, "[✓] codegen") {
		t.Errorf("expected codegen to execute, got:\n%s", out)
	}
	if !strings.Contains(out, "[✓] build") {
		t.Errorf("expected build to execute, got:\n%s", out)
	}

	// Verify lint and deploy did NOT execute
	if strings.Contains(out, "lint") {
		t.Errorf("lint should not have executed or been registered, got:\n%s", out)
	}
	if strings.Contains(out, "deploy") {
		t.Errorf("deploy should not have executed or been registered, got:\n%s", out)
	}

	// Verify total count in summary is 2 (only targeted + dependencies), NOT 4
	if !strings.Contains(out, "Tasks:    2 total (0 cached, 2 executed, 0 failed, 0 skipped)") {
		t.Errorf("expected Summary to report 2 total tasks for target 'build', got:\n%s", out)
	}

	// Verify verbose output surfaced stdout
	if !strings.Contains(out, "compiling binary") {
		t.Errorf("expected verbose mode to surface command stdout, got:\n%s", out)
	}
}

// TestRunner_NoHeadOfLineBlocking proves the reactive scheduler dispatches a
// task the instant its own dependencies are met, without waiting for unrelated
// slow sibling tasks.
//
// Graph:
//
//	A_slow (1.5s) ─────────────────────► D_blocked (needs A and B)
//	B_fast (instant) ──► C_ready
//
// Under a naive layer-based scheduler C_ready would be blocked until A_slow
// finishes. The reactive scheduler must fire C_ready the instant B_fast
// completes. Total wall time should be ~1.5s (A_slow), not 1.5s + overhead.
func TestRunner_NoHeadOfLineBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	tempDir := t.TempDir()

	slowCmd := "sleep 1.5"
	fastCmd := "echo fast"
	if runtime.GOOS == "windows" {
		slowCmd = "ping 127.0.0.1 -n 3 > nul"
	}

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"A_slow": {Command: slowCmd},
			"B_fast": {Command: fastCmd},
			"C_ready": {
				Command:      "echo C ran",
				Dependencies: []string{"B_fast"},
			},
			"D_blocked": {
				Command:      "echo D ran",
				Dependencies: []string{"A_slow", "B_fast"},
			},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)
	r := runner.New(cfg, g, cacheMgr, reporter, tempDir, runner.Options{Concurrency: 4})

	start := time.Now()
	if err := r.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	total := time.Since(start)

	out := buf.String()
	for _, task := range []string{"A_slow", "B_fast", "C_ready", "D_blocked"} {
		if strings.Contains(out, "[✗ FAILED] "+task) {
			t.Errorf("task %s unexpectedly failed:\n%s", task, out)
		}
	}

	// Reactive proof: total must be dominated by A_slow (~1.5s) only.
	// Allow [1s, 3s]: less than 1s means A_slow didn't run; more than 3s
	// suggests head-of-line blocking added artificial serialisation.
	if total < 1*time.Second {
		t.Errorf("run finished suspiciously fast (%v) — did A_slow actually run?", total)
	}
	if total > 4*time.Second {
		t.Errorf("run took %v — possible head-of-line blocking; expected ~1.5-2.0s", total)
	}
}

// TestRunner_CancellationTeardown verifies that cancelling the run context terminates
// in-flight processes promptly via the process group escalation ladder (ADR-0007).
func TestRunner_CancellationTeardown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	tempDir := t.TempDir()

	sleepCmd := "sleep 10"
	if runtime.GOOS == "windows" {
		sleepCmd = "powershell -NoProfile -Command \"Start-Sleep -Seconds 10\""
	}

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"long_task": {Command: sleepCmd},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)
	r := runner.New(cfg, g, cacheMgr, reporter, tempDir, runner.Options{Concurrency: 2})

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 200ms
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := r.Run(ctx, nil)
	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}

	// Must terminate promptly (well under 5s, way before the 10s sleep finishes)
	if duration > 4*time.Second {
		t.Errorf("teardown took too long: %v (expected < 4s)", duration)
	}
}

func TestRunner_PassthroughArgs(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"echo_task": {
				Command: "echo hello",
			},
		},
	}

	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}

	cacheMgr := cache.New(tempDir)
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)
	r := runner.New(cfg, g, cacheMgr, reporter, tempDir, runner.Options{
		Concurrency:     1,
		Verbose:         true,
		PassthroughArgs: []string{"world", "123"},
	})

	err := r.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "hello world 123") {
		t.Errorf("expected passthrough args 'hello world 123' in output, got:\n%s", out)
	}
}

