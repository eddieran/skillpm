package resolver

import (
	"sort"
	"strings"
	"testing"
)

// assertOrder verifies that every element in before appears before every
// element in after within the result slice.
func assertOrder(t *testing.T, result []string, before, after string) {
	t.Helper()
	bi, ai := -1, -1
	for i, v := range result {
		if v == before {
			bi = i
		}
		if v == after {
			ai = i
		}
	}
	if bi == -1 {
		t.Errorf("expected %q in result %v", before, result)
		return
	}
	if ai == -1 {
		t.Errorf("expected %q in result %v", after, result)
		return
	}
	if bi >= ai {
		t.Errorf("expected %q (idx %d) to come before %q (idx %d) in %v", before, bi, after, ai, result)
	}
}

func TestTopologicalSort_Simple(t *testing.T) {
	// A depends on B, B depends on C  →  order must be C, B, A
	g := NewDepGraph()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOrder(t, result, "C", "B")
	assertOrder(t, result, "B", "A")
}

func TestTopologicalSort_Diamond(t *testing.T) {
	// A -> B, A -> C, B -> D, C -> D  →  D must come first, A must come last
	g := NewDepGraph()
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "D")
	g.AddEdge("C", "D")

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertOrder(t, result, "D", "B")
	assertOrder(t, result, "D", "C")
	assertOrder(t, result, "B", "A")
	assertOrder(t, result, "C", "A")
}

func TestTopologicalSort_CircularDetection(t *testing.T) {
	tests := []struct {
		name  string
		edges [][2]string
	}{
		{
			name:  "self-loop",
			edges: [][2]string{{"A", "A"}},
		},
		{
			name:  "two-node cycle",
			edges: [][2]string{{"A", "B"}, {"B", "A"}},
		},
		{
			name:  "three-node cycle",
			edges: [][2]string{{"A", "B"}, {"B", "C"}, {"C", "A"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewDepGraph()
			for _, e := range tt.edges {
				g.AddEdge(e[0], e[1])
			}
			_, err := g.TopologicalSort()
			if err == nil {
				t.Fatal("expected circular dependency error, got nil")
			}
			if !strings.Contains(err.Error(), "DEP_CIRCULAR") {
				t.Errorf("expected DEP_CIRCULAR in error, got: %v", err)
			}
		})
	}
}

func TestDetectOrphans(t *testing.T) {
	// Graph: app -> lib, app -> util, tool -> lib
	// Installed: app, lib, util, tool
	// Directly requested: app only
	// Expected orphan: tool (lib and util are still needed transitively via app)
	g := NewDepGraph()
	g.AddEdge("app", "lib")
	g.AddEdge("app", "util")
	g.AddEdge("tool", "lib")

	installed := []string{"app", "lib", "util", "tool"}
	directlyRequested := []string{"app"}

	orphans := g.DetectOrphans(installed, directlyRequested)
	sort.Strings(orphans)

	if len(orphans) != 1 || orphans[0] != "tool" {
		t.Errorf("expected [tool] orphan, got %v", orphans)
	}
}

func TestDetectOrphans_NoneWhenAllRequired(t *testing.T) {
	g := NewDepGraph()
	g.AddEdge("A", "B")

	orphans := g.DetectOrphans([]string{"A", "B"}, []string{"A"})
	if len(orphans) != 0 {
		t.Errorf("expected no orphans, got %v", orphans)
	}
}

func TestEmptyGraph(t *testing.T) {
	g := NewDepGraph()

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error on empty graph: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}

	orphans := g.DetectOrphans(nil, nil)
	if len(orphans) != 0 {
		t.Errorf("expected no orphans from empty graph, got %v", orphans)
	}
}

func TestSingleNode(t *testing.T) {
	// A single node with no edges: AddEdge is never called so the node won't
	// appear in the graph at all. Verify that Deps returns nil and TopologicalSort
	// on an effectively-empty graph still works.
	g := NewDepGraph()

	deps := g.Deps("standalone")
	if deps != nil {
		t.Errorf("expected nil deps for unknown node, got %v", deps)
	}

	result, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result for graph with no edges, got %v", result)
	}
}

func TestSingleNode_WithSelfEdge(t *testing.T) {
	// A node that references itself is the simplest possible circular dep.
	g := NewDepGraph()
	g.AddEdge("X", "X")

	_, err := g.TopologicalSort()
	if err == nil {
		t.Fatal("expected error for self-referential node")
	}
}
