package ui_test

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/ui"
)

func TestUI_ConcurrentReporting(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)

	tasks := []string{"lint", "test", "build", "deploy"}
	reporter.RegisterTasks(tasks)

	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			reporter.TaskStarted(name)
			time.Sleep(10 * time.Millisecond)
			if name == "test" {
				reporter.TaskCached(name)
			} else {
				reporter.TaskCompleted(name, 10*time.Millisecond)
			}
		}(task)
	}

	wg.Wait()
	reporter.Summary(50 * time.Millisecond)

	output := buf.String()
	for _, task := range tasks {
		if !strings.Contains(output, task) {
			t.Errorf("expected output to contain task %s, got:\n%s", task, output)
		}
	}
	if !strings.Contains(output, "CACHED") {
		t.Errorf("expected CACHED in output, got:\n%s", output)
	}
}

func TestUI_TaskFailedShowsOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, false)

	reporter.RegisterTasks([]string{"compile"})
	reporter.TaskStarted("compile")
	compilerOutput := []byte("main.go:14: syntax error: unexpected newline")
	reporter.TaskFailed("compile", errors.New("exit status 1"), compilerOutput)

	out := buf.String()
	if !strings.Contains(out, "syntax error: unexpected newline") {
		t.Errorf("expected compiler error output in UI, got:\n%s", out)
	}
}

func TestUI_TTYColorOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	reporter := ui.NewReporter(buf, true)

	reporter.RegisterTasks([]string{"build"})
	reporter.TaskStarted("build")
	reporter.TaskCompleted("build", 50*time.Millisecond)
	reporter.TaskCached("build")
	reporter.TaskSkipped("build")
	reporter.TaskFailed("build", errors.New("boom"), nil)

	out := buf.String()
	// Check for ANSI color escape sequences
	if !strings.Contains(out, "\033[32m[✓]\033[0m") {
		t.Errorf("expected green [✓] ANSI escape in TTY mode, got:\n%s", out)
	}
	if !strings.Contains(out, "\033[33m[⚡ CACHED]\033[0m") {
		t.Errorf("expected yellow [⚡ CACHED] ANSI escape in TTY mode, got:\n%s", out)
	}
	if !strings.Contains(out, "\033[31m[✗ FAILED]\033[0m") {
		t.Errorf("expected red [✗ FAILED] ANSI escape in TTY mode, got:\n%s", out)
	}
}
