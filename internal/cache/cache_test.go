package cache_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
)

func TestCache_StoreAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.New(tempDir)

	hashVal := "abc1234567890def"
	taskName := "compile"
	outputLog := []byte("compiling main.go... done!\n")

	// Create an artifact file with executable permissions
	artifactRel := filepath.Join("dist", "binary.bin")
	artifactFull := filepath.Join(tempDir, artifactRel)
	if err := os.MkdirAll(filepath.Dir(artifactFull), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(artifactFull, []byte("BINARY_DATA_PAYLOAD"), 0755); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	if mgr.Has(hashVal) {
		t.Errorf("expected Has to be false initially")
	}

	// Store
	err := mgr.Store(hashVal, taskName, 0, 150*time.Millisecond, outputLog, []string{artifactRel})
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if !mgr.Has(hashVal) {
		t.Errorf("expected Has to be true after store")
	}

	// Delete original artifact to test restoration
	_ = os.Remove(artifactFull)

	// Restore
	entry, logs, err := mgr.Restore(hashVal)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if entry.TaskName != taskName {
		t.Errorf("expected task name %s, got %s", taskName, entry.TaskName)
	}
	if string(logs) != string(outputLog) {
		t.Errorf("expected logs %q, got %q", string(outputLog), string(logs))
	}

	// Verify artifact restored
	restoredData, err := os.ReadFile(artifactFull)
	if err != nil {
		t.Fatalf("failed to read restored artifact: %v", err)
	}
	if string(restoredData) != "BINARY_DATA_PAYLOAD" {
		t.Errorf("restored artifact content mismatch")
	}

	// Verify permissions restored (on Unix)
	if runtime.GOOS != "windows" {
		info, err := os.Stat(artifactFull)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		if info.Mode().Perm()&0111 == 0 {
			t.Errorf("executable permissions were lost on cache restore: mode is %v", info.Mode())
		}
	}
}

func TestCache_Prune(t *testing.T) {
	tempDir := t.TempDir()
	mgr := cache.New(tempDir)

	// Entry 1: Old entry (created 2 hours ago)
	err := mgr.Store("old_hash", "old_task", 0, 100*time.Millisecond, []byte("old logs"), nil)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Manually backdate old_hash meta.json
	metaFile := filepath.Join(mgr.CacheDir, "old_hash", "meta.json")
	data, _ := os.ReadFile(metaFile)
	var entry cache.Entry
	_ = json.Unmarshal(data, &entry)
	entry.Timestamp = time.Now().Add(-2 * time.Hour)
	backdated, _ := json.Marshal(entry)
	_ = os.WriteFile(metaFile, backdated, 0644)

	// Entry 2: Fresh entry
	_ = mgr.Store("fresh_hash", "fresh_task", 0, 50*time.Millisecond, []byte("fresh logs"), nil)

	// Prune older than 1 hour
	pruned, err := mgr.Prune(1 * time.Hour)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned entry, got %d", pruned)
	}

	if mgr.Has("old_hash") {
		t.Errorf("expected old_hash to be deleted by prune")
	}
	if !mgr.Has("fresh_hash") {
		t.Errorf("expected fresh_hash to remain after prune")
	}
}

func BenchmarkCache_Restore(b *testing.B) {
	tempDir := b.TempDir()
	mgr := cache.New(tempDir)
	_ = mgr.Store("bench_hash", "bench_task", 0, 10*time.Millisecond, []byte("benchmark log content"), nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = mgr.Restore("bench_hash")
	}
}
