package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
)

// FingerprintFromCoverage computes a task fingerprint in the VFI (Merkle
// File Index) namespace, ADR-0019. It has the same deterministic structure
// as ComputeTaskFingerprint — command, sorted dependency fingerprints,
// sorted env vars — but the file section is replaced by the glob-coverage
// digest from the Merkle file index, and a scheme marker is included so v2
// keys can never collide with v1 (legacy) keys. The digest is a function of
// exactly the matched (relative path, content) pairs, so the fingerprint
// stays content-addressed (ADR-0004).
func FingerprintFromCoverage(command string, envVars []string, depFingerprints map[string]string, coverageDigest string) string {
	h := sha256.New()

	// 1. Hash command
	fmt.Fprintf(h, "cmd:%s\n", command)

	// 2. Hash upstream dependency fingerprints in sorted order
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
	envs := append([]string{}, envVars...)
	sort.Strings(envs)
	for _, envKey := range envs {
		val := os.Getenv(envKey)
		fmt.Fprintf(h, "env:%s=%s\n", envKey, val)
	}

	// 4. Scheme marker + glob-coverage digest (Merkle file index)
	fmt.Fprintf(h, "scheme:fileindex-v2\n")
	fmt.Fprintf(h, "coverage:%s\n", coverageDigest)

	return hex.EncodeToString(h.Sum(nil))
}
