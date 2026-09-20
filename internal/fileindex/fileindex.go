// Package fileindex implements the Merkle File Index (VFI) — ADR-0019.
//
// VFI is a persistent, content-addressed, stat-cached Merkle forest over the
// working tree. It lets Vecto compute task fingerprints in O(changed) instead
// of O(tree) per run:
//
//   - R1: every file/dir node hashes (path, content) with SHA-256; directory
//     nodes hash their sorted children (Merkle, like git tree objects).
//   - R2: the stat tuple (dev, ino, ctime, mtime, size) is a fast path for
//     trusting a stored content hash (git ce_match_stat model); racy
//     timestamps are re-hashed (git racy-git model); symlinks are always
//     re-hashed.
//   - R3: directory hashes are recomputed bottom-up with dirty cutoff — a
//     directory whose recomputed hash equals the stored one stops invalidation
//     (rustc try-mark-green / git cache-tree model).
//   - R4: task fingerprints digest the *covering nodes* of the input globs,
//     so a directory-covering glob (e.g. packages/**) is O(1), not
//     O(matched files).
//
// The index is never trusted to make a wrong cache decision: any doubt
// (missing entry, stat tuple mismatch, racy timestamp, symlink, corrupted
// index file) degrades to re-hashing content, and a load failure degrades the
// whole run to legacy full-scan hashing in the runner.
package fileindex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sehaan-1/vecto/internal/hash"
)

// SchemaVersion is the on-disk format version. A mismatch forces a full
// rebuild (safe: one cold run).
const SchemaVersion = 1

const (
	indexDirName  = ".vecto"
	indexFileName = "fileindex.json"
)

// Stat is the lstat(2) tuple backing the R2 fast path.
type Stat struct {
	Dev    uint64 `json:"d"`
	Ino    uint64 `json:"i"`
	CTime  int64  `json:"ct"`  // seconds
	CNTime int64  `json:"ctn"` // nanosecond fraction
	MTime  int64  `json:"mt"`  // seconds
	MNTime int64  `json:"mtn"` // nanosecond fraction
	Size   uint64 `json:"sz"`
}

func (s Stat) equal(o Stat) bool {
	return s.Dev == o.Dev && s.Ino == o.Ino &&
		s.CTime == o.CTime && s.CNTime == o.CNTime &&
		s.MTime == o.MTime && s.MNTime == o.MNTime &&
		s.Size == o.Size
}

// Node is one file or directory in the Merkle forest. Paths are
// slash-separated and relative to the base directory; the root directory is
// the empty string.
type Node struct {
	Path  string `json:"p"`
	IsDir bool   `json:"dir"`
	Hash  string `json:"h"`
	Stat  Stat   `json:"s"`
}

// TaskRec caches the previous run's fingerprint for task-level memoization:
// if the task definition, root hash, and dependency fingerprints are all
// unchanged, the stored fingerprint is reused without recomputing coverage.
type TaskRec struct {
	Def  string            `json:"d"`
	Fp   string            `json:"f"`
	Root string            `json:"r"`
	Deps map[string]string `json:"deps,omitempty"`
}

type fileJSON struct {
	V        int                 `json:"v"`
	Snapshot time.Time           `json:"ts"`
	RootHash string              `json:"root"`
	Nodes    map[string]*Node    `json:"nodes"`
	Tasks    map[string]*TaskRec `json:"tasks"`
}

// Index is the in-memory Merkle forest.
type Index struct {
	Schema   int
	Snapshot time.Time
	RootHash string
	Nodes    map[string]*Node
	Tasks    map[string]*TaskRec

	baseDir string

	mu            sync.Mutex
	fileTotals    map[string]int // dir rel -> file descendants incl. direct
	fileTotalsKey string         // root hash the cached totals were built for
}

// SyncReport summarizes one Sync pass (for --stats and tests).
type SyncReport struct {
	FilesSeen      int
	FilesHashed    int // re-hashed (changed/new/racy/symlink)
	FilesTrusted   int // fast path: stat tuple matched, content hash reused
	FilesDeleted   int
	DirsSeen       int
	DirsRecomputed int // recomputed with a changed hash
	DirsSettled    int // recomputed, hash unchanged (dirty cutoff)
	Elapsed        time.Duration
}

