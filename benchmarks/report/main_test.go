package main

import (
	"os"
	"strings"
	"testing"
)

func TestParseEvents(t *testing.T) {
	f, err := os.Open("testdata/events.jsonl")
	if err != nil {
		t.Fatalf("opening events fixture: %v", err)
	}
	defer f.Close()

	got, err := parseEvents(f)
	if err != nil {
		t.Fatalf("parseEvents: %v", err)
	}

	want := map[string]string{
		"TestBenchmark_CacheHit_LatencyDistribution": "pass",
		"TestBenchmark_MissingTool":                   "skip",
		"TestBenchmark_Failing":                      "fail",
	}
	if len(got) != len(want) {
		t.Fatalf("parseEvents returned %v, want %v", got, want)
	}
	for name, status := range want {
		if got[name] != status {
			t.Fatalf("parseEvents[%q] = %q, want %q", name, got[name], status)
		}
	}
}

func TestRender_SkippedIsNotZero(t *testing.T) {
	env := Environment{
		GoOS:      "linux",
		GoArch:    "amd64",
		GoVersion: "1.24",
		GoBinary:  "/usr/local/go/bin/go",
		Turbo:     "missing",
		Just:      "missing",
		Make:      "missing",
		NumCPU:    2,
		Label:     "test",
	}
	md := renderMarkdown(env, []Result{
		{Suite: "cache_hit", Test: "TestBenchmark_MissingTool", Status: "skip"},
	})

	if !strings.Contains(md, "skipped") {
		t.Fatalf("expected skipped marker in markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "skipped (missing tool)") {
		t.Fatalf("expected skipped (missing tool) in markdown, got:\n%s", md)
	}
	if strings.Contains(md, "vecto_min_ms") {
		t.Fatalf("skipped result must not render numeric metric rows, got:\n%s", md)
	}
}

func TestRender_Deterministic(t *testing.T) {
	env := Environment{
		GoOS:      "linux",
		GoArch:    "amd64",
		GoVersion: "1.24",
		GoBinary:  "/usr/local/go/bin/go",
		Turbo:     "missing",
		Just:      "missing",
		Make:      "missing",
		NumCPU:    2,
		Label:     "test",
	}
	results := []Result{
		{Suite: "multi_lang", Test: "TestBenchmark_MultiLang_Comparison", Status: "pass", Metrics: map[string]any{"vecto_cold_ms": 10.0}},
		{Suite: "cache_hit", Test: "TestBenchmark_CacheHit_LatencyDistribution", Status: "pass", Metrics: map[string]any{"vecto_min_ms": 5.0}},
	}

	md := renderMarkdown(env, results)

	cacheIdx := strings.Index(md, "## cache_hit")
	multiIdx := strings.Index(md, "## multi_lang")
	if cacheIdx == -1 || multiIdx == -1 {
		t.Fatalf("expected both suite headers in markdown, got:\n%s", md)
	}
	if cacheIdx > multiIdx {
		t.Fatalf("renderMarkdown must sort by suite then test, got:\n%s", md)
	}
}

func TestReadResults(t *testing.T) {
	f, err := os.Open("testdata/results.jsonl")
	if err != nil {
		t.Fatalf("opening results fixture: %v", err)
	}
	defer f.Close()

	results, err := readResults(f)
	if err != nil {
		t.Fatalf("readResults: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("readResults returned %d results, want 2", len(results))
	}
	if results[0].Suite != "cache_hit" || results[0].Test != "TestBenchmark_CacheHit_LatencyDistribution" {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if results[0].Metrics["vecto_min_ms"] != 25.768 {
		t.Fatalf("unexpected first metrics: %+v", results[0].Metrics)
	}
}

