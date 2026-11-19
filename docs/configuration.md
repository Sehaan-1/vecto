# Configuration Guide (`vecto.yaml`)

Every Vecto project is configured with a single `vecto.yaml` file in the root directory.

---

## Schema Overview

```yaml
version: "1"

tasks:
  <task_name>:
    command: <string>
    deps: [<task_name>, ...]
    inputs: [<glob_pattern>, ...]
    outputs: [<path_pattern>, ...]
    env:
      <KEY>: <VALUE>
```

---

## Fields

### `version` (required)
Must be set to `"1"`.

### `tasks` (required)
A map of named tasks defining the dependency graph.

#### Task Properties:
- `command` *(required)*: The shell command to execute. Executed within an isolated process group.
- `deps` *(optional)*: List of task names that must successfully complete before this task can execute.
- `inputs` *(optional)*: List of file globs that determine cache validity. Supports `**/*.ext`, directory globs, and negation.
- `outputs` *(optional)*: Files or directories produced by the task. Staged atomically and restored on cache hits.
- `env` *(optional)*: Environment variables injected into the task execution environment and included in the fingerprint.

---

## Example: Full Pipeline

```yaml
version: "1"

tasks:
  lint:
    command: "golangci-lint run"
    inputs:
      - "**/*.go"
      - ".golangci.yml"

  test:
    command: "go test -v ./..."
    deps: ["lint"]
    inputs:
      - "**/*.go"
      - "go.mod"
      - "go.sum"

  build:
    command: "go build -o bin/app ./cmd/app"
    deps: ["test"]
    inputs:
      - "**/*.go"
      - "go.mod"
      - "go.sum"
    outputs:
      - "bin/app"

  docker:
    command: "docker build -t myapp:latest ."
    deps: ["build"]
    inputs:
      - "bin/app"
      - "Dockerfile"
```
