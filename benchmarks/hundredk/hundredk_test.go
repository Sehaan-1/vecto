// Package hundredk is the ADR-0019 proof harness: a generated large
// repository (100,000 files by default) on which Vecto's legacy full-scan
// hashing, Vecto's Merkle File Index (VFI), and Turborepo 2.x (git and
// non-git modes) are measured on identical workloads, per ADR-0015's
// honesty rules (same machine, same files, documented method, reproducible
// command).
//
// The full 100k matrix is expensive, so it is gated behind
// VECTO_HUNDREDK=1. Without the variable the test runs a lightweight
// 500-file smoke matrix (no external tools) to keep CI fast.
package hundredk

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
	"github.com/Sehaan-1/vecto/internal/config"
	"github.com/Sehaan-1/vecto/internal/runner"
	"github.com/Sehaan-1/vecto/internal/ui"
)

const (
	heavyPackages  = 500 // 500 × 200 = 100,000 files
	heavyPerPkg    = 200
	lightPackages  = 25 // 25 × 20 = 500 files (CI smoke)
	lightPerPkg    = 20
	vectoTaskNames = "lint test build"
)

type row struct {
	label string
	dur   time.Duration
	note  string
}

// generateTree writes packages/pNNN/fMM.txt with deterministic content.
func generateTree(t *testing.T, dir string, packages, perPkg int) {
	t.Helper()
	for p := 0; p < packages; p++ {
		pkgDir := filepath.Join(dir, "packages", fmt.Sprintf("p%03d", p))
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < perPkg; f++ {
			body := fmt.Sprintf("package p%03d file %03d — deterministic payload line 1\nline 2 for p%03d/%03d\n", p, f, p, f)
			if err := os.WriteFile(filepath.Join(pkgDir, fmt.Sprintf("f%03d.txt", f)), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

const vectoManifest = `version: "1"
tasks:
  lint:
    command: "echo lint"
    inputs: ["packages/**"]

  test:
    command: "echo test"
    deps: ["lint"]
    inputs: ["packages/**"]

  build:
    command: "echo build"
    deps: ["test"]
    inputs: ["packages/**"]
    outputs: ["build-output.txt"]
`

const turboJSON = `{
  "tasks": {
    "lint":  { "inputs": ["packages/**"] },
    "test":  { "dependsOn": ["lint"], "inputs": ["packages/**"] },
    "build": { "dependsOn": ["test"], "inputs": ["packages/**"] }
  }
}
`

const packageJSON = `{
  "name": "hundredk",
  "packageManager": "npm@10.9.0",
  "version": "0.0.0",
  "private": true,
  "scripts": {
    "lint": "echo lint",
    "test": "echo test",
    "build": "echo build"
  }
}
`

// prepareProject writes a fresh project dir with the file tree + manifests.
func prepareProject(t *testing.T, packages, perPkg int, withTurbo bool) string {
	t.Helper()
	dir := t.TempDir()
	generateTree(t, dir, packages, perPkg)
	if err := os.WriteFile(filepath.Join(dir, "vecto.yaml"), []byte(vectoManifest), 0644); err != nil {
		t.Fatal(err)
	}
	// Predating every file by >1s guarantees the first sync's snapshot
	// second is strictly newer than every mtime second, so the "warm"
	// measurement is a pure stat cascade — not the one-time racy
	// re-hash that files created in the snapshot's own second would
	// legitimately trigger (git's exact racy rule, ADR-0019).
	time.Sleep(1100 * time.Millisecond)
	if withTurbo {
		if err := os.WriteFile(filepath.Join(dir, "turbo.json"), []byte(turboJSON), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runVectoScenario runs the task graph `runs` times and returns the duration
// of every run. fileIndex selects VFI (true) vs legacy full-scan (false).
func runVectoScenario(t *testing.T, dir string, fileIndex bool, runs int) []time.Duration {
	t.Helper()
	cfg, err := config.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	g, err := cfg.BuildGraph()
	if err != nil {
		t.Fatal(err)
	}
	cacheMgr := cache.New(dir)
	durs := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		buf := &bytes.Buffer{}
		r := runner.New(cfg, g, cacheMgr, ui.NewReporter(buf, false), dir,
			runner.Options{Concurrency: 4, FileIndex: fileIndex})
		start := time.Now()
		if err := r.Run(context.Background(), nil); err != nil {
			t.Fatalf("vecto run %d failed: %v\n%s", i, err, buf.String())
		}
		durs = append(durs, time.Since(start))
	}
	return durs
}

func runTurbo(t *testing.T, dir string, warmUp bool) (durs []time.Duration, fullTurbo bool, out string) {
	t.Helper()
	n := 1
	if warmUp {
		n = 2
	}
	for i := 0; i < n; i++ {
		cmd := exec.Command("turbo", "run", "build")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"TURBO_TELEMETRY_DISABLED=1",
			"CI=false",
			"NO_COLOR=1",
		)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		start := time.Now()
		if err := cmd.Run(); err != nil {
			t.Fatalf("turbo run %d failed: %v\n%s", i, err, buf.String())
		}
		durs = append(durs, time.Since(start))
		out = buf.String()
	}
	fullTurbo = strings.Contains(out, "FULL TURBO")
	return durs, fullTurbo, out
}

func gitCommit(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "bench@vecto.local")
	run("config", "user.name", "Vecto Bench")
	run("add", "-A")
	run("commit", "-q", "-m", "bench")
}

func printTable(t *testing.T, title string, rows []row) {
	t.Logf("\n===== %s =====", title)
	for _, r := range rows {
		t.Logf("  %-34s %10v  %s", r.label, r.dur.Round(time.Millisecond), r.note)
	}
}

// TestBenchmark_HundredK is the ADR-0019 proof. Set VECTO_HUNDREDK=1 for the
// full 100,000-file matrix (includes Turborepo when installed); otherwise a
// 500-file smoke matrix runs (Vecto only) to keep CI cheap.
func TestBenchmark_HundredK(t *testing.T) {
	heavy := os.Getenv("VECTO_HUNDREDK") != ""
	packages, perPkg := lightPackages, lightPerPkg
	if heavy {
		packages, perPkg = heavyPackages, heavyPerPkg
	}
	total := packages * perPkg

	t.Logf("machine: %s/%s, %d vCPU, Go %s — %d-file tree (%d pkgs × %d files), VECTO_HUNDREDK=%v",
		runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(), total, packages, perPkg, heavy)

	// ---- Vecto legacy (per-task full-scan hashing, the pre-ADR-0019 path)
	dir := prepareProject(t, packages, perPkg, false)
	durs := runVectoScenario(t, dir, false, 2)
	legacy := durs[1]

	// ---- Vecto VFI (Merkle File Index)
	dir = prepareProject(t, packages, perPkg, false)
	durs = runVectoScenario(t, dir, true, 2)
	vfi := durs[1]

	// ---- Vecto VFI with one file changed (detection + one task re-run)
	dir = prepareProject(t, packages, perPkg, false)
	_ = runVectoScenario(t, dir, true, 2) // cold + warm
	target := filepath.Join(dir, "packages", fmt.Sprintf("p%03d", packages/2), "f000.txt")
	time.Sleep(1100 * time.Millisecond) // clear the 1-second racy window
	if err := os.WriteFile(target, []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	durs = runVectoScenario(t, dir, true, 1)
	vfiChange := durs[0]

	var rows []row
	rows = append(rows,
		row{"vecto legacy, 2nd run (0 changes)", legacy, "walk+rehash every input, per task"},
		row{"vecto VFI, 2nd run (0 changes)", vfi, "1 parallel stat cascade, 0 file reads"},
		row{"vecto VFI, run after 1-file edit", vfiChange, "k=1: stat cascade + 1 re-hash"},
	)
	if legacy > 0 && vfi > 0 {
		t.Logf("VFI speedup vs legacy (0 changes): %.1fx", float64(legacy)/float64(vfi))
	}
	printTable(t, fmt.Sprintf("Vecto @ %d files", total), rows)

	// ---- Turborepo 2.x (git mode + non-git mode)
	turboBin, err := exec.LookPath("turbo")
	if err != nil {
		t.Logf("turbo not on PATH — skipping Turborepo rows (install: npm i -g turbo)")
		return
	}
	t.Logf("turbo: %s", turboBin)
	if !heavy {
		t.Logf("VECTO_HUNDREDK not set — skipping Turborepo rows (run with VECTO_HUNDREDK=1)")
		return
	}

	// git mode: warm-up run (starts daemon, cold hashes) + measured warm run.
	dir = prepareProject(t, packages, perPkg, true)
	gitCommit(t, dir)
	var fullTurbo bool
	durs, fullTurbo, _ = runTurbo(t, dir, true)
	turboGit := durs[1]

	// non-git mode: turbo's SCM falls back to manual (full walk + hash).
	dir = prepareProject(t, packages, perPkg, true)
	durs, _, _ = runTurbo(t, dir, true)
	turboNonGit := durs[1]

	rows = []row{
		{"vecto legacy, 2nd run (0 changes)", legacy, ""},
		{"vecto VFI, 2nd run (0 changes)", vfi, ""},
		{"turbo 2.x (git), 2nd run (0 changes)", turboGit, fmt.Sprintf("FULL TURBO=%v", fullTurbo)},
		{"turbo 2.x (non-git), 2nd run (0 changes)", turboNonGit, "manual SCM: full walk + re-hash"},
		{"vecto VFI, run after 1-file edit", vfiChange, "k=1"},
	}
	printTable(t, fmt.Sprintf("Full matrix @ %d files", total), rows)
	if vfi > 0 && turboGit > 0 {
		t.Logf("VFI vs turbo (git, warm): %.1fx", float64(turboGit)/float64(vfi))
	}
	if vfi > 0 && turboNonGit > 0 {
		t.Logf("VFI vs turbo (non-git): %.1fx", float64(turboNonGit)/float64(vfi))
	}
}
