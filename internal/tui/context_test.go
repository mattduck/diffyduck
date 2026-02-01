package tui

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

func TestFindHunkBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []sidebyside.LinePair
		expected []hunkBoundary
	}{
		{
			name:     "empty pairs",
			pairs:    nil,
			expected: nil,
		},
		{
			name: "single contiguous hunk",
			pairs: []sidebyside.LinePair{
				{New: sidebyside.Line{Num: 1}},
				{New: sidebyside.Line{Num: 2}},
				{New: sidebyside.Line{Num: 3}},
			},
			expected: []hunkBoundary{{startIdx: 0, endIdx: 3}},
		},
		{
			name: "two hunks with gap",
			pairs: []sidebyside.LinePair{
				{New: sidebyside.Line{Num: 1}},
				{New: sidebyside.Line{Num: 2}},
				// gap here
				{New: sidebyside.Line{Num: 10}},
				{New: sidebyside.Line{Num: 11}},
			},
			expected: []hunkBoundary{
				{startIdx: 0, endIdx: 2},
				{startIdx: 2, endIdx: 4},
			},
		},
		{
			name: "three hunks",
			pairs: []sidebyside.LinePair{
				{New: sidebyside.Line{Num: 1}},
				// gap
				{New: sidebyside.Line{Num: 5}},
				// gap
				{New: sidebyside.Line{Num: 20}},
			},
			expected: []hunkBoundary{
				{startIdx: 0, endIdx: 1},
				{startIdx: 1, endIdx: 2},
				{startIdx: 2, endIdx: 3},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findHunkBoundaries(tt.pairs)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExpandSemanticContext(t *testing.T) {
	t.Run("expands to include function start when close", func(t *testing.T) {
		// Function starts at line 3, hunk starts at line 5
		// Gap of 2 lines should trigger expansion
		fp := &sidebyside.FilePair{
			NewContent: []string{
				"package main",        // line 1
				"",                    // line 2
				"func MyFunction() {", // line 3
				"    x := 1",          // line 4
				"    y := 2",          // line 5
				"}",                   // line 6
			},
			OldContent: []string{
				"package main",        // line 1
				"",                    // line 2
				"func MyFunction() {", // line 3
				"    x := 1",          // line 4
				"    y := 2",          // line 5
				"}",                   // line 6
			},
			Pairs: []sidebyside.LinePair{
				// Hunk starts at line 5 (inside function)
				{
					Old: sidebyside.Line{Num: 5, Content: "    y := 2", Type: sidebyside.Removed},
					New: sidebyside.Line{Num: 5, Content: "    y := 99", Type: sidebyside.Added},
				},
			},
		}

		structMap := structure.NewMap([]structure.Entry{
			{StartLine: 3, EndLine: 6, Name: "MyFunction", Kind: "func"},
		})

		// Before expansion: 1 pair
		require.Equal(t, 1, len(fp.Pairs))

		expandSemanticContext(fp, structMap, 15)

		// After expansion: should have lines 3, 4, 5 (function start + context + change)
		require.Equal(t, 3, len(fp.Pairs), "should expand to include function start")

		// Verify line 3 (function signature) was inserted
		assert.Equal(t, 3, fp.Pairs[0].New.Num, "first pair should be line 3")
		assert.Contains(t, fp.Pairs[0].New.Content, "func MyFunction")
	})

	t.Run("does not expand when function start is too far", func(t *testing.T) {
		// Function starts at line 3, hunk starts at line 25
		// Gap of 22 lines exceeds threshold of 15
		fp := &sidebyside.FilePair{
			NewContent: make([]string, 30), // dummy content
			OldContent: make([]string, 30),
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 25, Content: "code"},
					New: sidebyside.Line{Num: 25, Content: "code"},
				},
			},
		}
		fp.NewContent[2] = "func MyFunction() {"
		fp.NewContent[24] = "code"

		structMap := structure.NewMap([]structure.Entry{
			{StartLine: 3, EndLine: 30, Name: "MyFunction", Kind: "func"},
		})

		expandSemanticContext(fp, structMap, 15)

		// Should not expand - gap too large
		assert.Equal(t, 1, len(fp.Pairs), "should not expand when gap exceeds threshold")
	})

	t.Run("handles no structure gracefully", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			NewContent: []string{"line 1", "line 2"},
			OldContent: []string{"line 1", "line 2"},
			Pairs: []sidebyside.LinePair{
				{New: sidebyside.Line{Num: 1}},
			},
		}

		// No panic with nil structure
		expandSemanticContext(fp, nil, 15)
		assert.Equal(t, 1, len(fp.Pairs))
	})

	t.Run("handles empty content gracefully", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			Pairs: []sidebyside.LinePair{
				{New: sidebyside.Line{Num: 1}},
			},
		}

		structMap := structure.NewMap([]structure.Entry{
			{StartLine: 1, EndLine: 10, Name: "Func", Kind: "func"},
		})

		// No panic with empty content
		expandSemanticContext(fp, structMap, 15)
		assert.Equal(t, 1, len(fp.Pairs))
	})
}

func TestFindInnermostFunction(t *testing.T) {
	tests := []struct {
		name     string
		entries  []structure.Entry
		wantName string
	}{
		{
			name:     "empty entries",
			entries:  nil,
			wantName: "",
		},
		{
			name: "single function",
			entries: []structure.Entry{
				{Name: "MyFunc", Kind: "func"},
			},
			wantName: "MyFunc",
		},
		{
			name: "nested: class contains method",
			entries: []structure.Entry{
				{Name: "MyClass", Kind: "class"},
				{Name: "myMethod", Kind: "method"},
			},
			wantName: "myMethod",
		},
		{
			name: "only type, no function",
			entries: []structure.Entry{
				{Name: "MyStruct", Kind: "type"},
			},
			wantName: "",
		},
		{
			name: "Python def",
			entries: []structure.Entry{
				{Name: "my_function", Kind: "def"},
			},
			wantName: "my_function",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findInnermostFunction(tt.entries)
			if tt.wantName == "" {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.wantName, got.Name)
			}
		})
	}
}

