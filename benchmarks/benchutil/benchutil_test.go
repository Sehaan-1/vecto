package benchutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestRecord_NoEnvIsNoop proves Record is a no-op when VECTO_BENCH_OUT is unset.
func TestRecord_NoEnvIsNoop(t *testing.T) {
	t.Setenv("VECTO_BENCH_OUT", "")
	out := filepath.Join(t.TempDir(), "out.jsonl")
	Record(t, "cache_hit", map[string]any{"n": 1})
	if _, err := os.Stat(out); err == nil {
		t.Fatalf("expected no file at %s when VECTO_BENCH_OUT is unset", out)
	}
}

// TestRecord_ConcurrentAppend proves 20 concurrent Record calls produce one
// valid JSON object per line with no interleaving.
func TestRecord_ConcurrentAppend(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.jsonl")
	t.Setenv("VECTO_BENCH_OUT", out)
	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Record(t, "cache_hit", map[string]any{"n": i})
		}(i)
	}
	wg.Wait()
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading %s: %v", out, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d: %q", n, len(lines), string(data))
	}
	for _, l := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(l), &rec); err != nil {
			t.Fatalf("line %q is not valid JSON: %v", l, err)
		}
		suite, _ := rec["suite"].(string)
		test, _ := rec["test"].(string)
		metrics, _ := rec["metrics"].(map[string]any)
		if suite == "" || test == "" {
			t.Fatalf("line %q missing suite/test: %v", l, rec)
		}
		if metrics["n"] == nil {
			t.Fatalf("line %q missing metrics.n: %v", l, rec)
		}
	}
}
