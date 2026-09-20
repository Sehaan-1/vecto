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
	"github.com/Sehaan-1/vecto/internal/fileindex"
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
	// FileIndex enables Merkle File Index (VFI) incremental fingerprinting
	// (ADR-0019). When false, the legacy per-task full-scan hashing path is
	// used.
	FileIndex bool
}

// Runner coordinates concurrent task scheduling and execution.
type Runner struct {
	Config   *config.Config
	Graph    *dag.Graph
	Cache    *cache.Manager
	Reporter *ui.Reporter
	BaseDir  string
	Opts     Options

	// fileIndex is populated at the start of Run when Opts.FileIndex is set
	// and the index loads + syncs cleanly; nil means legacy hashing.
	fileIndex *fileindex.Index
	// indexDirty marks that this run stored a new task fingerprint in the
	// index (memoization miss) — used to decide whether the index must be
	// persisted even when the tree root hash did not change.
	indexDirty bool
	// indexTreeUnchanged is true when the sync produced the same root hash
	// as the loaded index (0 stat-level changes in the tree).
	indexTreeUnchanged bool
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

	// ADR-0019: load + sync the Merkle file index once, before dispatch.
	// Every failure mode degrades to legacy full-scan hashing (the safe
	// direction): a corrupted index or sync error can never skip a task.
	if r.Opts.FileIndex {
		if ix, err := fileindex.Load(r.BaseDir); err == nil {
			preSyncRoot := ix.RootHash
			if rep, serr := ix.Sync(r.BaseDir, hash.LoadIgnorePatterns(r.BaseDir), r.Opts.Concurrency); serr == nil {
				r.fileIndex = ix
				r.indexTreeUnchanged = ix.RootHash == preSyncRoot
				if r.Opts.Verbose {
					fmt.Fprintf(r.Reporter.Writer(),
						"file index: %d files (%d re-hashed, %d trusted, %d dirs) in %s, root %s…\n",
						rep.FilesSeen, rep.FilesHashed, rep.FilesTrusted, rep.DirsSeen,
						rep.Elapsed.Round(time.Millisecond), ix.RootHash[:12])
				}
			} else {
				fmt.Fprintf(r.Reporter.Writer(),
					"warning: file index sync failed (%v); falling back to full-scan hashing\n", serr)
			}
		} else {
			fmt.Fprintf(r.Reporter.Writer(),
				"warning: file index unavailable (%v); falling back to full-scan hashing\n", err)
		}
	}

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

	// Persist the refreshed index (task fingerprints + stat tuples) so the
	// next run's sync starts warm. A 0-change run with no new task records
	// modifies nothing — skip the write entirely (the warm path is then one
	// read + one stat cascade, zero index writes). Keeping the stale stat
	// tuples is sound: any real change still shows up as a tuple mismatch,
	// and a same-second rewrite is caught by the racy rule, which compares
	// against the retained (older) snapshot second. Best-effort: a failure
	// here only costs the next run a cold-ish sync.
	if r.fileIndex != nil && (!r.indexTreeUnchanged || r.indexDirty) {
		if err := r.fileIndex.Save(r.BaseDir); err != nil {
			fmt.Fprintf(r.Reporter.Writer(), "warning: saving file index: %v\n", err)
		}
	}

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

	// Compute content-addressable fingerprint: VFI coverage digest when the
	// Merkle file index is available, legacy full-scan hashing otherwise.
	var fingerprint string
	var err error
	if r.fileIndex != nil {
		fingerprint, err = r.indexedFingerprint(name, cmdToRun, taskCfg, depFingerprints, mu)
	} else {
		fingerprint, err = hash.ComputeTaskFingerprint(r.BaseDir, cmdToRun, taskCfg.Inputs, taskCfg.Env, depFingerprints)
	}
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

// indexedFingerprint computes a task fingerprint from the Merkle file index
// (ADR-0019). With task-level memoization: if the task definition, the root
// hash, and all dependency fingerprints are unchanged since the last run,
// the stored fingerprint is reused without recomputing coverage. The index
// is only ever a shortcut to the same deterministic digest — it can never
// change what a fingerprint *is*.
func (r *Runner) indexedFingerprint(name, cmd string, taskCfg config.TaskConfig, depFingerprints map[string]string, mu *sync.Mutex) (string, error) {
	depNames := make([]string, 0, len(taskCfg.Dependencies))
	for _, d := range taskCfg.Dependencies {
		depNames = append(depNames, d)
	}
	def := fileindex.DefHash(cmd, taskCfg.Inputs, taskCfg.Env, depNames)

	mu.Lock()
	rec := r.fileIndex.Tasks[name]
	root := r.fileIndex.RootHash
	mu.Unlock()

	if rec != nil && rec.Def == def && rec.Root == root && fileindex.DepsEqual(rec.Deps, depFingerprints) {
		return rec.Fp, nil
	}
	// Recomputing → a fresh record will be stored: mark the index dirty so
	// it is persisted even when the tree root hash did not change (the new
	// record is what makes the next run O(1) for this task).
	r.indexDirty = true

	coverage, _, err := r.fileIndex.Coverage(taskCfg.Inputs)
	if err != nil {
		return "", err
	}
	fp := hash.FingerprintFromCoverage(cmd, taskCfg.Env, depFingerprints, coverage)

	mu.Lock()
	r.fileIndex.Tasks[name] = &fileindex.TaskRec{Def: def, Fp: fp, Root: root, Deps: depFingerprints}
	mu.Unlock()
	return fp, nil
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
