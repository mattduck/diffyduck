package structure

import "sort"

// DisplayNode represents a top-level structural element and its children
// (e.g., methods nested inside a type/class).
type DisplayNode struct {
	Change   ElementChange
	Children []ElementChange
}

// TotalLines returns the sum of added and removed lines for this node
// and all its children.
func (n DisplayNode) TotalLines() int {
	total := n.Change.LinesAdded + n.Change.LinesRemoved
	for _, child := range n.Children {
		total += child.LinesAdded + child.LinesRemoved
	}
	return total
}

// TopChanges groups methods under parent types, sorts by total lines
// changed (descending), and truncates to maxItems (counting each parent
// and child as one row). Returns the visible nodes and count of truncated
// top-level nodes.
func TopChanges(diff *StructuralDiff, maxItems int) (nodes []DisplayNode, truncated int) {
	if diff == nil || !diff.HasChanges() {
		return nil, 0
	}

	changes := diff.ChangedOnly()
	if len(changes) == 0 {
		return nil, 0
	}

	// First pass: find types/classes and their contained methods
	var topLevel []DisplayNode
	methodsAssigned := make(map[int]bool)

	for i, c := range changes {
		entry := c.Entry()
		if entry == nil {
			continue
		}
		if entry.Kind == "type" || entry.Kind == "class" {
			node := DisplayNode{Change: c}
			for j, other := range changes {
				if i == j {
					continue
				}
				otherEntry := other.Entry()
				if otherEntry == nil {
					continue
				}
				if otherEntry.Kind == "func" || otherEntry.Kind == "def" {
					typeStart, typeEnd := entry.StartLine, entry.EndLine
					otherStart := otherEntry.StartLine
					if otherStart >= typeStart && otherStart <= typeEnd {
						node.Children = append(node.Children, other)
						methodsAssigned[j] = true
					}
				}
			}
			topLevel = append(topLevel, node)
			methodsAssigned[i] = true
		}
	}

	// Second pass: remaining items as top-level
	for i, c := range changes {
		if !methodsAssigned[i] {
			topLevel = append(topLevel, DisplayNode{Change: c})
		}
	}

	// Sort top-level by total lines changed (descending)
	sort.SliceStable(topLevel, func(i, j int) bool {
		return topLevel[i].TotalLines() > topLevel[j].TotalLines()
	})

	// Sort children within each node by lines changed (descending)
	for i := range topLevel {
		sort.SliceStable(topLevel[i].Children, func(a, b int) bool {
			ca := topLevel[i].Children[a]
			cb := topLevel[i].Children[b]
			return (ca.LinesAdded + ca.LinesRemoved) > (cb.LinesAdded + cb.LinesRemoved)
		})
	}

	// Truncate to maxItems displayed rows (parent + children each count as one)
	rowCount := 0
	keptCount := 0
	for _, node := range topLevel {
		nodeRows := 1 + len(node.Children)
		if rowCount+nodeRows > maxItems {
			break
		}
		keptCount++
		rowCount += nodeRows
	}
	if keptCount < len(topLevel) {
		truncated = len(topLevel) - keptCount
		topLevel = topLevel[:keptCount]
	}

	return topLevel, truncated
}
