package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/dag"
	"github.com/Sehaan-1/vecto/internal/hash"
	"github.com/Sehaan-1/vecto/internal/ui"
)

// Options configures the runner execution behavior.
type Options struct {
	Concurrency     int
	KeepGoing       bool
	Force           bool
	Verbose         bool
	PassthroughArgs []string
}

// Runner coordinates concurrent task scheduling and execution.
type Runner struct {
	Config   *config.Config
	Graph    *dag.Graph
	Cache    *cache.Manager
	Reporter *ui.Reporter
	BaseDir  string
	Opts     Options
}

// New creates a new Runner instance.
func New(cfg *config.Config, g *dag.Graph, c *cache.Manager, r *ui.Reporter, baseDir string, opts Options) *Runner {
	if opts.Concurrency <= 0 {
		opts.Concurrency = runtime.NumCPU()
	}
	return &Runner{
		Config:   cfg,
		Graph:    g,
		Cache:    c,
		Reporter: r,
		BaseDir:  baseDir,
		Opts:     opts,
	}
}

// Run executes the requested tasks and their dependencies using a reactive,
// event-driven scheduler. A task is dispatched the instant its last
// dependency completes — there is no layer-level head-of-line blocking.
func (r *Runner) Run(ctx context.Context, targetTasks []string) error {
	startTime := time.Now()

	// If no specific tasks specified, run all tasks
	if len(targetTasks) == 0 {
		targetTasks = r.Graph.Tasks()
	}

	// Filter and validate targets
	for _, t := range targetTasks {
		if !r.Graph.HasTask(t) {
			return fmt.Errorf("task %q is not defined in vecto.yaml", t)
		}
	}

	// Precompute needed tasks using graph ancestor closure
	needed := r.Graph.NeededTasks(targetTasks)
	targetSet := make(map[string]bool, len(targetTasks))
	for _, t := range targetTasks {
		targetSet[t] = true
	}

	// Register only needed tasks with reporter so summary counts are accurate
	neededSlice := make([]string, 0, len(needed))
	for t := range needed {
		neededSlice = append(neededSlice, t)
	}
	r.Reporter.RegisterTasks(neededSlice)

	// Context with cancellation on failure
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// pendingDeps[task] = number of needed dependencies not yet resolved.
	// A task becomes "ready" when this counter reaches zero.
	pendingDeps := make(map[string]int, len(needed))
	for task := range needed {
		for _, dep := range r.Config.Tasks[task].Dependencies {
			if needed[dep] {
				pendingDeps[task]++
			}
		}
	}

	// ready is buffered to len(needed) so goroutines can push without blocking.
	ready := make(chan string, len(needed))

	// Seed the queue with all zero-dep tasks, in sorted order for determinism.
	seeds := make([]string, 0, len(needed))
	for task := range needed {
		if pendingDeps[task] == 0 {
			seeds = append(seeds, task)
		}
	}
	sort.Strings(seeds)
	for _, t := range seeds {
		ready <- t
	}

	failedTasks := make(map[string]error)
	unmetTasks := make(map[string]bool)
	taskFingerprints := make(map[string]string)
	var mu sync.Mutex

	sem := make(chan struct{}, r.Opts.Concurrency)
	var wg sync.WaitGroup

	// Drain exactly len(needed) tasks from the ready channel.
	// Goroutines push dependents to ready as they finish, so the channel
	// stays live until all tasks have been dispatched.
	for remaining := len(needed); remaining > 0; remaining-- {
		name := <-ready // blocks until a task's deps are met

		wg.Add(1)
		go func(n string) {
			defer wg.Done()

			// Determine skip status before acquiring a semaphore slot so
			// cancelled / unmet tasks don't consume worker capacity.
			mu.Lock()
			skip := false
			if unmetTasks[n] {
				skip = true
			} else {
				for _, dep := range r.Config.Tasks[n].Dependencies {
					if unmetTasks[dep] {
						unmetTasks[n] = true
						skip = true
						break
					}
				}
			}
			if !skip && runCtx.Err() != nil && !r.Opts.KeepGoing {
				unmetTasks[n] = true
				skip = true
			}
			mu.Unlock()

			if skip {
				r.Reporter.TaskSkipped(n)
			} else {
				// Acquire concurrency slot only for tasks that will actually run.
				sem <- struct{}{}
				r.runTask(runCtx, cancel, n, targetSet, &mu, failedTasks, unmetTasks, taskFingerprints)
				<-sem
			}

			// Push dependents whose dep-counter just hit zero to the ready queue.
			// This fires regardless of success/skip/fail so the chain always drains.
			mu.Lock()
			for _, dep := range r.Graph.Dependents(n) {
				if !needed[dep] {
					continue
				}
				pendingDeps[dep]--
				if pendingDeps[dep] == 0 {
					ready <- dep
				}
			}
			mu.Unlock()
		}(name)
	}

	wg.Wait()

	r.Reporter.Summary(time.Since(startTime))

	if len(failedTasks) > 0 {
		return fmt.Errorf("%d task(s) failed during execution", len(failedTasks))
	}
	return nil
}