// New returns an empty index (a Sync on it is a cold full build).
func New() *Index {
	return &Index{
		Schema: SchemaVersion,
		Nodes:  map[string]*Node{},
		Tasks:  map[string]*TaskRec{},
	}
}

// SetBaseDir records the base directory (used by Coverage's literal-input
// existence check). Sync sets it automatically.
func (ix *Index) SetBaseDir(baseDir string) { ix.baseDir = baseDir }

// IndexPath is where the index persists (under .vecto/, git-ignored).
func IndexPath(baseDir string) string {
	return filepath.Join(baseDir, indexDirName, indexFileName)
}

// Load reads the persisted index. A missing file yields an empty index; a
// corrupted file or schema mismatch yields an error (runner falls back to
// legacy hashing).
func Load(baseDir string) (*Index, error) {
	data, err := os.ReadFile(IndexPath(baseDir))
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, fmt.Errorf("reading file index: %w", err)
	}
	var f fileJSON
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("corrupted file index: %w", err)
	}
	if f.V != SchemaVersion {
		return nil, fmt.Errorf("file index schema %d != expected %d", f.V, SchemaVersion)
	}
	ix := &Index{
		Schema:   f.V,
		Snapshot: f.Snapshot,
		RootHash: f.RootHash,
		Nodes:    f.Nodes,
		Tasks:    f.Tasks,
		baseDir:  baseDir,
	}
	if ix.Nodes == nil {
		ix.Nodes = map[string]*Node{}
	}
	if ix.Tasks == nil {
		ix.Tasks = map[string]*TaskRec{}
	}
	return ix, nil
}

