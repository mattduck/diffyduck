package content

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/diff"
)

func TestReadLimitedLines_NormalContent(t *testing.T) {
	content := "line1\nline2\nline3"
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestReadLimitedLines_LongLinesNotTruncated(t *testing.T) {
	// ReadLimitedLines no longer truncates long lines because:
	// 1. The diff parser already handles line truncation for display
	// 2. Truncating lines breaks syntax for tree-sitter parsing
	longLine := strings.Repeat("x", diff.MaxLineLength+100)
	content := longLine + "\nshort"
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated, "long lines should NOT trigger truncation flag")
	assert.Len(t, lines, 2)

	// Long line should NOT be truncated
	assert.Len(t, lines[0], diff.MaxLineLength+100)
	assert.False(t, strings.HasSuffix(lines[0], diff.LineTruncationText))
}

func TestReadLimitedLinesWithLimits_LineLengthLimit(t *testing.T) {
	// When explicitly passing maxLineLen > 0, lines should be truncated
	longLine := strings.Repeat("x", diff.MaxLineLength+100)
	content := longLine + "\nshort"
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLinesWithLimits(reader, diff.MaxLinesPerFile, diff.MaxLineLength, diff.MaxContentBytes)
	require.NoError(t, err)
	assert.True(t, truncated, "should be truncated due to long line")
	assert.Len(t, lines, 2)

	// First line should be truncated with suffix
	assert.Len(t, lines[0], diff.MaxLineLength)
	assert.True(t, strings.HasSuffix(lines[0], diff.LineTruncationText))
}

func TestReadLimitedLines_LineCountLimit(t *testing.T) {
	// Use smaller limits for testing
	maxLines := 5
	maxLineLen := 100
	maxBytes := 1024 * 1024

	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "line")
	}
	content := strings.Join(lines, "\n")
	reader := strings.NewReader(content)

	result, truncated, err := ReadLimitedLinesWithLimits(reader, maxLines, maxLineLen, maxBytes)
	require.NoError(t, err)
	assert.True(t, truncated, "should be truncated due to line count")
	assert.Len(t, result, maxLines)
}

func TestReadLimitedLines_ByteLimit(t *testing.T) {
	// Use smaller limits for testing
	maxLines := 10000
	maxLineLen := 1000
	maxBytes := 50 // Very small byte limit

	content := strings.Repeat("x", 100) // 100 bytes of content
	reader := strings.NewReader(content)

	result, truncated, err := ReadLimitedLinesWithLimits(reader, maxLines, maxLineLen, maxBytes)
	require.NoError(t, err)
	assert.True(t, truncated, "should be truncated due to byte limit")
	// Should have read something but not everything
	assert.NotEmpty(t, result)
}

func TestReadLimitedLines_MinifiedJS(t *testing.T) {
	// Simulate minified JS: multiple long lines
	// Test that long lines are NOT truncated per-line (only by byte limit)
	longLine := strings.Repeat("var x=1;", 1000) // ~8KB line, exceeds MaxLineLength (300) but not byte limit
	content := longLine + "\n" + longLine + "\n" + longLine
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated, "should NOT be truncated (under byte and line count limits)")
	assert.Len(t, lines, 3)

	// Lines should NOT be truncated at MaxLineLength
	assert.Greater(t, len(lines[0]), diff.MaxLineLength, "line should NOT be truncated at MaxLineLength")
	assert.False(t, strings.HasSuffix(lines[0], diff.LineTruncationText), "should NOT have truncation suffix")
}

func TestReadLimitedLines_ByteLimitStopsReading(t *testing.T) {
	// Test that we stop reading when byte limit is hit
	maxLines := 10000
	maxLineLen := 1000
	maxBytes := 100

	// Create content larger than byte limit with multiple lines
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, strings.Repeat("x", 20)) // 20 bytes per line
	}
	content := strings.Join(lines, "\n")
	reader := strings.NewReader(content)

	result, truncated, err := ReadLimitedLinesWithLimits(reader, maxLines, maxLineLen, maxBytes)
	require.NoError(t, err)
	assert.True(t, truncated, "should be truncated due to byte limit")
	// Should have stopped before reading all 100 lines
	assert.Less(t, len(result), 100)
}

func TestReadLimitedLines_EmptyContent(t *testing.T) {
	reader := strings.NewReader("")

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Empty(t, lines)
}

func TestReadLimitedLines_SingleNewline(t *testing.T) {
	reader := strings.NewReader("\n")

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, []string{""}, lines)
}
