package link

import (
	"sync"
)

// LinkAnalyzer performs link analysis on CDR relationships
type LinkAnalyzer struct {
	mu       sync.RWMutex
	graph    *Graph
}

// Graph represents a call relationship graph
type Graph struct {
	Nodes map[string]*Node
	Edges map[string]*Edge
}

// Node represents a phone number in the graph
type Node struct {
	ID           string            `json:"id"`
	PhoneNumber  string            `json:"phone_number"`
	Label        string            `json:"label,omitempty"`
	TotalCalls   int               `json:"total_calls"`
	UniqueTargets int              `json:"unique_targets"`
	Properties   map[string]string `json:"properties,omitempty"`
}

// Edge represents a call relationship between two nodes
type Edge struct {
	ID          string  `json:"id"`
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	CallCount   int     `json:"call_count"`
	TotalDuration int   `json:"total_duration"`
	AvgDuration float64 `json:"avg_duration"`
	FirstCall   string  `json:"first_call"`
	LastCall    string  `json:"last_call"`
}

// Cluster represents a group of closely connected numbers
type Cluster struct {
	ID          string   `json:"id"`
	NodeIDs     []string `json:"node_ids"`
	Size        int      `json:"size"`
	Density     float64  `json:"density"`
	Centrality  float64  `json:"centrality"`
}

// LinkAnalysis holds the results of link analysis
type LinkAnalysis struct {
	TotalNodes      int        `json:"total_nodes"`
	TotalEdges      int        `json:"total_edges"`
	Clusters        []Cluster  `json:"clusters"`
	CentralNodes    []string   `json:"central_nodes"`
	SuspiciousPatterns []Pattern `json:"suspicious_patterns"`
}

