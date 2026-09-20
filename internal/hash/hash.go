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
// 2. Upstream dependency fingerprints (transitive hashing)
// 3. The specified environment variables (sorted by key)
// 4. The contents of all matching input files (sorted by relative path)
func ComputeTaskFingerprint(baseDir string, command string, inputGlobs []string, envVars []string, depFingerprints map[string]string) (string, error) {
	h := sha256.New()

	// 1. Hash command
	fmt.Fprintf(h, "cmd:%s\n", command)

	// 2. Hash upstream dependency fingerprints in sorted order (transitive hashing)
	if len(depFingerprints) > 0 {
		depNames := make([]string, 0, len(depFingerprints))
		for d := range depFingerprints {
			depNames = append(depNames, d)
		}
		sort.Strings(depNames)
		for _, dep := range depNames {
			fmt.Fprintf(h, "dep:%s=%s\n", dep, depFingerprints[dep])
		}
	}

	// 3. Hash environment variables (sorted)
	sort.Strings(envVars)
	for _, envKey := range envVars {
		val := os.Getenv(envKey)
		fmt.Fprintf(h, "env:%s=%s\n", envKey, val)
	}

	// 4. Resolve input files
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

	ignores := LoadIgnorePatterns(baseDir)
	fileSet := make(map[string]bool)

	for _, rawPattern := range globs {
		pattern := filepath.ToSlash(rawPattern)

		if strings.ContainsAny(pattern, "*?[") {
			// Wildcard pattern: walk directory and match
			err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(baseDir, path)
				if err != nil {
					return err
				}
				if rel == "." {
					return nil
				}
				relSlash := filepath.ToSlash(rel)

				if info.IsDir() {
					if IsIgnored(relSlash, ignores) {
						return filepath.SkipDir
					}
					return nil
				}

				if IsIgnored(relSlash, ignores) {
					return nil
				}

				if MatchGlob(pattern, relSlash) {
					fileSet[relSlash] = true
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			// Literal path: MUST exist or return error
			fullPath := filepath.Join(baseDir, pattern)
			info, err := os.Stat(fullPath)
			if err != nil {
				return nil, fmt.Errorf("declared input %q does not exist: %w", rawPattern, err)
			}
			if info.IsDir() {
				err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					rel, err := filepath.Rel(baseDir, path)
					if err != nil {
						return err
					}
					if rel == "." {
						return nil
					}
					relSlash := filepath.ToSlash(rel)

					if info.IsDir() {
						if IsIgnored(relSlash, ignores) {
							return filepath.SkipDir
						}
						return nil
					}
					if !IsIgnored(relSlash, ignores) {
						fileSet[relSlash] = true
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
			} else {
				rel, err := filepath.Rel(baseDir, fullPath)
				if err == nil {
					relSlash := filepath.ToSlash(rel)
					if !IsIgnored(relSlash, ignores) {
						fileSet[relSlash] = true
					}
				}
			}
		}
	}

	result := make([]string, 0, len(fileSet))
	for f := range fileSet {
		result = append(result, f)
	}
	sort.Strings(result)
	return result, nil
}