func TestExpandContextDown(t *testing.T) {
	t.Run("appends 15 lines after hunk", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(30),
			NewContent: makeLines(30),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "1"}, New: sidebyside.Line{Num: 1, Content: "1"}},
				{Old: sidebyside.Line{Num: 2, Content: "2"}, New: sidebyside.Line{Num: 2, Content: "2"}},
				// gap: 3-29
				{Old: sidebyside.Line{Num: 30, Content: "30"}, New: sidebyside.Line{Num: 30, Content: "30"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		require.Equal(t, 2, len(boundaries))

		n := expandContextDown(fp, boundaries, 0)
		assert.Equal(t, 15, n, "should insert 15 lines")
		assert.Equal(t, 18, len(fp.Pairs), "should have 3 original + 15 new")

		// Verify first inserted line
		assert.Equal(t, 3, fp.Pairs[2].New.Num, "inserted context should start at line 3")
		assert.Equal(t, sidebyside.Context, fp.Pairs[2].New.Type)
		// Verify last inserted line
		assert.Equal(t, 17, fp.Pairs[16].New.Num, "last inserted should be line 17")
	})

	t.Run("clamps to file boundary", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(5),
			NewContent: makeLines(5),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "1"}, New: sidebyside.Line{Num: 1, Content: "1"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		n := expandContextDown(fp, boundaries, 0)
		assert.Equal(t, 4, n, "should only add 4 lines (2-5)")
	})

	t.Run("clamps to next hunk", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(20),
			NewContent: makeLines(20),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "1"}, New: sidebyside.Line{Num: 1, Content: "1"}},
				// gap: only 5 lines (2-6)
				{Old: sidebyside.Line{Num: 7, Content: "7"}, New: sidebyside.Line{Num: 7, Content: "7"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		n := expandContextDown(fp, boundaries, 0)
		assert.Equal(t, 5, n, "should only add 5 lines (gap is 2-6)")
	})
}

func TestExpandContextUp(t *testing.T) {
	t.Run("prepends 15 lines before hunk", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(30),
			NewContent: makeLines(30),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Content: "1"}, New: sidebyside.Line{Num: 1, Content: "1"}},
				// gap
				{Old: sidebyside.Line{Num: 25, Content: "25"}, New: sidebyside.Line{Num: 25, Content: "25"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		require.Equal(t, 2, len(boundaries))

		n := expandContextUp(fp, boundaries, 1)
		assert.Equal(t, 15, n, "should insert 15 lines")

		// First pair of second hunk should now be at line 10 (25-15)
		assert.Equal(t, 10, fp.Pairs[1].New.Num, "inserted context should start at line 10")
	})

	t.Run("clamps to file start", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(10),
			NewContent: makeLines(10),
			Pairs: []sidebyside.LinePair{
				// single hunk starting at line 5
				{Old: sidebyside.Line{Num: 5, Content: "5"}, New: sidebyside.Line{Num: 5, Content: "5"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		n := expandContextUp(fp, boundaries, 0)
		assert.Equal(t, 4, n, "should only add 4 lines (1-4)")
		assert.Equal(t, 1, fp.Pairs[0].New.Num, "first line should be 1")
	})

	t.Run("clamps to previous hunk", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(20),
			NewContent: makeLines(20),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 3, Content: "3"}, New: sidebyside.Line{Num: 3, Content: "3"}},
				// gap: only 5 lines (4-8)
				{Old: sidebyside.Line{Num: 9, Content: "9"}, New: sidebyside.Line{Num: 9, Content: "9"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		n := expandContextUp(fp, boundaries, 1)
		assert.Equal(t, 5, n, "should only add 5 lines (gap is 4-8)")
		assert.Equal(t, 4, fp.Pairs[1].New.Num, "first inserted should be line 4")
	})
}

func TestExpandContextToSignature(t *testing.T) {
	t.Run("expands to signature minus 2", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(30),
			NewContent: makeLines(30),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 20, Content: "20"}, New: sidebyside.Line{Num: 20, Content: "20"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		n := expandContextToSignature(fp, boundaries, 0, 15)
		// Should expand from line 13 (15-2) to line 19
		assert.Equal(t, 7, n, "should insert 7 lines (13-19)")
		assert.Equal(t, 13, fp.Pairs[0].New.Num, "first inserted should be line 13 (sig-2)")
	})

	t.Run("clamps padding to file start", func(t *testing.T) {
		fp := &sidebyside.FilePair{
			OldContent: makeLines(10),
			NewContent: makeLines(10),
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 5, Content: "5"}, New: sidebyside.Line{Num: 5, Content: "5"}},
			},
		}

		boundaries := findHunkBoundaries(fp.Pairs)
		// Signature at line 2, minus 2 padding would be line 0 → clamped to 1
		n := expandContextToSignature(fp, boundaries, 0, 2)
		assert.Equal(t, 4, n, "should insert 4 lines (1-4)")
		assert.Equal(t, 1, fp.Pairs[0].New.Num, "should clamp to line 1")
	})
}

// makeLines creates a slice of n strings ["1", "2", ..., "n"] for test content.
func makeLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%d", i+1)
	}
	return lines
}
