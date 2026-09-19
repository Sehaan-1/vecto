package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Entry stores metadata for a cached task execution.
type Entry struct {
	Hash       string    `json:"hash"`
	TaskName   string    `json:"task_name"`
	ExitCode   int       `json:"exit_code"`
	DurationMs int64     `json:"duration_ms"`
	Timestamp  time.Time `json:"timestamp"`
	Artifacts  []string  `json:"artifacts"`
}

// Manager manages reading and writing task execution caches.
type Manager struct {
	BaseDir  string
	CacheDir string
}

// New creates a new Cache Manager honoring ADR-0003.
func New(baseDir string) *Manager {
	cacheDir := os.Getenv("VECTO_CACHE_DIR")
	if cacheDir == "" {
		if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
			cacheDir = filepath.Join(xdg, "vecto")
		} else {
			cacheDir = filepath.Join(baseDir, ".vecto", "cache")
		}
	}
	return &Manager{
		BaseDir:  baseDir,
		CacheDir: cacheDir,
	}
}

// Has checks if a valid cache entry exists for the given hash.
func (m *Manager) Has(hash string) bool {
	if hash == "" {
		return false
	}
	metaFile := filepath.Join(m.CacheDir, hash, "meta.json")
	info, err := os.Stat(metaFile)
	return err == nil && !info.IsDir()
}

// Store writes metadata, logs, and artifacts atomically into the cache directory.
func (m *Manager) Store(hash, taskName string, exitCode int, duration time.Duration, output []byte, outputPaths []string) error {
	taskCacheDir := filepath.Join(m.CacheDir, hash)
	if err := os.MkdirAll(taskCacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	// 1. Write log output
	logFile := filepath.Join(taskCacheDir, "output.log")
	if err := os.WriteFile(logFile, output, 0644); err != nil {
		return fmt.Errorf("writing cache log: %w", err)
	}

	// 2. Copy artifacts to cache
	artifactsDir := filepath.Join(taskCacheDir, "artifacts")
	var savedArtifacts []string
	for _, relPath := range outputPaths {
		src := filepath.Join(m.BaseDir, relPath)
		dst := filepath.Join(artifactsDir, relPath)

		if info, err := os.Stat(src); err == nil && !info.IsDir() {
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return err
			}
			if err := copyFile(src, dst); err == nil {
				savedArtifacts = append(savedArtifacts, relPath)
			}
		}
	}

	// 3. Write metadata JSON
	entry := Entry{
		Hash:       hash,
		TaskName:   taskName,
		ExitCode:   exitCode,
		DurationMs: duration.Milliseconds(),
		Timestamp:  time.Now().UTC(),
		Artifacts:  savedArtifacts,
	}

	metaBytes, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	metaFile := filepath.Join(taskCacheDir, "meta.json")
	return os.WriteFile(metaFile, metaBytes, 0644)
}

// Restore restores cached outputs and returns metadata and log output.
func (m *Manager) Restore(hash string) (*Entry, []byte, error) {
	taskCacheDir := filepath.Join(m.CacheDir, hash)
	metaFile := filepath.Join(taskCacheDir, "meta.json")

	metaBytes, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, nil, fmt.Errorf("reading cache meta: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(metaBytes, &entry); err != nil {
		return nil, nil, fmt.Errorf("decoding cache meta: %w", err)
	}

	logFile := filepath.Join(taskCacheDir, "output.log")
	logBytes, _ := os.ReadFile(logFile)

	// Restore artifacts back to project workspace
	artifactsDir := filepath.Join(taskCacheDir, "artifacts")
	for _, relPath := range entry.Artifacts {
		src := filepath.Join(artifactsDir, relPath)
		dst := filepath.Join(m.BaseDir, relPath)

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			continue
		}
		_ = copyFile(src, dst)
	}

	return &entry, logBytes, nil
}

// Clean clears the cache directory.
func (m *Manager) Clean() error {
	return os.RemoveAll(m.CacheDir)
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}
