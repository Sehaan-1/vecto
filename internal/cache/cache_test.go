package cache_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/cache"
)

func TestCache_StoreAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	cacheMgr := cache.New(tempDir)

	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	taskName := "build"
	exitCode := 0
	duration := 120 * time.Millisecond
	output := []byte("Build successful: created bin/app\n")

	// Create dummy artifact in workspace
	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create bin dir: %v", err)
	}
	binFile := filepath.Join(binDir, "app")
	if err := os.WriteFile(binFile, []byte("ELF binary dummy"), 0755); err != nil {
		t.Fatalf("failed to write dummy artifact: %v", err)
	}

	outputPaths := []string{"bin/app"}

	// 1. Check Has() returns false initially
	if cacheMgr.Has(hash) {
		t.Fatalf("expected Has() to be false before Store()")
	}

	// 2. Store in cache
	if err := cacheMgr.Store(hash, taskName, exitCode, duration, output, outputPaths); err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// 3. Has() should now be true
	if !cacheMgr.Has(hash) {
		t.Fatalf("expected Has() to be true after Store()")
	}

	// 4. Wipe the workspace artifact to verify restoration
	if err := os.Remove(binFile); err != nil {
		t.Fatalf("failed to remove workspace artifact: %v", err)
	}

	// 5. Restore from cache
	entry, logs, err := cacheMgr.Restore(hash)
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	if entry.TaskName != taskName {
		t.Errorf("expected TaskName %s, got %s", taskName, entry.TaskName)
	}
	if entry.ExitCode != exitCode {
		t.Errorf("expected ExitCode %d, got %d", exitCode, entry.ExitCode)
	}
	if string(logs) != string(output) {
		t.Errorf("expected logs %q, got %q", string(output), string(logs))
	}
	if entry.SchemaVersion != cache.CurrentSchemaVersion {
		t.Errorf("expected schema version %d, got %d", cache.CurrentSchemaVersion, entry.SchemaVersion)
	}

	// 6. Verify restored artifact in workspace
	restoredBytes, err := os.ReadFile(binFile)
	if err != nil {
		t.Fatalf("restored artifact not found in workspace: %v", err)
	}
	if string(restoredBytes) != "ELF binary dummy" {
		t.Errorf("expected restored content 'ELF binary dummy', got %q", string(restoredBytes))
	}
}

