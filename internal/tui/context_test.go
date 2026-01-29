package tui

import (
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
