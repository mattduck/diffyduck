package comments

import (
	"testing"
	"time"
)

func TestFindCommentPositionExactMatch(t *testing.T) {
	fileLines := []string{
		"line 1",
		"line 2",
		"target line",
		"line 4",
		"line 5",
	}

	// Create a comment anchored to line 3
	ctx := ExtractContextForLine(fileLines, 3)
	c := &Comment{
		Line:    3,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
	}

	result := FindCommentPosition(c, fileLines)

	if !result.Found {
		t.Error("expected to find comment")
	}
	if result.Line != 3 {
		t.Errorf("expected line 3, got %d", result.Line)
	}
	if !result.Exact {
		t.Error("expected exact match")
	}
}

func TestFindCommentPositionLineMoved(t *testing.T) {
	// Original file when comment was created
	originalLines := []string{
		"line 1",
		"line 2",
		"target line",
		"line 4",
		"line 5",
	}

	ctx := ExtractContextForLine(originalLines, 3)
	c := &Comment{
		Line:    3,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
	}

	// New file with lines inserted at top
	newLines := []string{
		"new line A",
		"new line B",
		"line 1",
		"line 2",
		"target line", // Now at line 5
		"line 4",
		"line 5",
	}

	result := FindCommentPosition(c, newLines)

	if !result.Found {
		t.Error("expected to find comment")
	}
	if result.Line != 5 {
		t.Errorf("expected line 5, got %d", result.Line)
	}
	if !result.Exact {
		t.Error("expected exact match (context moved together)")
	}
}

func TestFindCommentPositionContextChanged(t *testing.T) {
	// Original file
	originalLines := []string{
		"line 1",
		"line 2",
		"target line",
		"line 4",
		"line 5",
	}

	ctx := ExtractContextForLine(originalLines, 3)
	c := &Comment{
		Line:    3,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
	}

	// New file with context changed but target line same
	newLines := []string{
		"changed 1",
		"changed 2",
		"target line", // Same content, different context
		"changed 4",
		"changed 5",
	}

	result := FindCommentPosition(c, newLines)

	if !result.Found {
		t.Error("expected to find comment")
	}
	if result.Line != 3 {
		t.Errorf("expected line 3, got %d", result.Line)
	}
	if result.Exact {
		t.Error("expected non-exact match (context changed)")
	}
}

func TestFindCommentPositionLineDeleted(t *testing.T) {
	originalLines := []string{
		"line 1",
		"line 2",
		"target line",
		"line 4",
		"line 5",
	}

	ctx := ExtractContextForLine(originalLines, 3)
	c := &Comment{
		Line:    3,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
	}

	// New file with target line removed
	newLines := []string{
		"line 1",
		"line 2",
		"line 4",
		"line 5",
	}

	result := FindCommentPosition(c, newLines)

	if result.Found {
		t.Error("expected to NOT find comment when line deleted")
	}
}

func TestFindCommentPositionMultipleMatches(t *testing.T) {
	// File with duplicate line content
	fileLines := []string{
		"return nil",
		"some code",
		"return nil", // Original position (line 3)
		"more code",
		"return nil",
	}

	// Comment was on line 3
	ctx := LineContext{
		Above: []string{"return nil", "some code"},
		Line:  "return nil",
		Below: []string{"more code", "return nil"},
	}
	c := &Comment{
		Line:    3,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
	}

	result := FindCommentPosition(c, fileLines)

	if !result.Found {
		t.Error("expected to find comment")
	}
	if result.Line != 3 {
		t.Errorf("expected line 3 (exact anchor match), got %d", result.Line)
	}
	if !result.Exact {
		t.Error("expected exact match")
	}
}

func TestFindCommentPositionMultipleMatchesFallback(t *testing.T) {
	// File with duplicate line content but context changed
	originalLines := []string{
		"unique above",
		"code",
		"return nil", // line 3
		"unique below",
	}

	ctx := ExtractContextForLine(originalLines, 3)
	c := &Comment{
		Line:    3,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
	}

	// New file: duplicates and context changed, prefer closest to original
	newLines := []string{
		"return nil", // line 1
		"changed",
		"return nil", // line 3 - closest to original
		"changed",
		"return nil", // line 5
	}

	result := FindCommentPosition(c, newLines)

	if !result.Found {
		t.Error("expected to find comment")
	}
	// Should prefer line 3 as closest to original position
	if result.Line != 3 {
		t.Errorf("expected line 3 (closest to original), got %d", result.Line)
	}
	if result.Exact {
		t.Error("expected non-exact match")
	}
}

