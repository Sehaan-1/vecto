package config

import (
	"fmt"
	"sort"
	"strings"
)

// Diagnostic represents a user-friendly validation error or warning.
type Diagnostic struct {
	TaskName string
	Message  string
	Hint     string
}

func (d Diagnostic) Error() string {
	if d.Hint != "" {
		if d.TaskName != "" {
			return fmt.Sprintf("task %q: %s (hint: %s)", d.TaskName, d.Message, d.Hint)
		}
		return fmt.Sprintf("%s (hint: %s)", d.Message, d.Hint)
	}
	if d.TaskName != "" {
		return fmt.Sprintf("task %q: %s", d.TaskName, d.Message)
	}
	return d.Message
}

// Validate performs deep semantic validation of a parsed configuration,
// catching self-dependencies, unknown dependencies with typo suggestions,
// empty commands, and circular dependencies.
func Validate(cfg *Config) error {
	if cfg == nil {
		return Diagnostic{Message: "configuration is nil"}
	}

	if len(cfg.Tasks) == 0 {
		return Diagnostic{
			Message: "vecto.yaml must define at least one task",
			Hint:    "add a task under 'tasks:' in vecto.yaml",
		}
	}

	if cfg.Version != "1" {
		return Diagnostic{
			Message: fmt.Sprintf("unsupported manifest version %q", cfg.Version),
			Hint:    "set 'version: \"1\"' in vecto.yaml",
		}
	}

	allTaskNames := make([]string, 0, len(cfg.Tasks))
	for name := range cfg.Tasks {
		allTaskNames = append(allTaskNames, name)
	}
	sort.Strings(allTaskNames)

	for _, taskName := range allTaskNames {
		task := cfg.Tasks[taskName]

		// 1. Task must either have a command or dependencies (group task)
		if strings.TrimSpace(task.Command) == "" && len(task.Dependencies) == 0 {
			return Diagnostic{
				TaskName: taskName,
				Message:  "task defines neither a command to execute nor dependencies to group",
				Hint:     "provide a 'command:' string or at least one 'deps:' item",
			}
		}

		// 2. Self-dependency check
		for _, dep := range task.Dependencies {
			if dep == taskName {
				return Diagnostic{
					TaskName: taskName,
					Message:  "self-dependency detected: task cannot depend on itself",
					Hint:     fmt.Sprintf("remove %q from deps of %q", dep, taskName),
				}
			}

			// 3. Unknown dependency check with typo suggestions
			if _, exists := cfg.Tasks[dep]; !exists {
				suggestion := SuggestTaskName(dep, allTaskNames)
				hint := fmt.Sprintf("define task %q in vecto.yaml", dep)
				if suggestion != "" {
					hint = fmt.Sprintf("did you mean %q?", suggestion)
				}
				return Diagnostic{
					TaskName: taskName,
					Message:  fmt.Sprintf("depends on non-existent task %q", dep),
					Hint:     hint,
				}
			}
		}

		// 4. Check for overlapping inputs and outputs
		outSet := make(map[string]bool, len(task.Outputs))
		for _, out := range task.Outputs {
			outSet[filepathToSlash(out)] = true
		}
		for _, in := range task.Inputs {
			if outSet[filepathToSlash(in)] {
				return Diagnostic{
					TaskName: taskName,
					Message:  fmt.Sprintf("path %q is declared as both an input and an output", in),
					Hint:     "outputs should not overlap with inputs to avoid circular invalidation",
				}
			}
		}
	}

	// 5. Circular dependency check via DAG
	g, err := cfg.BuildGraph()
	if err != nil {
		return Diagnostic{
			Message: err.Error(),
			Hint:    "break the dependency cycle between the mentioned tasks",
		}
	}
	_ = g

	return nil
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// SuggestTaskName finds the closest task name using Levenshtein distance.
func SuggestTaskName(typo string, validNames []string) string {
	bestMatch := ""
	minDist := 999

	for _, name := range validNames {
		dist := levenshteinDistance(strings.ToLower(typo), strings.ToLower(name))
		if dist < minDist {
			minDist = dist
			bestMatch = name
		}
	}

	// Suggest only if distance is reasonable relative to string length
	maxAllowedDist := len(typo) / 2
	if maxAllowedDist < 2 {
		maxAllowedDist = 2
	}
	if minDist <= maxAllowedDist && minDist < len(bestMatch) {
		return bestMatch
	}
	return ""
}

func levenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	l1, l2 := len(r1), len(r2)

	dp := make([][]int, l1+1)
	for i := range dp {
		dp[i] = make([]int, l2+1)
		dp[i][0] = i
	}
	for j := range dp[0] {
		dp[0][j] = j
	}

	for i := 1; i <= l1; i++ {
		for j := 1; j <= l2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			dp[i][j] = min(
				dp[i-1][j]+1,      // deletion
				dp[i][j-1]+1,      // insertion
				dp[i-1][j-1]+cost, // substitution
			)
		}
	}

	return dp[l1][l2]
}

func min(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
