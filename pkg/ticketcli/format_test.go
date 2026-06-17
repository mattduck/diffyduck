package ticketcli

import (
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattduck/diffyduck/pkg/config"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stripANSI removes ANSI escape codes for test assertions.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// testCommentStyles returns default CLI comment styles for tests (cgo-free).
func testCommentStyles() CommentListStyles {
	return StylesFromConfig(config.ThemeConfig{})
}

// testOneline renders a comment as a oneline row using default styles.
func testOneline(c *ticketdb.Comment, displayID string, termWidth int, now time.Time) string {
	cols := computeOnelineCols([]*ticketdb.Comment{c}, map[string]string{c.ID: displayID}, now)
	return formatCommentOneline(c, displayID, termWidth, now, cols, testCommentStyles())
}

// ansiStub is a ContextHighlighter that wraps each context line in an ANSI color
// code, used to verify the block formatter renders injected highlighting.
type ansiStub struct{}

func (ansiStub) HighlightContext(c *ticketdb.Comment) []string {
	lines := make([]string, 0, len(c.Context.Above)+1+len(c.Context.Below))
	lines = append(lines, c.Context.Above...)
	lines = append(lines, c.Context.Line)
	lines = append(lines, c.Context.Below...)
	for i, l := range lines {
		lines[i] = "\x1b[31m" + l + "\x1b[0m"
	}
	return lines
}

func TestParseCommentTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wantFile string
		wantLine int
		wantErr  bool
	}{
		{"simple", "main.go:42", "main.go", 42, false},
		{"path with dirs", "src/pkg/file.go:100", "src/pkg/file.go", 100, false},
		{"line 1", "f.go:1", "f.go", 1, false},
		{"no colon", "main.go", "", 0, true},
		{"no line number", "main.go:", "", 0, true},
		{"zero line", "main.go:0", "", 0, true},
		{"negative line", "main.go:-1", "", 0, true},
		{"non-numeric line", "main.go:abc", "", 0, true},
		{"empty file", ":42", "", 0, true},
		{"colon in path", "src/a:b/file.go:10", "src/a:b/file.go", 10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, line, err := parseCommentTarget(tt.target)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantFile, file)
				assert.Equal(t, tt.wantLine, line)
			}
		})
	}
}

func TestIsLineInDiff(t *testing.T) {
	diffOutput := `diff --git a/main.go b/main.go
index abc..def 100644
--- a/main.go
+++ b/main.go
@@ -10,6 +10,7 @@ func main() {
 	existing := 1
 	more := 2
 	stuff := 3
+	newLine := 4
 	after := 5
 	end := 6
 }
`
	// Line 13 is the added line (newLine := 4)
	assert.True(t, isLineInDiff(diffOutput, "main.go", 13))
	// Line 10 is context, not changed
	assert.False(t, isLineInDiff(diffOutput, "main.go", 10))
	// Different file
	assert.False(t, isLineInDiff(diffOutput, "other.go", 13))
	// Empty diff
	assert.False(t, isLineInDiff("", "main.go", 1))
}

func TestFormatCommentOneline(t *testing.T) {
	tests := []struct {
		name      string
		comment   *ticketdb.Comment
		termWidth int
		contains  []string
	}{
		{
			name: "basic",
			comment: &ticketdb.Comment{
				ID:        "1705312200000",
				File:      "src/foo.go",
				Line:      42,
				CommitSHA: "abc123def456",
				Text:      "Fix this bug",
			},
			termWidth: 120,
			contains:  []string{"17053122", "src/foo.go:42", "abc123d", "Fix this bug"},
		},
		{
			name: "resolved",
			comment: &ticketdb.Comment{
				ID:        "100",
				File:      "test.go",
				Line:      1,
				CommitSHA: "abc1234",
				Resolved:  true,
				Text:      "Done",
			},
			termWidth: 120,
			contains:  []string{"Done"},
		},
		{
			name: "no commit",
			comment: &ticketdb.Comment{
				ID:   "101",
				File: "test.go",
				Line: 1,
				Text: "No commit",
			},
			termWidth: 120,
			contains:  []string{" - "},
		},
		{
			name: "long text truncated",
			comment: &ticketdb.Comment{
				ID:        "102",
				File:      "test.go",
				Line:      1,
				CommitSHA: "abc1234",
				Text:      "This is a very long comment that should be truncated after sixty characters total",
			},
			termWidth: 80,
			contains:  []string{"..."},
		},
		{
			name: "multiline uses first line",
			comment: &ticketdb.Comment{
				ID:        "103",
				File:      "test.go",
				Line:      1,
				CommitSHA: "abc1234",
				Text:      "First line\nSecond line\nThird line",
			},
			termWidth: 120,
			contains:  []string{"First line"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := stripANSI(testOneline(tt.comment, "", tt.termWidth, time.Now()))
			for _, s := range tt.contains {
				assert.Contains(t, line, s)
			}
		})
	}
}

