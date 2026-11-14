package benchutil

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
)

type record struct {
	Suite   string         `json:"suite"`
	Test    string         `json:"test"`
	Metrics map[string]any `json:"metrics"`
}

var mu sync.Mutex

// Record appends one JSONL benchmark record to the file named by VECTO_BENCH_OUT.
// Format: {"suite":"...","test":"...","metrics":{...}} — one object per line.
// No-op when VECTO_BENCH_OUT is unset. Safe for concurrent use.
func Record(t testing.TB, suite string, metrics map[string]any) {
	t.Helper()
	out := os.Getenv("VECTO_BENCH_OUT")
	if out == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	data, err := json.Marshal(record{Suite: suite, Test: t.Name(), Metrics: metrics})
	if err != nil {
		t.Fatalf("benchutil: marshal record: %v", err)
	}
	f, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("benchutil: open %s: %v", out, err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		t.Fatalf("benchutil: write record: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("benchutil: close record: %v", err)
	}
}
