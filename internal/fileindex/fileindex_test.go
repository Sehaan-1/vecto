package fileindex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sehaan-1/vecto/internal/hash"
)

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func ignoresFor(baseDir string) []string { return hash.LoadIgnorePatterns(baseDir) }

// TestColdThenWarmSync: first sync hashes everything; a no-op second sync
// re-hashes nothing and reproduces the same root hash.
func TestColdThenWarmSync(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a", "one.txt"), "1")
	mustWrite(t, filepath.Join(dir, "a", "b", "two.txt"), "22")
	mustWrite(t, filepath.Join(dir, "c", "three.txt"), "333")
	ix := New()
	rep1, err := ix.Sync(dir, ignoresFor(dir), 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep1.FilesHashed != 3 || rep1.FilesTrusted != 0 {
		t.Fatalf("cold sync: hashed=%d trusted=%d, want 3/0", rep1.FilesHashed, rep1.FilesTrusted)
	}
	root1 := ix.RootHash
	if root1 == "" {
		t.Fatal("empty root hash")
	}

	// Back-date the snapshot to avoid the 1-second racy window deterministically.
	ix.Snapshot = time.Now().Add(-2 * time.Second)

	ix2 := New()
	ix2.RootHash = root1
	ix2.Snapshot = ix.Snapshot
	ix2.Nodes = ix.Nodes
	rep2, err := ix2.Sync(dir, ignoresFor(dir), 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.FilesHashed != 0 || rep2.FilesTrusted != 3 {
		t.Fatalf("warm no-op sync: hashed=%d trusted=%d, want 0/3", rep2.FilesHashed, rep2.FilesTrusted)
	}
	if ix2.RootHash != root1 {
		t.Fatalf("root changed with no edits: %s != %s", ix2.RootHash, root1)
	}
}

// TestEditRehashesOnlyEditedFile: editing one deep file re-hashes exactly
// that file, changes exactly the ancestor chain of dir hashes, and leaves
// sibling subtrees untouched.
func TestEditRehashesOnlyEditedFile(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "x", "y", "z", "deep.txt"), "deep")
	mustWrite(t, filepath.Join(dir, "x", "other.txt"), "other")
	mustWrite(t, filepath.Join(dir, "sibling", "s1.txt"), "s1")
	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	rootBefore := ix.RootHash
	siblingBefore := ix.Nodes["sibling"].Hash

	// Back-date the snapshot so the edit is not "racy" for the previous snapshot.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "x", "y", "z", "deep.txt"), "deep-edited")

	rep, err := ix.Sync(dir, ignoresFor(dir), 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep.FilesHashed != 1 {
		t.Fatalf("edited file: hashed=%d, want exactly 1", rep.FilesHashed)
	}
	if ix.RootHash == rootBefore {
		t.Fatal("root hash did not change after edit")
	}
	if ix.Nodes["sibling"].Hash != siblingBefore {
		t.Fatal("sibling subtree hash changed — dirty cutoff failed")
	}
	// Exactly the ancestor chain of the edited file must be recomputed:
	// "x/y/z", "x/y", "x", "" (root) = 4.
	if rep.DirsRecomputed != 4 {
		t.Fatalf("dirs recomputed=%d, want 4 (ancestor chain)", rep.DirsRecomputed)
	}
}

// TestContentRestoreConverges: a file edited and then restored to its exact
// original content produces the original node hash — no false invalidation.
func TestContentRestoreConverges(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "f.txt"), "original")

	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	rootOriginal := ix.RootHash
	fileHashOriginal := ix.Nodes["f.txt"].Hash

	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "f.txt"), "changed")
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if ix.RootHash == rootOriginal {
		t.Fatal("edit not detected")
	}

	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "f.txt"), "original")
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if ix.Nodes["f.txt"].Hash != fileHashOriginal {
		t.Fatalf("restored content produced a different node hash (content addressing broken)")
	}
	if ix.RootHash != rootOriginal {
		t.Fatalf("restored tree produced a different root hash")
	}
}

