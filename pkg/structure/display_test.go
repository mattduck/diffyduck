package structure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTopChanges_Nil(t *testing.T) {
	nodes, truncated := TopChanges(nil, 5)
	assert.Nil(t, nodes)
	assert.Equal(t, 0, truncated)
}

func TestTopChanges_Empty(t *testing.T) {
	d := &StructuralDiff{}
	nodes, truncated := TopChanges(d, 5)
	assert.Nil(t, nodes)
	assert.Equal(t, 0, truncated)
}

func TestTopChanges_SortsByTotalLines(t *testing.T) {
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 1, EndLine: 10, Name: "Small", Kind: "func"}, LinesAdded: 2, LinesRemoved: 1},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 20, EndLine: 40, Name: "Large", Kind: "func"}, LinesAdded: 10, LinesRemoved: 5},
			{Kind: ChangeAdded, NewEntry: &Entry{StartLine: 50, EndLine: 55, Name: "Medium", Kind: "func"}, LinesAdded: 5},
		},
	}

	nodes, truncated := TopChanges(d, 10)
	assert.Equal(t, 0, truncated)
	assert.Len(t, nodes, 3)
	// Largest first
	assert.Equal(t, "Large", nodes[0].Change.Name())
	assert.Equal(t, "Medium", nodes[1].Change.Name())
	assert.Equal(t, "Small", nodes[2].Change.Name())
}

func TestTopChanges_GroupsMethodsUnderTypes(t *testing.T) {
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 1, EndLine: 50, Name: "MyType", Kind: "type"}, LinesAdded: 1},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 10, EndLine: 20, Name: "Method1", Kind: "func"}, LinesAdded: 5},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 25, EndLine: 35, Name: "Method2", Kind: "func"}, LinesAdded: 3},
			{Kind: ChangeAdded, NewEntry: &Entry{StartLine: 100, EndLine: 110, Name: "FreeFunc", Kind: "func"}, LinesAdded: 10},
		},
	}

	nodes, truncated := TopChanges(d, 10)
	assert.Equal(t, 0, truncated)
	// FreeFunc (10) is top-level; MyType (1), Method1 (5), Method2 (3) all rank in top 10.
	// Method1 and Method2 are contained in MyType, so shown as children.
	assert.Len(t, nodes, 2) // FreeFunc + MyType (with 2 children)

	// FreeFunc has most lines so comes first
	assert.Equal(t, "FreeFunc", nodes[0].Change.Name())
	assert.Len(t, nodes[0].Children, 0)

	// MyType with 2 children (total: 1+5+3 = 9)
	assert.Equal(t, "MyType", nodes[1].Change.Name())
	assert.Len(t, nodes[1].Children, 2)
	// Children sorted by lines changed (descending)
	assert.Equal(t, "Method1", nodes[1].Children[0].Name())
	assert.Equal(t, "Method2", nodes[1].Children[1].Name())
}

func TestTopChanges_GroupsFuncContainsType(t *testing.T) {
	// Go: a type defined inside a function should nest under the function.
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 1, EndLine: 50, Name: "handler", Kind: "func"}, LinesAdded: 20, LinesRemoved: 5},
			{Kind: ChangeAdded, NewEntry: &Entry{StartLine: 5, EndLine: 15, Name: "request", Kind: "type"}, LinesAdded: 10},
		},
	}

	nodes, truncated := TopChanges(d, 10)
	assert.Equal(t, 0, truncated)
	assert.Len(t, nodes, 1) // handler with request as child

	assert.Equal(t, "handler", nodes[0].Change.Name())
	assert.Len(t, nodes[0].Children, 1)
	assert.Equal(t, "request", nodes[0].Children[0].Name())
}

func TestTopChanges_Truncation(t *testing.T) {
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 1, EndLine: 10, Name: "A", Kind: "func"}, LinesAdded: 10},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 20, EndLine: 30, Name: "B", Kind: "func"}, LinesAdded: 5},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 40, EndLine: 50, Name: "C", Kind: "func"}, LinesAdded: 3},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 60, EndLine: 70, Name: "D", Kind: "func"}, LinesAdded: 1},
		},
	}

	nodes, truncated := TopChanges(d, 2)
	assert.Equal(t, 2, truncated)
	assert.Len(t, nodes, 2)
	assert.Equal(t, "A", nodes[0].Change.Name())
	assert.Equal(t, "B", nodes[1].Change.Name())
}

func TestTopChanges_SmallChildrenDontStealSpots(t *testing.T) {
	// A type with 3 tiny methods (1 line each) + 3 big standalone funcs (50 lines each).
	// With maxItems=5, the flat ranking takes: 3 big funcs (50 each) + type (10) + 1 method (1).
	// The other 2 methods are below the cutoff.
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 1, EndLine: 100, Name: "MyType", Kind: "type"}, LinesAdded: 10},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 10, EndLine: 12, Name: "method_a", Kind: "func"}, LinesAdded: 1},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 20, EndLine: 22, Name: "method_b", Kind: "func"}, LinesAdded: 1},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 30, EndLine: 32, Name: "method_c", Kind: "func"}, LinesAdded: 1},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 200, EndLine: 250, Name: "BigA", Kind: "func"}, LinesAdded: 50},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 300, EndLine: 350, Name: "BigB", Kind: "func"}, LinesAdded: 50},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 400, EndLine: 450, Name: "BigC", Kind: "func"}, LinesAdded: 50},
		},
	}

	nodes, truncated := TopChanges(d, 5)
	assert.Equal(t, 2, truncated) // method_b and method_c below cutoff

	// All 3 big funcs should appear as top-level
	names := make(map[string]bool)
	for _, n := range nodes {
		names[n.Change.Name()] = true
	}
	assert.True(t, names["BigA"], "BigA should appear")
	assert.True(t, names["BigB"], "BigB should appear")
	assert.True(t, names["BigC"], "BigC should appear")
	assert.True(t, names["MyType"], "MyType should appear")
}

func TestTopChanges_NoNestingAcrossFileVersions(t *testing.T) {
	// A deleted function (old-file lines 10-50) and an added function (new-file lines 20-40).
	// Their line numbers overlap but they're from different file versions — no nesting.
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeDeleted, OldEntry: &Entry{StartLine: 10, EndLine: 50, Name: "OldFunc", Kind: "func"}, LinesRemoved: 40},
			{Kind: ChangeAdded, NewEntry: &Entry{StartLine: 20, EndLine: 40, Name: "NewFunc", Kind: "func"}, LinesAdded: 20},
		},
	}

	nodes, truncated := TopChanges(d, 10)
	assert.Equal(t, 0, truncated)
	assert.Len(t, nodes, 2) // Both top-level, no nesting
	assert.Len(t, nodes[0].Children, 0)
	assert.Len(t, nodes[1].Children, 0)
}

func TestTopChanges_IgnoresUnchanged(t *testing.T) {
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeUnchanged, NewEntry: &Entry{StartLine: 1, EndLine: 10, Name: "Unchanged", Kind: "func"}},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 20, EndLine: 30, Name: "Changed", Kind: "func"}, LinesAdded: 5},
		},
	}

	nodes, truncated := TopChanges(d, 10)
	assert.Equal(t, 0, truncated)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "Changed", nodes[0].Change.Name())
}

func TestDisplayNode_TotalLines(t *testing.T) {
	node := DisplayNode{
		Change: ElementChange{LinesAdded: 3, LinesRemoved: 2},
		Children: []ElementChange{
			{LinesAdded: 5, LinesRemoved: 1},
			{LinesAdded: 2, LinesRemoved: 0},
		},
	}
	assert.Equal(t, 13, node.TotalLines())
}
