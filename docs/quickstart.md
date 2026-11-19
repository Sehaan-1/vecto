# Quickstart Guide

Get up and running with **Vecto** in less than 2 minutes.

---

## 1. Installation

### One-Liner (No dependencies needed)

**macOS & Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/Sehaan-1/vecto/master/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/Sehaan-1/vecto/master/install.ps1 | iex
```

### Package Managers & Docker

* **Homebrew:**
  ```bash
  brew install Sehaan-1/vecto/vecto
  ```

* **Scoop (Windows):**
  ```powershell
  scoop install https://raw.githubusercontent.com/Sehaan-1/vecto/master/scoop/vecto.json
  ```

* **Docker:**
  ```bash
  docker run --rm -v "$(pwd):/workspace" -w /workspace ghcr.io/sehaan-1/vecto:latest run build
  ```

* **Go Install:**
  ```bash
  go install github.com/Sehaan-1/vecto/cmd/vecto@latest
  ```

* **Pre-built Binaries:**
  Direct downloads for all architectures on [GitHub Releases](https://github.com/Sehaan-1/vecto/releases/latest).

---

## 2. Initialize a Project

In your project repository, run:

```bash
vecto init
```

This generates a starter `vecto.yaml` file defining a sample lint, test, and build DAG:

```yaml
version: "1"

tasks:
  lint:
    command: "golangci-lint run"
    inputs: ["**/*.go", ".golangci.yml"]

  test:
    command: "go test -v ./..."
    deps: ["lint"]
    inputs: ["**/*.go", "go.mod", "go.sum"]

  build:
    command: "go build -o bin/app ./cmd/app"
    deps: ["test"]
    inputs: ["**/*.go", "go.mod", "go.sum"]
    outputs: ["bin/app"]
```

---

## 3. Run Tasks

Execute your target task and all of its dependencies concurrently:

```bash
vecto run build
```

First run (cold):
```
[-] lint ... running
[✓] lint (1.80s)
[-] test ... running
[✓] test (1.58s)
[-] build ... running
[✓] build (1.77s)

--- Summary ---
Tasks:    3 total (0 cached, 3 executed, 0 failed, 0 skipped)
Duration: 5.61s
```

Second run (hot cache hit):
```bash
vecto run build
```
```
[⚡ CACHED] lint
[⚡ CACHED] test
[⚡ CACHED] build

--- Summary ---
Tasks:    3 total (3 cached, 0 executed, 0 failed, 0 skipped)
Duration: 0.02s
```

---

## 4. Graph Visualization

Inspect your task graph before running:

```bash
# Mermaid graph syntax (renderable in GitHub/Notion)
vecto graph --format=mermaid

# JSON graph (for CI/CD pipeline automation)
vecto graph --format=json
```

---

## 5. Merkle File Index (VFI)

Vecto includes a persistent stat-cached Merkle forest under `.vecto/fileindex.json` that provides O(changed files) incremental fingerprinting. It hashes only what changed and does 0 disk reads when nothing has changed.

```bash
# Inspect the index
vecto index

# Force a complete re-fingerprinting
vecto index --rebuild
```
