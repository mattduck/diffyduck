package ticketcli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs_MissingCommand(t *testing.T) {
	_, err := ParseArgs(nil)
	require.Error(t, err)
}

func TestParseArgs_UnknownCommand(t *testing.T) {
	// The write parser only handles add/edit/resolve/unresolve; list has its own.
	_, err := ParseArgs([]string{"diff"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")

	_, err = ParseArgs([]string{"comment"})
	require.Error(t, err)
}

func TestParseArgs_AddAttached(t *testing.T) {
	// A file:line target makes this a db comment (kind inferred downstream).
	o, err := ParseArgs([]string{"add", "f.go:12", "-m", "hello", "--commit", "HEAD"})
	require.NoError(t, err)
	assert.Equal(t, "add", o.Sub)
	assert.Equal(t, "f.go:12", o.AddTarget)
	assert.Equal(t, "hello", o.AddMessage)
	assert.Equal(t, "HEAD", o.AddCommit)
}

func TestParseArgs_AddStandalone(t *testing.T) {
	// No file:line target makes this a db issue.
	o, err := ParseArgs([]string{"add", "-m", "remember this"})
	require.NoError(t, err)
	assert.Equal(t, "add", o.Sub)
	assert.Equal(t, "", o.AddTarget)
	assert.Equal(t, "remember this", o.AddMessage)
}

func TestParseArgs_AddTags(t *testing.T) {
	o, err := ParseArgs([]string{"add", "f.go:1", "-m", "x", "--prefix", "RPT", "--type", "refactor", "--scope", "use-tailwind", "--ticket", "ABC-123"})
	require.NoError(t, err)
	assert.Equal(t, "RPT", o.Prefix)
	assert.Equal(t, "refactor", o.Type)
	assert.Equal(t, "use-tailwind", o.Scope)
	assert.Equal(t, "ABC-123", o.Ticket)
}

func TestParseArgs_AddAuthor(t *testing.T) {
	o, err := ParseArgs([]string{"add", "f.go:1", "-m", "x", "--author", "Bot"})
	require.NoError(t, err)
	assert.True(t, o.AuthorSet)
	assert.Equal(t, "Bot", o.Author)
}

func TestParseArgs_AddBareAuthorErrors(t *testing.T) {
	_, err := ParseArgs([]string{"add", "f.go:1", "-m", "x", "--author"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an author argument")
}

func TestParseArgs_Edit(t *testing.T) {
	o, err := ParseArgs([]string{"edit", "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "edit", o.Sub)
	assert.Equal(t, "abc123", o.ID)
}

func TestParseArgs_EditResolved(t *testing.T) {
	o, err := ParseArgs([]string{"edit", "abc", "--resolved", "true"})
	require.NoError(t, err)
	require.NotNil(t, o.Resolved)
	assert.True(t, *o.Resolved)

	o, err = ParseArgs([]string{"edit", "abc", "--resolved=false"})
	require.NoError(t, err)
	require.NotNil(t, o.Resolved)
	assert.False(t, *o.Resolved)
}

func TestParseArgs_ResolvedInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"edit", "abc", "--resolved", "maybe"})
	require.Error(t, err)
}

func TestParseArgs_ResolveUnresolve(t *testing.T) {
	o, err := ParseArgs([]string{"resolve", "abc"})
	require.NoError(t, err)
	assert.Equal(t, "resolve", o.Sub)
	assert.Equal(t, "abc", o.ID)

	o, err = ParseArgs([]string{"unresolve", "abc"})
	require.NoError(t, err)
	assert.Equal(t, "unresolve", o.Sub)
}

func TestParseArgs_IDRequired(t *testing.T) {
	for _, sub := range []string{"edit", "resolve", "unresolve"} {
		_, err := ParseArgs([]string{sub})
		require.Error(t, err, sub)
		assert.Contains(t, err.Error(), "requires an entry ID", sub)
	}
}

func TestParseArgs_ResolvedOnlyForEdit(t *testing.T) {
	_, err := ParseArgs([]string{"add", "--resolved", "true"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--resolved is only valid for edit")
}

func TestParseArgs_ResolvedWithResolveSubcommand(t *testing.T) {
	_, err := ParseArgs([]string{"resolve", "abc", "--resolved", "true"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be combined with")
}

func TestParseArgs_ReaderFlagsRejected(t *testing.T) {
	// Reader-only flags are pointed back to `tdb list`.
	for _, args := range [][]string{
		{"add", "-m", "x", "--file", "a.go"},
		{"add", "-m", "x", "--grep", "TODO"},
		{"add", "-m", "x", "--status", "resolved"},
		{"add", "-m", "x", "--since", "2w"},
		{"edit", "abc", "-v"},
		{"edit", "abc", "--raw"},
		{"edit", "abc", "-n5"},
		{"edit", "abc", "-b", "main"},
		{"edit", "abc", "--all-branches"},
	} {
		_, err := ParseArgs(args)
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), "tdb list", "%v", args)
	}
}

func TestParseArgs_AddOnlyFlagsRejectedElsewhere(t *testing.T) {
	// Content/tag flags apply only to add.
	for _, flag := range [][]string{
		{"-m", "x"},
		{"--commit", "HEAD"},
		{"--prefix", "RPT"},
		{"--type", "fix"},
		{"--scope", "x"},
		{"--ticket", "ABC-1"},
	} {
		args := append([]string{"resolve", "abc"}, flag...)
		_, err := ParseArgs(args)
		require.Error(t, err, "%v", args)
		assert.Contains(t, err.Error(), "only valid for add", "%v", args)
	}
}

func TestParseArgs_UnknownFlag(t *testing.T) {
	_, err := ParseArgs([]string{"add", "-m", "x", "--bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

func TestParseArgs_Help(t *testing.T) {
	_, err := ParseArgs([]string{"add", "--help"})
	assert.ErrorIs(t, err, errHelp)
}