// Save persists the index atomically (tmp + rename, ADR-0008 pattern).
func (ix *Index) Save(baseDir string) error {
	dir := filepath.Join(baseDir, indexDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating %s dir: %w", indexDirName, err)
	}
	data, err := json.Marshal(fileJSON{
		V:        SchemaVersion,
		Snapshot: ix.Snapshot,
		RootHash: ix.RootHash,
		Nodes:    ix.Nodes,
		Tasks:    ix.Tasks,
	})
	if err != nil {
		return fmt.Errorf("encoding file index: %w", err)
	}
	tmp := filepath.Join(dir, fmt.Sprintf("%s.tmp-%d", indexFileName, os.Getpid()))
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("writing file index: %w", err)
	}
	if err := os.Rename(tmp, IndexPath(baseDir)); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("committing file index: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// R1 — node hashing
// ---------------------------------------------------------------------------

// FileNodeHash is the Merkle hash of a file node: (relative path, size,
// content hash). The path is hashed so renames change keys, matching the
// legacy fingerprint rule.
func FileNodeHash(rel string, size uint64, contentHex string) string {
	h := sha256.New()
	fmt.Fprintf(h, "VFI/1 file\x00%s\x00%d\x00%s", rel, size, contentHex)
	return hex.EncodeToString(h.Sum(nil))
}

// DirNodeHash is the Merkle hash of a directory node: (relative path, sorted
// child (name, node hash) pairs). Root is rel "".
func DirNodeHash(rel string, childNames []string, childHashes []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "VFI/1 dir\x00%s\x00", rel)
	for i, name := range childNames {
		fmt.Fprintf(h, "%s\x00%s\x00", name, childHashes[i])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func statTuple(st *syscall.Stat_t) Stat {
	return Stat{
		Dev:    st.Dev,
		Ino:    st.Ino,
		CTime:  st.Ctim.Sec,
		CNTime: st.Ctim.Nsec,
		MTime:  st.Mtim.Sec,
		MNTime: st.Mtim.Nsec,
		Size:   uint64(st.Size),
	}
}

func joinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

func parent(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return ""
	}
	return rel[:i]
}

// depth is the number of path segments below the root. The root itself is
// -1 so it always sorts after (is processed before, in descending order)
// every real directory.
func depth(rel string) int {
	if rel == "" {
		return -1
	}
	return strings.Count(rel, "/")
}

// buildChildren indexes every node by its parent in one O(N) pass, returning
// sorted direct-child name lists (a directory is never its own child — the
// root has no parent entry, and parent("") == "" only for the root itself,
// which is skipped). Calling this once per sync is what keeps the Phase C
// dir pass O(N) instead of O(N × dirs): scanning the whole node map per
// directory costs ~100M iterations on a 100k-file tree.
func buildChildren(nodes map[string]*Node) map[string][]string {
	children := make(map[string][]string, len(nodes))
	for p := range nodes {
		if p == "" {
			continue // the root has no parent
		}
		d := parent(p)
		name := p
		if d != "" {
			name = p[len(d)+1:]
		}
		children[d] = append(children[d], name)
	}
	for _, names := range children {
		sort.Strings(names)
	}
	return children
}

// ---------------------------------------------------------------------------
// R2 + R3 — Sync: stat cascade with racy re-hash, then dirty-cutoff dir pass
// ---------------------------------------------------------------------------

// Sync walks baseDir, refreshes the Merkle forest, and returns a report.
// ignores must be the loaded ignore patterns (hash.LoadIgnorePatterns).
// workers <= 0 selects a sensible default (8, capped at 16).
func (ix *Index) Sync(baseDir string, ignores []string, workers int) (*SyncReport, error) {
	start := time.Now()
	if workers <= 0 {
		workers = 8
	}
	if workers > 16 {
		workers = 16
	}
	ix.baseDir = baseDir
	oldNodes := ix.Nodes

	// Snapshot time taken BEFORE the walk. It is stored with the index and
	// drives the racy rule on the NEXT sync: a file whose recorded stat
	// timestamp falls in the same second as this snapshot may have been
	// modified after we read the stat (coarse-timestamp filesystems), so it
	// is re-hashed rather than trusted — the same rule and the same accepted
	// residual risk as git's racy-git handling.
	snapshot := time.Now()

	// --- Phase A: parallel crawl, lstat everything ---
	type seenFile struct {
		st        Stat
		isSymlink bool
	}
	seenFiles := map[string]*seenFile{}
	seenDirs := map[string]Stat{}
	var (
		ef   sync.Mutex
		fErr error
	)
	// Directory work is coordinated with an in-flight counter: every dir in
	// the queue or being processed keeps the count ≥ 1, so it can only
	// reach 0 when the crawl is complete — at which point (and only then)
	// the queue is closed and workers drain out.
	queue := make(chan string, 8192)
	var inflight sync.WaitGroup
	inflight.Add(1)
	queue <- ""
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for rel := range queue {
				abs := baseDir
				if rel != "" {
					abs = filepath.Join(baseDir, rel)
				}
				entries, err := os.ReadDir(abs)
				if err != nil {
					if rel == "" {
						ef.Lock()
						if fErr == nil {
							fErr = fmt.Errorf("reading base dir: %w", err)
						}
						ef.Unlock()
						inflight.Done()
						return
					}
					// Subtree vanished mid-walk (deleted concurrently): skip
					// it rather than failing the whole run.
					if _, statErr := os.Lstat(abs); os.IsNotExist(statErr) {
						inflight.Done()
						continue
					}
					ef.Lock()
					if fErr == nil {
						fErr = fmt.Errorf("reading dir %s: %w", rel, err)
					}
					ef.Unlock()
					inflight.Done()
					return
				}
				for _, e := range entries {
					name := e.Name()
					relPath := joinRel(rel, name)
					if hash.IsIgnored(relPath, ignores) {
						continue
					}
					absPath := filepath.Join(abs, name)
					var st syscall.Stat_t
					if err := syscall.Lstat(absPath, &st); err != nil {
						if os.IsNotExist(err) {
							continue
						}
						ef.Lock()
						if fErr == nil {
							fErr = fmt.Errorf("stat %s: %w", relPath, err)
						}
						ef.Unlock()
						inflight.Done()
						return
					}
					if e.IsDir() {
						ef.Lock()
						seenDirs[relPath] = statTuple(&st)
						ef.Unlock()
						inflight.Add(1)
						queue <- relPath
						continue
					}
					ef.Lock()
					seenFiles[relPath] = &seenFile{st: statTuple(&st), isSymlink: e.Type()&fs.ModeSymlink != 0}
					ef.Unlock()
				}
				inflight.Done() // this dir is fully processed (children already added)
			}
		}()
	}
	inflight.Wait()
	close(queue)
	wg.Wait()
	ef.Lock()
	crawlErr := fErr
	ef.Unlock()
	if crawlErr != nil {
		return nil, crawlErr
	}

	report := &SyncReport{FilesSeen: len(seenFiles), DirsSeen: len(seenDirs)}

	// --- Phase B: R2 fast-path decision, then parallel re-hash ---
	newNodes := make(map[string]*Node, len(seenFiles)+len(seenDirs)+1)
	type hashOutcome struct {
		rel  string
		node *Node // nil means "drop this path"
		err  error
	}
	hashRes := make(chan hashOutcome, len(seenFiles))
	var (
		hwg   sync.WaitGroup
		hErrs int
		hJobs int
	)
	for rel, sf := range seenFiles {
		old, exists := oldNodes[rel]
		needsHash := false
		if !exists || old.IsDir {
			needsHash = true // new, or was a directory last time
		} else if sf.isSymlink {
			needsHash = true // symlink metadata does not track its target
		} else if !old.Stat.equal(sf.st) {
			needsHash = true // dev/ino/ctime/mtime/size changed
		} else if !ix.Snapshot.IsZero() &&
			(old.Stat.MTime == ix.Snapshot.Unix() || old.Stat.CTime == ix.Snapshot.Unix()) {
			needsHash = true // racy: stat in the same second as the last snapshot
		}
		if needsHash {
			hJobs++
			hwg.Add(1)
			go func(rel string, st Stat) {
				defer hwg.Done()
				abs := filepath.Join(baseDir, rel)
				f, err := os.Open(abs)
				if err != nil {
					if os.IsNotExist(err) {
						hashRes <- hashOutcome{rel: rel} // deleted mid-sync
						return
					}
					hashRes <- hashOutcome{rel: rel, err: fmt.Errorf("opening %s: %w", rel, err)}
					return
				}
				h := sha256.New()
				n, err := io.Copy(h, f)
				if err == nil {
					// fstat the open fd: catches a path swapped for a
					// directory between Lstat and Open.
					fi, serr := f.Stat()
					if serr != nil {
						err = serr
					} else if fi.IsDir() {
						err = fmt.Errorf("changed type during sync; re-run")
					}
				}
				if cerr := f.Close(); err == nil {
					err = cerr
				}
				if err != nil {
					hashRes <- hashOutcome{rel: rel, err: fmt.Errorf("hashing %s: %w", rel, err)}
					return
				}
				hashRes <- hashOutcome{rel: rel, node: &Node{
					Path:  rel,
					IsDir: false,
					Hash:  FileNodeHash(rel, uint64(n), hex.EncodeToString(h.Sum(nil))),
					Stat:  st,
				}}
			}(rel, sf.st)
			continue
		}
		// Fast path: trust the stored content hash (zero file reads).
		newNodes[rel] = &Node{Path: rel, IsDir: false, Hash: old.Hash, Stat: sf.st}
	}
	hwg.Wait()
	close(hashRes)
	for res := range hashRes {
		if res.err != nil {
			hErrs++
			continue
		}
		if res.node == nil {
			continue // deleted between ReadDir and Open
		}
		newNodes[res.rel] = res.node
	}
	report.FilesHashed = hJobs
	if hErrs > 0 {
		return nil, fmt.Errorf("file index sync had %d file error(s); legacy hashing will be used", hErrs)
	}
	// Directory nodes (hash computed in Phase C).
	for rel, st := range seenDirs {
		old, exists := oldNodes[rel]
		oldHash := ""
		if exists {
			oldHash = old.Hash
		}
		newNodes[rel] = &Node{Path: rel, IsDir: true, Hash: oldHash, Stat: st}
	}
	newNodes[""] = &Node{Path: "", IsDir: true, Stat: rootStat(baseDir)}

	// Deletions: anything in the old index that was not seen is gone.
	for rel := range oldNodes {
		if rel == "" {
			continue
		}
		if _, ok := newNodes[rel]; !ok {
			report.FilesDeleted++
		}
	}

	// --- Phase C: R3 bottom-up dir hashes with dirty cutoff ---
	var dirRels []string
	for rel := range seenDirs {
		dirRels = append(dirRels, rel)
	}
	dirRels = append(dirRels, "")
	sort.Slice(dirRels, func(i, j int) bool {
		di, dj := depth(dirRels[i]), depth(dirRels[j])
		if di != dj {
			return di > dj
		}
		return dirRels[i] < dirRels[j]
	})

	// Direct-child lists, precomputed once per sync (O(N) total).
	oldChildren := buildChildren(oldNodes)
	newChildren := buildChildren(newNodes)

	// Mark dirty: a dir needs recomputation if a direct child is new/deleted
	// or a direct file child's hash changed.
	dirty := map[string]bool{}
	for rel, n := range newNodes {
		if n.IsDir {
			continue
		}
		old, exists := oldNodes[rel]
		if !exists || old.Hash != n.Hash {
			dirty[parent(rel)] = true
		}
	}
	for rel := range seenDirs {
		if _, exists := oldNodes[rel]; !exists {
			dirty[parent(rel)] = true
		}
	}
	// Child-set changes (add/remove) at every depth, including the root. Both
	// lists are sorted, so compare element-wise.
	for _, rel := range dirRels {
		if !sameSorted(oldChildren[rel], newChildren[rel]) {
			dirty[rel] = true
		}
	}

	for _, rel := range dirRels {
		n := newNodes[rel]
		old, exists := oldNodes[rel]
		oldHash := ""
		if exists {
			oldHash = old.Hash
		}
		if !dirty[rel] && n.Hash == oldHash && oldHash != "" {
			continue // settled without recomputation
		}
		names := newChildren[rel]
		hashes := make([]string, len(names))
		for i, name := range names {
			childRel := name
			if rel != "" {
				childRel = rel + "/" + name
			}
			hashes[i] = newNodes[childRel].Hash
		}
		newHash := DirNodeHash(rel, names, hashes)
		if newHash == oldHash {
			report.DirsSettled++ // try-mark-green: stop propagation
			delete(dirty, rel)
		} else {
			report.DirsRecomputed++
			dirty[parent(rel)] = true
		}
		n.Hash = newHash
	}

	root, ok := newNodes[""]
	if !ok || root.Hash == "" {
		return nil, fmt.Errorf("base directory missing")
	}
	ix.RootHash = root.Hash
	ix.Nodes = newNodes
	ix.Snapshot = snapshot
	report.FilesTrusted = report.FilesSeen - report.FilesHashed
	report.Elapsed = time.Since(start)
	ix.mu.Lock()
	ix.fileTotalsKey = "" // invalidate cached coverage totals
	ix.mu.Unlock()
	return report, nil
}

