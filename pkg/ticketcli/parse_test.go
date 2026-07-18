package ticketcli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseArgs_DefaultsToList(t *testing.T) {
	o, err := ParseArgs([]string{"comment"})
	require.NoError(t, err)
	assert.False(t, o.Note)
	assert.Equal(t, "list", o.Sub)
}

func TestParseArgs_NoteAliasAndDefault(t *testing.T) {
	o, err := ParseArgs([]string{"note"})
	require.NoError(t, err)
	assert.True(t, o.Note)
	assert.Equal(t, "list", o.Sub)

	o, err = ParseArgs([]string{"n", "list"})
	require.NoError(t, err)
	assert.True(t, o.Note)
}

func TestParseArgs_CommentAlias(t *testing.T) {
	o, err := ParseArgs([]string{"c", "list"})
	require.NoError(t, err)
	assert.False(t, o.Note)
	assert.Equal(t, "list", o.Sub)
}

func TestParseArgs_NotACommentCommand(t *testing.T) {
	_, err := ParseArgs([]string{"diff"})
	require.Error(t, err)
}

func TestParseArgs_ListFlags(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "list", "--status", "resolved", "--kind", "note", "--file", "a.go", "--grep", "TODO", "--since", "2w"})
	require.NoError(t, err)
	assert.Equal(t, "resolved", o.Status)
	assert.Equal(t, "note", o.Kind)
	assert.Equal(t, "a.go", o.File)
	assert.Equal(t, "TODO", o.Grep)
	assert.Equal(t, "2w", o.Since)
}

func TestParseArgs_ListN(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "list", "-n", "5"})
	require.NoError(t, err)
	assert.True(t, o.NSet)
	assert.Equal(t, 5, o.N)

	o, err = ParseArgs([]string{"comment", "list", "-n"})
	require.NoError(t, err)
	assert.True(t, o.NSet)
	assert.Equal(t, 0, o.N)

	o, err = ParseArgs([]string{"comment", "list", "-n-3"})
	require.NoError(t, err)
	assert.Equal(t, -3, o.N)
}

func TestParseArgs_Verbose(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "list", "-v"})
	require.NoError(t, err)
	assert.True(t, o.Verbose)
}

func TestParseArgs_BranchAndAllBranches(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "list", "--branch", "main"})
	require.NoError(t, err)
	assert.Equal(t, "main", o.Branch)

	o, err = ParseArgs([]string{"comment", "list", "-b"})
	require.NoError(t, err)
	assert.True(t, o.AllBranches)

	o, err = ParseArgs([]string{"comment", "list", "--all-branches"})
	require.NoError(t, err)
	assert.True(t, o.AllBranches)
}

func TestParseArgs_BranchAllBranchesConflict(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "list", "--branch", "main", "--all-branches"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be used together")
}

func TestParseArgs_ListSuffixLookup(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "list", "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "list", o.Sub)
	assert.Equal(t, "abc123", o.ID)
}

func TestParseArgs_Edit(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "edit", "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "edit", o.Sub)
	assert.Equal(t, "abc123", o.ID)
}

func TestParseArgs_EditMissingID(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "edit"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a comment ID")
}

func TestParseArgs_EditResolved(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "edit", "abc", "--resolved", "true"})
	require.NoError(t, err)
	require.NotNil(t, o.Resolved)
	assert.True(t, *o.Resolved)

	o, err = ParseArgs([]string{"comment", "edit", "abc", "--resolved=false"})
	require.NoError(t, err)
	require.NotNil(t, o.Resolved)
	assert.False(t, *o.Resolved)
}

func TestParseArgs_ResolvedInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "edit", "abc", "--resolved", "maybe"})
	require.Error(t, err)
}

func TestParseArgs_ResolvedOnlyForEdit(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "list", "--resolved", "true"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only valid for comment edit")
}

func TestParseArgs_ResolvedWithResolveSubcommand(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "resolve", "abc", "--resolved", "true"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be combined with")
}

func TestParseArgs_ResolveUnresolveNeedID(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "resolve"})
	require.Error(t, err)
	_, err = ParseArgs([]string{"comment", "unresolve"})
	require.Error(t, err)
}

func TestParseArgs_KindOnlyForList(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "edit", "abc", "--kind", "note"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--kind is only valid for comment list")
}

func TestParseArgs_KindInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "list", "--kind", "bogus"})
	require.Error(t, err)
}

func TestParseArgs_NoteRejectsKind(t *testing.T) {
	_, err := ParseArgs([]string{"note", "list", "--kind", "note"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--kind cannot be used with note")
}

func TestParseArgs_Add(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "add", "f.go:12", "-m", "hello", "--commit", "HEAD"})
	require.NoError(t, err)
	assert.Equal(t, "add", o.Sub)
	assert.Equal(t, "f.go:12", o.AddTarget)
	assert.Equal(t, "hello", o.AddMessage)
	assert.Equal(t, "HEAD", o.AddCommit)
}

func TestParseArgs_AddStandalone(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "add", "-m", "note text"})
	require.NoError(t, err)
	assert.Equal(t, "add", o.Sub)
	assert.Equal(t, "", o.AddTarget)
	assert.Equal(t, "note text", o.AddMessage)
}

func TestParseArgs_AddAuthor(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--author", "Bot"})
	require.NoError(t, err)
	assert.True(t, o.AuthorSet)
	assert.Equal(t, "Bot", o.Author)
}

func TestParseArgs_AddBareAuthorErrors(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--author"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires an author argument")
}

func TestParseArgs_AddTags(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "add", "f.go:1", "-m", "x", "--prefix", "RPT", "--type", "refactor", "--scope", "use-tailwind"})
	require.NoError(t, err)
	assert.Equal(t, "RPT", o.Prefix)
	assert.Equal(t, "refactor", o.Type)
	assert.Equal(t, "use-tailwind", o.Scope)
}

func TestParseArgs_TagsValidOnListAndAdd(t *testing.T) {
	// Valid as filters on list.
	o, err := ParseArgs([]string{"comment", "list", "--prefix", "RPT", "--type", "fix", "--scope", "x"})
	require.NoError(t, err)
	assert.Equal(t, "RPT", o.Prefix)
	assert.Equal(t, "fix", o.Type)
	assert.Equal(t, "x", o.Scope)

	// Rejected on edit/resolve.
	for _, flag := range []string{"--prefix", "--type", "--scope"} {
		_, err := ParseArgs([]string{"comment", "resolve", "abc", flag, "v"})
		assert.Error(t, err, flag)
	}
}

func TestParseArgs_ListBareAuthor(t *testing.T) {
	o, err := ParseArgs([]string{"comment", "list", "--author"})
	require.NoError(t, err)
	assert.True(t, o.AuthorSet)
	assert.Equal(t, "", o.Author)
}

func TestParseArgs_MOnNonAddErrors(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "list", "-m", "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-m is only valid for comment add")
}

func TestParseArgs_NoteAddRejectsFile(t *testing.T) {
	_, err := ParseArgs([]string{"note", "add", "f.go:1", "-m", "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not accept a file:line argument")
}

func TestParseArgs_UnknownFlag(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "list", "--bogus"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown flag")
}

func TestParseArgs_Help(t *testing.T) {
	_, err := ParseArgs([]string{"comment", "--help"})
	assert.ErrorIs(t, err, errHelp)
}
