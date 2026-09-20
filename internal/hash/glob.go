package hash

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var defaultIgnores = []string{
	".git",
	".vecto",
	"node_modules",
	"bin",
	"dist",
	"target",
	"vendor",
	".cache",
}

// LoadIgnorePatterns loads default ignores and rules from .vectoignore if present.
func LoadIgnorePatterns(baseDir string) []string {
	ignores := make([]string, len(defaultIgnores))
	copy(ignores, defaultIgnores)

	ignoreFile := filepath.Join(baseDir, ".vectoignore")
	f, err := os.Open(ignoreFile)
	if err != nil {
		return ignores
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ignores = append(ignores, filepath.ToSlash(line))
	}

	return ignores
}

// IsIgnored reports whether the given slash-separated relative path should be ignored.
func IsIgnored(relPath string, ignores []string) bool {
	relPath = filepath.ToSlash(relPath)
	segments := strings.Split(relPath, "/")

	for _, pattern := range ignores {
		pattern = filepath.ToSlash(pattern)

		// Check if any segment matches directly (e.g. "node_modules")
		for _, seg := range segments {
			if seg == pattern {
				return true
			}
		}

		// Check full glob match
		if MatchGlob(pattern, relPath) {
			return true
		}
	}
	return false
}

// MatchGlob checks if a slash-separated path matches a pattern supporting '**'.
func MatchGlob(pattern, path string) bool {
	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	// Clean trailing slashes
	pattern = strings.TrimSuffix(pattern, "/")
	path = strings.TrimSuffix(path, "/")

	if pattern == "**" {
		return true
	}

	// If no double-star, use filepath.Match
	if !strings.Contains(pattern, "**") {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}

	pSegs := strings.Split(pattern, "/")
	pathSegs := strings.Split(path, "/")

	return matchSegments(pSegs, pathSegs)
}

func matchSegments(pSegs, pathSegs []string) bool {
	if len(pSegs) == 0 {
		return len(pathSegs) == 0
	}

	if pSegs[0] == "**" {
		// Try matching '**' against 0, 1, 2, ... segments of pathSegs
		for i := 0; i <= len(pathSegs); i++ {
			if matchSegments(pSegs[1:], pathSegs[i:]) {
				return true
			}
		}
		return false
	}

	if len(pathSegs) == 0 {
		return false
	}

	matched, err := filepath.Match(pSegs[0], pathSegs[0])
	if err != nil || !matched {
		return false
	}

	return matchSegments(pSegs[1:], pathSegs[1:])
}