func rootStat(baseDir string) Stat {
	var st syscall.Stat_t
	if err := syscall.Lstat(baseDir, &st); err != nil {
		return Stat{}
	}
	return statTuple(&st)
}

// sameSorted reports whether two sorted string slices are identical.
func sameSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// R4 — Coverage: the glob-coverage digest a task fingerprint hashes
// ---------------------------------------------------------------------------

// Coverage resolves patterns against the index and returns the digest over
// the covering nodes plus the sorted list of covering paths. A wildcard
// pattern that matches a whole subtree collapses to the single covering
// directory node (O(1) per such pattern); partial patterns collapse to the
// maximal set of dir/file nodes exactly covering the matched file set.
func (ix *Index) Coverage(patterns []string) (digest string, covers []string, err error) {
	seenP := map[string]bool{}
	unique := make([]string, 0, len(patterns))
	for _, raw := range patterns {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p == "" || seenP[p] {
			continue
		}
		seenP[p] = true
		unique = append(unique, p)
	}

	totals := ix.fileTotalsCache()
	coverSet := map[string]bool{}
	for _, pattern := range unique {
		if !strings.ContainsAny(pattern, "*?[") {
			// Literal path: one node, or (exists but ignored) nothing —
			// parity with legacy resolveInputFiles.
			norm := path.Clean(pattern)
			if _, ok := ix.Nodes[norm]; ok {
				coverSet[norm] = true
			} else if ix.baseDir != "" {
				if _, statErr := os.Stat(filepath.Join(ix.baseDir, norm)); statErr != nil {
					return "", nil, fmt.Errorf("declared input %q does not exist", pattern)
				}
				// Present on disk but excluded (ignored) → no coverage.
			} else if _, statErr := os.Stat(norm); statErr != nil {
				return "", nil, fmt.Errorf("declared input %q does not exist", pattern)
			}
			continue
		}
		// Wildcard: matched files, then collapse to maximal covering nodes.
		counts := map[string]int{} // dir rel -> matched-not-yet-covered files under it
		var matched []string
		for rel, n := range ix.Nodes {
			if n.IsDir || !hash.MatchGlob(pattern, rel) {
				continue
			}
			matched = append(matched, rel)
			for a := parent(rel); ; a = parent(a) {
				counts[a]++
				if a == "" {
					break
				}
			}
		}
		var dirs []string
		for rel, n := range ix.Nodes {
			if n.IsDir {
				dirs = append(dirs, rel)
			}
		}
		// Parents before children: a fully-matched subtree collapses to the
		// highest directory that covers it; subsumption pruning below drops
		// any redundant descendant covers.
		sort.Slice(dirs, func(i, j int) bool {
			di, dj := depth(dirs[i]), depth(dirs[j])
			if di != dj {
				return di < dj
			}
			return dirs[i] < dirs[j]
		})
		for _, d := range dirs {
			c := counts[d]
			if c == 0 {
				continue
			}
			if c == totals[d] {
				coverSet[d] = true
				for a := parent(d); ; a = parent(a) {
					counts[a] -= totals[d]
					if a == "" {
						break
					}
				}
			}
		}
		for _, f := range matched {
			covered := false
			for a := parent(f); ; a = parent(a) {
				if coverSet[a] {
					covered = true
					break
				}
				if a == "" {
					break
				}
			}
			if !covered {
				coverSet[f] = true
			}
		}
	}

	// Prune subsumed covers: a cover that sits under another cover is
	// redundant. This makes the cover set the maximal (canonical) covering
	// for the given pattern list, so equivalent patterns yield identical
	// digests. (parent("") == "", so the root has no ancestors and is never
	// pruned.)
	for p := range coverSet {
		if p == "" {
			continue
		}
		for a := parent(p); ; a = parent(a) {
			if coverSet[a] {
				delete(coverSet, p)
				break
			}
			if a == "" {
				break
			}
		}
	}

	covers = make([]string, 0, len(coverSet))
	for p := range coverSet {
		covers = append(covers, p)
	}
	sort.Strings(covers)
	h := sha256.New()
	for _, p := range covers {
		fmt.Fprintf(h, "cov:%s\x00%s\n", p, ix.Nodes[p].Hash)
	}
	return hex.EncodeToString(h.Sum(nil)), covers, nil
}

