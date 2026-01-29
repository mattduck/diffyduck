package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderTreeContinuation(t *testing.T) {
	// Create a simple style that renders text as-is (no ANSI codes)
	plainStyle := lipgloss.NewStyle()

	tests := []struct {
		name      string
		ancestors []TreeLevel
		want      string
	}{
		{
			name:      "empty ancestors",
			ancestors: nil,
			want:      "",
		},
		{
			name: "single non-last",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0},
			},
			want: "│    ", // │ + 4 spaces = 5 chars
		},
		{
			name: "single last",
			ancestors: []TreeLevel{
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 0},
			},
			want: "     ", // 5 spaces (no continuation for last)
		},
		{
			name: "single folded",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: true, Style: plainStyle, Depth: 0},
			},
			want: "     ", // 5 spaces (no continuation for folded)
		},
		{
			name: "two levels, neither last",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0},
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			want: "│    │    ", // 10 chars total
		},
		{
			name: "two levels, first non-last, second last",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0},
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			want: "│         ", // │ + 4 spaces + 5 spaces
		},
		{
			name: "two levels, first last, second non-last",
			ancestors: []TreeLevel{
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 0},
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			want: "     │    ", // 5 spaces + │ + 4 spaces
		},
		{
			name: "three levels for hunk support",
			ancestors: []TreeLevel{
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
				{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1}, // file
				{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 2},  // hunk (last)
			},
			want: "│    │         ", // 15 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTreeContinuation(tt.ancestors)
			if got != tt.want {
				t.Errorf("renderTreeContinuation() = %q (len=%d), want %q (len=%d)",
					got, len(got), tt.want, len(tt.want))
			}
		})
	}
}

func TestRenderTreeBranch(t *testing.T) {
	plainStyle := lipgloss.NewStyle()

	tests := []struct {
		name  string
		level TreeLevel
		want  string
	}{
		{
			name:  "non-last branch",
			level: TreeLevel{IsLast: false, Style: plainStyle},
			want:  "├━━━", // uses heavy horizontal line
		},
		{
			name:  "last branch",
			level: TreeLevel{IsLast: true, Style: plainStyle},
			want:  "└━━━", // uses heavy horizontal line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTreeBranch(tt.level)
			if got != tt.want {
				t.Errorf("renderTreeBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTreePrefix(t *testing.T) {
	plainStyle := lipgloss.NewStyle()

	tests := []struct {
		name     string
		path     TreePath
		isHeader bool
		want     string
	}{
		{
			name:     "empty path, content row",
			path:     TreePath{},
			isHeader: false,
			want:     "   ", // margin(1) + contentIndent(2) = 3 chars
		},
		{
			name: "file header (depth 1)",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
				},
				Current: &TreeLevel{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			isHeader: true,
			want:     " │    ├━━━", // margin(1) + continuation(5) + branch(4) = 10 chars
		},
		{
			name: "last file header",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
				},
				Current: &TreeLevel{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 1},
			},
			isHeader: true,
			want:     " │    └━━━", // margin(1) + continuation(5) + branch(4)
		},
		{
			name: "content row under non-last file",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit (outer)
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 1}, // file (innermost parent)
				},
				Current: nil,
			},
			isHeader: false,
			want:     " │    │      ", // margin(1) + innermost(│+4=5) + contentIndent(2)
		},
		{
			name: "content row under last file in non-last commit",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit (not last)
					{IsLast: true, IsFolded: false, Style: plainStyle, Depth: 1},  // file (last in commit)
				},
				Current: nil,
			},
			isHeader: false,
			want:     " │           ", // margin(1) + outer(0) + innermost(5 spaces for last) + contentIndent(2)
		},
		{
			name: "content under folded parent",
			path: TreePath{
				Ancestors: []TreeLevel{
					{IsLast: false, IsFolded: false, Style: plainStyle, Depth: 0}, // commit
					{IsLast: false, IsFolded: true, Style: plainStyle, Depth: 1},  // file (folded)
				},
				Current: nil,
			},
			isHeader: false,
			want:     " │           ", // margin(1) + outer(0) + innermost(5 spaces for folded) + contentIndent(2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTreePrefix(tt.path, tt.isHeader)
			if got != tt.want {
				t.Errorf("renderTreePrefix() = %q (len=%d), want %q (len=%d)",
					got, len(got), tt.want, len(tt.want))
			}
		})
	}
}

func TestTreeWidth(t *testing.T) {
	tests := []struct {
		name         string
		numAncestors int
		isHeader     bool
		want         int
	}{
		// Headers: margin(1) + outer*(5) + innermost(5) + branch(4)
		{"0 ancestors header (root)", 0, true, 5},  // margin(1) + branch(4) = 5
		{"1 ancestor header (file)", 1, true, 10},  // 1 + 5 + 4 = 10
		{"2 ancestors header (hunk)", 2, true, 15}, // 1 + 10 + 4 = 15
		{"3 ancestors header", 3, true, 20},        // 1 + 15 + 4 = 20

		// Content: margin(1) + ancestors*(5) + contentIndent(2)
		{"0 ancestors content", 0, false, 3},                 // margin(1) + contentIndent(2) = 3
		{"1 ancestor content (file content)", 1, false, 8},   // 1 + 5 + 2 = 8
		{"2 ancestors content", 2, false, 13},                // 1 + 10 + 2 = 13
		{"3 ancestors content (hunk content)", 3, false, 18}, // 1 + 15 + 2 = 18
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := treeWidth(tt.numAncestors, tt.isHeader)
			if got != tt.want {
				t.Errorf("treeWidth(%d, %v) = %d, want %d", tt.numAncestors, tt.isHeader, got, tt.want)
			}
		})
	}
}
