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

// TopChanges ranks changes flat by lines changed, takes the top maxItems,
// then groups children under parents using line containment. Each item earns
// its spot independently — nesting is purely visual grouping. Returns the
// visible nodes and count of truncated items.
func TopChanges(diff *StructuralDiff, maxItems int) (nodes []DisplayNode, truncated int) {
	if diff == nil || !diff.HasChanges() {
		return nil, 0
	}

	changes := diff.ChangedOnly()
	if len(changes) == 0 {
		return nil, 0
	}

	// Rank all changes flat by lines changed, take top N.
	sort.SliceStable(changes, func(i, j int) bool {
		totalI := changes[i].LinesAdded + changes[i].LinesRemoved
		totalJ := changes[j].LinesAdded + changes[j].LinesRemoved
		return totalI > totalJ
	})
	if len(changes) > maxItems {
		truncated = len(changes) - maxItems
		changes = changes[:maxItems]
	}

	// Among the top N, detect containment and group children under parents.
	// Only compare entries from the same file version (both new or both old)
	// to avoid false matches between added and deleted items.
	parentOf := make(map[int]int) // child index -> parent index
	for i, child := range changes {
		bestParent := -1
		bestSpan := 0
		for j, parent := range changes {
			if i == j {
				continue
			}
			var childEntry, parentEntry *Entry
			if child.NewEntry != nil && parent.NewEntry != nil {
				childEntry, parentEntry = child.NewEntry, parent.NewEntry
			} else if child.OldEntry != nil && parent.OldEntry != nil {
				childEntry, parentEntry = child.OldEntry, parent.OldEntry
			} else {
				continue
			}
			parentSpan := parentEntry.EndLine - parentEntry.StartLine
			childSpan := childEntry.EndLine - childEntry.StartLine
			if parentSpan <= childSpan {
				continue // parent must be strictly larger
			}
			if childEntry.StartLine >= parentEntry.StartLine &&
				childEntry.EndLine <= parentEntry.EndLine {
				if bestParent == -1 || parentSpan < bestSpan {
					bestSpan = parentSpan
					bestParent = j
				}
			}
		}
		if bestParent >= 0 {
			parentOf[i] = bestParent
		}
	}

	// Build single-level tree: walk each child up to its top-level ancestor.
	isChild := make(map[int]bool)
	for c := range parentOf {
		isChild[c] = true
	}
	childrenOf := make(map[int][]int)
	for child := range parentOf {
		top := parentOf[child]
		for isChild[top] {
			top = parentOf[top]
		}
		childrenOf[top] = append(childrenOf[top], child)
	}

	var topLevel []DisplayNode
	for i, c := range changes {
		if isChild[i] {
			continue
		}
		node := DisplayNode{Change: c}
		for _, childIdx := range childrenOf[i] {
			node.Children = append(node.Children, changes[childIdx])
		}
		topLevel = append(topLevel, node)
	}

	// Sort top-level by total lines changed (own + visible children)
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

	return topLevel, truncated
}
