package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ComputeTaskFingerprint calculates a deterministic SHA-256 hex string over:
// 1. The command string
// 2. The contents of all matching input files (sorted by relative path)
// 3. The specified environment variables (sorted by key)
func ComputeTaskFingerprint(baseDir string, command string, inputGlobs []string, envVars []string) (string, error) {
	h := sha256.New()

	// 1. Hash command
	fmt.Fprintf(h, "cmd:%s\n", command)

	// 2. Hash environment variables (sorted)
	sort.Strings(envVars)
	for _, envKey := range envVars {
		val := os.Getenv(envKey)
		fmt.Fprintf(h, "env:%s=%s\n", envKey, val)
	}

	// 3. Resolve input files
	matchedFiles, err := resolveInputFiles(baseDir, inputGlobs)
	if err != nil {
		return "", fmt.Errorf("resolving input files: %w", err)
	}

	sort.Strings(matchedFiles)

	for _, relPath := range matchedFiles {
		fullPath := filepath.Join(baseDir, relPath)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			return "", fmt.Errorf("stat input file %s: %w", relPath, err)
		}
		if fileInfo.IsDir() {
			continue
		}

		// Hash the relative path so file renaming changes the key
		fmt.Fprintf(h, "file:%s\n", filepath.ToSlash(relPath))

		f, err := os.Open(fullPath)
		if err != nil {
			return "", fmt.Errorf("opening input file %s: %w", relPath, err)
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", fmt.Errorf("hashing input file %s: %w", relPath, err)
		}
		f.Close()
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func resolveInputFiles(baseDir string, globs []string) ([]string, error) {
	if len(globs) == 0 {
		return nil, nil
	}

	fileSet := make(map[string]bool)

	for _, pattern := range globs {
		// Clean and normalize pattern
		pattern = filepath.ToSlash(pattern)

		// Check if pattern has wildcard
		if strings.Contains(pattern, "*") {
			// Walk directory and match
			err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					baseName := filepath.Base(path)
					if baseName == ".git" || baseName == ".vecto" || baseName == "bin" {
						return filepath.SkipDir
					}
					return nil
				}

				rel, err := filepath.Rel(baseDir, path)
				if err != nil {
					return err
				}
				relSlash := filepath.ToSlash(rel)

				matched, err := matchGlob(pattern, relSlash)
				if err == nil && matched {
					fileSet[relSlash] = true
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			// Direct file or directory
			fullPath := filepath.Join(baseDir, pattern)
			info, err := os.Stat(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					// Pattern refers to non-existent file, skip or treat as empty
					continue
				}
				return nil, err
			}
			if info.IsDir() {
				err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						rel, err := filepath.Rel(baseDir, path)
						if err == nil {
							fileSet[filepath.ToSlash(rel)] = true
						}
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
			} else {
				rel, err := filepath.Rel(baseDir, fullPath)
				if err == nil {
					fileSet[filepath.ToSlash(rel)] = true
				}
			}
		}
	}

	result := make([]string, 0, len(fileSet))
	for f := range fileSet {
		result = append(result, f)
	}
	return result, nil
}

func matchGlob(pattern, path string) (bool, error) {
	// Simple support for recursive glob ** or single-level *
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "/**/")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			if prefix != "" && !strings.HasPrefix(path, prefix+"/") {
				return false, nil
			}
			matched, err := filepath.Match(suffix, filepath.Base(path))
			return matched, err
		}
	}
	return filepath.Match(pattern, path)
}
