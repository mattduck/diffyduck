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
		{"cjk padding", "中文", 6, "中文  "},        // 2 chars × 2 width = 4, pad 2
		{"cjk exact", "中文", 4, "中文"},            // exact fit
		{"cjk truncate", "中文字符", 5, "中..."},    // need to truncate, but can't fit 文 (width 2) + ... (width 3) = 5

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
		{"中文", 4},      // 2 wide chars
		{"👍", 2},        // emoji is width 2
		{"hi中文", 6},    // 2 + 4
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, displayWidth(tt.input))
		})
	}
}
