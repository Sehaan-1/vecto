package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// TaskStatus defines the state of a task in the UI.
type TaskStatus string

const (
	StatusPending TaskStatus = "PENDING"
	StatusRunning TaskStatus = "RUNNING"
	StatusCached  TaskStatus = "CACHED"
	StatusSuccess TaskStatus = "SUCCESS"
	StatusFailed  TaskStatus = "FAILED"
	StatusSkipped TaskStatus = "SKIPPED"
)

// TaskState tracks state of a task in the dashboard.
type TaskState struct {
	Name      string
	Status    TaskStatus
	StartTime time.Time
	Duration  time.Duration
	Err       error
}

// Reporter coordinates thread-safe terminal reporting.
type Reporter struct {
	mu     sync.Mutex
	writer io.Writer
	isTTY  bool
	tasks  map[string]*TaskState
}

// NewReporter creates a new Reporter.
func NewReporter(w io.Writer, isTTY bool) *Reporter {
	if w == nil {
		w = os.Stdout
	}
	return &Reporter{
		writer: w,
		isTTY:  isTTY,
		tasks:  make(map[string]*TaskState),
	}
}

// Writer returns the underlying io.Writer.
func (r *Reporter) Writer() io.Writer {
	return r.writer
}

// RegisterTasks initializes task ordering.
func (r *Reporter) RegisterTasks(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, name := range names {
		if _, exists := r.tasks[name]; !exists {
			r.tasks[name] = &TaskState{
				Name:   name,
				Status: StatusPending,
			}
		}
	}
}

// TaskStarted marks a task as currently running.
func (r *Reporter) TaskStarted(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusRunning
		t.StartTime = time.Now()
	}
	prefix := "[-]"
	if r.isTTY {
		prefix = "\033[36m[-]\033[0m"
	}
	fmt.Fprintf(r.writer, "%s %s ... running\n", prefix, name)
}

// TaskCompleted marks a task as successfully finished.
func (r *Reporter) TaskCompleted(name string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusSuccess
		t.Duration = duration
	}
	prefix := "[✓]"
	if r.isTTY {
		prefix = "\033[32m[✓]\033[0m"
	}
	fmt.Fprintf(r.writer, "%s %s (%.2fs)\n", prefix, name, duration.Seconds())
}

// TaskCached marks a task as replayed from cache.
func (r *Reporter) TaskCached(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusCached
		t.Duration = 0
	}
	prefix := "[⚡ CACHED]"
	if r.isTTY {
		prefix = "\033[33m[⚡ CACHED]\033[0m"
	}
	fmt.Fprintf(r.writer, "%s %s\n", prefix, name)
}

// TaskOutput outputs detailed command logs (e.g. for verbose mode).
func (r *Reporter) TaskOutput(name string, output []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(output) > 0 {
		fmt.Fprintf(r.writer, "--- Output: %s ---\n%s--------------------\n", name, string(output))
	}
}

// TaskFailed marks a task as failed with an error and prints command output.
func (r *Reporter) TaskFailed(name string, err error, output []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusFailed
		t.Err = err
	}
	prefix := "[✗ FAILED]"
	if r.isTTY {
		prefix = "\033[31m[✗ FAILED]\033[0m"
	}
	fmt.Fprintf(r.writer, "%s %s: %v\n", prefix, name, err)
	if len(output) > 0 {
		fmt.Fprintf(r.writer, "\n--- Output: %s ---\n%s--------------------\n\n", name, string(output))
	}
}

// TaskSkipped marks a task as skipped due to upstream failure.
func (r *Reporter) TaskSkipped(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusSkipped
	}
	prefix := "[○ SKIPPED]"
	if r.isTTY {
		prefix = "\033[90m[○ SKIPPED]\033[0m"
	}
	fmt.Fprintf(r.writer, "%s %s\n", prefix, name)
}

// Summary prints total execution summary.
func (r *Reporter) Summary(totalDuration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var success, cached, failed, skipped int
	for _, t := range r.tasks {
		switch t.Status {
		case StatusSuccess:
			success++
		case StatusCached:
			cached++
		case StatusFailed:
			failed++
		case StatusSkipped:
			skipped++
		}
	}

	fmt.Fprintf(r.writer, "\n--- Summary ---\n")
	fmt.Fprintf(r.writer, "Tasks:    %d total (%d cached, %d executed, %d failed, %d skipped)\n",
		len(r.tasks), cached, success, failed, skipped)
	fmt.Fprintf(r.writer, "Duration: %.2fs\n", totalDuration.Seconds())
}
