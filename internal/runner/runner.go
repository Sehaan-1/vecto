package runner

import (
	"context"
	"time"
)

// TaskResult holds the execution result of a single task.
type TaskResult struct {
	TaskName string
	Duration time.Duration
	Cached   bool
	Err      error
}

// Runner coordinates concurrent task execution over a DAG.
type Runner struct {
	Concurrency int
	KeepGoing   bool
}

// New creates a new task Runner.
func New(concurrency int, keepGoing bool) *Runner {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Runner{
		Concurrency: concurrency,
		KeepGoing:   keepGoing,
	}
}
