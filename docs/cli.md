# CLI Reference

Command line interface reference for `vecto` and `vecto-server`.

---

## `vecto run`

Execute one or more tasks from `vecto.yaml`.

```bash
vecto run [flags] [tasks...]
```

### Flags:
- `--dry-run`: Traverse the graph and calculate fingerprints without executing any commands.
- `--keep-going`: Continue executing independent tasks even if another task fails.
- `--concurrency <N>`: Maximum number of parallel tasks to execute concurrently (default: number of logical CPUs).
- `--remote-cache <URL>`: HTTP URL of remote cache server (e.g. `http://localhost:8463`).
- `--no-file-index`: Bypass the incremental Merkle File Index and use full directory scanning.
- `--json`: Output execution summary as JSON (ideal for CI/CD metrics).

---

## `vecto graph`

Inspect and export the task dependency graph.

```bash
vecto graph [flags]
```

### Flags:
- `--format=mermaid`: Export as Mermaid diagram code (default).
- `--format=json`: Export topological order and adjacency list as JSON.
- `--format=dot`: Export as Graphviz DOT format.

---

## `vecto index`

Manage the Merkle File Index (VFI).

```bash
vecto index [flags]
```

### Flags:
- `--rebuild`: Invalidate the cached stat tree and re-compute all fingerprints from disk.

---

## `vecto init`

Create a starter `vecto.yaml` in the current working directory.

```bash
vecto init
```

---

## `vecto-server`

Run the HTTP remote cache daemon.

```bash
vecto-server [flags]
```

### Flags:
- `-addr <host:port>`: TCP address to bind to (default: `127.0.0.1:8463`).
- `-dir <path>`: Directory holding cache blobs (default: `.vecto-remote`).
