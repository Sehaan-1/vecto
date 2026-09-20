package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
// It stages all files in a temporary directory inside CacheDir and commits via
// an atomic filesystem rename (rename(2) on POSIX), preventing cache poisoning
// from partial writes or concurrent store races (ADR-0008).
func (m *Manager) Store(hash, taskName string, exitCode int, duration time.Duration, output []byte, outputPaths []string) error {
	if err := os.MkdirAll(m.CacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache root dir: %w", err)
	}

	taskCacheDir := filepath.Join(m.CacheDir, hash)

	// If entry is already valid and complete, no need to overwrite
	if m.Has(hash) {
		return nil
	}

	// 1. Stage in an isolated temporary directory on the SAME filesystem mount
	stagingDir := filepath.Join(m.CacheDir, fmt.Sprintf("tmp-%s-%d-%d", hash, os.Getpid(), time.Now().UnixNano()))
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return fmt.Errorf("creating staging dir: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	// 2. Write log output
	logFile := filepath.Join(stagingDir, "output.log")
	if err := os.WriteFile(logFile, output, 0644); err != nil {
		return fmt.Errorf("writing cache log: %w", err)
	}

	// 3. Copy artifacts to staging
	artifactsDir := filepath.Join(stagingDir, "artifacts")
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

	// 4. Write metadata JSON
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

	metaFile := filepath.Join(stagingDir, "meta.json")
	if err := os.WriteFile(metaFile, metaBytes, 0644); err != nil {
		return fmt.Errorf("writing cache meta: %w", err)
	}

	// 5. Commit atomically via filesystem rename
	if err := os.Rename(stagingDir, taskCacheDir); err != nil {
		// Handle concurrent write collision: if another process already committed
		// the same content-addressed hash, discard staging and return success
		if m.Has(hash) {
			committed = false // cleanup stagingDir in defer
			return nil
		}
		// If taskCacheDir exists in a broken state from previous non-atomic crash,
		// remove it and retry rename once
		_ = os.RemoveAll(taskCacheDir)
		if retryErr := os.Rename(stagingDir, taskCacheDir); retryErr != nil {
			return fmt.Errorf("committing cache entry: %w", retryErr)
		}
	}

	committed = true
	return nil
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

// Prune removes cache entries older than maxAge based on their recorded Timestamp.
func (m *Manager) Prune(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(m.CacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	pruned := 0

	for _, d := range entries {
		if !d.IsDir() {
			continue
		}
		entryDir := filepath.Join(m.CacheDir, d.Name())

		// Clean up abandoned staging directories older than 5 minutes or older than cutoff
		if strings.HasPrefix(d.Name(), "tmp-") {
			info, err := d.Info()
			if err == nil && (time.Since(info.ModTime()) > 5*time.Minute || info.ModTime().Before(cutoff)) {
				_ = os.RemoveAll(entryDir)
			}
			continue
		}

		metaFile := filepath.Join(entryDir, "meta.json")

		data, err := os.ReadFile(metaFile)
		if err != nil {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}

		if entry.Timestamp.Before(cutoff) {
			if err := os.RemoveAll(entryDir); err == nil {
				pruned++
			}
		}
	}

	return pruned, nil
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
