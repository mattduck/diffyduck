package sidebyside

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/diff"
)

func TestTransformHunk_ContextOnly(t *testing.T) {
	hunk := diff.Hunk{
		OldStart: 1,
		NewStart: 1,
		Lines: []diff.Line{
			{Type: diff.Context, Content: "line one"},
			{Type: diff.Context, Content: "line two"},
		},
	}

	pairs := TransformHunk(hunk)

	require.Len(t, pairs, 2)

	assert.Equal(t, LinePair{
		Old: Line{Num: 1, Content: "line one", Type: Context},
		New: Line{Num: 1, Content: "line one", Type: Context},
	}, pairs[0])

	assert.Equal(t, LinePair{
		Old: Line{Num: 2, Content: "line two", Type: Context},
		New: Line{Num: 2, Content: "line two", Type: Context},
	}, pairs[1])
}

func TestTransformHunk_AddedLine(t *testing.T) {
	hunk := diff.Hunk{
		OldStart: 1,
		NewStart: 1,
		Lines: []diff.Line{
			{Type: diff.Context, Content: "before"},
			{Type: diff.Added, Content: "new line"},
			{Type: diff.Context, Content: "after"},
		},
	}

	pairs := TransformHunk(hunk)

	require.Len(t, pairs, 3)

	// Context line
	assert.Equal(t, 1, pairs[0].Old.Num)
	assert.Equal(t, 1, pairs[0].New.Num)

	// Added line: empty left, content right
	assert.Equal(t, LinePair{
		Old: Line{Num: 0, Content: "", Type: Empty},
		New: Line{Num: 2, Content: "new line", Type: Added},
	}, pairs[1])

	// Context line: left stays at 2, right moves to 3
	assert.Equal(t, 2, pairs[2].Old.Num)
	assert.Equal(t, 3, pairs[2].New.Num)
}

func TestTransformHunk_RemovedLine(t *testing.T) {
	hunk := diff.Hunk{
		OldStart: 1,
		NewStart: 1,
		Lines: []diff.Line{
			{Type: diff.Context, Content: "before"},
			{Type: diff.Removed, Content: "old line"},
			{Type: diff.Context, Content: "after"},
		},
	}

	pairs := TransformHunk(hunk)

	require.Len(t, pairs, 3)

	// Removed line: content left, empty right
	assert.Equal(t, LinePair{
		Old: Line{Num: 2, Content: "old line", Type: Removed},
		New: Line{Num: 0, Content: "", Type: Empty},
	}, pairs[1])

	// Context after: left at 3, right at 2
	assert.Equal(t, 3, pairs[2].Old.Num)
	assert.Equal(t, 2, pairs[2].New.Num)
}

func TestTransformHunk_ModifiedLine(t *testing.T) {
	// A modification shows as remove + add, should align side by side
	hunk := diff.Hunk{
		OldStart: 1,
		NewStart: 1,
		Lines: []diff.Line{
			{Type: diff.Removed, Content: "old version"},
			{Type: diff.Added, Content: "new version"},
		},
	}

	pairs := TransformHunk(hunk)

	// Should align the remove and add on the same row
	require.Len(t, pairs, 1)
	assert.Equal(t, LinePair{
		Old: Line{Num: 1, Content: "old version", Type: Removed},
		New: Line{Num: 1, Content: "new version", Type: Added},
	}, pairs[0])
}

func TestTransformHunk_MultipleModifications(t *testing.T) {
	hunk := diff.Hunk{
		OldStart: 1,
		NewStart: 1,
		Lines: []diff.Line{
			{Type: diff.Removed, Content: "old1"},
			{Type: diff.Removed, Content: "old2"},
			{Type: diff.Added, Content: "new1"},
			{Type: diff.Added, Content: "new2"},
			{Type: diff.Added, Content: "new3"},
		},
	}

	pairs := TransformHunk(hunk)

	// 2 removes, 3 adds -> should pair up first 2, then 1 extra add
	require.Len(t, pairs, 3)

	assert.Equal(t, LinePair{
		Old: Line{Num: 1, Content: "old1", Type: Removed},
		New: Line{Num: 1, Content: "new1", Type: Added},
	}, pairs[0])

	assert.Equal(t, LinePair{
		Old: Line{Num: 2, Content: "old2", Type: Removed},
		New: Line{Num: 2, Content: "new2", Type: Added},
	}, pairs[1])

	assert.Equal(t, LinePair{
		Old: Line{Num: 0, Content: "", Type: Empty},
		New: Line{Num: 3, Content: "new3", Type: Added},
	}, pairs[2])
}

func TestTransformHunk_MoreRemovesThanAdds(t *testing.T) {
	hunk := diff.Hunk{
		OldStart: 1,
		NewStart: 1,
		Lines: []diff.Line{
			{Type: diff.Removed, Content: "old1"},
			{Type: diff.Removed, Content: "old2"},
			{Type: diff.Removed, Content: "old3"},
			{Type: diff.Added, Content: "new1"},
		},
	}

	pairs := TransformHunk(hunk)

	require.Len(t, pairs, 3)

	assert.Equal(t, LinePair{
		Old: Line{Num: 1, Content: "old1", Type: Removed},
		New: Line{Num: 1, Content: "new1", Type: Added},
	}, pairs[0])

	assert.Equal(t, LinePair{
		Old: Line{Num: 2, Content: "old2", Type: Removed},
		New: Line{Num: 0, Content: "", Type: Empty},
	}, pairs[1])

	assert.Equal(t, LinePair{
		Old: Line{Num: 3, Content: "old3", Type: Removed},
		New: Line{Num: 0, Content: "", Type: Empty},
	}, pairs[2])
}

func TestTransformFile(t *testing.T) {
	file := diff.File{
		OldPath: "a/foo.go",
		NewPath: "b/foo.go",
		Hunks: []diff.Hunk{
			{
				OldStart: 1,
				NewStart: 1,
				Lines: []diff.Line{
					{Type: diff.Context, Content: "line1"},
				},
			},
			{
				OldStart: 10,
				NewStart: 10,
				Lines: []diff.Line{
					{Type: diff.Context, Content: "line10"},
				},
			},
		},
	}

	fp := TransformFile(file)

	assert.Equal(t, "a/foo.go", fp.OldPath)
	assert.Equal(t, "b/foo.go", fp.NewPath)
	require.Len(t, fp.Pairs, 2) // one pair from each hunk
}

func TestTransformDiff(t *testing.T) {
	d := &diff.Diff{
		Files: []diff.File{
			{
				OldPath: "a/one.go",
				NewPath: "b/one.go",
				Hunks: []diff.Hunk{
					{OldStart: 1, NewStart: 1, Lines: []diff.Line{{Type: diff.Context, Content: "x"}}},
				},
			},
			{
				OldPath: "a/two.go",
				NewPath: "b/two.go",
				Hunks: []diff.Hunk{
					{OldStart: 1, NewStart: 1, Lines: []diff.Line{{Type: diff.Context, Content: "y"}}},
				},
			},
		},
	}

	fps, _ := TransformDiff(d)

	require.Len(t, fps, 2)
	assert.Equal(t, "a/one.go", fps[0].OldPath)
	assert.Equal(t, "a/two.go", fps[1].OldPath)
}