// TestAddDeleteRename: adds, deletes, and renames all move the root hash the
// right way (paths are part of node hashes, so renames change keys).
func TestAddDeleteRename(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "A")
	mustWrite(t, filepath.Join(dir, "b.txt"), "B")

	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	root0 := ix.RootHash

	// Add.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "c.txt"), "C")
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if ix.RootHash == root0 {
		t.Fatal("add not detected")
	}

	// Delete.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	if err := os.Remove(filepath.Join(dir, "c.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if ix.RootHash != root0 {
		t.Fatalf("delete did not restore the original root hash")
	}

	// Rename.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	if err := os.Rename(filepath.Join(dir, "a.txt"), filepath.Join(dir, "renamed.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if ix.RootHash == root0 {
		t.Fatal("rename not detected (paths must be part of node hashes)")
	}
}

// TestRacyTimestampRehashes: a stored stat in the same second as the last
// snapshot is never trusted (git racy-git rule) — it is re-hashed.
func TestRacyTimestampRehashes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "r.txt"), "racy")

	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}

	// Force the stored mtime into the racy second: exactly the second of the
	// recorded snapshot. Deterministic — no clock dependency.
	stored := ix.Nodes["r.txt"].Stat
	stored.MTime = ix.Snapshot.Unix()
	ix.Nodes["r.txt"].Stat = stored

	rep, err := ix.Sync(dir, ignoresFor(dir), 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep.FilesHashed != 1 {
		t.Fatalf("racy entry: hashed=%d, want 1 (must not trust same-second stat)", rep.FilesHashed)
	}
}

// TestSymlinkAlwaysRehashed: symlinks never use the stat fast path, and an
// in-place edit of the target is detected on the next sync.
func TestSymlinkAlwaysRehashed(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "target.txt"), "v1")
	if err := os.Symlink(filepath.Join(dir, "target.txt"), filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	root1 := ix.RootHash

	// Warm sync: target untouched, but the symlink must still be re-hashed.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	rep, err := ix.Sync(dir, ignoresFor(dir), 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep.FilesHashed != 1 {
		t.Fatalf("symlink warm sync: hashed=%d, want 1 (symlinks are never trusted)", rep.FilesHashed)
	}
	if ix.RootHash != root1 {
		t.Fatal("symlink re-hash changed the root hash with unchanged content")
	}

	// Edit the target in place.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "target.txt"), "v2")
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if ix.RootHash == root1 {
		t.Fatal("in-place target edit not detected via symlink")
	}
}

