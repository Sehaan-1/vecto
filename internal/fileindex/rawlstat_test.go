package fileindex

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestRawLstatCost is an env-gated diagnostic (VECTO_PROFILE=1): raw cost of Lstat'ing 100k
// existing files, serial vs N-goroutine, to bound the crawl cost.
func TestRawLstatCost(t *testing.T) {
	if os.Getenv("VECTO_PROFILE") == "" {
		t.Skip("set VECTO_PROFILE=1")
	}
	dir := t.TempDir()
	var paths []string
	for p := 0; p < 500; p++ {
		pkgDir := filepath.Join(dir, "packages", fmt.Sprintf("p%03d", p))
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 200; f++ {
			rel := filepath.Join(pkgDir, fmt.Sprintf("f%03d.txt", f))
			if err := os.WriteFile(rel, []byte("x"), 0644); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, rel)
		}
	}

	t0 := time.Now()
	for _, p := range paths {
		if _, err := os.Lstat(p); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("serial Lstat x%d: %s", len(paths), time.Since(t0))

	for _, n := range []int{2, 4, 8, 16, runtime.NumCPU()} {
		t0 := time.Now()
		jobs := make(chan string, 4096)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for p := range jobs {
					if _, err := os.Lstat(p); err != nil {
						t.Error(err)
					}
				}
			}()
		}
		for _, p := range paths {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		t.Logf("parallel Lstat x%d with %d workers: %s", len(paths), n, time.Since(t0))
	}
}
