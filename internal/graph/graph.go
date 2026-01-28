package graph

import (
	"sync"
)

// RelationType represents the type of relationship between nodes
type RelationType string

const (
	RelationImport RelationType = "import"
	RelationCall   RelationType = "call"
	RelationDefine RelationType = "define"
)

// Node represents a code entity in the graph
type Node struct {
	ID       string            // Unique identifier (chunk ID)
	Type     string            // Type of node: "function", "class", "file"
	Name     string            // Name of the entity
	FilePath string            // File path
	Metadata map[string]string // Additional metadata
}

// Edge represents a relationship between two nodes
type Edge struct {
	From     string       // Source node ID
	To       string       // Target node ID
	Relation RelationType // Type of relationship
}

// Graph represents an in-memory dependency graph
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]*Node      // nodeID -> Node
	edges map[string][]*Edge    // nodeID -> outgoing edges
	index map[string][]string   // name -> nodeIDs (for lookup by name)
}

// NewGraph creates a new empty graph
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]*Node),
		edges: make(map[string][]*Edge),
		index: make(map[string][]string),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[node.ID] = node
	
	// Index by name for efficient lookup
	if node.Name != "" {
		g.index[node.Name] = append(g.index[node.Name], node.ID)
	}
}

// AddEdge adds an edge between two nodes
func (g *Graph) AddEdge(from, to string, relation RelationType) {
	g.mu.Lock()
	defer g.mu.Unlock()

	edge := &Edge{
		From:     from,
		To:       to,
		Relation: relation,
	}
	g.edges[from] = append(g.edges[from], edge)
}

// GetNode retrieves a node by ID
func (g *Graph) GetNode(id string) (*Node, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.nodes[id]
	return node, exists
}

// GetNodesByName retrieves nodes by name
func (g *Graph) GetNodesByName(name string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ids, exists := g.index[name]
	if !exists {
		return nil
	}

	nodes := make([]*Node, 0, len(ids))
	for _, id := range ids {
		if node, ok := g.nodes[id]; ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetRelated retrieves nodes related to the given node ID
func (g *Graph) GetRelated(nodeID string, relationType RelationType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges, exists := g.edges[nodeID]
	if !exists {
		return nil
	}

	related := make([]*Node, 0)
	for _, edge := range edges {
		if edge.Relation == relationType {
			if node, ok := g.nodes[edge.To]; ok {
				related = append(related, node)
			}
		}
	}
	return related
}

// GetAllRelated retrieves all nodes related to the given node ID (any relation type)
func (g *Graph) GetAllRelated(nodeID string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edges, exists := g.edges[nodeID]
	if !exists {
		return nil
	}

	seen := make(map[string]bool)
	related := make([]*Node, 0)
	
	for _, edge := range edges {
		if !seen[edge.To] {
			if node, ok := g.nodes[edge.To]; ok {
				related = append(related, node)
				seen[edge.To] = true
			}
		}
	}
	return related
}

// Clear removes all nodes and edges from the graph
func (g *Graph) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[string]*Node)
	g.edges = make(map[string][]*Edge)
	g.index = make(map[string][]string)
}

// Stats returns statistics about the graph
func (g *Graph) Stats() map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	edgeCount := 0
	for _, edges := range g.edges {
		edgeCount += len(edges)
	}

	return map[string]int{
		"nodes": len(g.nodes),
		"edges": edgeCount,
	}
}
