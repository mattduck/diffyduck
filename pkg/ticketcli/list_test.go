package ticketcli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseListArgs_Defaults(t *testing.T) {
	o, err := ParseListArgs([]string{"list"})
	require.NoError(t, err)
	assert.Equal(t, SourceAll, o.Source)
	assert.Empty(t, o.Markers)
	assert.False(t, o.NSet)
}

func TestParseListArgs_Flags(t *testing.T) {
	o, err := ParseListArgs([]string{
		"list",
		"--source", "code",
		"--marker", "TODO,FIXME",
		"--file=pkg/",
		"--grep", "buffer",
		"-n5",
	})
	require.NoError(t, err)
	assert.Equal(t, SourceCode, o.Source)
	assert.Equal(t, []string{"TODO", "FIXME"}, o.Markers)
	assert.Equal(t, "pkg/", o.File)
	assert.Equal(t, "buffer", o.Grep)
	assert.True(t, o.NSet)
	assert.Equal(t, 5, o.N)
}

func TestParseListArgs_SourceAliases(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"tickets", SourceState},
		{"state", SourceState},
		{"markers", SourceCode},
		{"code", SourceCode},
		{"all", SourceAll},
	} {
		o, err := ParseListArgs([]string{"list", "--source", tc.in})
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, o.Source, tc.in)
	}
}

func TestParseListArgs_Errors(t *testing.T) {
	cases := [][]string{
		{"list", "--source", "bogus"},
		{"list", "--status", "bogus"},
		{"list", "--bogus"},
		{"list", "--all-branches", "--branch", "main"},
		{"list", "--source", "state", "--marker", "TODO"},
		{"list", "-nfoo"},
	}
	for _, c := range cases {
		_, err := ParseListArgs(c)
		assert.Error(t, err, "%v", c)
	}
}

func TestParseListArgs_Help(t *testing.T) {
	_, err := ParseListArgs([]string{"list", "-h"})
	assert.True(t, errors.Is(err, errHelp))
}

func TestParseListArgs_BranchValueVsAll(t *testing.T) {
	o, err := ParseListArgs([]string{"list", "-b", "feature"})
	require.NoError(t, err)
	assert.Equal(t, "feature", o.Branch)
	assert.False(t, o.AllBranches)

	o, err = ParseListArgs([]string{"list", "-b"})
	require.NoError(t, err)
	assert.True(t, o.AllBranches)
	assert.Empty(t, o.Branch)
}

func TestFileMatches(t *testing.T) {
	assert.True(t, fileMatches("", "anything"))
	assert.True(t, fileMatches("pkg/foo.go", "pkg/foo.go"))
	assert.False(t, fileMatches("pkg/foo.go", "pkg/bar.go"))
	assert.True(t, fileMatches("pkg/", "pkg/foo.go"))
	assert.False(t, fileMatches("pkg/", "cmd/foo.go"))
}

func TestGrepMatches(t *testing.T) {
	assert.True(t, grepMatches("", "no needle"))
	assert.True(t, grepMatches("BUF", "fix the buffer"))
	assert.True(t, grepMatches("title", "", "Use a Title here"))
	assert.False(t, grepMatches("zzz", "nope", "also nope"))
}

func TestParseListArgs_Rule(t *testing.T) {
	o, err := ParseListArgs([]string{"list", "--rule", "no-bare-dict"})
	require.NoError(t, err)
	assert.Equal(t, "no-bare-dict", o.Rule)

	o, err = ParseListArgs([]string{"list", "--rule=other"})
	require.NoError(t, err)
	assert.Equal(t, "other", o.Rule)

	// --rule is valid for every source: it filters ticket rule tags and
	// code-marker scopes alike.
	for _, src := range []string{"state", "code", "all"} {
		o, err := ParseListArgs([]string{"list", "--source", src, "--rule", "r"})
		require.NoError(t, err, src)
		assert.Equal(t, "r", o.Rule, src)
	}
}

func TestParseListArgs_ExitCode(t *testing.T) {
	o, err := ParseListArgs([]string{"list"})
	require.NoError(t, err)
	assert.False(t, o.ExitCode)

	// Valid for every source.
	for _, src := range []string{"all", "state", "code"} {
		o, err := ParseListArgs([]string{"list", "--source", src, "--exit-code"})
		require.NoError(t, err, src)
		assert.True(t, o.ExitCode, src)
	}
}

func TestExitCodeResult(t *testing.T) {
	assert.Nil(t, exitCodeResult(false, true), "gate off → nil")
	assert.Nil(t, exitCodeResult(true, false), "no matches → nil")
	assert.Nil(t, exitCodeResult(false, false))
	assert.ErrorIs(t, exitCodeResult(true, true), ErrExitCode, "gate on + matches → sentinel")
}

func TestMarkerForKeyword(t *testing.T) {
	// All user-supplied keywords use the loose form in tdb list so malformed
	// annotations remain visible.
	for _, kw := range []string{"rpt", "todo", "FIXME", "RPT"} {
		m := markerForKeyword(kw)
		assert.Equal(t, strings.ToUpper(kw), m.Keyword)
		assert.False(t, m.Strict)
		assert.Empty(t, m.Suppress)
	}
}

func TestSplitList(t *testing.T) {
	assert.Equal(t, []string{"a", "b", "c"}, splitList("a, b ,c"))
	assert.Nil(t, splitList(" , "))
}

func TestFormatListRows(t *testing.T) {
	rows := []listRow{
		{kind: "comment", file: "pkg/foo.go", line: 12, text: "refactor this", created: time.Now()},
		{kind: "note", id: "a1b", text: "remember to bump version"},
		{kind: "TODO", file: "cmd/x/main.go", line: 88, text: "handle empty case", code: true},
		{kind: "comment", file: "pkg/bar.go", line: 3, text: "done", dim: true},
	}
	lines := formatListRows(rows, CommentListStyles{}, 120)
	require.Len(t, lines, 4)
	// With zero-value (no-color) styles the output is plain text we can assert on.
	assert.Contains(t, lines[0], "comment")
	assert.Contains(t, lines[0], "pkg/foo.go:12")
	assert.Contains(t, lines[0], "refactor this")
	assert.Contains(t, lines[1], "a1b")
	assert.Contains(t, lines[2], "TODO")
	assert.Contains(t, lines[2], "cmd/x/main.go:88")
	// Columns are aligned: the kind column is padded to the widest kind ("comment").
	for _, l := range lines {
		assert.True(t, strings.HasPrefix(l, "comment") || strings.HasPrefix(l, "note   ") || strings.HasPrefix(l, "TODO   "))
	}
}
