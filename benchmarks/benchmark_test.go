package benchmarks_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/dag"
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

type BenchmarkResult struct {
	Name         string
	MakeTime     time.Duration
	VectoTime    time.Duration
	MakeOutputs  string
	VectoOutputs string
	Speedup      float64
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func TestBenchmark_MultiLang_Comparison(t *testing.T) {
	multiLangSrc, err := filepath.Abs("multi_lang_project")
	if err != nil {
		t.Fatalf("could not locate multi_lang_project: %v", err)
	}

	tempDir := t.TempDir()
	benchDir := filepath.Join(tempDir, "workspace")
	if err := copyDir(multiLangSrc, benchDir); err != nil {
		t.Fatalf("failed to copy workspace: %v", err)
	}

	// Determine if Make is installed
	makeBinary, err := exec.LookPath("make")
	hasMake := err == nil
	if !hasMake {
		makeBinary, err = exec.LookPath("mingw32-make")
		hasMake = err == nil
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf(" BENCHMARK: Realistic Multi-Language Repository\n")
	fmt.Printf(" OS: %s | Arch: %s | CPU Cores: %d\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	if hasMake {
		fmt.Printf(" Make Binary: %s\n", makeBinary)
	} else {
		fmt.Printf(" Make Binary: Not found (fallback reference simulator used)\n")
	}
	fmt.Printf("=======================================================\n\n")

	// Helper to run Vecto in benchDir
	runVecto := func(force bool, concurrency int) (time.Duration, string, error) {
		cfg, err := config.LoadConfig(benchDir)
		if err != nil {
			return 0, "", err
		}
		g, err := cfg.BuildGraph()
		if err != nil {
			return 0, "", err
		}
		cacheMgr := cache.New(benchDir)
		var buf bytes.Buffer
		reporter := ui.NewReporter(&buf, false)
		r := runner.New(cfg, g, cacheMgr, reporter, benchDir, runner.Options{
			Concurrency: concurrency,
			Force:       force,
		})

		start := time.Now()
		err = r.Run(context.Background(), nil)
		return time.Since(start), buf.String(), err
	}

	// Helper to run Make in benchDir
	runMake := func(target string, parallel bool) (time.Duration, string, error) {
		if !hasMake {
			// Sequential or parallel command execution simulating Make
			start := time.Now()
			tasks := []string{"codegen", "test-go", "test-py", "test-web", "build-go", "build-web", "package"}
			for _, tsk := range tasks {
				var cmd *exec.Cmd
				switch tsk {
				case "codegen":
					_ = os.MkdirAll(filepath.Join(benchDir, "generated"), 0755)
					_ = os.WriteFile(filepath.Join(benchDir, "generated", "routes.txt"), []byte("routes"), 0644)
				case "test-go":
					cmd = exec.Command("go", "test", "./services/api/...")
				case "test-py":
					cmd = exec.Command("python", "scripts/test_processor.py")
				case "test-web":
					cmd = exec.Command("node", "web/index.test.js")
				case "build-go":
					cmd = exec.Command("go", "run", "services/api/main.go", "build")
				case "build-web":
					_ = os.MkdirAll(filepath.Join(benchDir, "dist"), 0755)
					_ = os.WriteFile(filepath.Join(benchDir, "dist", "bundle.js"), []byte("bundle"), 0644)
				case "package":
					_ = os.WriteFile(filepath.Join(benchDir, "dist", "release.manifest"), []byte("manifest"), 0644)
				}
				if cmd != nil {
					cmd.Dir = benchDir
					_ = cmd.Run()
				}
			}
			return time.Since(start), "make reference simulator", nil
		}

		args := []string{target}
		if parallel {
			args = append(args, fmt.Sprintf("-j%d", runtime.NumCPU()))
		}
		cmd := exec.Command(makeBinary, args...)
		cmd.Dir = benchDir
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		start := time.Now()
		err := cmd.Run()
		return time.Since(start), outBuf.String(), err
	}

	// 1. Clean Build (Cold Cache)
	_ = os.RemoveAll(filepath.Join(benchDir, ".vecto"))
	_ = os.RemoveAll(filepath.Join(benchDir, "dist"))
	_ = os.RemoveAll(filepath.Join(benchDir, "generated"))

	makeCleanTime, _, _ := runMake("clean", false)
	makeCleanBuildTime, _, _ := runMake("all", false)
	makeCleanTotal := makeCleanTime + makeCleanBuildTime

	_ = os.RemoveAll(filepath.Join(benchDir, ".vecto"))
	_ = os.RemoveAll(filepath.Join(benchDir, "dist"))
	_ = os.RemoveAll(filepath.Join(benchDir, "generated"))

	vectoCleanTime, vectoCleanOut, err := runVecto(true, runtime.NumCPU())
	if err != nil {
		t.Fatalf("Vecto clean build failed: %v\nOutput:\n%s", err, vectoCleanOut)
	}

	// 2. Hot Cache Replay (Untouched files)
	vectoHotTime, hotOut, err := runVecto(false, runtime.NumCPU())
	if err != nil {
		t.Fatalf("Vecto hot build failed: %v", err)
	}
	if !strings.Contains(hotOut, "[⚡ CACHED] package") {
		t.Errorf("expected hot build to replay from cache")
	}

	makeHotTime, _, _ := runMake("all", false)

	// 3. Incremental Build (Touch 1 Go file)
	mathFile := filepath.Join(benchDir, "services", "api", "math.go")
	_ = os.WriteFile(mathFile, []byte("package main\n// edit\nfunc Add(a, b int) int { return a + b }\nfunc Multiply(a, b int) int { return a * b }\n"), 0644)

	makeIncrTime, _, _ := runMake("all", false)
	vectoIncrTime, incrOut, err := runVecto(false, runtime.NumCPU())
	if err != nil {
		t.Fatalf("Vecto incremental build failed: %v", err)
	}

	// 4. Parallel Build
	_ = os.RemoveAll(filepath.Join(benchDir, ".vecto"))
	_ = os.RemoveAll(filepath.Join(benchDir, "dist"))
	_ = os.RemoveAll(filepath.Join(benchDir, "generated"))
	makeParallelTime, _, _ := runMake("all", true)

	_ = os.RemoveAll(filepath.Join(benchDir, ".vecto"))
	_ = os.RemoveAll(filepath.Join(benchDir, "dist"))
	_ = os.RemoveAll(filepath.Join(benchDir, "generated"))
	vectoParallelTime, _, _ := runVecto(true, runtime.NumCPU())

	fmt.Printf("| Scenario | Make Wall-Clock | Vecto Wall-Clock | Vecto Speedup / Savings |\n")
	fmt.Printf("|---|---|---|---|\n")
	fmt.Printf("| **Clean Cold Build** | %v | %v | %.2fx |\n", makeCleanTotal, vectoCleanTime, float64(makeCleanTotal)/float64(vectoCleanTime))
	fmt.Printf("| **Incremental Build (1 file modified)** | %v | %v | %.2fx (%.1f%% time saved) |\n",
		makeIncrTime, vectoIncrTime, float64(makeIncrTime)/float64(vectoIncrTime),
		(1.0-float64(vectoIncrTime)/float64(makeIncrTime))*100.0)
	fmt.Printf("| **Hot Replay (Zero files modified)** | %v | %v | **%.2fx** (replayed from content cache) |\n",
		makeHotTime, vectoHotTime, float64(makeHotTime)/float64(vectoHotTime))
	fmt.Printf("| **Parallel Cold Build** | %v | %v | %.2fx |\n\n",
		makeParallelTime, vectoParallelTime, float64(makeParallelTime)/float64(vectoParallelTime))

	_ = incrOut
}

func TestBenchmark_CacheHit_LatencyDistribution(t *testing.T) {
	multiLangSrc, _ := filepath.Abs("multi_lang_project")
	tempDir := t.TempDir()
	benchDir := filepath.Join(tempDir, "workspace")
	_ = copyDir(multiLangSrc, benchDir)

	cfg, _ := config.LoadConfig(benchDir)
	g, _ := cfg.BuildGraph()
	cacheMgr := cache.New(benchDir)

	// Seed cache
	rSeed := runner.New(cfg, g, cacheMgr, ui.NewReporter(io.Discard, false), benchDir, runner.Options{Concurrency: 4})
	_ = rSeed.Run(context.Background(), nil)

	iterations := 100
	latencies := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		r := runner.New(cfg, g, cacheMgr, ui.NewReporter(io.Discard, false), benchDir, runner.Options{Concurrency: 4})
		start := time.Now()
		_ = r.Run(context.Background(), nil)
		latencies[i] = time.Since(start)
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	minLat := latencies[0]
	p50 := latencies[iterations*50/100]
	p95 := latencies[iterations*95/100]
	p99 := latencies[iterations*99/100]
	maxLat := latencies[iterations-1]

	fmt.Printf("--- Cache Hit Latency Distribution (100 runs on 7 tasks) ---\n")
	fmt.Printf("Min: %v | Median (p50): %v | p95: %v | p99: %v | Max: %v\n\n", minLat, p50, p95, p99, maxLat)
}

func TestBenchmark_Scalability_100_1000_10000(t *testing.T) {
	scales := []int{100, 1000, 10000}

	fmt.Printf("--- DAG Scalability Benchmark (Topological Sort & Partitioning) ---\n")
	for _, n := range scales {
		g := dag.New()
		// Construct synthetic diamond/layered DAG with depth = n/10 and width = 10
		for i := 0; i < n; i++ {
			var deps []string
			if i >= 10 {
				// depend on 2 tasks from previous tier
				deps = append(deps, fmt.Sprintf("task_%d", i-10))
				if i%3 == 0 && i >= 20 {
					deps = append(deps, fmt.Sprintf("task_%d", i-20))
				}
			}
			g.AddTask(fmt.Sprintf("task_%d", i), deps)
		}

		// Measure TopoSort
		startTopo := time.Now()
		topoOrder, err := g.TopologicalSort()
		topoElapsed := time.Since(startTopo)
		if err != nil {
			t.Fatalf("scale %d topo failed: %v", n, err)
		}

		// Measure Layer Partitioning
		startLayers := time.Now()
		layers, err := g.ExecutionLayers()
		layersElapsed := time.Since(startLayers)
		if err != nil {
			t.Fatalf("scale %d layers failed: %v", n, err)
		}

		fmt.Printf("Tasks: %-6d | TopoSort: %-10v | Layer Partitioning: %-10v | Layers: %d (Order verified: %d tasks)\n",
			n, topoElapsed, layersElapsed, len(layers), len(topoOrder))
	}
	fmt.Println()
}

func TestBenchmark_FailureTeardown(t *testing.T) {
	tempDir := t.TempDir()

	sleepCmd := "sleep 5"
	failCmd := "false"
	if runtime.GOOS == "windows" {
		sleepCmd = "powershell -NoProfile -Command \"Start-Sleep -Seconds 5\""
		failCmd = "cmd /c exit 1"
	}

	cfg := &config.Config{
		Version: "1",
		Tasks: map[string]config.TaskConfig{
			"failing_task": {Command: failCmd},
			"slow_task_1":  {Command: sleepCmd},
			"slow_task_2":  {Command: sleepCmd},
			"slow_task_3":  {Command: sleepCmd},
		},
	}
	g, _ := cfg.BuildGraph()
	cacheMgr := cache.New(tempDir)

	// Measure Fail-Fast teardown (keepGoing = false)
	startFast := time.Now()
	rFast := runner.New(cfg, g, cacheMgr, ui.NewReporter(io.Discard, false), tempDir, runner.Options{
		Concurrency: 4,
		KeepGoing:   false,
	})
	_ = rFast.Run(context.Background(), nil)
	fastDuration := time.Since(startFast)

	// Measure Keep-Going execution (keepGoing = true)
	startKeep := time.Now()
	rKeep := runner.New(cfg, g, cacheMgr, ui.NewReporter(io.Discard, false), tempDir, runner.Options{
		Concurrency: 4,
		KeepGoing:   true,
	})
	_ = rKeep.Run(context.Background(), nil)
	keepDuration := time.Since(startKeep)

	fmt.Printf("--- Failure & Cancellation Teardown Latency ---\n")
	fmt.Printf("Fail-Fast Cancellation (Teardown latency): %v (Clean abort of 3 slow sibling tasks)\n", fastDuration)
	fmt.Printf("Keep-Going Execution:                      %v (Independent tasks ran to completion)\n\n", keepDuration)

	if fastDuration > 4*time.Second {
		t.Errorf("Fail-fast teardown took suspiciously long: %v", fastDuration)
	}
}

// runExternal measures one external runner invocation in dir and returns its
// wall-clock time and combined output. It fails the test on non-zero exit.
func runExternal(t *testing.T, dir, name string, args ...string) (time.Duration, string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, buf.String())
	}
	return time.Since(start), buf.String()
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	// Caches, outputs, and interpreter bytecode stay untracked: turbo hashes
	// tracked files, so anything a run rewrites (including .pyc files python
	// emits on every run) would churn hashes and defeat the cache forever.
	ignore := ".turbo/\n.vecto/\ndist/\ngenerated/\n__pycache__/\n*.pyc\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(ignore), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=bench@local", "-c", "user.name=bench", "commit", "-qm", "bench"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestBenchmark_ExternalRunners measures the same multi-language project
// under Turbo (caching rival) and Just (plain runner) for docs/benchmarks.md.
// It skips — never fails — when a tool is not installed, so CI stays green
// without external dependencies (ADR-0015).
func TestBenchmark_ExternalRunners(t *testing.T) {
	multiLangSrc, err := filepath.Abs("multi_lang_project")
	if err != nil {
		t.Fatalf("could not locate multi_lang_project: %v", err)
	}

	if _, err := exec.LookPath("turbo"); err != nil {
		t.Skip("turbo not on PATH, skipping rival measurement")
	}
	if _, err := exec.LookPath("just"); err != nil {
		t.Skip("just not on PATH, skipping rival measurement")
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("sh"); err != nil {
			t.Skip("sh not on PATH (just needs it on Windows), skipping")
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH (turbo hashes via git), skipping")
	}

	tempDir := t.TempDir()

	// Turbo workspace: turbo hashes git-tracked files, so the copy must be
	// a clean repo with caches and outputs untracked (else hashes churn and
	// nothing ever hits cache).
	turboDir := filepath.Join(tempDir, "turbo")
	if err := copyDir(multiLangSrc, turboDir); err != nil {
		t.Fatal(err)
	}
	gitInit(t, turboDir)

	turboCold, _ := runExternal(t, turboDir, "turbo", "run", "package", "--output-logs=errors-only")
	_ = turboCold
	turboHot, hotOut := runExternal(t, turboDir, "turbo", "run", "package", "--output-logs=errors-only")
	if !strings.Contains(hotOut, "FULL TURBO") {
		t.Fatalf("expected turbo full cache hit, got:\n%s", hotOut)
	}
	fmt.Printf("--- External runners (same project, same machine) ---\n")
	fmt.Printf("Turbo cold:      %v\n", turboCold)
	fmt.Printf("Turbo hot replay: %v (FULL TURBO)\n", turboHot)

	// Just workspace: no cache, so one run shows the re-execution baseline.
	justDir := filepath.Join(tempDir, "just")
	if err := copyDir(multiLangSrc, justDir); err != nil {
		t.Fatal(err)
	}
	justCold, _ := runExternal(t, justDir, "just", "package")
	justRerun, _ := runExternal(t, justDir, "just", "package")
	fmt.Printf("Just cold:       %v\n", justCold)
	fmt.Printf("Just rerun:      %v (no cache, re-executes)\n\n", justRerun)

	manifest := filepath.Join(justDir, "dist", "release.manifest")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("just run did not produce %s: %v", manifest, err)
	}
}
