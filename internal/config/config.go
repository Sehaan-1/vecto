package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sehaan-1/vecto/internal/dag"
	"gopkg.in/yaml.v3"
)

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

// LoadConfig reads and parses vecto.yaml from the given directory.
func LoadConfig(dir string) (*Config, error) {
	manifestPath := filepath.Join(dir, "vecto.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Also check vecto.yml
		manifestPath = filepath.Join(dir, "vecto.yml")
		data, err = os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("could not find vecto.yaml in %s: %w", dir, err)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing vecto.yaml: %w", err)
	}

	if len(cfg.Tasks) == 0 {
		return nil, fmt.Errorf("vecto.yaml must define at least one task")
	}

	// Validate dependencies exist
	for taskName, task := range cfg.Tasks {
		for _, dep := range task.Dependencies {
			if _, exists := cfg.Tasks[dep]; !exists {
				return nil, fmt.Errorf("task %q depends on non-existent task %q", taskName, dep)
			}
		}
	}

	return &cfg, nil
}

// BuildGraph constructs a directed acyclic graph from the parsed configuration.
func (c *Config) BuildGraph() (*dag.Graph, error) {
	g := dag.New()
	for taskName, task := range c.Tasks {
		g.AddTask(taskName, task.Dependencies)
	}

	// Validate that there are no cycles
	if _, err := g.TopologicalSort(); err != nil {
		return nil, err
	}

	return g, nil
}