// runTask executes a single task: computes its fingerprint, checks the cache,
// runs the command if needed, and stores the result.
func (r *Runner) runTask(
	runCtx context.Context,
	cancel context.CancelFunc,
	name string,
	targetSet map[string]bool,
	mu *sync.Mutex,
	failedTasks map[string]error,
	unmetTasks map[string]bool,
	taskFingerprints map[string]string,
) {
	taskCfg := r.Config.Tasks[name]

	cmdToRun := taskCfg.Command
	if len(r.Opts.PassthroughArgs) > 0 && (len(targetSet) == 0 || targetSet[name]) {
		cmdToRun = cmdToRun + " " + strings.Join(r.Opts.PassthroughArgs, " ")
	}

	// Collect transitive dependency fingerprints for cache key computation
	mu.Lock()
	depFingerprints := make(map[string]string, len(taskCfg.Dependencies))
	for _, dep := range taskCfg.Dependencies {
		if fp, ok := taskFingerprints[dep]; ok {
			depFingerprints[dep] = fp
		}
	}
	mu.Unlock()

	// Compute content-addressable fingerprint
	fingerprint, err := hash.ComputeTaskFingerprint(r.BaseDir, cmdToRun, taskCfg.Inputs, taskCfg.Env, depFingerprints)
	if err != nil {
		mu.Lock()
		failedTasks[name] = err
		unmetTasks[name] = true
		mu.Unlock()
		r.Reporter.TaskFailed(name, fmt.Errorf("fingerprint error: %w", err), nil)
		if !r.Opts.KeepGoing {
			cancel()
		}
		return
	}

	mu.Lock()
	taskFingerprints[name] = fingerprint
	mu.Unlock()

	// Cache hit: restore and return
	if !r.Opts.Force && r.Cache.Has(fingerprint) {
		_, logs, err := r.Cache.Restore(fingerprint)
		if err == nil {
			r.Reporter.TaskCached(name)
			if r.Opts.Verbose && len(logs) > 0 {
				r.Reporter.TaskOutput(name, logs)
			}
			return
		}
	}

	// Execute the task command
	r.Reporter.TaskStarted(name)
	taskStart := time.Now()

	output, execErr := r.executeCommand(runCtx, cmdToRun)
	duration := time.Since(taskStart)

	if execErr != nil {
		mu.Lock()
		failedTasks[name] = execErr
		unmetTasks[name] = true
		mu.Unlock()
		r.Reporter.TaskFailed(name, execErr, output)
		if !r.Opts.KeepGoing {
			cancel()
		}
		return
	}

	// Store successful result; warn and continue on store failure
	if storeErr := r.Cache.Store(fingerprint, name, 0, duration, output, taskCfg.Outputs); storeErr != nil {
		fmt.Fprintf(r.Reporter.Writer(), "warning: failed to cache task %s: %v\n", name, storeErr)
	}

	r.Reporter.TaskCompleted(name, duration)
	if r.Opts.Verbose && len(output) > 0 {
		r.Reporter.TaskOutput(name, output)
	}
}

// executeCommand runs cmdStr in a subprocess with process-group isolation.
// setProcAttrs (platform-specific) places the child in its own process group
// so that terminateProcessGroup can cleanly signal or terminate all descendants.
func (r *Runner) executeCommand(ctx context.Context, cmdStr string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	cmd.Dir = r.BaseDir
	cmd.WaitDelay = 1 * time.Second
	setProcAttrs(cmd) // platform-specific process group setup

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Two-phase graceful teardown when context is cancelled:
	// 1. Sends SIGTERM to entire process group (-pgid) allowing child processes
	//    to flush disk buffers and release socket handles.
	// 2. Waits up to 1 second grace period, then escalates to SIGKILL if still alive.
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			terminateProcessGroup(cmd, 1*time.Second, watchDone)
		case <-watchDone:
		}
	}()

	err := cmd.Wait()
	close(watchDone)

	// Surface context cancellation as the canonical error
	if ctx.Err() != nil {
		return buf.Bytes(), ctx.Err()
	}
	return buf.Bytes(), err
}
