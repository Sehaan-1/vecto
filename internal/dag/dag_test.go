package dag_test

import (
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
