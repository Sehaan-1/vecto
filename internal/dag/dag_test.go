package dag_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Sehaan-1/vecto/internal/dag"
)

func TestDAG_LinearPipeline(t *testing.T) {
	g := dag.New()
	g.AddTask("test", []string{"build"})
	g.AddTask("build", []string{"compile"})
	g.AddTask("compile", nil)

	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"compile", "build", "test"}
	if !reflect.DeepEqual(sorted, expected) {
		t.Errorf("expected %v, got %v", expected, sorted)
	}
}

func TestDAG_DiamondDependency(t *testing.T) {
	// D depends on B and C; B and C both depend on A
	g := dag.New()
	g.AddTask("A", nil)
	g.AddTask("B", []string{"A"})
	g.AddTask("C", []string{"A"})
	g.AddTask("D", []string{"B", "C"})

	layers, err := g.ExecutionLayers()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d: %v", len(layers), layers)
	}

	if !reflect.DeepEqual(layers[0], []string{"A"}) {
		t.Errorf("layer 0 expected [A], got %v", layers[0])
	}
	if !reflect.DeepEqual(layers[1], []string{"B", "C"}) {
		t.Errorf("layer 1 expected [B, C], got %v", layers[1])
	}
	if !reflect.DeepEqual(layers[2], []string{"D"}) {
		t.Errorf("layer 2 expected [D], got %v", layers[2])
	}
}

func TestDAG_DeterministicTopologicalSort(t *testing.T) {
	// Repeat 50 times to ensure Go map iteration randomness never alters the topological order
	for iter := 0; iter < 50; iter++ {
		g := dag.New()
		g.AddTask("D", []string{"B", "C"})
		g.AddTask("C", []string{"A"})
		g.AddTask("B", []string{"A"})
		g.AddTask("A", nil)

		sorted, err := g.TopologicalSort()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := []string{"A", "B", "C", "D"}
		if !reflect.DeepEqual(sorted, expected) {
			t.Fatalf("iteration %d: non-deterministic sort! Expected %v, got %v", iter, expected, sorted)
		}
	}
}

func TestDAG_NeededTasks(t *testing.T) {
	g := dag.New()
	g.AddTask("compile", nil)
	g.AddTask("test", []string{"compile"})
	g.AddTask("lint", nil)
	g.AddTask("docs", nil)

	needed := g.NeededTasks([]string{"test"})
	if !needed["compile"] || !needed["test"] {
		t.Errorf("expected compile and test to be needed, got %v", needed)
	}
	if needed["lint"] || needed["docs"] {
		t.Errorf("lint and docs should not be needed for test, got %v", needed)
	}
}

func TestDAG_CycleDetection(t *testing.T) {
	g := dag.New()
	g.AddTask("A", []string{"B"})
	g.AddTask("B", []string{"C"})
	g.AddTask("C", []string{"A"})

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}

	if !strings.Contains(err.Error(), "cycle detected") {
		t.Errorf("expected cycle error message, got: %v", err)
	}
}

func build1000NodeGraph() *dag.Graph {
	g := dag.New()
	// Build a 100-layer by 10-width grid graph with cross-dependencies
	for layer := 0; layer < 100; layer++ {
		for width := 0; width < 10; width++ {
			nodeName := fmt.Sprintf("node_L%d_W%d", layer, width)
			var deps []string
			if layer > 0 {
				deps = append(deps, fmt.Sprintf("node_L%d_W%d", layer-1, width))
				if width > 0 {
					deps = append(deps, fmt.Sprintf("node_L%d_W%d", layer-1, width-1))
				}
			}
			g.AddTask(nodeName, deps)
		}
	}
	return g
}

