package content

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadLimitedLines_NormalContent(t *testing.T) {
	content := "line1\nline2\nline3"
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Equal(t, []string{"line1", "line2", "line3"}, lines)
}

func TestReadLimitedLines_LongLinesPreserved(t *testing.T) {
	// Long lines are preserved (not truncated) to allow tree-sitter parsing
	longLine := strings.Repeat("x", 1000)
	content := longLine + "\nshort"
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated)
	assert.Len(t, lines, 2)
	assert.Len(t, lines[0], 1000)
}

func TestReadLimitedLines_LineCountLimit(t *testing.T) {
	// Use smaller limits for testing
	maxLines := 5
	maxBytes := 1024 * 1024

	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "line")
	}
	content := strings.Join(lines, "\n")
	reader := strings.NewReader(content)

	result, truncated, err := ReadLimitedLinesWithLimits(reader, maxLines, maxBytes)
	require.NoError(t, err)
	assert.True(t, truncated, "should be truncated due to line count")
	assert.Len(t, result, maxLines)
}

func TestReadLimitedLines_ByteLimit(t *testing.T) {
	// Use smaller limits for testing
	maxLines := 10000
	maxBytes := 50 // Very small byte limit

	content := strings.Repeat("x", 100) // 100 bytes of content
	reader := strings.NewReader(content)

	result, truncated, err := ReadLimitedLinesWithLimits(reader, maxLines, maxBytes)
	require.NoError(t, err)
	assert.True(t, truncated, "should be truncated due to byte limit")
	// Should have read something but not everything
	assert.NotEmpty(t, result)
}

func TestReadLimitedLines_LongLinesWithinByteLimit(t *testing.T) {
	// Long lines are preserved as long as we're under the byte limit
	longLine := strings.Repeat("var x=1;", 1000) // ~8KB line
	content := longLine + "\n" + longLine + "\n" + longLine
	reader := strings.NewReader(content)

	lines, truncated, err := ReadLimitedLines(reader)
	require.NoError(t, err)
	assert.False(t, truncated, "should NOT be truncated (under byte and line count limits)")
	assert.Len(t, lines, 3)
	assert.Equal(t, len(longLine), len(lines[0]), "long lines should be preserved")
}

func TestReadLimitedLines_ByteLimitStopsReading(t *testing.T) {
	// Test that we stop reading when byte limit is hit
	maxLines := 10000
	maxBytes := 100

	// Create content larger than byte limit with multiple lines
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, strings.Repeat("x", 20)) // 20 bytes per line
	}
	content := strings.Join(lines, "\n")
	reader := strings.NewReader(content)

	result, truncated, err := ReadLimitedLinesWithLimits(reader, maxLines, maxBytes)
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