// TestIncrementalConvergesToFull: an index built incrementally (cold sync,
// edits, syncs) must land on exactly the same root hash as a from-scratch
// index of the final tree state. This is the determinism guarantee the whole
// scheme stands on.
func TestIncrementalConvergesToFull(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "p", "a.txt"), "a1")
	mustWrite(t, filepath.Join(dir, "p", "b.txt"), "b1")

	inc := New()
	if _, err := inc.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}

	steps := []struct{ rel, content string }{
		{"p/c.txt", "c1"},
		{"p/a.txt", "a2"},
		{"q/d.txt", "d1"},
		{"q/d.txt", "d2"},
	}
	for i, s := range steps {
		inc.Snapshot = time.Now().Add(-2 * time.Second)
		p := filepath.Join(dir, s.rel)
		mustWrite(t, p, s.content)
		mt := time.Now().Add(time.Duration(i+1) * time.Second)
		_ = os.Chtimes(p, mt, mt)
		if _, err := inc.Sync(dir, ignoresFor(dir), 4); err != nil {
			t.Fatalf("incremental sync %d: %v", i, err)
		}
	}
	// Delete one file.
	inc.Snapshot = time.Now().Add(-2 * time.Second)
	if err := os.Remove(filepath.Join(dir, "p", "b.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := inc.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}

	full := New()
	if _, err := full.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	if inc.RootHash != full.RootHash {
		t.Fatalf("incremental history diverged from full rebuild:\n incremental=%s\n full       =%s", inc.RootHash, full.RootHash)
	}
	// Node-level equality too (not just the root digest).
	if len(inc.Nodes) != len(full.Nodes) {
		t.Fatalf("node count mismatch: incremental=%d full=%d", len(inc.Nodes), len(full.Nodes))
	}
	for rel, n := range full.Nodes {
		if inc.Nodes[rel] == nil || inc.Nodes[rel].Hash != n.Hash {
			t.Fatalf("node %q hash mismatch: incremental=%v full=%s", rel, inc.Nodes[rel], n.Hash)
		}
	}
}

// TestCoverageCollapse: directory-covering globs collapse to a single dir
// node; partial globs keep per-file covers; the digest is a pure function of
// the covered (path, content) set.
func TestCoverageCollapse(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "packages", "a", "x1.txt"), "1")
	mustWrite(t, filepath.Join(dir, "packages", "a", "x2.txt"), "2")
	// zz.txt does NOT match "x*.txt": it is what keeps packages/a from
	// collapsing for that pattern (a dir cover requires the glob to match
	// every file under it).
	mustWrite(t, filepath.Join(dir, "packages", "a", "zz.txt"), "zz")
	mustWrite(t, filepath.Join(dir, "packages", "b", "y1.txt"), "3")
	mustWrite(t, filepath.Join(dir, "outside", "z.txt"), "4")

	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}

	d1, c1, err := ix.Coverage([]string{"packages/**"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c1) != 1 || c1[0] != "packages" {
		t.Fatalf("packages/** should collapse to the single dir node, got %v", c1)
	}

	d2, c2, err := ix.Coverage([]string{"**"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c2) != 1 || c2[0] != "" {
		t.Fatalf("** should collapse to the root dir node, got %v", c2)
	}

	// Partial glob: only package "a".
	d3, c3, err := ix.Coverage([]string{"packages/a/**"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c3) != 1 || c3[0] != "packages/a" {
		t.Fatalf("packages/a/** should collapse to packages/a, got %v", c3)
	}

	// File-level partial glob.
	_, c4, err := ix.Coverage([]string{"packages/*/x*.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c4) != 2 || c4[0] != "packages/a/x1.txt" || c4[1] != "packages/a/x2.txt" {
		t.Fatalf("packages/*/x*.txt should cover the two x files, got %v", c4)
	}

	// Unrelated edit: coverage of packages must not move.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "outside", "z.txt"), "4-edited")
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	d1b, _, err := ix.Coverage([]string{"packages/**"})
	if err != nil {
		t.Fatal(err)
	}
	if d1b != d1 {
		t.Fatal("coverage changed for an edit outside the glob")
	}

	// Related edit: coverage must move.
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	mustWrite(t, filepath.Join(dir, "packages", "a", "x1.txt"), "1-edited")
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	d1c, _, err := ix.Coverage([]string{"packages/**"})
	if err != nil {
		t.Fatal(err)
	}
	if d1c == d1 {
		t.Fatal("coverage did not change for an edit inside the glob")
	}
	if d2 == d3 {
		t.Fatal("distinct globs over distinct sets produced identical digests")
	}
}

// TestCoverageLiteral: literal inputs cover a single node; a missing literal
// input is an error (parity with legacy); an ignored literal input is a no-op.
func TestCoverageLiteral(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "sub", "in.txt"), "in")
	mustWrite(t, filepath.Join(dir, "node_modules", "dep.js"), "dep")

	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}

	if _, c, err := ix.Coverage([]string{"sub"}); err != nil || len(c) != 1 || c[0] != "sub" {
		t.Fatalf("literal dir input: covers=%v err=%v, want [sub]", c, err)
	}
	if _, c, err := ix.Coverage([]string{"sub/in.txt"}); err != nil || len(c) != 1 || c[0] != "sub/in.txt" {
		t.Fatalf("literal file input: covers=%v err=%v", c, err)
	}
	if _, _, err := ix.Coverage([]string{"missing.txt"}); err == nil {
		t.Fatal("missing literal input must be an error")
	}
	// node_modules is ignored by default: the literal path exists on disk
	// but is excluded — no coverage, no error (parity with legacy).
	if d, c, err := ix.Coverage([]string{"node_modules"}); err != nil || len(c) != 0 {
		t.Fatalf("ignored literal input: covers=%v digest=%s err=%v, want empty", c, d, err)
	}
}

