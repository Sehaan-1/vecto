package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
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
	Concurrency int
	KeepGoing   bool
	Force       bool
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

// Run executes the requested tasks and their dependencies.
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

	layers, err := r.Graph.ExecutionLayers()
	if err != nil {
		return fmt.Errorf("resolving execution graph: %w", err)
	}

	allTasks := r.Graph.Tasks()
	r.Reporter.RegisterTasks(allTasks)

	// Context with cancellation on failure
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	failedTasks := make(map[string]error)
	var mu sync.Mutex

	for _, layer := range layers {
		// Filter layer to tasks needed for target execution
		var activeInLayer []string
		for _, taskName := range layer {
			if r.isNeeded(taskName, targetTasks) {
				activeInLayer = append(activeInLayer, taskName)
			}
		}

		if len(activeInLayer) == 0 {
			continue
		}

		// Check if any dependencies of this layer failed
		var toRun []string
		for _, taskName := range activeInLayer {
			mu.Lock()
			hasFailedDep := false
			for _, dep := range r.Config.Tasks[taskName].Dependencies {
				if _, failed := failedTasks[dep]; failed {
					hasFailedDep = true
					break
				}
			}
			mu.Unlock()

			if hasFailedDep {
				r.Reporter.TaskSkipped(taskName)
			} else {
				toRun = append(toRun, taskName)
			}
		}

		if len(toRun) == 0 {
			continue
		}

		// Execute toRun concurrently with bounded semaphore
		sem := make(chan struct{}, r.Opts.Concurrency)
		var wg sync.WaitGroup

		for _, taskName := range toRun {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				// If context was cancelled by a sibling and not keep-going, skip
				if runCtx.Err() != nil && !r.Opts.KeepGoing {
					r.Reporter.TaskSkipped(name)
					return
				}

				taskCfg := r.Config.Tasks[name]

				// Compute cache fingerprint
				fingerprint, err := hash.ComputeTaskFingerprint(r.BaseDir, taskCfg.Command, taskCfg.Inputs, taskCfg.Env)
				if err != nil {
					mu.Lock()
					failedTasks[name] = err
					mu.Unlock()
					r.Reporter.TaskFailed(name, fmt.Errorf("fingerprint error: %w", err))
					if !r.Opts.KeepGoing {
						cancel()
					}
					return
				}

				// Check cache hit
				if !r.Opts.Force && r.Cache.Has(fingerprint) {
					_, _, err := r.Cache.Restore(fingerprint)
					if err == nil {
						r.Reporter.TaskCached(name)
						return
					}
				}

				// Execute task
				r.Reporter.TaskStarted(name)
				taskStart := time.Now()

				output, execErr := r.executeCommand(runCtx, taskCfg.Command)
				duration := time.Since(taskStart)

				if execErr != nil {
					mu.Lock()
					failedTasks[name] = execErr
					mu.Unlock()
					r.Reporter.TaskFailed(name, execErr)
					if !r.Opts.KeepGoing {
						cancel()
					}
					return
				}

				// Cache successful result
				_ = r.Cache.Store(fingerprint, name, 0, duration, output, taskCfg.Outputs)
				r.Reporter.TaskCompleted(name, duration)
			}(taskName)
		}

		wg.Wait()

		if len(failedTasks) > 0 && !r.Opts.KeepGoing {
			break
		}
	}

	r.Reporter.Summary(time.Since(startTime))

	if len(failedTasks) > 0 {
		return fmt.Errorf("%d task(s) failed during execution", len(failedTasks))
	}

	return nil
}

func (r *Runner) isNeeded(taskName string, targets []string) bool {
	for _, target := range targets {
		if taskName == target {
			return true
		}
		// Or if target depends on taskName
		if r.dependsOn(target, taskName) {
			return true
		}
	}
	return false
}

func (r *Runner) dependsOn(target, candidate string) bool {
	deps := r.Config.Tasks[target].Dependencies
	for _, dep := range deps {
		if dep == candidate || r.dependsOn(dep, candidate) {
			return true
		}
	}
	return false
}

func (r *Runner) executeCommand(ctx context.Context, cmdStr string) ([]byte, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	cmd.Dir = r.BaseDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	return buf.Bytes(), err
}
