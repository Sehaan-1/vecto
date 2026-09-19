package config

// TaskConfig defines a single task execution spec.
type TaskConfig struct {
	Command      string   `yaml:"command" json:"command"`
	Dependencies []string `yaml:"deps" json:"deps"`
	Inputs       []string `yaml:"inputs" json:"inputs"`
	Outputs      []string `yaml:"outputs" json:"outputs"`
	Env          []string `yaml:"env" json:"env"`
}

// Config represents the root vecto.yaml manifest.
type Config struct {
	Version string                `yaml:"version" json:"version"`
	Tasks   map[string]TaskConfig `yaml:"tasks" json:"tasks"`
}
