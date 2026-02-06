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
	assert.Len(t, nodes, 2) // MyType (with 2 children) + FreeFunc

	// FreeFunc has more lines so comes first
	assert.Equal(t, "FreeFunc", nodes[0].Change.Name())
	assert.Len(t, nodes[0].Children, 0)

	// MyType with 2 children (total: 1+5+3 = 9)
	assert.Equal(t, "MyType", nodes[1].Change.Name())
	assert.Len(t, nodes[1].Children, 2)
	// Children sorted by lines changed (descending)
	assert.Equal(t, "Method1", nodes[1].Children[0].Name())
	assert.Equal(t, "Method2", nodes[1].Children[1].Name())
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

func TestTopChanges_TruncationCountsChildRows(t *testing.T) {
	// Type with 1 child (2 rows) fits in maxItems=5, then two more functions
	// fit (total 4 rows), last one is truncated.
	d := &StructuralDiff{
		Changes: []ElementChange{
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 1, EndLine: 50, Name: "MyType", Kind: "type"}, LinesAdded: 1},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 10, EndLine: 20, Name: "Method1", Kind: "func"}, LinesAdded: 10},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 100, EndLine: 110, Name: "FuncA", Kind: "func"}, LinesAdded: 8},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 200, EndLine: 210, Name: "FuncB", Kind: "func"}, LinesAdded: 5},
			{Kind: ChangeModified, NewEntry: &Entry{StartLine: 300, EndLine: 310, Name: "FuncC", Kind: "func"}, LinesAdded: 1},
		},
	}

	// MyType+Method1 = 2 rows, FuncA = 1 row, FuncB = 1 row (total 4), FuncC truncated
	nodes, truncated := TopChanges(d, 4)
	assert.Equal(t, 1, truncated)
	assert.Len(t, nodes, 3)
	// Sorted by total lines: MyType(1+10=11), FuncA(8), FuncB(5)
	assert.Equal(t, "MyType", nodes[0].Change.Name())
	assert.Len(t, nodes[0].Children, 1)
	assert.Equal(t, "FuncA", nodes[1].Change.Name())
	assert.Equal(t, "FuncB", nodes[2].Change.Name())
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