// TestIgnoredPathsExcluded: default-ignored directories are never indexed.
func TestIgnoredPathsExcluded(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "src", "main.go"), "go")
	mustWrite(t, filepath.Join(dir, "node_modules", "junk.js"), "junk")
	mustWrite(t, filepath.Join(dir, "dist", "bundle.js"), "bundle")

	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	for rel := range ix.Nodes {
		if strings.HasPrefix(rel, "node_modules") || strings.HasPrefix(rel, "dist") {
			t.Fatalf("ignored path %q was indexed", rel)
		}
	}
	if _, ok := ix.Nodes["src/main.go"]; !ok {
		t.Fatal("non-ignored file missing from index")
	}
}

// TestLoadSaveRoundtrip: stat tuples survive the roundtrip, so a reload +
// sync is a warm no-op.
func TestLoadSaveRoundtrip(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "k", "v.txt"), "value")
	ix := New()
	if _, err := ix.Sync(dir, ignoresFor(dir), 4); err != nil {
		t.Fatal(err)
	}
	root := ix.RootHash
	ix.Snapshot = time.Now().Add(-2 * time.Second)
	if err := ix.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RootHash != root {
		t.Fatalf("roundtrip root mismatch: %s != %s", loaded.RootHash, root)
	}
	rep, err := loaded.Sync(dir, ignoresFor(dir), 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep.FilesHashed != 0 {
		t.Fatalf("post-reload sync re-hashed %d files, want 0 (stat tuples must persist)", rep.FilesHashed)
	}
}

// TestCorruptedIndexLoadFails: garbage on disk must fail loudly (runner then
// falls back to legacy hashing) rather than silently trusting nothing.
func TestCorruptedIndexLoadFails(t *testing.T) {
	dir := t.TempDir()
	vectoDir := filepath.Join(dir, ".vecto")
	if err := os.MkdirAll(vectoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vectoDir, "fileindex.json"), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("corrupted index must produce a load error")
	}
}

// TestDefHashAndDepsEqual: memoization keys behave.
func TestDefHashAndDepsEqual(t *testing.T) {
	a := DefHash("cmd", []string{"b", "a"}, []string{"Y", "X"}, []string{"d2", "d1"})
	b := DefHash("cmd", []string{"a", "b"}, []string{"X", "Y"}, []string{"d1", "d2"})
	if a != b {
		t.Fatal("def hash must be order-insensitive")
	}
	c := DefHash("cmd2", []string{"a"}, []string{"X"}, []string{"d1"})
	if a == c {
		t.Fatal("different commands must have different def hashes")
	}
	if !DepsEqual(map[string]string{"x": "1"}, map[string]string{"x": "1"}) {
		t.Fatal("equal dep maps must compare equal")
	}
	if DepsEqual(map[string]string{"x": "1"}, map[string]string{"x": "2"}) {
		t.Fatal("different dep values must compare unequal")
	}
}

// TestConcurrentSyncIsDeterministic: many parallel workers over the same tree
// must produce the same root hash as a single-worker sync.
func TestConcurrentSyncIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 500; i++ {
		mustWrite(t, filepath.Join(dir, fmt.Sprintf("d%02d", i%10), fmt.Sprintf("f%03d.txt", i)), fmt.Sprintf("file %d", i))
	}
	ix1 := New()
	if _, err := ix1.Sync(dir, ignoresFor(dir), 1); err != nil {
		t.Fatal(err)
	}
	ix4 := New()
	if _, err := ix4.Sync(dir, ignoresFor(dir), 8); err != nil {
		t.Fatal(err)
	}
	if ix1.RootHash != ix4.RootHash {
		t.Fatal("worker count changed the root hash — sync is not deterministic")
	}
}
