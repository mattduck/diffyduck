package ticketcli

import (
	"encoding/json"
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

func TestParseListArgs_JSON(t *testing.T) {
	// --json parses and is valid for every source.
	for _, src := range []string{"all", "state", "code"} {
		o, err := ParseListArgs([]string{"list", "--source", src, "--json"})
		require.NoError(t, err, src)
		assert.True(t, o.JSON, src)
	}

	// --json is mutually exclusive with the human-only detail modes and ID lookup.
	for _, c := range [][]string{
		{"list", "--json", "--raw"},
		{"list", "--json", "-v"},
		{"list", "--source", "state", "--json", "--raw"},
		{"list", "--json", "abc123"},
	} {
		_, err := ParseListArgs(c)
		assert.Error(t, err, "%v", c)
	}
}

func TestParseListArgs_Random(t *testing.T) {
	o, err := ParseListArgs([]string{"list", "--random"})
	require.NoError(t, err)
	assert.True(t, o.Random)

	// Valid alongside -n and filters.
	o, err = ParseListArgs([]string{"list", "--random", "-n3", "--marker", "RPT"})
	require.NoError(t, err)
	assert.True(t, o.Random)
	assert.Equal(t, 3, o.N)

	// --random is mutually exclusive with ID lookup and the detail modes.
	for _, c := range [][]string{
		{"list", "--random", "abc123"},
		{"list", "--random", "-v"},
		{"list", "--random", "--raw"},
	} {
		_, err := ParseListArgs(c)
		assert.Error(t, err, "%v", c)
	}
}

func TestSelectRows(t *testing.T) {
	mkRows := func() []listRow {
		return []listRow{
			{kind: "comment", file: "a.go", line: 1, created: time.Unix(300, 0)},
			{kind: "comment", file: "b.go", line: 2, created: time.Unix(100, 0)},
			{kind: "RPT", file: "c.go", line: 3, code: true},
			{kind: "TODO", file: "d.go", line: 4, code: true},
		}
	}

	// Default ordering: tickets (newest first) then markers (by file/line).
	sel, total, trunc := selectRows(mkRows(), ListOptions{})
	assert.Equal(t, 4, total)
	assert.False(t, trunc)
	assert.Equal(t, "a.go", sel[0].file)
	assert.Equal(t, "b.go", sel[1].file)
	assert.True(t, sel[2].code)

	// Deterministic stand-in for the shuffle: reverse the slice.
	orig := randShuffle
	randShuffle = func(n int, swap func(i, j int)) {
		for i := 0; i < n/2; i++ {
			swap(i, n-1-i)
		}
	}
	defer func() { randShuffle = orig }()

	// --random implies exactly one result unless -n overrides.
	sel, total, trunc = selectRows(mkRows(), ListOptions{Random: true})
	assert.Equal(t, 4, total)
	assert.True(t, trunc)
	require.Len(t, sel, 1)
	assert.Equal(t, "d.go", sel[0].file) // reversed → last element first

	// --random with -n returns that many.
	sel, _, trunc = selectRows(mkRows(), ListOptions{Random: true, N: 3, NSet: true})
	require.Len(t, sel, 3)
	assert.True(t, trunc)

	// -n above the available count returns all rows, untruncated.
	sel, _, trunc = selectRows(mkRows(), ListOptions{Random: true, N: 10, NSet: true})
	require.Len(t, sel, 4)
	assert.False(t, trunc)
}

func TestRowsToJSON(t *testing.T) {
	created := time.Date(2026, 5, 19, 17, 50, 18, 0, time.UTC)
	rows := []listRow{
		{
			kind: "comment", file: "pkg/foo.go", line: 12, id: "881",
			text: "refactor this", body: "refactor this\n\nsecond paragraph", created: created,
			fullID: "1779209418881", tkind: "comment", author: "Claude",
			status: "open", marker: "RPT", mtype: "refactor", scope: "no-bare-dict",
			branch: "main", resolved: false,
		},
		{
			kind: "RPT fix(rule-a)", file: "pkg/bar.go", line: 88, text: "fix this",
			code: true, marker: "RPT", mtype: "fix", scope: "rule-a",
		},
	}
	out := rowsToJSON(rows)
	require.Len(t, out, 2)

	// Ticket carries the actionable id + metadata; resolved is always present.
	tk := out[0]
	assert.Equal(t, "ticket", tk.Source)
	assert.Equal(t, "1779209418881", tk.ID)
	assert.Equal(t, "881", tk.ShortID)
	assert.Equal(t, "comment", tk.Kind)
	assert.Equal(t, "Claude", tk.Author)
	// A ticket can be tagged to mirror a marker: marker/type/scope on the ticket too.
	assert.Equal(t, "RPT", tk.Marker)
	assert.Equal(t, "refactor", tk.Type)
	assert.Equal(t, "no-bare-dict", tk.Scope)
	require.NotNil(t, tk.Resolved)
	assert.False(t, *tk.Resolved)
	assert.Equal(t, "2026-05-19T17:50:18Z", tk.Created)
	// Body carries the full multi-line text; Text is only the one-line summary.
	assert.Equal(t, "refactor this\n\nsecond paragraph", tk.Body)
	assert.Equal(t, "refactor this", tk.Text)

	// Marker carries no id or body; keyword/type/scope are split out.
	mk := out[1]
	assert.Equal(t, "marker", mk.Source)
	assert.Empty(t, mk.ID)
	assert.Empty(t, mk.Body)
	assert.Nil(t, mk.Resolved)
	assert.Equal(t, "RPT", mk.Marker)
	assert.Equal(t, "fix", mk.Type)
	assert.Equal(t, "rule-a", mk.Scope)

	// Marshalling drops the omitempty ticket-only fields from markers, and
	// resolved:false survives on tickets (pointer, not bare bool).
	b, err := json.Marshal(out)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"resolved":false`)
	assert.Contains(t, s, `"marker":"RPT"`)
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

func TestParseListArgs_Scope(t *testing.T) {
	o, err := ParseListArgs([]string{"list", "--scope", "no-bare-dict"})
	require.NoError(t, err)
	assert.Equal(t, "no-bare-dict", o.Scope)

	o, err = ParseListArgs([]string{"list", "--scope=other"})
	require.NoError(t, err)
	assert.Equal(t, "other", o.Scope)

	// --scope/--type/--marker are valid for every source: they filter ticket
	// tags and code-marker fields alike.
	for _, src := range []string{"state", "code", "all"} {
		o, err := ParseListArgs([]string{"list", "--source", src, "--scope", "r", "--type", "fix", "--marker", "RPT"})
		require.NoError(t, err, src)
		assert.Equal(t, "r", o.Scope, src)
		assert.Equal(t, "fix", o.Type, src)
		assert.Equal(t, []string{"RPT"}, o.Markers, src)
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
