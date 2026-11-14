package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Environment captures the machine and tool state for a benchmark report.
type Environment struct {
	GoOS      string `json:"goos"`
	GoArch    string `json:"goarch"`
	GoVersion string `json:"go_version"`
	GoBinary  string `json:"go_binary"`
	Turbo     string `json:"turbo"`
	Just      string `json:"just"`
	Make      string `json:"make"`
	NumCPU    int    `json:"num_cpu"`
	Label     string `json:"label"`
}

// Result is one benchmark measurement row in the JSONL stream.
type Result struct {
	Suite   string         `json:"suite"`
	Test    string         `json:"test"`
	Status  string         `json:"status"`
	Metrics map[string]any `json:"metrics"`
}

// parseEvents turns a `go test -json` event stream into a test-to-status map.
func parseEvents(r io.Reader) (map[string]string, error) {
	dec := json.NewDecoder(r)
	statuses := map[string]string{}
	for {
		var ev struct {
			Action string `json:"Action"`
			Test   string `json:"Test"`
		}
		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch ev.Action {
		case "pass":
			statuses[ev.Test] = "pass"
		case "fail":
			statuses[ev.Test] = "fail"
		case "skip":
			statuses[ev.Test] = "skip"
		}
	}
	return statuses, nil
}

// readResults parses a JSONL benchmark record stream into results.
func readResults(r io.Reader) ([]Result, error) {
	dec := json.NewDecoder(r)
	var results []Result
	for {
		var res Result
		if err := dec.Decode(&res); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if res.Suite != "" || res.Test != "" {
			results = append(results, res)
		}
	}
	return results, nil
}

// renderMarkdown renders the results markdown: environment fence, then one
// table per test with Metric | Value. Skipped tests render no numeric rows.
func renderMarkdown(env Environment, results []Result) string {
	sorted := make([]Result, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Suite != sorted[j].Suite {
			return sorted[i].Suite < sorted[j].Suite
		}
		return sorted[i].Test < sorted[j].Test
	})
	var sb strings.Builder
	sb.WriteString("# Benchmark results\n\n")
	fmt.Fprintf(&sb, "- goos: %s\n", env.GoOS)
	fmt.Fprintf(&sb, "- goarch: %s\n", env.GoArch)
	fmt.Fprintf(&sb, "- go_version: %s\n", env.GoVersion)
	fmt.Fprintf(&sb, "- go_binary: %s\n", env.GoBinary)
	fmt.Fprintf(&sb, "- turbo: %s\n", env.Turbo)
	fmt.Fprintf(&sb, "- just: %s\n", env.Just)
	fmt.Fprintf(&sb, "- make: %s\n", env.Make)
	fmt.Fprintf(&sb, "- num_cpu: %d\n", env.NumCPU)
	fmt.Fprintf(&sb, "- label: %s\n", env.Label)
	sb.WriteString("\n")
	for _, r := range sorted {
		sb.WriteString(fmt.Sprintf("## %s / %s\n\n", r.Suite, r.Test))
		if r.Status == "skip" {
			sb.WriteString("skipped (missing tool)\n\n")
			continue
		}
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("|---|---|---|\n")
		keys := make([]string, 0, len(r.Metrics))
		for k := range r.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&sb, "| %s | %v |\n", k, r.Metrics[k])
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func toolOrMissing(name string) string {
	if _, err := exec.LookPath(name); err != nil {
		return "missing"
	}
	return name
}

func goBinary() string {
	if b, err := exec.LookPath("go"); err == nil {
		return b
	}
	return "go"
}

func runSuite(pkg, pattern, jsonlPath string) (map[string]string, error) {
	cmd := exec.Command("go", "test", "-json", "-count=1", "-run", pattern, pkg)
	cmd.Env = append(os.Environ(), "VECTO_BENCH_OUT="+jsonlPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go test %s %s: %w\n%s", pkg, pattern, err, stderr.String())
	}
	return parseEvents(&stdout)
}

func main() {
	outDir := flag.String("out", "bench-results", "directory receiving results.json and results.md")
	includeExternal := flag.Bool("external", false, "also run external-runner benchmarks (turbo, just)")
	label := flag.String("label", "", "label recorded in the environment block")
	flag.Parse()

	env := Environment{
		GoOS:      runtime.GOOS,
		GoArch:    runtime.GOARCH,
		GoVersion: runtime.Version(),
		GoBinary:  goBinary(),
		Turbo:     toolOrMissing("turbo"),
		Just:      toolOrMissing("just"),
		Make:      toolOrMissing("make"),
		NumCPU:    runtime.NumCPU(),
		Label:     *label,
	}

	type suiteCmd struct {
		suite  string
		pattern string
		pkg    string
	}
	suites := []suiteCmd{
		{"multi_lang", `TestBenchmark_(MultiLang_Comparison|CacheHit_LatencyDistribution|Scalability_100_1000_10000|FailureTeardown)`, "./benchmarks"},
		{"hundredk", "TestBenchmark_HundredK", "./benchmarks/hundredk"},
	}
	if *includeExternal {
		suites = append(suites, suiteCmd{"external", "TestBenchmark_ExternalRunners", "./benchmarks"})
	}

	var results []Result
	failed := false
	for _, s := range suites {
		jsonlPath := filepath.Join(os.TempDir(), fmt.Sprintf("vecto-bench-%s.jsonl", s.suite))
		statuses, err := runSuite(s.pkg, s.pattern, jsonlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "report: %v\n", err)
			os.Exit(1)
		}
		var res []Result
		f, err := os.Open(jsonlPath)
		switch {
		case err == nil:
			res, err = readResults(f)
			f.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "report: reading %s: %v\n", jsonlPath, err)
				os.Exit(1)
			}
		case os.IsNotExist(err):
			// A suite whose tests all skipped wrote no JSONL records; keep the
			// skip statuses from the event stream instead of failing to open.
			res = nil
		default:
			fmt.Fprintf(os.Stderr, "report: opening %s: %v\n", jsonlPath, err)
			os.Exit(1)
		}
		known := make(map[string]bool, len(res))
		for _, r := range res {
			known[r.Test] = true
			if st, ok := statuses[r.Test]; ok {
				r.Status = st
			}
			results = append(results, r)
			if r.Status == "fail" {
				failed = true
			}
		}
		for name, st := range statuses {
			if st == "skip" && !known[name] {
				results = append(results, Result{Suite: s.suite, Test: name, Status: "skip", Metrics: nil})
			}
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "report: creating %s: %v\n", *outDir, err)
		os.Exit(1)
	}
	out, err := json.MarshalIndent(struct {
		Environment Environment `json:"environment"`
		Results     []Result     `json:"results"`
	}{Environment: env, Results: results}, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "report: marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "results.json"), append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "report: writing results.json: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "results.md"), []byte(renderMarkdown(env, results)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "report: writing results.md: %v\n", err)
		os.Exit(1)
	}

	if failed {
		os.Exit(1)
	}
}
