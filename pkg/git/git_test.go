package git

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockGit_Show(t *testing.T) {
	mock := &MockGit{
		ShowOutput: "diff --git a/foo.go b/foo.go\n",
	}

	out, err := mock.Show("HEAD")
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/foo.go b/foo.go\n", out)
}

func TestMockGit_ShowError(t *testing.T) {
	mock := &MockGit{
		ShowError: errors.New("not a git repo"),
	}

	_, err := mock.Show("HEAD")
	require.Error(t, err)
	assert.Equal(t, "not a git repo", err.Error())
}

func TestMockGit_Diff(t *testing.T) {
	mock := &MockGit{
		DiffOutput: "diff --git a/bar.go b/bar.go\n",
	}

	out, err := mock.Diff()
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/bar.go b/bar.go\n", out)
}

func TestMockGit_DiffWithArgs(t *testing.T) {
	mock := &MockGit{
		DiffOutput: "cached diff output",
	}

	out, err := mock.Diff("--cached")
	require.NoError(t, err)
	assert.Equal(t, "cached diff output", out)
}

func TestMockGit_DiffError(t *testing.T) {
	mock := &MockGit{
		DiffError: errors.New("not a git repo"),
	}

	_, err := mock.Diff()
	require.Error(t, err)
	assert.Equal(t, "not a git repo", err.Error())
}

func TestGitInterface(t *testing.T) {
	// Verify both implementations satisfy the interface
	var _ Git = &RealGit{}
	var _ Git = &MockGit{}
}

func TestMockGit_GetFileContent(t *testing.T) {
	mock := &MockGit{
		FileContents: map[string]string{
			"HEAD:foo.go":   "package foo\n",
			"HEAD^:foo.go":  "package old\n",
			"abc123:bar.go": "package bar\n",
		},
	}

	// Get file at HEAD
	content, err := mock.GetFileContent("HEAD", "foo.go")
	require.NoError(t, err)
	assert.Equal(t, "package foo\n", content)

	// Get file at parent
	content, err = mock.GetFileContent("HEAD^", "foo.go")
	require.NoError(t, err)
	assert.Equal(t, "package old\n", content)

	// Get file at specific commit
	content, err = mock.GetFileContent("abc123", "bar.go")
	require.NoError(t, err)
	assert.Equal(t, "package bar\n", content)
}

func TestMockGit_GetFileContent_NotFound(t *testing.T) {
	mock := &MockGit{
		FileContents: map[string]string{},
	}

	_, err := mock.GetFileContent("HEAD", "nonexistent.go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent.go")
}

func TestMockGit_GetFileContent_Index(t *testing.T) {
	// Empty ref means index (staged content)
	mock := &MockGit{
		FileContents: map[string]string{
			":foo.go": "staged content\n",
		},
	}

	content, err := mock.GetFileContent("", "foo.go")
	require.NoError(t, err)
	assert.Equal(t, "staged content\n", content)
}

func TestMockGit_ShowWithMeta(t *testing.T) {
	mock := &MockGit{
		ShowOutput: "diff --git a/foo.go b/foo.go\n",
		ShowMeta: &CommitMeta{
			SHA:     "abc123def456",
			Author:  "Test Author",
			Email:   "test@example.com",
			Date:    "2024-01-15T10:30:00Z",
			Subject: "Add new feature",
			Body:    "Detailed description",
		},
	}

	meta, diff, err := mock.ShowWithMeta("HEAD")
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", meta.SHA)
	assert.Equal(t, "Test Author", meta.Author)
	assert.Equal(t, "Add new feature", meta.Subject)
	assert.Equal(t, "diff --git a/foo.go b/foo.go\n", diff)
}

func TestMockGit_ShowWithMeta_NoMeta(t *testing.T) {
	// When ShowMeta is nil, should return empty metadata
	mock := &MockGit{
		ShowOutput: "diff --git a/foo.go b/foo.go\n",
	}

	meta, diff, err := mock.ShowWithMeta("HEAD")
	require.NoError(t, err)
	assert.NotNil(t, meta)
	assert.Equal(t, "", meta.SHA)
	assert.Equal(t, "diff --git a/foo.go b/foo.go\n", diff)
}

func TestParseShowOutput(t *testing.T) {
	// Simulate git show output with our custom format
	input := `DIFFYDUCK_SHA:abc123def4567890
DIFFYDUCK_AUTHOR:John Doe
DIFFYDUCK_EMAIL:john@example.com
DIFFYDUCK_DATE:2024-01-15T10:30:00+00:00
DIFFYDUCK_SUBJECT:Fix bug in parser
DIFFYDUCK_BODY_START
This commit fixes a critical bug.

The bug was causing crashes.
DIFFYDUCK_BODY_END
diff --git a/foo.go b/foo.go
index 123..456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package foo
+// new line
`

	meta, diff := parseShowOutput(input)

	assert.Equal(t, "abc123def4567890", meta.SHA)
	assert.Equal(t, "John Doe", meta.Author)
	assert.Equal(t, "john@example.com", meta.Email)
	assert.Equal(t, "2024-01-15T10:30:00+00:00", meta.Date)
	assert.Equal(t, "Fix bug in parser", meta.Subject)
	assert.Equal(t, "This commit fixes a critical bug.\n\nThe bug was causing crashes.", meta.Body)
	assert.True(t, len(diff) > 0)
	assert.True(t, diff[0:10] == "diff --git", "diff should start with 'diff --git'")
}

func TestParseShowOutput_EmptyBody(t *testing.T) {
	input := `DIFFYDUCK_SHA:abc123
DIFFYDUCK_AUTHOR:John
DIFFYDUCK_EMAIL:john@example.com
DIFFYDUCK_DATE:2024-01-15
DIFFYDUCK_SUBJECT:Short commit
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
diff --git a/foo.go b/foo.go
`

	meta, diff := parseShowOutput(input)

	assert.Equal(t, "abc123", meta.SHA)
	assert.Equal(t, "Short commit", meta.Subject)
	assert.Equal(t, "", meta.Body)
	assert.Contains(t, diff, "diff --git")
}

func TestParseShowOutput_NoDiff(t *testing.T) {
	// Some commits might have no diff (e.g., merge commits)
	input := `DIFFYDUCK_SHA:abc123
DIFFYDUCK_AUTHOR:John
DIFFYDUCK_EMAIL:john@example.com
DIFFYDUCK_DATE:2024-01-15
DIFFYDUCK_SUBJECT:Merge commit
DIFFYDUCK_BODY_START
Merging branch
DIFFYDUCK_BODY_END
`

	meta, diff := parseShowOutput(input)

	assert.Equal(t, "abc123", meta.SHA)
	assert.Equal(t, "Merging branch", meta.Body)
	assert.Equal(t, "", diff)
}