func TestCache_SchemaVersioning(t *testing.T) {
	tempDir := t.TempDir()
	cacheMgr := cache.New(tempDir)
	hash := "version-test-hash"

	// Store valid entry
	_ = cacheMgr.Store(hash, "test", 0, time.Second, []byte("ok"), nil)

	// Tamper with meta.json to simulate an older or incompatible schema version
	metaFile := filepath.Join(cacheMgr.CacheDir, hash, "meta.json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		t.Fatalf("reading meta: %v", err)
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(data, &raw)
	raw["schema_version"] = 999 // incompatible version
	tamperedBytes, _ := json.Marshal(raw)
	_ = os.WriteFile(metaFile, tamperedBytes, 0644)

	// Restore should reject incompatible schema
	_, _, err = cacheMgr.Restore(hash)
	if err == nil {
		t.Fatalf("expected error restoring incompatible schema version, got nil")
	}
}

func TestCache_CorruptionDetection(t *testing.T) {
	tempDir := t.TempDir()
	cacheMgr := cache.New(tempDir)
	hash := "corruption-test-hash"

	// Create artifact
	artRel := "output/data.txt"
	fullArt := filepath.Join(tempDir, artRel)
	_ = os.MkdirAll(filepath.Dir(fullArt), 0755)
	_ = os.WriteFile(fullArt, []byte("valid artifact content"), 0644)

	// Store
	err := cacheMgr.Store(hash, "build", 0, time.Second, []byte("log"), []string{artRel})
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// Tamper with cached artifact content
	cachedArt := filepath.Join(cacheMgr.CacheDir, hash, "artifacts", artRel)
	_ = os.WriteFile(cachedArt, []byte("corrupted modified data!"), 0644)

	// Restore should detect corruption
	_, _, err = cacheMgr.Restore(hash)
	if err == nil {
		t.Fatalf("expected Restore() to fail with ErrCacheCorrupted, got nil")
	}

	// Cache entry directory should be removed on corruption detection
	if cacheMgr.Has(hash) {
		t.Errorf("corrupted cache entry should have been pruned from disk")
	}
}

func TestCache_FileLocking(t *testing.T) {
	tempDir := t.TempDir()
	lockPath := filepath.Join(tempDir, "test.lock")

	lock1, err := cache.AcquireLock(lockPath, cache.LockShared)
	if err != nil {
		t.Fatalf("AcquireLock shared failed: %v", err)
	}

	// A second shared lock should succeed
	lock2, err := cache.AcquireLock(lockPath, cache.LockShared)
	if err != nil {
		t.Fatalf("AcquireLock 2nd shared failed: %v", err)
	}

	_ = lock1.Unlock()
	_ = lock2.Unlock()

	// Exclusive lock should succeed
	exLock, err := cache.AcquireLock(lockPath, cache.LockExclusive)
	if err != nil {
		t.Fatalf("AcquireLock exclusive failed: %v", err)
	}
	_ = exLock.Unlock()
}

func TestCache_RemoteHTTPBackend(t *testing.T) {	storage := make(map[string][]byte)
	var mu sync.Mutex

	// Mock remote cache HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		key := r.URL.Path[1:] // trim leading slash

		switch r.Method {
		case http.MethodHead:
			if _, exists := storage[key]; exists {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case http.MethodGet:
			data, exists := storage[key]
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		case http.MethodPut:
			data := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(data)
			storage[key] = data
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	remoteBackend := cache.NewHTTPRemoteBackend(server.URL, "secret-token", 5*time.Second)

	tempDir := t.TempDir()
	cacheMgr := cache.New(tempDir)
	cacheMgr.SetRemote(remoteBackend)

	hash := "remote-test-hash"

	// Create and store task
	artRel := "out/bundle.js"
	fullArt := filepath.Join(tempDir, artRel)
	_ = os.MkdirAll(filepath.Dir(fullArt), 0755)
	_ = os.WriteFile(fullArt, []byte("console.log('remote');"), 0644)

	err := cacheMgr.Store(hash, "bundle", 0, 50*time.Millisecond, []byte("built bundle"), []string{artRel})
	if err != nil {
		t.Fatalf("store failed: %v", err)
	}

	// Give async remote upload a brief moment to finish
	time.Sleep(100 * time.Millisecond)

	// Clean local disk cache to force remote fetch
	_ = os.RemoveAll(filepath.Join(cacheMgr.CacheDir, hash))
	_ = os.Remove(fullArt)

	// Has() should query remote and return true
	ctx := context.Background()
	hasRemote, err := remoteBackend.Has(ctx, hash)
	if err != nil || !hasRemote {
		t.Fatalf("remote backend does not have hash: %v", err)
	}

	// Restore() should hydrate from remote cache and restore the artifact!
	entry, logs, err := cacheMgr.Restore(hash)
	if err != nil {
		t.Fatalf("Restore from remote failed: %v", err)
	}
	if entry.TaskName != "bundle" {
		t.Errorf("expected task 'bundle', got %s", entry.TaskName)
	}
	if string(logs) != "built bundle" {
		t.Errorf("unexpected logs: %s", string(logs))
	}

	// Verify workspace artifact restored
	restoredData, err := os.ReadFile(fullArt)
	if err != nil || string(restoredData) != "console.log('remote');" {
		t.Errorf("failed to restore artifact from remote: %v, content: %s", err, string(restoredData))
	}
}

// captureRemote records the artifact map handed to Put so tests can prove
// large outputs never enter the in-memory upload path (ADR-0012).
type captureRemote struct {
	mu        sync.Mutex
	got       map[string][]byte
	putCalled chan struct{}
}

func (c *captureRemote) Has(ctx context.Context, hash string) (bool, error) {
	return false, nil
}

func (c *captureRemote) Get(ctx context.Context, hash string) (*cache.Entry, []byte, map[string][]byte, error) {
	return nil, nil, nil, cache.ErrRemoteNotFound
}

func (c *captureRemote) Put(ctx context.Context, hash string, entry *cache.Entry, logBytes []byte, artifacts map[string][]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = artifacts
	select {
	case <-c.putCalled:
	default:
		close(c.putCalled)
	}
	return nil
}

func TestCache_LargeArtifactSkippedFromMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 100MB artifact test in short mode")
	}
	tempDir := t.TempDir()
	cacheMgr := cache.New(tempDir)

	remote := &captureRemote{putCalled: make(chan struct{})}
	cacheMgr.SetRemote(remote)

	// 100MB output, written in 1MB chunks so the test itself never holds it.
	const size = 100 << 20
	artRel := "out/big.bin"
	fullArt := filepath.Join(tempDir, artRel)
	if err := os.MkdirAll(filepath.Dir(fullArt), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f, err := os.Create(fullArt)
	if err != nil {
		t.Fatalf("create big artifact: %v", err)
	}
	chunk := make([]byte, 1<<20)
	for written := int64(0); written < size; {
		n, err := f.Write(chunk)
		if err != nil {
			t.Fatalf("write big artifact: %v", err)
		}
		written += int64(n)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close big artifact: %v", err)
	}

	hash := "large-artifact-test-hash"
	if err := cacheMgr.Store(hash, "bundle", 0, time.Second, []byte("built big"), []string{artRel}); err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// Wait for the async remote upload.
	select {
	case <-remote.putCalled:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for async remote Put")
	}

	// The 100MB file must NOT be in the in-memory upload map.
	remote.mu.Lock()
	_, present := remote.got[artRel]
	remote.mu.Unlock()
	if present {
		t.Fatalf("artifact %q over %d bytes was buffered for remote upload", artRel, cache.MaxRemoteArtifactBytes)
	}

	// Local disk cache must still hold it, verified by restore.
	_ = os.Remove(fullArt)
	entry, _, err := cacheMgr.Restore(hash)
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}
	if len(entry.Artifacts) != 1 || entry.Artifacts[0] != artRel {
		t.Fatalf("expected artifact %q in entry, got %v", artRel, entry.Artifacts)
	}
	info, err := os.Stat(fullArt)
	if err != nil {
		t.Fatalf("restored artifact missing: %v", err)
	}
	if info.Size() != size {
		t.Fatalf("expected restored size %d, got %d", size, info.Size())
	}
}
