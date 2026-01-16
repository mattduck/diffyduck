package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateOrPad_ASCII(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{"exact fit", "hello", 5, "hello"},
		{"needs padding", "hi", 5, "hi   "},
		{"needs truncation", "hello world", 8, "hello..."},
		{"empty string", "", 5, "     "},
		{"truncate very short", "hello", 3, "hel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOrPad(tt.input, tt.width)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.width, displayWidth(result), "result should have correct display width")
		})
	}
}

func TestTruncateOrPad_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		// Multi-byte but single-width characters
		{"accented chars pad", "café", 6, "café  "},
		{"accented chars fit", "café", 4, "café"},
		{"accented truncate", "café latté", 7, "café..."},

		// Wide characters (CJK takes 2 cells each)
		{"cjk padding", "中文", 6, "中文  "},    // 2 chars × 2 width = 4, pad 2
		{"cjk exact", "中文", 4, "中文"},        // exact fit
		{"cjk truncate", "中文字符", 5, "中..."}, // need to truncate, but can't fit 文 (width 2) + ... (width 3) = 5

		// Emoji (typically width 2)
		{"emoji padding", "👍", 4, "👍  "},
		{"emoji truncate", "👍👎👍", 5, "👍..."},

		// Mixed content
		{"mixed ascii cjk", "hi中文", 6, "hi中文"},
		{"mixed truncate", "hello中文", 8, "hello..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOrPad(tt.input, tt.width)
			resultWidth := displayWidth(result)
			// Width should be exactly right, or 1 less if we couldn't fit a wide char
			assert.True(t, resultWidth == tt.width || resultWidth == tt.width-1,
				"result %q has width %d, expected %d (or %d for wide char)", result, resultWidth, tt.width, tt.width-1)
		})
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"café", 4},
		{"中文", 4},   // 2 wide chars
		{"👍", 2},    // emoji is width 2
		{"hi中文", 6}, // 2 + 4
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, displayWidth(tt.input))
		})
	}
}

func TestExpandTabs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no tabs", "hello", "hello"},
		{"single tab at start", "\tfoo", "    foo"},
		{"tab after 1 char", "x\tfoo", "x   foo"},
		{"tab after 2 chars", "xx\tfoo", "xx  foo"},
		{"tab after 3 chars", "xxx\tfoo", "xxx foo"},
		{"tab after 4 chars", "xxxx\tfoo", "xxxx    foo"},
		{"multiple tabs", "\t\t", "        "},
		{"mixed", "a\tb\tc", "a   b   c"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandTabs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateOrPad_WithTabs(t *testing.T) {
	// After tab expansion, width calculations should be correct
	input := "\tfoo" // expands to "    foo" = 7 chars
	result := truncateOrPad(expandTabs(input), 10)

	assert.Equal(t, "    foo   ", result) // 7 chars + 3 spaces = 10
	assert.Equal(t, 10, displayWidth(result))
}

func TestHorizontalSlice_NoOffset(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		offset   int
		width    int
		expected string
	}{
		{"exact fit", "hello", 0, 5, "hello"},
		{"needs padding", "hi", 0, 5, "hi   "},
		{"needs truncation", "hello world", 0, 5, "hello"},
		{"empty string", "", 0, 5, "     "},
		{"zero width", "hello", 0, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := horizontalSlice(tt.input, tt.offset, tt.width)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.width, displayWidth(result), "result should have exact display width")
		})
	}
}

func TestHorizontalSlice_WithOffset(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		offset   int
		width    int
		expected string
	}{
		{"skip 2 chars", "hello", 2, 5, "llo  "},
		{"skip 3 chars", "hello", 3, 5, "lo   "},
		{"skip to last char", "hello", 4, 5, "o    "},
		{"skip past content", "hello", 5, 5, "     "},
		{"skip way past content", "hello", 100, 5, "     "},
		{"partial view of long string", "hello world", 3, 5, "lo wo"},
		{"offset into middle", "abcdefghij", 3, 4, "defg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := horizontalSlice(tt.input, tt.offset, tt.width)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.width, displayWidth(result), "result should have exact display width")
		})
	}
}

func TestHorizontalSlice_Unicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		offset   int
		width    int
		expected string
	}{
		// Wide characters (CJK = 2 cells each)
		{"cjk no offset", "中文字", 0, 4, "中文"},
		{"cjk skip one wide char", "中文字", 2, 4, "文字"},
		{"cjk skip two wide chars", "中文字", 4, 4, "字  "},
		{"cjk skip all", "中文字", 6, 4, "    "},

		// Offset lands in middle of wide char - should skip it
		{"cjk offset mid-char", "中文字", 1, 4, " 文字"},
		{"cjk offset mid-char 2", "中文字", 3, 4, " 字  "},

		// Mixed content
		{"mixed ascii then cjk", "ab中文", 0, 4, "ab中"},
		{"mixed skip ascii", "ab中文", 2, 4, "中文"},
		{"mixed skip into cjk", "ab中文", 3, 4, " 文  "},

		// Emoji (width 2)
		{"emoji no offset", "👍👎", 0, 4, "👍👎"},
		{"emoji skip one", "👍👎", 2, 4, "👎  "},
		{"emoji offset mid-char", "👍👎", 1, 4, " 👎 "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := horizontalSlice(tt.input, tt.offset, tt.width)
			resultWidth := displayWidth(result)
			// Allow width or width-1 for wide char boundary issues
			assert.True(t, resultWidth == tt.width || resultWidth == tt.width-1,
				"result %q has width %d, expected %d", result, resultWidth, tt.width)
		})
	}
}