// fileTotalsCache returns (lazily, per root hash) the count of file
// descendants (including direct) for every directory, used by Coverage's
// collapse step.
func (ix *Index) fileTotalsCache() map[string]int {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.fileTotals != nil && ix.fileTotalsKey == ix.RootHash {
		return ix.fileTotals
	}
	totals := map[string]int{}
	for rel, n := range ix.Nodes {
		if n.IsDir {
			continue
		}
		for a := parent(rel); ; a = parent(a) {
			totals[a]++
			if a == "" {
				break
			}
		}
	}
	ix.fileTotals = totals
	ix.fileTotalsKey = ix.RootHash
	return totals
}

// ---------------------------------------------------------------------------
// Task-level helpers
// ---------------------------------------------------------------------------

// DefHash identifies a task definition for fingerprint memoization: the
// command, its input globs, its env vars, and its dependency names.
func DefHash(command string, inputs []string, env []string, depNames []string) string {
	h := sha256.New()
	fmt.Fprintf(h, "def\x00cmd=%s\x00", command)
	in := append([]string{}, inputs...)
	sort.Strings(in)
	for _, g := range in {
		fmt.Fprintf(h, "in=%s\x00", g)
	}
	envs := append([]string{}, env...)
	sort.Strings(envs)
	for _, e := range envs {
		fmt.Fprintf(h, "env=%s\x00", e)
	}
	deps := append([]string{}, depNames...)
	sort.Strings(deps)
	for _, d := range deps {
		fmt.Fprintf(h, "dep=%s\x00", d)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DepsEqual reports whether two dependency-fingerprint maps are identical.
func DepsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