// Pattern represents a suspicious calling pattern
type Pattern struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"node_ids"`
	Confidence  float64  `json:"confidence"`
}

// NewLinkAnalyzer creates a new link analyzer
func NewLinkAnalyzer() *LinkAnalyzer {
	return &LinkAnalyzer{
		graph: &Graph{
			Nodes: make(map[string]*Node),
			Edges: make(map[string]*Edge),
		},
	}
}

// AddCallRecord adds a call record to the graph
func (la *LinkAnalyzer) AddCallRecord(source, target string, duration int, timestamp string) {
	la.mu.Lock()
	defer la.mu.Unlock()

	la.getOrCreateNode(source)
	la.getOrCreateNode(target)

	la.graph.Nodes[source].TotalCalls++
	la.graph.Nodes[source].UniqueTargets++
	la.graph.Nodes[target].TotalCalls++

	edgeKey := la.getEdgeKey(source, target)
	if edge, exists := la.graph.Edges[edgeKey]; exists {
		edge.CallCount++
		edge.TotalDuration += duration
		edge.AvgDuration = float64(edge.TotalDuration) / float64(edge.CallCount)
		edge.LastCall = timestamp
	} else {
		la.graph.Edges[edgeKey] = &Edge{
			ID:          edgeKey,
			Source:      source,
			Target:      target,
			CallCount:   1,
			TotalDuration: duration,
			AvgDuration: float64(duration),
			FirstCall:   timestamp,
			LastCall:    timestamp,
		}
	}
}

// getOrCreateNode retrieves or creates a node
func (la *LinkAnalyzer) getOrCreateNode(phoneNumber string) *Node {
	if node, exists := la.graph.Nodes[phoneNumber]; exists {
		return node
	}
	node := &Node{
		ID:          phoneNumber,
		PhoneNumber: phoneNumber,
		Properties:  make(map[string]string),
	}
	la.graph.Nodes[phoneNumber] = node
	return node
}

// getEdgeKey creates a unique edge key
func (la *LinkAnalyzer) getEdgeKey(source, target string) string {
	if source < target {
		return source + "->" + target
	}
	return target + "->" + source
}

// Analyze performs full link analysis
func (la *LinkAnalyzer) Analyze() *LinkAnalysis {
	la.mu.RLock()
	defer la.mu.RUnlock()

	analysis := &LinkAnalysis{
		TotalNodes: len(la.graph.Nodes),
		TotalEdges: len(la.graph.Edges),
	}

	analysis.Clusters = la.detectClusters()
	analysis.CentralNodes = la.findCentralNodes(5)
	analysis.SuspiciousPatterns = la.detectSuspiciousPatterns()

	return analysis
}

// detectClusters detects groups of closely connected numbers
func (la *LinkAnalyzer) detectClusters() []Cluster {
	visited := make(map[string]bool)
	var clusters []Cluster

	for nodeID := range la.graph.Nodes {
		if !visited[nodeID] {
			cluster := la.bfsCluster(nodeID, visited)
			if len(cluster.NodeIDs) > 1 {
				clusters = append(clusters, cluster)
			}
		}
	}

	return clusters
}

// bfsCluster finds a cluster using BFS
func (la *LinkAnalyzer) bfsCluster(startID string, visited map[string]bool) Cluster {
	cluster := Cluster{
		ID:      startID,
		NodeIDs: []string{},
	}

	queue := []string{startID}
	visited[startID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		cluster.NodeIDs = append(cluster.NodeIDs, current)

		for edgeKey, edge := range la.graph.Edges {
			var neighbor string
			if edge.Source == current {
				neighbor = edge.Target
			} else if edge.Target == current {
				neighbor = edge.Source
			}

			if neighbor != "" && !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
			_ = edgeKey
		}
	}

	cluster.Size = len(cluster.NodeIDs)
	cluster.Density = la.calculateClusterDensity(cluster)
	return cluster
}

// calculateClusterDensity calculates the density of a cluster
func (la *LinkAnalyzer) calculateClusterDensity(cluster Cluster) float64 {
	if cluster.Size < 2 {
		return 0
	}

	maxEdges := float64(cluster.Size * (cluster.Size - 1) / 2)
	actualEdges := 0.0

	for _, edge := range la.graph.Edges {
		sourceInCluster := false
		targetInCluster := false
		for _, nodeID := range cluster.NodeIDs {
			if edge.Source == nodeID {
				sourceInCluster = true
			}
			if edge.Target == nodeID {
				targetInCluster = true
			}
		}
		if sourceInCluster && targetInCluster {
			actualEdges++
		}
	}

	if maxEdges > 0 {
		return actualEdges / maxEdges
	}
	return 0
}

// findCentralNodes finds the most central nodes
func (la *LinkAnalyzer) findCentralNodes(n int) []string {
	type nodeScore struct {
		ID    string
		Score float64
	}

	var scores []nodeScore
	for nodeID, node := range la.graph.Nodes {
		score := float64(node.TotalCalls) * 0.5 + float64(node.UniqueTargets) * 0.5
		scores = append(scores, nodeScore{ID: nodeID, Score: score})
	}

	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].Score > scores[i].Score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	if n > len(scores) {
		n = len(scores)
	}

	var result []string
	for i := 0; i < n; i++ {
		result = append(result, scores[i].ID)
	}
	return result
}

// detectSuspiciousPatterns detects suspicious calling patterns
func (la *LinkAnalyzer) detectSuspiciousPatterns() []Pattern {
	var patterns []Pattern

	patterns = append(patterns, la.detectFanPattern()...)
	patterns = append(patterns, la.detectStarPattern()...)
	patterns = append(patterns, la.detectChainPattern()...)

	return patterns
}

// detectFanPattern detects fan-out patterns (one number calling many)
func (la *LinkAnalyzer) detectFanPattern() []Pattern {
	var patterns []Pattern

	for nodeID, node := range la.graph.Nodes {
		if node.UniqueTargets > 20 {
			patterns = append(patterns, Pattern{
				Type:        "fan_out",
				Description: "Single number calling many unique targets",
				NodeIDs:     []string{nodeID},
				Confidence:  0.7,
			})
		}
	}

	return patterns
}

// detectStarPattern detects star patterns (many calling one)
func (la *LinkAnalyzer) detectStarPattern() []Pattern {
	var patterns []Pattern

	targetCounts := make(map[string]int)
	for _, edge := range la.graph.Edges {
		targetCounts[edge.Target]++
	}

	for target, count := range targetCounts {
		if count > 15 {
			patterns = append(patterns, Pattern{
				Type:        "star_incoming",
				Description: "Many numbers calling single target",
				NodeIDs:     []string{target},
				Confidence:  0.6,
			})
		}
	}

	return patterns
}

// detectChainPattern detects chain patterns (A calls B calls C)
func (la *LinkAnalyzer) detectChainPattern() []Pattern {
	var patterns []Pattern

	for source, node := range la.graph.Nodes {
		if node.UniqueTargets == 1 {
			for _, edge := range la.graph.Edges {
				if edge.Source == source {
					targetNode := la.graph.Nodes[edge.Target]
					if targetNode != nil && targetNode.UniqueTargets == 1 {
						patterns = append(patterns, Pattern{
							Type:        "chain",
							Description: "Sequential chain calling pattern",
							NodeIDs:     []string{source, edge.Target},
							Confidence:  0.5,
						})
					}
				}
			}
		}
	}

	return patterns
}

// GetNode retrieves a node by phone number
func (la *LinkAnalyzer) GetNode(phoneNumber string) *Node {
	la.mu.RLock()
	defer la.mu.RUnlock()
	return la.graph.Nodes[phoneNumber]
}

// GetEdges returns all edges
func (la *LinkAnalyzer) GetEdges() map[string]*Edge {
	la.mu.RLock()
	defer la.mu.RUnlock()
	return la.graph.Edges
}

// GetGraph returns the full graph
func (la *LinkAnalyzer) GetGraph() *Graph {
	la.mu.RLock()
	defer la.mu.RUnlock()
	return la.graph
}

// ExportDOT exports the graph in DOT format
func (la *LinkAnalyzer) ExportDOT() string {
	la.mu.RLock()
	defer la.mu.RUnlock()

	dot := "graph CDR {\n"
	dot += "  overlap=false;\n"
	dot += "  splines=true;\n\n"

	for _, node := range la.graph.Nodes {
		dot += "  \"" + node.PhoneNumber + "\" [label=\"" + node.PhoneNumber +
			"\\nCalls:" + string(rune(node.TotalCalls)) + "\"];\n"
	}

	dot += "\n"

	for _, edge := range la.graph.Edges {
		dot += "  \"" + edge.Source + "\" -- \"" + edge.Target +
			"\" [label=\"" + string(rune(edge.CallCount)) + "\"];\n"
	}

	dot += "}\n"
	return dot
}
