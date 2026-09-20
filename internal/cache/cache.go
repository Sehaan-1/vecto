package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const CurrentSchemaVersion = 1

var (
	ErrCacheCorrupted     = errors.New("cache entry corrupted: integrity check failed")
	ErrIncompatibleSchema = errors.New("cache schema version mismatch")
)

// ArtifactRecord stores file path, checksum, and size for integrity verification.
type ArtifactRecord struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Entry stores metadata for a cached task execution.
type Entry struct {
	SchemaVersion int              `json:"schema_version"`
	Hash          string           `json:"hash"`
	TaskName      string           `json:"task_name"`
	ExitCode      int              `json:"exit_code"`
	DurationMs    int64            `json:"duration_ms"`
	Timestamp     time.Time        `json:"timestamp"`
	LogSHA256     string           `json:"log_sha256"`
	Artifacts     []string         `json:"artifacts"`
	ArtifactMeta  []ArtifactRecord `json:"artifact_meta,omitempty"`
}

// Manager manages reading and writing task execution caches.
type Manager struct {
	BaseDir  string
	CacheDir string
	Remote   RemoteBackend
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

	var remote RemoteBackend
	if remoteURL := os.Getenv("VECTO_REMOTE_CACHE_URL"); remoteURL != "" {
		token := os.Getenv("VECTO_REMOTE_CACHE_TOKEN")
		remote = NewHTTPRemoteBackend(remoteURL, token, 10*time.Second)
	}

	return &Manager{
		BaseDir:  baseDir,
		CacheDir: cacheDir,
		Remote:   remote,
	}
}

// SetRemote assigns a custom remote cache backend.
func (m *Manager) SetRemote(backend RemoteBackend) {
	m.Remote = backend
}

func (m *Manager) lockPath(hash string) string {
	return filepath.Join(m.CacheDir, "locks", hash+".lock")
}

// Has checks if a valid cache entry exists for the given hash locally or remotely.
func (m *Manager) Has(hash string) bool {
	if hash == "" {
		return false
	}
	metaFile := filepath.Join(m.CacheDir, hash, "meta.json")
	if info, err := os.Stat(metaFile); err == nil && !info.IsDir() {
		return true
	}

	if m.Remote != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if ok, err := m.Remote.Has(ctx, hash); err == nil && ok {
			return true
		}
	}

	return false
}

// Store writes metadata, logs, and artifacts atomically into the cache directory.
// It stages all files in a temporary directory inside CacheDir, computes SHA-256
// checksums for all artifacts, acquires an exclusive file lock, and commits via
// an atomic filesystem rename (ADR-0008).
func (m *Manager) Store(hash, taskName string, exitCode int, duration time.Duration, output []byte, outputPaths []string) error {
	if err := os.MkdirAll(m.CacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache root dir: %w", err)
	}

	taskCacheDir := filepath.Join(m.CacheDir, hash)

	// Acquire exclusive file lock for this hash
	lock, err := AcquireLock(m.lockPath(hash), LockExclusive)
	if err == nil {
		defer lock.Unlock()
	}

	// If entry is already valid and complete, no need to overwrite
	if m.hasLocal(hash) {
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

	// 2. Write log output and record log SHA-256
	logFile := filepath.Join(stagingDir, "output.log")
	if err := os.WriteFile(logFile, output, 0644); err != nil {
		return fmt.Errorf("writing cache log: %w", err)
	}
	logSum := sha256.Sum256(output)
	logSHA256 := hex.EncodeToString(logSum[:])

	// 3. Copy artifacts to staging and compute checksums
	artifactsDir := filepath.Join(stagingDir, "artifacts")
	var savedArtifacts []string
	var artifactMetas []ArtifactRecord
	artifactBytesMap := make(map[string][]byte)

	for _, relPath := range outputPaths {
		src := filepath.Join(m.BaseDir, relPath)
		dst := filepath.Join(artifactsDir, relPath)

		if info, err := os.Stat(src); err == nil && !info.IsDir() {
			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return err
			}
			fileBytes, hashHex, err := copyAndHashFile(src, dst)
			if err == nil {
				savedArtifacts = append(savedArtifacts, relPath)
				artifactMetas = append(artifactMetas, ArtifactRecord{
					Path:   relPath,
					SHA256: hashHex,
					Size:   info.Size(),
				})
				artifactBytesMap[relPath] = fileBytes
			}
		}
	}

	// 4. Write metadata JSON with schema version and artifact checksums
	entry := Entry{
		SchemaVersion: CurrentSchemaVersion,
		Hash:          hash,
		TaskName:      taskName,
		ExitCode:      exitCode,
		DurationMs:    duration.Milliseconds(),
		Timestamp:     time.Now().UTC(),
		LogSHA256:     logSHA256,
		Artifacts:     savedArtifacts,
		ArtifactMeta:  artifactMetas,
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
		if m.hasLocal(hash) {
			committed = false
			return nil
		}
		_ = os.RemoveAll(taskCacheDir)
		if retryErr := os.Rename(stagingDir, taskCacheDir); retryErr != nil {
			return fmt.Errorf("committing cache entry: %w", retryErr)
		}
	}

	committed = true

	// 6. Asynchronously upload to remote cache if enabled
	if m.Remote != nil {
		go func(e Entry, log []byte, artMap map[string][]byte) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = m.Remote.Put(ctx, hash, &e, log, artMap)
		}(entry, output, artifactBytesMap)
	}

	return nil
}

