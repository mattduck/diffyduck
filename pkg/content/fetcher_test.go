package content

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattduck/diffyduck/pkg/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestFetcher_ConcurrentAccess(t *testing.T) {
	// Test that concurrent access to the fetcher is safe.
	// This test would cause a data race (and potential segfault) without
	// proper mutex protection on the cache map.
	// Run with: go test -race ./pkg/content/...

	// Create a mock with many files
	fileContents := make(map[string]string)
	numFiles := 50
	for i := 0; i < numFiles; i++ {
		oldKey := "abc123^:file" + string(rune('a'+i)) + ".go"
		newKey := "abc123:file" + string(rune('a'+i)) + ".go"
		fileContents[oldKey] = "old content " + string(rune('a'+i)) + "\n"
		fileContents[newKey] = "new content " + string(rune('a'+i)) + "\n"
	}

	mock := &git.MockGit{FileContents: fileContents}
	f := NewFetcher(mock, ModeShow, "abc123", "")

	// Spawn many goroutines to access different files concurrently
	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			fileIdx := idx % numFiles
			path := "file" + string(rune('a'+fileIdx)) + ".go"

			// Interleave old and new content fetches
			if idx%2 == 0 {
				_, _ = f.GetOldContent(path)
				_, _ = f.GetNewContent(path)
			} else {
				_, _ = f.GetNewContent(path)
				_, _ = f.GetOldContent(path)
			}
		}(i)
	}

	wg.Wait()

	// If we get here without a race condition panic, the test passes.
	// Verify some content was cached correctly
	old, err := f.GetOldContent("filea.go")
	require.NoError(t, err)
	assert.Equal(t, "old content a\n", old)

	new, err := f.GetNewContent("filea.go")
	require.NoError(t, err)
	assert.Equal(t, "new content a\n", new)
}

func TestFetcher_GitContentLinesStopsLargeBlob(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runContentGit(t, tmpDir, "init")

	var b strings.Builder
	for b.Len() <= 2*1024*1024 {
		b.WriteString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n")
	}
	err := os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte(b.String()), 0644)
	require.NoError(t, err)

	runContentGit(t, tmpDir, "add", "large.txt")
	runContentGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "large")

	f := NewFetcher(git.NewWithDir(tmpDir), ModeShow, "HEAD", "")

	type result struct {
		lines     []string
		truncated bool
		err       error
	}
	done := make(chan result, 1)
	go func() {
		lines, truncated, err := f.GetNewContentLines("large.txt")
		done <- result{lines: lines, truncated: truncated, err: err}
	}()

	select {
	case got := <-done:
		require.NoError(t, got.err)
		assert.True(t, got.truncated)
		assert.NotEmpty(t, got.lines)
	case <-time.After(2 * time.Second):
		t.Fatal("GetNewContentLines hung after stopping early on a large git blob")
	}
}

// cleanGitEnv returns os.Environ() with git-specific variables removed, so
// test git commands use cmd.Dir instead of inheriting GIT_DIR/GIT_INDEX_FILE
// from pre-commit hooks.
func cleanGitEnv(environ []string) []string {
	var env []string
	for _, e := range environ {
		if strings.HasPrefix(e, "GIT_DIR=") ||
			strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_INDEX_FILE=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

func runContentGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return runContentGitWithEnv(t, dir, nil, args...)
}

func runContentGitWithEnv(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cleanGitEnv(os.Environ()), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}
