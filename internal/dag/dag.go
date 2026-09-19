package dag

import (
	"fmt"
	"sort"
	"strings"
)

// Graph represents a directed acyclic graph of tasks.
type Graph struct {
	// dependencies maps task name to tasks it depends on
	dependencies map[string][]string
	// dependents maps task name to tasks that depend on it
	dependents map[string][]string
}

// New creates a new empty Graph.
func New() *Graph {
	return &Graph{
		dependencies: make(map[string][]string),
		dependents:   make(map[string][]string),
	}
}

// AddTask adds a task and its declared dependencies.
func (g *Graph) AddTask(name string, deps []string) {
	if _, exists := g.dependencies[name]; !exists {
		g.dependencies[name] = make([]string, 0)
	}
	for _, dep := range deps {
		g.dependencies[name] = append(g.dependencies[name], dep)
		g.dependents[dep] = append(g.dependents[dep], name)
		if _, exists := g.dependencies[dep]; !exists {
			g.dependencies[dep] = make([]string, 0)
		}
	}
}

// HasTask checks if a task exists in the graph.
func (g *Graph) HasTask(name string) bool {
	_, exists := g.dependencies[name]
	return exists
}

// Tasks returns all task names in deterministic sorted order.
func (g *Graph) Tasks() []string {
	tasks := make([]string, 0, len(g.dependencies))
	for t := range g.dependencies {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)
	return tasks
}

// TopologicalSort returns a 100% deterministic linear order of tasks where all dependencies precede dependents.
// If a cycle exists, it returns an error with the exact cycle path.
func (g *Graph) TopologicalSort() ([]string, error) {
	indegree := make(map[string]int)
	for task := range g.dependencies {
		indegree[task] = len(g.dependencies[task])
	}

	queue := make([]string, 0)
	for task, deg := range indegree {
		if deg == 0 {
			queue = append(queue, task)
		}
	}
	sort.Strings(queue)

	result := make([]string, 0, len(g.dependencies))
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, curr)

		// Sort dependents deterministically before visiting
		deps := make([]string, len(g.dependents[curr]))
		copy(deps, g.dependents[curr])
		sort.Strings(deps)

		for _, dependent := range deps {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
		sort.Strings(queue)
	}

	if len(result) != len(g.dependencies) {
		cycle := g.findCyclePath(indegree)
		return nil, fmt.Errorf("cycle detected in task graph: %s", cycle)
	}

	return result, nil
}

// ExecutionLayers groups tasks into parallel tiers where each layer can be executed concurrently.
func (g *Graph) ExecutionLayers() ([][]string, error) {
	indegree := make(map[string]int)
	for task := range g.dependencies {
		indegree[task] = len(g.dependencies[task])
	}

	currentLayer := make([]string, 0)
	for task, deg := range indegree {
		if deg == 0 {
			currentLayer = append(currentLayer, task)
		}
	}
	sort.Strings(currentLayer)

	var layers [][]string
	processed := 0

	for len(currentLayer) > 0 {
		layers = append(layers, currentLayer)
		processed += len(currentLayer)

		nextLayer := make([]string, 0)
		for _, task := range currentLayer {
			deps := make([]string, len(g.dependents[task]))
			copy(deps, g.dependents[task])
			sort.Strings(deps)

			for _, dependent := range deps {
				indegree[dependent]--
				if indegree[dependent] == 0 {
					nextLayer = append(nextLayer, dependent)
				}
			}
		}
		sort.Strings(nextLayer)
		currentLayer = nextLayer
	}

	if processed != len(g.dependencies) {
		cycle := g.findCyclePath(indegree)
		return nil, fmt.Errorf("cycle detected in task graph: %s", cycle)
	}

	return layers, nil
}

// Ancestors returns the set of all upstream tasks that target directly or transitively depends on.
func (g *Graph) Ancestors(target string) map[string]bool {
	visited := make(map[string]bool)
	var dfs func(u string)
	dfs = func(u string) {
		for _, dep := range g.dependencies[u] {
			if !visited[dep] {
				visited[dep] = true
				dfs(dep)
			}
		}
	}
	dfs(target)
	return visited
}

// NeededTasks returns the set of all tasks required to execute the given targets.
// If targets is empty, all tasks in the graph are needed.
func (g *Graph) NeededTasks(targets []string) map[string]bool {
	if len(targets) == 0 {
		all := make(map[string]bool, len(g.dependencies))
		for t := range g.dependencies {
			all[t] = true
		}
		return all
	}

	needed := make(map[string]bool)
	for _, t := range targets {
		needed[t] = true
		for anc := range g.Ancestors(t) {
			needed[anc] = true
		}
	}
	return needed
}

func (g *Graph) findCyclePath(remaining map[string]int) string {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var path []string

	var dfs func(u string) bool
	dfs = func(u string) bool {
		visited[u] = true
		recStack[u] = true
		path = append(path, u)

		// Sort dependencies deterministically for consistent cycle error messages
		sortedDeps := make([]string, len(g.dependencies[u]))
		copy(sortedDeps, g.dependencies[u])
		sort.Strings(sortedDeps)

		for _, v := range sortedDeps {
			if !visited[v] {
				if dfs(v) {
					return true
				}
			} else if recStack[v] {
				path = append(path, v)
				return true
			}
		}

		recStack[u] = false
		path = path[:len(path)-1]
		return false
	}

	allNodes := make([]string, 0, len(remaining))
	for task, deg := range remaining {
		if deg > 0 {
			allNodes = append(allNodes, task)
		}
	}
	sort.Strings(allNodes)

	for _, task := range allNodes {
		if !visited[task] {
			if dfs(task) {
				return strings.Join(path, " -> ")
			}
		}
	}
	return "unresolved dependencies"
}
