package hash_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/hash"
)

func TestHash_DeterministicAndSensitive(t *testing.T) {
	tempDir := t.TempDir()

	file1 := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(file1, []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	cmd := "go build -o bin/app ."
	inputs := []string{"*.go"}

	// 1. Initial hash
	h1, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}

	// 2. Same files and command should yield identical hash
	h2, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}
	if h1 != h2 {
		t.Errorf("expected deterministic hash, got %s != %s", h1, h2)
	}

	// 3. Touch file without changing content (timestamp updates) -> hash MUST remain identical
	time.Sleep(10 * time.Millisecond)
	now := time.Now().Add(5 * time.Minute)
	_ = os.Chtimes(file1, now, now)

	h3, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}
	if h1 != h3 {
		t.Errorf("timestamp change altered hash! Expected %s == %s", h1, h3)
	}

	// 4. Modify content by 1 byte -> hash MUST change
	if err := os.WriteFile(file1, []byte("package main\nfunc main() { /* edit */ }\n"), 0644); err != nil {
		t.Fatalf("failed to edit file: %v", err)
	}

	h4, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}
	if h1 == h4 {
		t.Errorf("content edit failed to alter hash! Got %s == %s", h1, h4)
	}
}

func TestHash_GlobMatchesRootAndNestedFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create root main.go
	rootFile := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(rootFile, []byte("package main"), 0644)

	// Create subpackage nested file
	subDir := filepath.Join(tempDir, "pkg", "sub")
	_ = os.MkdirAll(subDir, 0755)
	nestedFile := filepath.Join(subDir, "foo.go")
	_ = os.WriteFile(nestedFile, []byte("package sub"), 0644)

	cmd := "go build ."
	inputs := []string{"**/*.go"}

	h1, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("failed to hash with recursive glob: %v", err)
	}

	// Modify ROOT main.go -> MUST change hash
	_ = os.WriteFile(rootFile, []byte("package main // modified"), 0644)
	h2, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	if h1 == h2 {
		t.Fatalf("CRITICAL: editing root file main.go did NOT change hash when using **/*.go!")
	}

	// Modify NESTED foo.go -> MUST change hash
	_ = os.WriteFile(nestedFile, []byte("package sub // modified"), 0644)
	h3, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err != nil {
		t.Fatalf("failed to hash: %v", err)
	}
	if h2 == h3 {
		t.Fatalf("CRITICAL: editing nested file foo.go did NOT change hash when using **/*.go!")
	}
}

func TestHash_MissingInputFileReturnsError(t *testing.T) {
	tempDir := t.TempDir()

	cmd := "go build ."
	inputs := []string{"nonexistent_file.go"}

	_, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	if err == nil {
		t.Fatal("expected error for nonexistent input file, got nil")
	}
}

func TestHash_TransitiveDependencyHashing(t *testing.T) {
	tempDir := t.TempDir()
	cmd := "go build ."
	inputs := []string{"*.go"}
	_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main"), 0644)

	depsV1 := map[string]string{"lib": "hash_version_1"}
	depsV2 := map[string]string{"lib": "hash_version_2"}

	h1, _ := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, depsV1)
	h2, _ := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, depsV2)

	if h1 == h2 {
		t.Errorf("upstream dependency hash change failed to alter task fingerprint!")
	}
}

func TestGlob_RecursiveDoubleStar(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tempDir, "src", "pkg", "deep"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "src", "pkg", "deep", "util.go"), []byte("package deep"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "src", "main.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "root.txt"), []byte("root"), 0644)

	h1, err := hash.ComputeTaskFingerprint(tempDir, "echo 1", []string{"src/**/*.go"}, nil, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}

	// Editing deeply nested file must change the fingerprint
	_ = os.WriteFile(filepath.Join(tempDir, "src", "pkg", "deep", "util.go"), []byte("package deep // edit"), 0644)
	h2, err := hash.ComputeTaskFingerprint(tempDir, "echo 1", []string{"src/**/*.go"}, nil, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}
	if h1 == h2 {
		t.Errorf("expected hash to change when nested file changed")
	}

	// Editing root.txt (not matching src/**/*.go) should NOT change fingerprint
	_ = os.WriteFile(filepath.Join(tempDir, "root.txt"), []byte("root edit"), 0644)
	h3, _ := hash.ComputeTaskFingerprint(tempDir, "echo 1", []string{"src/**/*.go"}, nil, nil)
	if h2 != h3 {
		t.Errorf("unmatched file edit changed fingerprint!")
	}
}

func TestGlob_IgnorePatterns(t *testing.T) {
	tempDir := t.TempDir()

	// Default ignored dir: node_modules
	_ = os.MkdirAll(filepath.Join(tempDir, "node_modules", "dep"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "node_modules", "dep", "index.js"), []byte("console.log()"), 0644)

	_ = os.WriteFile(filepath.Join(tempDir, "app.js"), []byte("console.log('app')"), 0644)

	h1, err := hash.ComputeTaskFingerprint(tempDir, "node app.js", []string{"**/*.js"}, nil, nil)
	if err != nil {
		t.Fatalf("hashing failed: %v", err)
	}

	// Modifying file inside node_modules must NOT change fingerprint because it is ignored by default
	_ = os.WriteFile(filepath.Join(tempDir, "node_modules", "dep", "index.js"), []byte("console.log('tampered')"), 0644)
	h2, _ := hash.ComputeTaskFingerprint(tempDir, "node app.js", []string{"**/*.js"}, nil, nil)
	if h1 != h2 {
		t.Errorf("node_modules was not ignored!")
	}

	// Custom .vectoignore
	_ = os.WriteFile(filepath.Join(tempDir, ".vectoignore"), []byte("*.tmp\nscratch/**\n"), 0644)
	_ = os.MkdirAll(filepath.Join(tempDir, "scratch"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "scratch", "test.js"), []byte("scratch js"), 0644)

	h3, _ := hash.ComputeTaskFingerprint(tempDir, "node app.js", []string{"**/*.js"}, nil, nil)
	if h1 != h3 {
		t.Errorf(".vectoignore rule was not honored!")
	}
}

func BenchmarkHash_100Files(b *testing.B) {
	tempDir := b.TempDir()
	for i := 0; i < 100; i++ {
		p := filepath.Join(tempDir, fmt.Sprintf("file_%d.go", i))
		_ = os.WriteFile(p, []byte(fmt.Sprintf("package main\nvar x%d = %d", i, i)), 0644)
	}

	cmd := "go build ."
	inputs := []string{"**/*.go"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil, nil)
	}
}