func TestDAG_TopologicalOrderValidity(t *testing.T) {
	g := build1000NodeGraph()
	sorted, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Build position index: every node must appear exactly once.
	pos := make(map[string]int, len(sorted))
	for i, name := range sorted {
		if _, dup := pos[name]; dup {
			t.Fatalf("duplicate node in sort output: %q", name)
		}
		pos[name] = i
	}

	// For every edge dep→task, dep must appear strictly before task.
	// We rebuild edge set inline from the same generator.
	for layer := 0; layer < 100; layer++ {
		for width := 0; width < 10; width++ {
			nodeName := fmt.Sprintf("node_L%d_W%d", layer, width)
			if layer > 0 {
				dep1 := fmt.Sprintf("node_L%d_W%d", layer-1, width)
				if pos[dep1] >= pos[nodeName] {
					t.Errorf("edge violation: %q (pos %d) should precede %q (pos %d)", dep1, pos[dep1], nodeName, pos[nodeName])
				}
				if width > 0 {
					dep2 := fmt.Sprintf("node_L%d_W%d", layer-1, width-1)
					if pos[dep2] >= pos[nodeName] {
						t.Errorf("edge violation: %q (pos %d) should precede %q (pos %d)", dep2, pos[dep2], nodeName, pos[nodeName])
					}
				}
			}
		}
	}
}

func BenchmarkDAG_1000Nodes_TopologicalSort(b *testing.B) {
	g := build1000NodeGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.TopologicalSort()
		if err != nil {
			b.Fatalf("sort failed: %v", err)
		}
	}
}

func BenchmarkDAG_1000Nodes_ExecutionLayers(b *testing.B) {
	g := build1000NodeGraph()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := g.ExecutionLayers()
		if err != nil {
			b.Fatalf("layers failed: %v", err)
		}
	}
}

func TestDAG_MermaidExport(t *testing.T) {
	g := dag.New()
	g.AddTask("lint", nil)
	g.AddTask("codegen", nil)
	g.AddTask("test", []string{"lint"})
	g.AddTask("build", []string{"test", "codegen"})
	g.AddTask("isolated", nil)

	// Full graph export
	mermaid, err := g.ToMermaid(nil)
	if err != nil {
		t.Fatalf("ToMermaid failed: %v", err)
	}

	if !strings.HasPrefix(mermaid, "graph TD") {
		t.Errorf("expected graph TD prefix, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "lint --> test") {
		t.Errorf("expected 'lint --> test' in mermaid, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "codegen --> build") {
		t.Errorf("expected 'codegen --> build' in mermaid, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "isolated") {
		t.Errorf("expected 'isolated' node in mermaid, got:\n%s", mermaid)
	}

	// Subgraph export with target "test"
	subMermaid, err := g.ToMermaid([]string{"test"})
	if err != nil {
		t.Fatalf("ToMermaid with target failed: %v", err)
	}
	if !strings.Contains(subMermaid, "lint --> test") {
		t.Errorf("expected 'lint --> test' in sub-mermaid, got:\n%s", subMermaid)
	}
	if strings.Contains(subMermaid, "build") {
		t.Errorf("build should not be present in sub-mermaid targeting 'test'")
	}
	if strings.Contains(subMermaid, "isolated") {
		t.Errorf("isolated should not be present in sub-mermaid targeting 'test'")
	}
}

func TestDAG_DOTExport(t *testing.T) {
	g := dag.New()
	g.AddTask("lint", nil)
	g.AddTask("test", []string{"lint"})
	g.AddTask("build", []string{"test"})

	dot, err := g.ToDOT(nil)
	if err != nil {
		t.Fatalf("ToDOT failed: %v", err)
	}

	if !strings.Contains(dot, "digraph G {") {
		t.Errorf("expected digraph G prefix, got:\n%s", dot)
	}
	if !strings.Contains(dot, `"lint" -> "test";`) {
		t.Errorf("expected '\"lint\" -> \"test\";' in DOT, got:\n%s", dot)
	}
	if !strings.Contains(dot, `"test" -> "build";`) {
		t.Errorf("expected '\"test\" -> \"build\";' in DOT, got:\n%s", dot)
	}
}

