package entity

import (
	"slices"
	"strings"
)

// MaxTenantHierarchyDepth bounds parent-chain length in nodes. This is v1
// policy: raising it changes E03-T09 grant semantics and needs a versioned
// policy review, not a silent constant edit.
const MaxTenantHierarchyDepth = 5

// TenantLink is one parent→child tenancy edge. Grants are explicit per child
// and live in the consolidation service; this type owns only DAG shape.
type TenantLink struct {
	ParentID string
	ChildID  string
}

// Validate checks non-empty, non-self link shape.
func (l TenantLink) Validate() error {
	if strings.TrimSpace(l.ParentID) == "" || strings.TrimSpace(l.ChildID) == "" {
		return NewError("HIERARCHY_ID_REQUIRED", "hierarchy link requires parent and child ids")
	}
	if strings.TrimSpace(l.ParentID) == strings.TrimSpace(l.ChildID) {
		return NewError("HIERARCHY_SELF_PARENT", "tenant cannot parent to itself")
	}
	return nil
}

// ValidateHierarchy rejects self-parents, cycles, and depth beyond
// MaxTenantHierarchyDepth. It is deterministic on the same input.
func ValidateHierarchy(links []TenantLink) error {
	for _, l := range links {
		if err := l.Validate(); err != nil {
			return err
		}
	}
	if hasHierarchyCycle(links) {
		return NewError("HIERARCHY_CYCLE", "tenant hierarchy must be acyclic")
	}
	if depth, err := hierarchyDepth(links); err != nil {
		return err
	} else if depth > MaxTenantHierarchyDepth {
		return NewError("HIERARCHY_DEPTH_EXCEEDED", "tenant hierarchy exceeds depth limit")
	}
	return nil
}

func hasHierarchyCycle(links []TenantLink) bool {
	children := make(map[string][]string, len(links))
	nodes := make(map[string]struct{}, len(links)*2)
	for _, l := range links {
		parent := strings.TrimSpace(l.ParentID)
		child := strings.TrimSpace(l.ChildID)
		children[parent] = append(children[parent], child)
		nodes[parent] = struct{}{}
		nodes[child] = struct{}{}
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(nodes))
	var visit func(node string) bool
	visit = func(node string) bool {
		color[node] = gray
		for _, next := range children[node] {
			if color[next] == gray {
				return true
			}
			if color[next] == white && visit(next) {
				return true
			}
		}
		color[node] = black
		return false
	}
	nodeList := make([]string, 0, len(nodes))
	for node := range nodes {
		nodeList = append(nodeList, node)
	}
	slices.Sort(nodeList)
	for _, node := range nodeList {
		if color[node] == white && visit(node) {
			return true
		}
	}
	return false
}

type hierarchyGraph struct {
	children map[string][]string
	roots    []string
}

func buildHierarchyGraph(links []TenantLink) hierarchyGraph {
	children := make(map[string][]string, len(links))
	parents := make(map[string]bool, len(links))
	childSet := make(map[string]bool, len(links))
	seenEdge := make(map[string]bool, len(links))
	for _, l := range links {
		parent := strings.TrimSpace(l.ParentID)
		child := strings.TrimSpace(l.ChildID)
		edgeKey := parent + "->" + child
		if seenEdge[edgeKey] {
			continue
		}
		seenEdge[edgeKey] = true
		children[parent] = append(children[parent], child)
		parents[parent] = true
		childSet[child] = true
	}
	roots := make([]string, 0, len(parents))
	for node := range parents {
		if !childSet[node] {
			roots = append(roots, node)
		}
	}
	slices.Sort(roots)
	for p := range children {
		slices.Sort(children[p])
	}
	return hierarchyGraph{children: children, roots: roots}
}

func depthFromNode(node string, children map[string][]string, memo map[string]int, visiting map[string]bool) (int, error) {
	if visiting[node] {
		return 0, NewError("HIERARCHY_CYCLE", "tenant hierarchy must be acyclic")
	}
	if d, ok := memo[node]; ok {
		return d, nil
	}
	visiting[node] = true
	maxChild := 0
	for _, next := range children[node] {
		d, err := depthFromNode(next, children, memo, visiting)
		if err != nil {
			return 0, err
		}
		if d+1 > maxChild {
			maxChild = d + 1
		}
	}
	delete(visiting, node)
	memo[node] = maxChild
	return maxChild, nil
}

func hierarchyDepth(links []TenantLink) (int, error) {
	if len(links) == 0 {
		return 0, nil
	}
	graph := buildHierarchyGraph(links)
	memo := make(map[string]int)
	maxDepth := 0
	for _, node := range graph.roots {
		d, err := depthFromNode(node, graph.children, memo, map[string]bool{})
		if err != nil {
			return 0, err
		}
		if d > maxDepth {
			maxDepth = d
		}
	}
	// Chains are edge counts; depth is nodes = edges + 1 when non-empty.
	if len(links) > 0 {
		maxDepth++
	}
	return maxDepth, nil
}