func TestFormatCommentOneline_Standalone(t *testing.T) {
	c := &ticketdb.Comment{
		ID:        "200",
		CommitSHA: "abc1234",
		Text:      "A general note",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.NotContains(t, line, "(standalone)")
	assert.Contains(t, line, "A general note")
}

func TestFormatCommentOneline_ResolvedStyling(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	c := &ticketdb.Comment{
		ID:        "400",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Branch:    "main",
		Resolved:  true,
		Text:      "Done with this",
	}
	raw := testOneline(c, "", 120, time.Now())
	// Resolved columns should have strikethrough ANSI sequences (;9m or [9m)
	assert.Regexp(t, `\x1b\[[0-9;]*9m`, raw, "expected strikethrough ANSI sequences for resolved comment")
	// Text should be styled (not plain)
	plain := stripANSI(raw)
	assert.Contains(t, plain, "Done with this")
	assert.Greater(t, len(raw), len(plain), "resolved text should have ANSI styling")
	// No [resolved] tag
	assert.NotContains(t, plain, "[resolved]")
}

func TestFormatCommentOneline_DateColumn(t *testing.T) {
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	c := &ticketdb.Comment{
		ID:        "300",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Created:   time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC),
		Text:      "Date test",
	}
	line := stripANSI(testOneline(c, "", 120, now))
	assert.Contains(t, line, "Mar 10 14:30")
	assert.Contains(t, line, "4d")
}

