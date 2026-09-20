package fileindex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestProfile100k is a TEMPORARY diagnostic (env-gated): decomposes the
// warm-run cost of a 100k-file index into Load / Sync phases.
func TestProfile100k(t *testing.T) {
	if os.Getenv("VECTO_PROFILE") == "" {
		t.Skip("set VECTO_PROFILE=1")
	}
	dir := t.TempDir()
	// generate 100k files
	for p := 0; p < 500; p++ {
		pkgDir := filepath.Join(dir, "packages", fmt.Sprintf("p%03d", p))
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < 200; f++ {
			body := fmt.Sprintf("package p%03d file %03d\nline 2\n", p, f)
			if err := os.WriteFile(filepath.Join(pkgDir, fmt.Sprintf("f%03d.txt", f)), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	time.Sleep(1100 * time.Millisecond) // predate mtimes for the snapshot

	// cold sync (writes the index)
	ix, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	rep, err := ix.Sync(dir, nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	cold := time.Since(t0)
	if err := ix.Save(dir); err != nil {
		t.Fatal(err)
	}
	t.Logf("cold sync: %s (files=%d hashed=%d)", cold, rep.FilesSeen, rep.FilesHashed)

	// warm: measure Load and Sync separately
	t0 = time.Now()
	ix2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	load := time.Since(t0)

	t0 = time.Now()
	rep2, err := ix2.Sync(dir, nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	syncDur := time.Since(t0)
	t.Logf("warm: load=%s sync=%s (files=%d hashed=%d)", load, syncDur, rep2.FilesSeen, rep2.FilesHashed)

	// and with concurrency = NumCPU-equivalent 8
	t0 = time.Now()
	ix3, _ := Load(dir)
	load3 := time.Since(t0)
	t0 = time.Now()
	rep3, err := ix3.Sync(dir, nil, 8)
	if err != nil {
		t.Fatal(err)
	}
	sync3 := time.Since(t0)
	t.Logf("warm c8: load=%s sync=%s (hashed=%d)", load3, sync3, rep3.FilesHashed)
}
