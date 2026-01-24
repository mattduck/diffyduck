package structure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeDiff_BothNil(t *testing.T) {
	diff := ComputeDiff(nil, nil, nil, nil)
	assert.NotNil(t, diff)
	assert.Empty(t, diff.Changes)
	assert.False(t, diff.HasChanges())
}

func TestComputeDiff_OnlyNew(t *testing.T) {
	// All entries in new are "added"
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
		{StartLine: 7, EndLine: 10, Name: "Bar", Kind: "type"},
	})

	diff := ComputeDiff(nil, newMap, nil, nil)

	require.Len(t, diff.Changes, 2)
	assert.Equal(t, ChangeAdded, diff.Changes[0].Kind)
	assert.Equal(t, "Foo", diff.Changes[0].Name())
	assert.Equal(t, ChangeAdded, diff.Changes[1].Kind)
	assert.Equal(t, "Bar", diff.Changes[1].Name())
	assert.True(t, diff.HasChanges())
}

func TestComputeDiff_OnlyOld(t *testing.T) {
	// All entries in old are "deleted"
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
	})

	diff := ComputeDiff(oldMap, nil, nil, nil)

	require.Len(t, diff.Changes, 1)
	assert.Equal(t, ChangeDeleted, diff.Changes[0].Kind)
	assert.Equal(t, "Foo", diff.Changes[0].Name())
	assert.True(t, diff.HasChanges())
}

func TestComputeDiff_Unchanged(t *testing.T) {
	// Same entry in both, no overlapping diff lines -> unchanged
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
	})
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
	})

	diff := ComputeDiff(oldMap, newMap, nil, nil)

	require.Len(t, diff.Changes, 1)
	assert.Equal(t, ChangeUnchanged, diff.Changes[0].Kind)
	assert.Equal(t, "Foo", diff.Changes[0].Name())
	assert.False(t, diff.HasChanges())
}

func TestComputeDiff_Modified_AddedLines(t *testing.T) {
	// Same entry in both, but added lines overlap with new entry -> modified
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
	})
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 7, Name: "Foo", Kind: "func"}, // grew by 2 lines
	})

	addedLines := map[int]bool{6: true, 7: true}

	diff := ComputeDiff(oldMap, newMap, addedLines, nil)

	require.Len(t, diff.Changes, 1)
	assert.Equal(t, ChangeModified, diff.Changes[0].Kind)
	assert.Equal(t, "Foo", diff.Changes[0].Name())
	assert.True(t, diff.HasChanges())
}

func TestComputeDiff_Modified_RemovedLines(t *testing.T) {
	// Same entry in both, but removed lines overlap with old entry -> modified
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 7, Name: "Foo", Kind: "func"},
	})
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"}, // shrank by 2 lines
	})

	removedLines := map[int]bool{6: true, 7: true}

	diff := ComputeDiff(oldMap, newMap, nil, removedLines)

	require.Len(t, diff.Changes, 1)
	assert.Equal(t, ChangeModified, diff.Changes[0].Kind)
	assert.True(t, diff.HasChanges())
}

func TestComputeDiff_Mixed(t *testing.T) {
	// oldMap: Foo (unchanged), Bar (deleted), Baz (modified)
	// newMap: Foo (unchanged), Baz (modified), Qux (added)
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
		{StartLine: 7, EndLine: 10, Name: "Bar", Kind: "func"},
		{StartLine: 12, EndLine: 15, Name: "Baz", Kind: "func"},
	})
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
		{StartLine: 7, EndLine: 12, Name: "Baz", Kind: "func"}, // moved and modified
		{StartLine: 14, EndLine: 18, Name: "Qux", Kind: "func"},
	})

	addedLines := map[int]bool{10: true, 11: true, 12: true, 14: true, 15: true, 16: true, 17: true, 18: true}

	diff := ComputeDiff(oldMap, newMap, addedLines, nil)

	require.Len(t, diff.Changes, 4)

	// Find each change by name
	byName := make(map[string]ElementChange)
	for _, c := range diff.Changes {
		byName[c.Name()] = c
	}

	assert.Equal(t, ChangeUnchanged, byName["Foo"].Kind)
	assert.Equal(t, ChangeDeleted, byName["Bar"].Kind)
	assert.Equal(t, ChangeModified, byName["Baz"].Kind)
	assert.Equal(t, ChangeAdded, byName["Qux"].Kind)

	assert.True(t, diff.HasChanges())
	assert.Len(t, diff.ChangedOnly(), 3) // Bar, Baz, Qux
}

