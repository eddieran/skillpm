package resolver

import "fmt"

// DepGraph represents a directed acyclic graph of skill dependencies.
type DepGraph struct {
	edges map[string][]string // skill -> dependencies
}

// NewDepGraph creates an empty dependency graph.
func NewDepGraph() *DepGraph {
	return &DepGraph{edges: make(map[string][]string)}
}

// AddEdge records that skill `from` depends on skill `to`.
func (g *DepGraph) AddEdge(from, to string) {
	g.edges[from] = append(g.edges[from], to)
}

// Deps returns the direct dependencies of the given skill.
func (g *DepGraph) Deps(skill string) []string {
	return g.edges[skill]
}

// TopologicalSort returns skills in dependency order (dependencies first).
// Returns an error if a circular dependency is detected.
func (g *DepGraph) TopologicalSort() ([]string, error) {
	visited := map[string]int{} // 0=unvisited, 1=in-progress, 2=done
	var result []string

	var visit func(node string) error
	visit = func(node string) error {
		switch visited[node] {
		case 1:
			return fmt.Errorf("DEP_CIRCULAR: circular dependency detected at %q", node)
		case 2:
			return nil
		}
		visited[node] = 1
		for _, dep := range g.edges[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visited[node] = 2
		result = append(result, node)
		return nil
	}

	for node := range g.edges {
		if visited[node] == 0 {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

// DetectOrphans returns skills that are only installed as transitive dependencies
// and are no longer required by any directly-requested skill.
func (g *DepGraph) DetectOrphans(installed, directlyRequested []string) []string {
	needed := make(map[string]bool)
	for _, ref := range directlyRequested {
		needed[ref] = true
		for _, dep := range g.allTransitiveDeps(ref) {
			needed[dep] = true
		}
	}
	var orphans []string
	for _, ref := range installed {
		if !needed[ref] {
			orphans = append(orphans, ref)
		}
	}
	return orphans
}

// allTransitiveDeps returns all transitive dependencies of node (not including node itself).
func (g *DepGraph) allTransitiveDeps(node string) []string {
	visited := map[string]bool{}
	var collect func(n string)
	collect = func(n string) {
		for _, dep := range g.edges[n] {
			if !visited[dep] {
				visited[dep] = true
				collect(dep)
			}
		}
	}
	collect(node)
	result := make([]string, 0, len(visited))
	for dep := range visited {
		result = append(result, dep)
	}
	return result
}
