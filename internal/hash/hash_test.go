package hash_test

import (
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
	h1, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}

	// 2. Same files and command should yield identical hash
	h2, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil)
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

	h3, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil)
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

	h4, err := hash.ComputeTaskFingerprint(tempDir, cmd, inputs, nil)
	if err != nil {
		t.Fatalf("ComputeTaskFingerprint failed: %v", err)
	}
	if h1 == h4 {
		t.Errorf("content edit failed to alter hash! Got %s == %s", h1, h4)
	}
}
