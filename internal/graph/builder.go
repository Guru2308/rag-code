package graph

import (
	"context"
	"strings"

	"github.com/Guru2308/rag-code/internal/domain"
	"github.com/Guru2308/rag-code/internal/logger"
)

// Builder constructs a dependency graph from code chunks
type Builder struct {
	graph *Graph
}

// NewBuilder creates a new graph builder with a fresh graph
func NewBuilder() *Builder {
	return &Builder{
		graph: NewGraph(),
	}
}

// NewBuilderWithGraph creates a new graph builder with an existing graph
func NewBuilderWithGraph(g *Graph) *Builder {
	return &Builder{
		graph: g,
	}
}

// Build constructs the graph from a list of code chunks
func (b *Builder) Build(ctx context.Context, chunks []*domain.CodeChunk) *Graph {
	// First pass: Add all nodes
	for _, chunk := range chunks {
		node := &Node{
			ID:       chunk.ID,
			Type:     string(chunk.ChunkType),
			Name:     chunk.Metadata["name"],
			FilePath: chunk.FilePath,
			Metadata: chunk.Metadata,
		}
		b.graph.AddNode(node)
	}

	// Second pass: Add edges based on relationships
	for _, chunk := range chunks {
		b.addEdgesForChunk(chunk)
	}

	// Third pass: Add parent/child (RelationDefine) edges for class→method
	b.addDefineEdges(chunks)

	stats := b.graph.Stats()
	logger.Info("Built dependency graph", 
		"nodes", stats["nodes"],
		"edges", stats["edges"],
	)
	return b.graph
}

// addEdgesForChunk adds edges for a single chunk
func (b *Builder) addEdgesForChunk(chunk *domain.CodeChunk) {
	edgesAdded := 0
	
	// Handle imports
	if imports, ok := chunk.Metadata["imports"]; ok {
		importList := strings.Split(imports, ",")
		for _, imp := range importList {
			imp = strings.TrimSpace(imp)
			if imp != "" {
				// Find nodes that match this import
				targetNodes := b.graph.GetNodesByName(imp)
				if len(targetNodes) > 0 {
					for _, target := range targetNodes {
						b.graph.AddEdge(chunk.ID, target.ID, RelationImport)
						edgesAdded++
					}
				}
			}
		}
	}

	// Handle function calls
	if calls, ok := chunk.Metadata["calls"]; ok {
		callList := strings.Split(calls, ",")
		logger.Debug("Processing calls for chunk",
			"chunk_id", chunk.ID,
			"chunk_name", chunk.Metadata["name"],
			"calls", calls,
		)
		
		for _, call := range callList {
			call = strings.TrimSpace(call)
			if call == "" {
				continue
			}

			// Try exact match first (for package-level functions)
			funcName := call
			if idx := strings.LastIndex(call, "."); idx != -1 {
				funcName = call[idx+1:]
			}

			targetNodes := b.graph.GetNodesByName(funcName)
			
			// If no exact match and it's a method call (r.Method or obj.Method),
			// try receiver-aware matching
			if len(targetNodes) == 0 && strings.Contains(call, ".") {
				targetNodes = b.findMethodsByReceiver(call, funcName)
			}

			if len(targetNodes) > 0 {
				for _, target := range targetNodes {
					b.graph.AddEdge(chunk.ID, target.ID, RelationCall)
					edgesAdded++
					logger.Debug("Created edge",
						"from", chunk.Metadata["name"],
						"to", target.Name,
						"type", "call",
					)
				}
			} else {
				logger.Debug("No target found for call",
					"call", call,
					"funcName", funcName,
				)
			}
		}
	}
	
	if edgesAdded == 0 && chunk.Metadata["calls"] != "" {
		logger.Debug("No edges created for chunk with calls",
			"chunk_name", chunk.Metadata["name"],
			"calls", chunk.Metadata["calls"],
		)
	}
}

// addDefineEdges creates RelationDefine edges for class/struct → method containment.
// When a method has a receiver (e.g. *MyStruct), we link the type declaration to the method.
func (b *Builder) addDefineEdges(chunks []*domain.CodeChunk) {
	for _, chunk := range chunks {
		if chunk.ChunkType != domain.ChunkTypeMethod {
			continue
		}
		receiver, ok := chunk.Metadata["receiver"]
		if !ok || receiver == "" {
			continue
		}

		// Find the class/struct/type declaration that defines this receiver
		parentNodes := b.findTypeByName(receiver, chunks)
		for _, parent := range parentNodes {
			b.graph.AddEdge(parent.ID, chunk.ID, RelationDefine)
			logger.Debug("Created define edge",
				"parent", parent.Name,
				"child", chunk.Metadata["name"],
			)
		}
	}
}

// findTypeByName finds nodes that represent a type (class/struct) with the given name.
// Matches metadata["name"] or metadata["types"] (comma-separated) across chunks.
func (b *Builder) findTypeByName(typeName string, chunks []*domain.CodeChunk) []*Node {
	seen := make(map[string]bool)
	var result []*Node

	// Direct name match via graph index
	byName := b.graph.GetNodesByName(typeName)
	for _, n := range byName {
		if (n.Type == string(domain.ChunkTypeClass) || n.Type == "struct") && !seen[n.ID] {
			seen[n.ID] = true
			result = append(result, n)
		}
	}

	// Check types metadata for multi-type blocks (e.g. "A,B,C")
	for _, chunk := range chunks {
		if chunk.ChunkType != domain.ChunkTypeClass {
			continue
		}
		if types, ok := chunk.Metadata["types"]; ok {
			for _, t := range strings.Split(types, ",") {
				if strings.TrimSpace(t) == typeName {
					node, exists := b.graph.GetNode(chunk.ID)
					if exists && !seen[node.ID] {
						seen[node.ID] = true
						result = append(result, node)
					}
					break
				}
			}
		}
	}
	return result
}

// findMethodsByReceiver attempts to find methods when receiver notation is used (e.g., r.Method)
func (b *Builder) findMethodsByReceiver(call, methodName string) []*Node {
	// For calls like "r.vectorSearch", find all methods named "vectorSearch"
	// that have a receiver (indicating they're methods, not package functions)
	allWithName := b.graph.GetNodesByName(methodName)
	
	var methods []*Node
	for _, node := range allWithName {
		// If this node has a receiver, it's a method
		if receiver, hasReceiver := node.Metadata["receiver"]; hasReceiver && receiver != "" {
			methods = append(methods, node)
		}
	}
	
	return methods
}

// GetGraph returns the constructed graph
func (b *Builder) GetGraph() *Graph {
	return b.graph
}

// Rebuild clears the graph and rebuilds it from scratch
func (b *Builder) Rebuild(ctx context.Context, chunks []*domain.CodeChunk) *Graph {
	b.graph.Clear()
	return b.Build(ctx, chunks)
}