func TestComputeDiff_MethodsWithReceiver(t *testing.T) {
	// Methods on different types with same name should be distinguished
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "String", Kind: "func", Receiver: "(f Foo)"},
		{StartLine: 7, EndLine: 10, Name: "String", Kind: "func", Receiver: "(b Bar)"},
	})
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "String", Kind: "func", Receiver: "(f Foo)"},
		// Bar.String was deleted
	})

	diff := ComputeDiff(oldMap, newMap, nil, nil)

	require.Len(t, diff.Changes, 2)

	// Find changes
	var fooChange, barChange ElementChange
	for _, c := range diff.Changes {
		if c.OldEntry != nil && c.OldEntry.Receiver == "(f Foo)" {
			fooChange = c
		} else if c.OldEntry != nil && c.OldEntry.Receiver == "(b Bar)" {
			barChange = c
		}
	}

	assert.Equal(t, ChangeUnchanged, fooChange.Kind)
	assert.Equal(t, ChangeDeleted, barChange.Kind)
}

func TestComputeDiff_TypeVsFunc(t *testing.T) {
	// Type and func with same name should be treated as different
	oldMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 3, Name: "Foo", Kind: "type"},
	})
	newMap := NewMap([]Entry{
		{StartLine: 1, EndLine: 5, Name: "Foo", Kind: "func"},
	})

	diff := ComputeDiff(oldMap, newMap, nil, nil)

	require.Len(t, diff.Changes, 2)

	// Find each change
	var typeChange, funcChange ElementChange
	for _, c := range diff.Changes {
		entry := c.Entry()
		if entry.Kind == "type" {
			typeChange = c
		} else if entry.Kind == "func" {
			funcChange = c
		}
	}

	assert.Equal(t, ChangeDeleted, typeChange.Kind)
	assert.Equal(t, ChangeAdded, funcChange.Kind)
}

func TestChangeKind_String(t *testing.T) {
	assert.Equal(t, "unchanged", ChangeUnchanged.String())
	assert.Equal(t, "added", ChangeAdded.String())
	assert.Equal(t, "deleted", ChangeDeleted.String())
	assert.Equal(t, "modified", ChangeModified.String())
}

func TestChangeKind_Symbol(t *testing.T) {
	assert.Equal(t, " ", ChangeUnchanged.Symbol())
	assert.Equal(t, "+", ChangeAdded.Symbol())
	assert.Equal(t, "-", ChangeDeleted.Symbol())
	assert.Equal(t, "~", ChangeModified.Symbol())
}

func TestElementChange_Entry(t *testing.T) {
	oldEntry := &Entry{Name: "Old"}
	newEntry := &Entry{Name: "New"}

	// Prefers NewEntry
	c := ElementChange{OldEntry: oldEntry, NewEntry: newEntry}
	assert.Equal(t, newEntry, c.Entry())
	assert.Equal(t, "New", c.Name())

	// Falls back to OldEntry
	c = ElementChange{OldEntry: oldEntry}
	assert.Equal(t, oldEntry, c.Entry())
	assert.Equal(t, "Old", c.Name())

	// Neither
	c = ElementChange{}
	assert.Nil(t, c.Entry())
	assert.Equal(t, "", c.Name())
}
