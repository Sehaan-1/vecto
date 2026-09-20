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
		// Maintain dependents in sorted order via a single insertion step so
		// ExecutionLayers can iterate directly without per-task copies.
		dl := g.dependents[dep]
		i := len(dl)
		dl = append(dl, name)
		for i > 0 && dl[i] < dl[i-1] {
			dl[i], dl[i-1] = dl[i-1], dl[i]
			i--
		}
		g.dependents[dep] = dl
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

// Dependents returns the tasks that directly depend on the given task,
// in sorted order. The slice must not be modified by the caller.
func (g *Graph) Dependents(task string) []string {
	return g.dependents[task]
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
// It uses an in-place typed min-heap to guarantee lexicographical tie-breaking in O((V + E) log V) with zero interface boxing.
// If a cycle exists, it returns an error with the exact cycle path.
func (g *Graph) TopologicalSort() ([]string, error) {
	indegree := make(map[string]int, len(g.dependencies))
	for task := range g.dependencies {
		indegree[task] = len(g.dependencies[task])
	}

	pq := make([]string, 0, len(g.dependencies))
	for task, deg := range indegree {
		if deg == 0 {
			heapPushString(&pq, task)
		}
	}

	result := make([]string, 0, len(g.dependencies))
	for len(pq) > 0 {
		curr := heapPopString(&pq)
		result = append(result, curr)

		for _, dependent := range g.dependents[curr] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				heapPushString(&pq, dependent)
			}
		}
	}

	if len(result) != len(g.dependencies) {
		cycle := g.findCyclePath(indegree)
		return nil, fmt.Errorf("cycle detected in task graph: %s", cycle)
	}

	return result, nil
}

func heapPushString(h *[]string, x string) {
	*h = append(*h, x)
	j := len(*h) - 1
	for {
		i := (j - 1) / 2
		if i == j || !((*h)[j] < (*h)[i]) {
			break
		}
		(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
		j = i
	}
}

func heapPopString(h *[]string) string {
	a := *h
	n := len(a) - 1
	a[0], a[n] = a[n], a[0]
	// sift down
	i := 0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}
		j := j1
		if j2 := j1 + 1; j2 < n && a[j2] < a[j1] {
			j = j2
		}
		if !(a[j] < a[i]) {
			break
		}
		a[i], a[j] = a[j], a[i]
		i = j
	}
	x := a[n]
	*h = a[0:n]
	return x
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
			// dependents[task] is already sorted (maintained by AddTask),
			// so no per-task copy or sort is needed here.
			for _, dependent := range g.dependents[task] {
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

// ToMermaid generates a deterministic Mermaid.js diagram representing the DAG.
// Dependencies point to dependents (upstream --> downstream).
// If targets are provided, only tasks in the needed subgraph are included.
func (g *Graph) ToMermaid(targets []string) (string, error) {
	if _, err := g.TopologicalSort(); err != nil {
		return "", err
	}

	needed := g.NeededTasks(targets)
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	tasks := make([]string, 0, len(needed))
	for t := range needed {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)

	connected := make(map[string]bool)

	// In Vecto, if task T has dependency D, D executes before T (D --> T)
	for _, task := range tasks {
		deps := make([]string, 0)
		for _, dep := range g.dependencies[task] {
			if needed[dep] {
				deps = append(deps, dep)
			}
		}
		sort.Strings(deps)

		for _, dep := range deps {
			sb.WriteString(fmt.Sprintf("  %s --> %s\n", dep, task))
			connected[dep] = true
			connected[task] = true
		}
	}

	// Output isolated nodes that have no edges
	for _, task := range tasks {
		if !connected[task] {
			sb.WriteString(fmt.Sprintf("  %s\n", task))
		}
	}

	return strings.TrimRight(sb.String(), "\n"), nil
}

// ToDOT generates a deterministic Graphviz DOT representation of the DAG.
func (g *Graph) ToDOT(targets []string) (string, error) {
	if _, err := g.TopologicalSort(); err != nil {
		return "", err
	}

	needed := g.NeededTasks(targets)
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=TB;\n")

	tasks := make([]string, 0, len(needed))
	for t := range needed {
		tasks = append(tasks, t)
	}
	sort.Strings(tasks)

	connected := make(map[string]bool)

	for _, task := range tasks {
		deps := make([]string, 0)
		for _, dep := range g.dependencies[task] {
			if needed[dep] {
				deps = append(deps, dep)
			}
		}
		sort.Strings(deps)

		for _, dep := range deps {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", dep, task))
			connected[dep] = true
			connected[task] = true
		}
	}

	// Output isolated nodes
	for _, task := range tasks {
		if !connected[task] {
			sb.WriteString(fmt.Sprintf("  \"%s\";\n", task))
		}
	}

	sb.WriteString("}")
	return sb.String(), nil
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
