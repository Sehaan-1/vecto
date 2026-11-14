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
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

// runOnce executes the full task graph from cfg against the scratch dir and
// returns the reporter output for the run.
func runOnce(t *testing.T, dir string, cfg *config.Config) string {
	t.Helper()
	g := dag.New()
	for name, task := range cfg.Tasks {
		g.AddTask(name, task.Dependencies)
	}
	var buf bytes.Buffer
	r := runner.New(cfg, g, cache.New(dir),
		ui.NewReporter(&buf, false), dir, runner.Options{Concurrency: 4})
	if err := r.Run(context.Background(), nil); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	return buf.String()
}

// TestCacheCorrectnessMatrix pins the cache invalidation contract end to end:
// which mutations of task inputs, commands, env values, upstream outputs, or
// workspace files must invalidate a task, and which must keep a warm cache hit.
func TestCacheCorrectnessMatrix(t *testing.T) {
	tests := []struct {
		name string
		// scenario runs the cold -> mutate -> hot sequence and asserts on the
		// second run's report.
		scenario func(t *testing.T, dir string, cfg *config.Config)
	}{
		{
			name: "hot_replay",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command:  "echo one > alpha.out",
					Inputs:   []string{"alpha.txt"},
					Outputs:  []string{"alpha.out"},
				}
				if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("one\n"), 0o644); err != nil {
					t.Fatalf("hot_replay: writing alpha.txt: %v", err)
				}
				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") {
					t.Fatalf("hot_replay: expected first run to execute alpha, got:\n%s", out)
				}
				out := runOnce(t, dir, cfg)
				if !strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("hot_replay: expected second run to replay from cache, got:\n%s", out)
				}
			},
		},
		{
			name: "unrelated_file",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command:  "echo one > alpha.out",
					Inputs:   []string{"alpha.txt"},
					Outputs:  []string{"alpha.out"},
				}
				if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("one\n"), 0o644); err != nil {
					t.Fatalf("unrelated_file: writing alpha.txt: %v", err)
				}
				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") {
					t.Fatalf("unrelated_file: expected first run to execute alpha, got:\n%s", out)
				}
				if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("unrelated\n"), 0o644); err != nil {
					t.Fatalf("unrelated_file: writing notes.txt: %v", err)
				}
				out := runOnce(t, dir, cfg)
				if !strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("unrelated_file: expected second run to stay cached despite notes.txt, got:\n%s", out)
				}
			},
		},
		{
			name: "input_content",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command:  "echo one > alpha.out",
					Inputs:   []string{"alpha.txt"},
					Outputs:  []string{"alpha.out"},
				}
				if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("one\n"), 0o644); err != nil {
					t.Fatalf("input_content: writing alpha.txt: %v", err)
				}
				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") {
					t.Fatalf("input_content: expected first run to execute alpha, got:\n%s", out)
				}
				if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("two\n"), 0o644); err != nil {
					t.Fatalf("input_content: rewriting alpha.txt: %v", err)
				}
				out := runOnce(t, dir, cfg)
				if strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("input_content: expected content change to invalidate cache, got cached:\n%s", out)
				}
			},
		},
		{
			name: "command_change",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command:  "echo one > alpha.out",
					Outputs:  []string{"alpha.out"},
				}
				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") {
					t.Fatalf("command_change: expected first run to execute alpha, got:\n%s", out)
				}
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command: "echo two > alpha.out",
					Outputs: []string{"alpha.out"},
				}
				out := runOnce(t, dir, cfg)
				if strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("command_change: expected command change to invalidate cache, got cached:\n%s", out)
				}
			},
		},
		{
			name: "env_value",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command: "echo env",
					Env:    []string{"VECTO_TEST_ENV"},
				}
				t.Setenv("VECTO_TEST_ENV", "one")
				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") {
					t.Fatalf("env_value: expected first run to execute alpha, got:\n%s", out)
				}
				t.Setenv("VECTO_TEST_ENV", "two")
				out := runOnce(t, dir, cfg)
				if strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("env_value: expected env value change to invalidate cache, got cached:\n%s", out)
				}
			},
		},
		{
			name: "upstream_change",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{Command: "echo alpha-v1"}
				cfg.Tasks["beta"] = config.TaskConfig{
					Command:      "echo beta-v1",
					Dependencies: []string{"alpha"},
				}
				cfg.Tasks["gamma"] = config.TaskConfig{Command: "echo gamma-v1"}

				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") || !strings.Contains(out, "[✓] beta") || !strings.Contains(out, "[✓] gamma") {
					t.Fatalf("upstream_change: expected first run to execute all tasks, got:\n%s", out)
				}

				cfg.Tasks["alpha"] = config.TaskConfig{Command: "echo alpha-v2"}
				out := runOnce(t, dir, cfg)
				if strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("upstream_change: expected alpha to re-execute after command change, got cached:\n%s", out)
				}
				if strings.Contains(out, "[⚡ CACHED] beta") {
					t.Fatalf("upstream_change: expected beta to re-execute after upstream alpha change, got cached:\n%s", out)
				}
				if !strings.Contains(out, "[⚡ CACHED] gamma") {
					t.Fatalf("upstream_change: expected independent gamma to stay cached, got:\n%s", out)
				}
			},
		},
		{
			name: "output_restore",
			scenario: func(t *testing.T, dir string, cfg *config.Config) {
				cfg.Tasks["alpha"] = config.TaskConfig{
					Command:  "echo one > alpha.out",
					Outputs:  []string{"alpha.out"},
				}
				if out := runOnce(t, dir, cfg); !strings.Contains(out, "[✓] alpha") {
					t.Fatalf("output_restore: expected first run to execute alpha, got:\n%s", out)
				}
				if err := os.Remove(filepath.Join(dir, "alpha.out")); err != nil {
					t.Fatalf("output_restore: removing alpha.out: %v", err)
				}
				out := runOnce(t, dir, cfg)
				if !strings.Contains(out, "[⚡ CACHED] alpha") {
					t.Fatalf("output_restore: expected second run to hit cache and restore alpha.out, got:\n%s", out)
				}
				if _, err := os.Stat(filepath.Join(dir, "alpha.out")); err != nil {
					t.Fatalf("output_restore: expected alpha.out to be restored from cache, got error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &config.Config{Version: "1", Tasks: map[string]config.TaskConfig{}}
			tt.scenario(t, dir, cfg)
		})
	}
}
