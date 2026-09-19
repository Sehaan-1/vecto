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
	order  []string
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
		order:  make([]string, 0),
	}
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
			r.order = append(r.order, name)
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
	fmt.Fprintf(r.writer, "[-] %s ... running\n", name)
}

// TaskCompleted marks a task as successfully finished.
func (r *Reporter) TaskCompleted(name string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusSuccess
		t.Duration = duration
	}
	fmt.Fprintf(r.writer, "[✓] %s (%.2fs)\n", name, duration.Seconds())
}

// TaskCached marks a task as replayed from cache in 0.00s.
func (r *Reporter) TaskCached(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusCached
		t.Duration = 0
	}
	fmt.Fprintf(r.writer, "[⚡ CACHED] %s (0.00s)\n", name)
}

// TaskFailed marks a task as failed with an error and prints command output.
func (r *Reporter) TaskFailed(name string, err error, output []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tasks[name]; ok {
		t.Status = StatusFailed
		t.Err = err
	}
	fmt.Fprintf(r.writer, "[✗ FAILED] %s: %v\n", name, err)
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
	fmt.Fprintf(r.writer, "[○ SKIPPED] %s\n", name)
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
