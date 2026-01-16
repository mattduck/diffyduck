package content

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/git"
)

func TestFetcher_ShowMode(t *testing.T) {
	// In show mode, old = commit^:path, new = commit:path
	mock := &git.MockGit{
		FileContents: map[string]string{
			"abc123^:foo.go": "old content\n",
			"abc123:foo.go":  "new content\n",
		},
	}

	f := NewFetcher(mock, ModeShow, "abc123", "")

	old, err := f.GetOldContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "old content\n", old)

	new, err := f.GetNewContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "new content\n", new)
}

func TestFetcher_DiffUnstaged(t *testing.T) {
	// In unstaged diff mode, old = index (:path), new = working tree (read file)
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "foo.go")
	err := os.WriteFile(workFile, []byte("working tree content\n"), 0644)
	require.NoError(t, err)

	mock := &git.MockGit{
		FileContents: map[string]string{
			":foo.go": "staged content\n",
		},
	}

	f := NewFetcher(mock, ModeDiffUnstaged, "", "")
	f.WorkDir = tmpDir

	old, err := f.GetOldContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "staged content\n", old)

	new, err := f.GetNewContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "working tree content\n", new)
}

func TestFetcher_DiffCached(t *testing.T) {
	// In cached diff mode, old = HEAD:path, new = index (:path)
	mock := &git.MockGit{
		FileContents: map[string]string{
			"HEAD:foo.go": "HEAD content\n",
			":foo.go":     "staged content\n",
		},
	}

	f := NewFetcher(mock, ModeDiffCached, "", "")

	old, err := f.GetOldContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "HEAD content\n", old)

	new, err := f.GetNewContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "staged content\n", new)
}

func TestFetcher_DiffRefs(t *testing.T) {
	// In ref diff mode, old = ref1:path, new = ref2:path
	mock := &git.MockGit{
		FileContents: map[string]string{
			"main:foo.go":    "main content\n",
			"feature:foo.go": "feature content\n",
		},
	}

	f := NewFetcher(mock, ModeDiffRefs, "main", "feature")

	old, err := f.GetOldContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "main content\n", old)

	new, err := f.GetNewContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "feature content\n", new)
}

func TestFetcher_NewFile(t *testing.T) {
	// New file: old content doesn't exist
	mock := &git.MockGit{
		FileContents: map[string]string{
			"abc123:new.go": "new file content\n",
			// abc123^:new.go doesn't exist
		},
	}

	f := NewFetcher(mock, ModeShow, "abc123", "")

	old, err := f.GetOldContent("new.go")
	// Should return empty string, not error, for new files
	assert.NoError(t, err)
	assert.Equal(t, "", old)

	new, err := f.GetNewContent("new.go")
	require.NoError(t, err)
	assert.Equal(t, "new file content\n", new)
}

func TestFetcher_DeletedFile(t *testing.T) {
	// Deleted file: new content doesn't exist
	mock := &git.MockGit{
		FileContents: map[string]string{
			"abc123^:deleted.go": "old file content\n",
			// abc123:deleted.go doesn't exist
		},
	}

	f := NewFetcher(mock, ModeShow, "abc123", "")

	old, err := f.GetOldContent("deleted.go")
	require.NoError(t, err)
	assert.Equal(t, "old file content\n", old)

	new, err := f.GetNewContent("deleted.go")
	// Should return empty string, not error, for deleted files
	assert.NoError(t, err)
	assert.Equal(t, "", new)
}

func TestFetcher_Caching(t *testing.T) {
	// Test that content is cached and git is only called once
	callCount := 0
	mock := &git.MockGit{
		FileContents: map[string]string{
			"abc123:foo.go": "content\n",
		},
	}

	f := NewFetcher(mock, ModeShow, "abc123", "")
	f.onFetch = func() { callCount++ }

	// First call
	content1, err := f.GetNewContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "content\n", content1)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	content2, err := f.GetNewContent("foo.go")
	require.NoError(t, err)
	assert.Equal(t, "content\n", content2)
	assert.Equal(t, 1, callCount) // Still 1, not 2
}

func TestFetcher_RenamedFile(t *testing.T) {
	// Renamed file: old path differs from new path
	mock := &git.MockGit{
		FileContents: map[string]string{
			"abc123^:old_name.go": "content\n",
			"abc123:new_name.go":  "content\n",
		},
	}

	f := NewFetcher(mock, ModeShow, "abc123", "")

	// Get old content using old path
	old, err := f.GetOldContent("old_name.go")
	require.NoError(t, err)
	assert.Equal(t, "content\n", old)

	// Get new content using new path
	new, err := f.GetNewContent("new_name.go")
	require.NoError(t, err)
	assert.Equal(t, "content\n", new)
}