func TestFormatCommentOneline_MultilineExcludesSecond(t *testing.T) {
	c := &ticketdb.Comment{
		ID:        "103",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Text:      "First line\nSecond line",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.NotContains(t, line, "Second line")
}

func TestShortSuffixes(t *testing.T) {
	tests := []struct {
		name string
		ids  []string
		want map[string]string
	}{
		{
			name: "single ID",
			ids:  []string{"1770968997415"},
			want: map[string]string{"1770968997415": "415"},
		},
		{
			name: "two distinct",
			ids:  []string{"1770968997415", "1770881758352"},
			want: map[string]string{"1770968997415": "415", "1770881758352": "352"},
		},
		{
			name: "need longer suffix",
			ids:  []string{"1770968997415", "1770881757415"},
			want: map[string]string{"1770968997415": "97415", "1770881757415": "57415"},
		},
		{
			name: "empty",
			ids:  []string{},
			want: map[string]string{},
		},
		{
			name: "per-ID suffix lengths",
			ids:  []string{"1770968997415", "1770881757415", "1770000000000"},
			want: map[string]string{
				"1770968997415": "97415",
				"1770881757415": "57415",
				"1770000000000": "000",
			},
		},
		{
			name: "differ only in first char",
			ids:  []string{"xabc", "yabc"},
			want: map[string]string{"xabc": "xabc", "yabc": "yabc"},
		},
		{
			name: "short IDs",
			ids:  []string{"abc", "def"},
			want: map[string]string{"abc": "abc", "def": "def"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortSuffixes(tt.ids)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatCommentOneline_ShortID(t *testing.T) {
	c := &ticketdb.Comment{
		ID:        "1770968997415",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Text:      "Hello",
	}
	line := stripANSI(testOneline(c, "7415", 120, time.Now()))
	assert.Contains(t, line, "7415")
	assert.NotContains(t, line, "1770968997415")
}

func TestFormatCommentBlock(t *testing.T) {
	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	c := &ticketdb.Comment{
		ID:        "1705312200000",
		File:      "src/foo.go",
		Line:      42,
		CommitSHA: "abc123def456",
		Branch:    "main",
		Created:   created,
		Text:      "Fix this bug\nIt causes crashes",
		Context: ticketdb.LineContext{
			Above: []string{"func foo() {", "    x := 1"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))

	// Metadata (two-column layout at width 120)
	assert.Contains(t, block, "┃ Date:   Jan 15 10:30 0m")
	assert.Contains(t, block, "File:   src/foo.go:42\n")
	assert.Contains(t, block, "┃ Status: unresolved")
	assert.Contains(t, block, "Ref:    abc123d on main\n")
	assert.Contains(t, block, "┃ ID:     1705312200000\n")
	// Diff context (line numbers: 40-43, gutter width 2)
	assert.Contains(t, block, "┃   40 func foo() {\n")
	assert.Contains(t, block, "┃   41     x := 1\n")
	assert.Contains(t, block, "┃ > 42     return x\n")
	assert.Contains(t, block, "┃   43 }\n")
	// Comment text
	assert.Contains(t, block, "┃ Fix this bug\n")
	assert.Contains(t, block, "┃ It causes crashes\n")
}

func TestFormatCommentBlock_NarrowTerminal(t *testing.T) {
	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	c := &ticketdb.Comment{
		ID:        "1705312200000",
		File:      "src/foo.go",
		Line:      42,
		CommitSHA: "abc123def456",
		Branch:    "main",
		Created:   created,
		Text:      "Fix this bug",
		Context: ticketdb.LineContext{
			Above: []string{"func foo() {"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	// Narrow terminal forces single-column layout
	block := stripANSI(formatCommentBlock(c, nil, 40, "", c.Created, testCommentStyles()))

	// Each field on its own line
	assert.Contains(t, block, "┃ Date:   Jan 15 10:30 0m\n")
	assert.Contains(t, block, "┃ Status: unresolved\n")
	assert.Contains(t, block, "┃ ID:     1705312200000\n")
	assert.Contains(t, block, "┃ File:   src/foo.go:42\n")
	assert.Contains(t, block, "┃ Ref:    abc123d on main\n")
}

func TestFormatCommentBlock_Resolved(t *testing.T) {
	c := &ticketdb.Comment{
		ID:       "100",
		File:     "test.go",
		Line:     1,
		Created:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Resolved: true,
		Text:     "Done",
		Context:  ticketdb.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	assert.Contains(t, block, "┃ Status: resolved\n")
	assert.Contains(t, block, "┃ ID:     100\n")
}

func TestFormatCommentBlock_NoCommit(t *testing.T) {
	c := &ticketdb.Comment{
		ID:      "101",
		File:    "test.go",
		Line:    1,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "No commit",
		Context: ticketdb.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	assert.NotContains(t, block, "Ref:")
}

func TestFormatCommentBlock_Highlighted(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	c := &ticketdb.Comment{
		ID:      "200",
		File:    "test.go",
		Line:    3,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Check this",
		Context: ticketdb.LineContext{
			Above: []string{"func foo() {", "    x := 1"},
			Line:  "    return x",
			Below: []string{"}"},
		},
	}

	// With an injected highlighter: context lines carry ANSI codes.
	block := formatCommentBlock(c, ansiStub{}, 120, "", c.Created, testCommentStyles())
	stripped := stripANSI(block)

	// Content should be the same after stripping ANSI
	assert.Contains(t, stripped, "func foo() {\n")
	assert.Contains(t, stripped, "    return x\n")

	// The highlighted version should have ANSI codes (more bytes than stripped)
	assert.Greater(t, len(block), len(stripped), "highlighting should add ANSI codes")

	// Nil highlighter should also work (plain text)
	plain := formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles())
	plainStripped := stripANSI(plain)
	assert.Equal(t, stripped, plainStripped, "stripped output should match regardless of highlighter")
}

func TestFormatCommentBlock_PlainContext(t *testing.T) {
	c := &ticketdb.Comment{
		ID:      "201",
		File:    "data.xyz",
		Line:    1,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Unknown file type",
		Context: ticketdb.LineContext{Line: "some content"},
	}

	// With no highlighter, context renders as plain text.
	block := formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles())
	stripped := stripANSI(block)
	assert.Contains(t, stripped, "some content")
}

func TestFormatCommentOneline_WithAuthor(t *testing.T) {
	c := &ticketdb.Comment{
		ID:        "200",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Author:    "Claude",
		Text:      "Review note",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.Contains(t, line, "[Claude]")
	assert.Contains(t, line, "Review note")
}

func TestFormatCommentOneline_WithoutAuthor(t *testing.T) {
	c := &ticketdb.Comment{
		ID:        "201",
		File:      "test.go",
		Line:      1,
		CommitSHA: "abc1234",
		Text:      "Human note",
	}
	line := stripANSI(testOneline(c, "", 120, time.Now()))
	assert.NotContains(t, line, "[")
	assert.Contains(t, line, "Human note")
}

func TestFormatCommentBlock_WithAuthor(t *testing.T) {
	c := &ticketdb.Comment{
		ID:      "300",
		File:    "test.go",
		Line:    1,
		Branch:  "main",
		Author:  "Claude",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Check this",
		Context: ticketdb.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	// Author should appear in the right column as a header
	assert.Contains(t, block, "Author: Claude")
	// Should NOT appear as a separate "commented" header line
	assert.NotContains(t, block, "Claude commented")
}

func TestFormatCommentBlock_WithoutAuthor(t *testing.T) {
	c := &ticketdb.Comment{
		ID:      "301",
		File:    "test.go",
		Line:    1,
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "No author",
		Context: ticketdb.LineContext{Line: "code"},
	}
	block := stripANSI(formatCommentBlock(c, nil, 120, "", c.Created, testCommentStyles()))
	assert.NotContains(t, block, "Author:")
}

func TestFormatCommentBlock_AuthorNarrowTerminal(t *testing.T) {
	c := &ticketdb.Comment{
		ID:      "302",
		File:    "test.go",
		Line:    1,
		Author:  "Bot",
		Created: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Text:    "Note",
		Context: ticketdb.LineContext{Line: "code"},
	}
	// Single column fallback
	block := stripANSI(formatCommentBlock(c, nil, 40, "", c.Created, testCommentStyles()))
	assert.Contains(t, block, "Author: Bot")
}