func TestFindCommentPositionEmptyContext(t *testing.T) {
	c := &Comment{
		Line:    1,
		Context: LineContext{Line: ""}, // Empty line content
	}

	fileLines := []string{"line 1", "line 2"}

	result := FindCommentPosition(c, fileLines)

	if result.Found {
		t.Error("expected to NOT find comment with empty context")
	}
}

func TestExtractContextForLine(t *testing.T) {
	fileLines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	}

	tests := []struct {
		name        string
		lineNum     int
		expectLine  string
		expectAbove int
		expectBelow int
	}{
		{
			name:        "middle of file",
			lineNum:     3,
			expectLine:  "line 3",
			expectAbove: 2,
			expectBelow: 2,
		},
		{
			name:        "first line",
			lineNum:     1,
			expectLine:  "line 1",
			expectAbove: 0,
			expectBelow: 2,
		},
		{
			name:        "second line",
			lineNum:     2,
			expectLine:  "line 2",
			expectAbove: 1,
			expectBelow: 2,
		},
		{
			name:        "last line",
			lineNum:     5,
			expectLine:  "line 5",
			expectAbove: 2,
			expectBelow: 0,
		},
		{
			name:        "second to last",
			lineNum:     4,
			expectLine:  "line 4",
			expectAbove: 2,
			expectBelow: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ExtractContextForLine(fileLines, tt.lineNum)

			if ctx.Line != tt.expectLine {
				t.Errorf("Line: got %q, want %q", ctx.Line, tt.expectLine)
			}
			if len(ctx.Above) != tt.expectAbove {
				t.Errorf("Above count: got %d, want %d", len(ctx.Above), tt.expectAbove)
			}
			if len(ctx.Below) != tt.expectBelow {
				t.Errorf("Below count: got %d, want %d", len(ctx.Below), tt.expectBelow)
			}
		})
	}
}

func TestExtractContextForLineOutOfBounds(t *testing.T) {
	fileLines := []string{"line 1", "line 2"}

	// Line 0
	ctx := ExtractContextForLine(fileLines, 0)
	if ctx.Line != "" {
		t.Errorf("expected empty context for line 0, got %q", ctx.Line)
	}

	// Line beyond file
	ctx = ExtractContextForLine(fileLines, 10)
	if ctx.Line != "" {
		t.Errorf("expected empty context for line 10, got %q", ctx.Line)
	}

	// Negative line
	ctx = ExtractContextForLine(fileLines, -1)
	if ctx.Line != "" {
		t.Errorf("expected empty context for line -1, got %q", ctx.Line)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "unix line endings",
			input:    "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "windows line endings",
			input:    "line1\r\nline2\r\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "trailing newline",
			input:    "line1\nline2\n",
			expected: []string{"line1", "line2"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single line no newline",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "single line with newline",
			input:    "single\n",
			expected: []string{"single"},
		},
		{
			name:     "mixed line endings",
			input:    "line1\r\nline2\nline3\r",
			expected: []string{"line1", "line2", "line3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitLines(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}

			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: got %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestMatchResultWithRealComment(t *testing.T) {
	// Simulate a real workflow
	originalContent := `package main

func foo() {
    if x > 0 {
        return true
    }
    return false
}
`
	originalLines := SplitLines(originalContent)

	// Create comment on "return true" line (line 5)
	now := time.Now()
	ctx := ExtractContextForLine(originalLines, 5)
	c := &Comment{
		ID:      "123",
		Text:    "Consider early return",
		File:    "main.go",
		Line:    5,
		Anchor:  ctx.ComputeAnchor(),
		Context: ctx,
		Created: now,
		Updated: now,
	}

	// Later, code is modified with new function added above
	modifiedContent := `package main

func bar() {
    // new function
}

func foo() {
    if x > 0 {
        return true
    }
    return false
}
`
	modifiedLines := SplitLines(modifiedContent)

	result := FindCommentPosition(c, modifiedLines)

	if !result.Found {
		t.Fatal("expected to find comment")
	}
	// "return true" is now at line 9
	if result.Line != 9 {
		t.Errorf("expected line 9, got %d", result.Line)
	}
	if !result.Exact {
		t.Error("expected exact match (context moved together)")
	}
}