func (m *Manager) hasLocal(hash string) bool {
	metaFile := filepath.Join(m.CacheDir, hash, "meta.json")
	info, err := os.Stat(metaFile)
	return err == nil && !info.IsDir()
}

// Restore restores cached outputs, verifying schema version and artifact integrity.
func (m *Manager) Restore(hash string) (*Entry, []byte, error) {
	// Acquire shared lock
	lock, err := AcquireLock(m.lockPath(hash), LockShared)
	if err == nil {
		defer lock.Unlock()
	}

	// If missing locally, try hydrating from remote cache
	if !m.hasLocal(hash) && m.Remote != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		entry, logBytes, artifacts, err := m.Remote.Get(ctx, hash)
		cancel()
		if err == nil && entry != nil {
			// Hydrate to local cache
			_ = m.hydrateLocal(hash, entry, logBytes, artifacts)
		}
	}

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

	// Verify schema version
	if entry.SchemaVersion != CurrentSchemaVersion {
		_ = os.RemoveAll(taskCacheDir)
		return nil, nil, fmt.Errorf("%w: expected %d, got %d", ErrIncompatibleSchema, CurrentSchemaVersion, entry.SchemaVersion)
	}

	logFile := filepath.Join(taskCacheDir, "output.log")
	logBytes, _ := os.ReadFile(logFile)

	// Verify log integrity if checksum present
	if entry.LogSHA256 != "" {
		actualLogSum := sha256.Sum256(logBytes)
		if hex.EncodeToString(actualLogSum[:]) != entry.LogSHA256 {
			_ = os.RemoveAll(taskCacheDir)
			return nil, nil, fmt.Errorf("%w: output.log checksum mismatch", ErrCacheCorrupted)
		}
	}

	// Verify artifact integrity prior to workspace restoration
	artifactsDir := filepath.Join(taskCacheDir, "artifacts")
	if len(entry.ArtifactMeta) > 0 {
		for _, art := range entry.ArtifactMeta {
			cachedArtPath := filepath.Join(artifactsDir, art.Path)
			info, err := os.Stat(cachedArtPath)
			if err != nil || info.IsDir() {
				_ = os.RemoveAll(taskCacheDir)
				return nil, nil, fmt.Errorf("%w: artifact missing: %s", ErrCacheCorrupted, art.Path)
			}
			if info.Size() != art.Size {
				_ = os.RemoveAll(taskCacheDir)
				return nil, nil, fmt.Errorf("%w: artifact size mismatch for %s (expected %d, got %d)", ErrCacheCorrupted, art.Path, art.Size, info.Size())
			}

			// Compute SHA-256
			actualHash, err := hashFile(cachedArtPath)
			if err != nil || actualHash != art.SHA256 {
				_ = os.RemoveAll(taskCacheDir)
				return nil, nil, fmt.Errorf("%w: artifact checksum mismatch for %s", ErrCacheCorrupted, art.Path)
			}
		}
	}

	// Restore artifacts back to project workspace
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

func (m *Manager) hydrateLocal(hash string, entry *Entry, logBytes []byte, artifacts map[string][]byte) error {
	stagingDir := filepath.Join(m.CacheDir, fmt.Sprintf("tmp-remote-%s-%d", hash, time.Now().UnixNano()))
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(stagingDir)

	if err := os.WriteFile(filepath.Join(stagingDir, "output.log"), logBytes, 0644); err != nil {
		return err
	}

	metaBytes, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stagingDir, "meta.json"), metaBytes, 0644); err != nil {
		return err
	}

	artifactsDir := filepath.Join(stagingDir, "artifacts")
	for relPath, data := range artifacts {
		dest := filepath.Join(artifactsDir, relPath)
		_ = os.MkdirAll(filepath.Dir(dest), 0755)
		_ = os.WriteFile(dest, data, 0644)
	}

	finalDir := filepath.Join(m.CacheDir, hash)
	return os.Rename(stagingDir, finalDir)
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
		name := d.Name()
		entryDir := filepath.Join(m.CacheDir, name)

		if name == "locks" {
			continue
		}

		if strings.HasPrefix(name, "tmp-") {
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

func copyAndHashFile(src, dst string) ([]byte, string, error) {
	in, err := os.Open(src)
	if err != nil {
		return nil, "", err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return nil, "", err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return nil, "", err
	}
	defer out.Close()

	hasher := sha256.New()
	var buf bytes.Buffer
	mw := io.MultiWriter(out, hasher, &buf)

	if _, err := io.Copy(mw, in); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), hex.EncodeToString(hasher.Sum(nil)), os.Chmod(dst, info.Mode())
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
