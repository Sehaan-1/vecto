package cache

// Entry stores metadata for a cached task execution.
type Entry struct {
	Hash       string   `json:"hash"`
	TaskName   string   `json:"task_name"`
	ExitCode   int      `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
	Artifacts  []string `json:"artifacts"`
}
