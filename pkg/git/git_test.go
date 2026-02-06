package git

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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

func TestParseShowOutput_WithRefs(t *testing.T) {
	input := `DIFFYDUCK_SHA:abc123
DIFFYDUCK_AUTHOR:John
DIFFYDUCK_EMAIL:john@example.com
DIFFYDUCK_DATE:2024-01-15
DIFFYDUCK_SUBJECT:Fix parser
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
DIFFYDUCK_REFS:HEAD -> main, origin/main, origin/HEAD
diff --git a/foo.go b/foo.go
`

	meta, diff := parseShowOutput(input)

	assert.Equal(t, "abc123", meta.SHA)
	assert.Equal(t, "Fix parser", meta.Subject)
	assert.Equal(t, "HEAD -> main, origin/main, origin/HEAD", meta.Refs)
	assert.Contains(t, diff, "diff --git")
}

func TestParseShowOutput_EmptyRefs(t *testing.T) {
	input := `DIFFYDUCK_SHA:abc123
DIFFYDUCK_AUTHOR:John
DIFFYDUCK_EMAIL:john@example.com
DIFFYDUCK_DATE:2024-01-15
DIFFYDUCK_SUBJECT:Old commit
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
DIFFYDUCK_REFS:
diff --git a/foo.go b/foo.go
`

	meta, _ := parseShowOutput(input)

	assert.Equal(t, "abc123", meta.SHA)
	assert.Equal(t, "", meta.Refs)
}

// =============================================================================
// parseLogOutput Tests (Multi-Commit Parsing)
// =============================================================================

func TestParseLogOutput_MultipleCommits(t *testing.T) {
	// Simulate git log -p output with multiple commits
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:aaa111111111111111111111111111111
DIFFYDUCK_AUTHOR:Alice
DIFFYDUCK_EMAIL:alice@example.com
DIFFYDUCK_DATE:2024-01-15T10:00:00+00:00
DIFFYDUCK_SUBJECT:First commit
DIFFYDUCK_BODY_START
First body
DIFFYDUCK_BODY_END
diff --git a/file1.go b/file1.go
+package main
DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:bbb222222222222222222222222222222
DIFFYDUCK_AUTHOR:Bob
DIFFYDUCK_EMAIL:bob@example.com
DIFFYDUCK_DATE:2024-01-14T09:00:00+00:00
DIFFYDUCK_SUBJECT:Second commit
DIFFYDUCK_BODY_START
Second body
DIFFYDUCK_BODY_END
diff --git a/file2.go b/file2.go
+package other
`

	commits := parseLogOutput(input)

	require.Equal(t, 2, len(commits), "should parse 2 commits")

	// First commit
	assert.Equal(t, "aaa111111111111111111111111111111", commits[0].Meta.SHA)
	assert.Equal(t, "Alice", commits[0].Meta.Author)
	assert.Equal(t, "First commit", commits[0].Meta.Subject)
	assert.Equal(t, "First body", commits[0].Meta.Body)
	assert.Contains(t, commits[0].Diff, "file1.go")

	// Second commit
	assert.Equal(t, "bbb222222222222222222222222222222", commits[1].Meta.SHA)
	assert.Equal(t, "Bob", commits[1].Meta.Author)
	assert.Equal(t, "Second commit", commits[1].Meta.Subject)
	assert.Equal(t, "Second body", commits[1].Meta.Body)
	assert.Contains(t, commits[1].Diff, "file2.go")
}

func TestParseLogOutput_SingleCommit(t *testing.T) {
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:abc123
DIFFYDUCK_AUTHOR:Jane
DIFFYDUCK_EMAIL:jane@example.com
DIFFYDUCK_DATE:2024-01-10
DIFFYDUCK_SUBJECT:Only commit
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
diff --git a/main.go b/main.go
`

	commits := parseLogOutput(input)

	require.Equal(t, 1, len(commits), "should parse 1 commit")
	assert.Equal(t, "abc123", commits[0].Meta.SHA)
	assert.Equal(t, "Only commit", commits[0].Meta.Subject)
}

func TestParseLogOutput_EmptyInput(t *testing.T) {
	commits := parseLogOutput("")

	assert.Equal(t, 0, len(commits), "empty input should return no commits")
}

func TestParseLogOutput_CommitWithNoDiff(t *testing.T) {
	// A commit without any file changes (e.g., empty commit)
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:empty123
DIFFYDUCK_AUTHOR:Dev
DIFFYDUCK_EMAIL:dev@example.com
DIFFYDUCK_DATE:2024-01-05
DIFFYDUCK_SUBJECT:Empty commit
DIFFYDUCK_BODY_START
No changes in this commit
DIFFYDUCK_BODY_END
`

	commits := parseLogOutput(input)

	require.Equal(t, 1, len(commits), "should parse 1 commit")
	assert.Equal(t, "empty123", commits[0].Meta.SHA)
	assert.Equal(t, "", commits[0].Diff, "diff should be empty")
	assert.Equal(t, "No changes in this commit", commits[0].Meta.Body)
}

func TestParseLogOutput_TenCommits(t *testing.T) {
	// Simulate 10 commits like the log command would return
	var input string
	for i := 0; i < 10; i++ {
		input += `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:` + string(rune('a'+i)) + `00000000000000000000
DIFFYDUCK_AUTHOR:Author` + string(rune('0'+i)) + `
DIFFYDUCK_EMAIL:author` + string(rune('0'+i)) + `@example.com
DIFFYDUCK_DATE:2024-01-` + string(rune('0'+i)) + `0
DIFFYDUCK_SUBJECT:Commit ` + string(rune('0'+i)) + `
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
diff --git a/file` + string(rune('0'+i)) + `.go b/file` + string(rune('0'+i)) + `.go
`
	}

	commits := parseLogOutput(input)

	assert.Equal(t, 10, len(commits), "should parse 10 commits")

	// Verify each commit has unique SHA
	shas := make(map[string]bool)
	for _, c := range commits {
		shas[c.Meta.SHA] = true
	}
	assert.Equal(t, 10, len(shas), "all 10 SHAs should be unique")
}

func TestParseLogOutput_CommitWithMultipleFiles(t *testing.T) {
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:multi123
DIFFYDUCK_AUTHOR:Multi
DIFFYDUCK_EMAIL:multi@example.com
DIFFYDUCK_DATE:2024-01-20
DIFFYDUCK_SUBJECT:Multi-file commit
DIFFYDUCK_BODY_START
Changed several files
DIFFYDUCK_BODY_END
diff --git a/file1.go b/file1.go
+package file1
diff --git a/file2.go b/file2.go
+package file2
diff --git a/file3.go b/file3.go
+package file3
`

	commits := parseLogOutput(input)

	require.Equal(t, 1, len(commits), "should parse 1 commit")
	// The diff should contain all three files
	assert.Contains(t, commits[0].Diff, "file1.go")
	assert.Contains(t, commits[0].Diff, "file2.go")
	assert.Contains(t, commits[0].Diff, "file3.go")
}

func TestParseLogOutput_CommitWithBinaryFile(t *testing.T) {
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:binary123
DIFFYDUCK_AUTHOR:Dev
DIFFYDUCK_EMAIL:dev@example.com
DIFFYDUCK_DATE:2024-01-25
DIFFYDUCK_SUBJECT:Add binary file
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
diff --git a/image.png b/image.png
Binary files differ
`

	commits := parseLogOutput(input)

	require.Equal(t, 1, len(commits), "should parse 1 commit")
	assert.Contains(t, commits[0].Diff, "Binary files differ")
}

func TestParseLogOutput_FewerThanRequestedCommits(t *testing.T) {
	// When repo has fewer commits than requested, output is shorter
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:only123
DIFFYDUCK_AUTHOR:Solo
DIFFYDUCK_EMAIL:solo@example.com
DIFFYDUCK_DATE:2024-01-01
DIFFYDUCK_SUBJECT:Initial commit
DIFFYDUCK_BODY_START
First ever commit
DIFFYDUCK_BODY_END
diff --git a/README.md b/README.md
+# New project
`

	// Even if we requested 10, we only get 1 if repo has 1 commit
	commits := parseLogOutput(input)

	assert.Equal(t, 1, len(commits), "should return however many commits exist")
}

func TestParseLogOutput_LongCommitBody(t *testing.T) {
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:long123
DIFFYDUCK_AUTHOR:Verbose
DIFFYDUCK_EMAIL:verbose@example.com
DIFFYDUCK_DATE:2024-01-30
DIFFYDUCK_SUBJECT:Commit with long body
DIFFYDUCK_BODY_START
This is a very long commit message.

It has multiple paragraphs explaining the changes in great detail.

- Bullet point 1
- Bullet point 2
- Bullet point 3

The changes include refactoring, bug fixes, and new features.

Signed-off-by: Verbose <verbose@example.com>
DIFFYDUCK_BODY_END
diff --git a/main.go b/main.go
+package main
`

	commits := parseLogOutput(input)

	require.Equal(t, 1, len(commits), "should parse 1 commit")
	assert.Contains(t, commits[0].Meta.Body, "multiple paragraphs")
	assert.Contains(t, commits[0].Meta.Body, "Bullet point 1")
	assert.Contains(t, commits[0].Meta.Body, "Signed-off-by")
}

func TestMockGit_LogWithMeta(t *testing.T) {
	mock := &MockGit{
		LogOutput: []CommitWithDiff{
			{
				Meta: &CommitMeta{
					SHA:     "aaa111",
					Author:  "Alice",
					Subject: "First",
				},
				Diff: "diff --git a/f1.go b/f1.go\n",
			},
			{
				Meta: &CommitMeta{
					SHA:     "bbb222",
					Author:  "Bob",
					Subject: "Second",
				},
				Diff: "diff --git a/f2.go b/f2.go\n",
			},
		},
	}

	commits, err := mock.LogWithMeta(10)
	require.NoError(t, err)
	assert.Equal(t, 2, len(commits))
	assert.Equal(t, "aaa111", commits[0].Meta.SHA)
	assert.Equal(t, "bbb222", commits[1].Meta.SHA)
}

func TestMockGit_LogWithMeta_Empty(t *testing.T) {
	mock := &MockGit{
		LogOutput: []CommitWithDiff{},
	}

	commits, err := mock.LogWithMeta(10)
	require.NoError(t, err)
	assert.Equal(t, 0, len(commits))
}

func TestMockGit_LogWithMeta_Error(t *testing.T) {
	mock := &MockGit{
		LogError: errors.New("git log failed"),
	}

	_, err := mock.LogWithMeta(10)
	require.Error(t, err)
	assert.Equal(t, "git log failed", err.Error())
}

func TestMockGit_ListUntrackedFiles(t *testing.T) {
	mock := &MockGit{}
	files, err := mock.ListUntrackedFiles()
	require.NoError(t, err)
	assert.Nil(t, files)
}

func TestMockGit_DiffNewFile(t *testing.T) {
	mock := &MockGit{}
	diff, err := mock.DiffNewFile("test.txt")
	require.NoError(t, err)
	assert.Equal(t, "", diff)
}

// =============================================================================
// parseLogPathsOnly Tests (Name-Only Parsing)
// =============================================================================

func TestParseLogPathsOnly_MultipleCommits(t *testing.T) {
	// Simulate git log --name-only output with multiple commits
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:aaa111111111111111111111111111111
DIFFYDUCK_AUTHOR:Alice
DIFFYDUCK_EMAIL:alice@example.com
DIFFYDUCK_DATE:2024-01-15T10:00:00+00:00
DIFFYDUCK_SUBJECT:First commit
DIFFYDUCK_BODY_START
First body
DIFFYDUCK_BODY_END

src/main.go
src/util.go
DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:bbb222222222222222222222222222222
DIFFYDUCK_AUTHOR:Bob
DIFFYDUCK_EMAIL:bob@example.com
DIFFYDUCK_DATE:2024-01-14T09:00:00+00:00
DIFFYDUCK_SUBJECT:Second commit
DIFFYDUCK_BODY_START
Second body
DIFFYDUCK_BODY_END

pkg/lib.go
`

	commits := parseLogPathsOnly(input)

	require.Equal(t, 2, len(commits), "should parse 2 commits")

	// First commit
	assert.Equal(t, "aaa111111111111111111111111111111", commits[0].Meta.SHA)
	assert.Equal(t, "Alice", commits[0].Meta.Author)
	assert.Equal(t, "First commit", commits[0].Meta.Subject)
	assert.Equal(t, "First body", commits[0].Meta.Body)
	require.Equal(t, 2, len(commits[0].Files), "first commit should have 2 files")
	assert.Equal(t, "src/main.go", commits[0].Files[0].Path)
	assert.Equal(t, "src/util.go", commits[0].Files[1].Path)

	// Second commit
	assert.Equal(t, "bbb222222222222222222222222222222", commits[1].Meta.SHA)
	assert.Equal(t, "Bob", commits[1].Meta.Author)
	assert.Equal(t, "Second commit", commits[1].Meta.Subject)
	require.Equal(t, 1, len(commits[1].Files), "second commit should have 1 file")
	assert.Equal(t, "pkg/lib.go", commits[1].Files[0].Path)
}

func TestParseLogPathsOnly_NoFiles(t *testing.T) {
	// A commit with no file changes (e.g., empty merge commit)
	input := `DIFFYDUCK_COMMIT_START
DIFFYDUCK_SHA:abc123
DIFFYDUCK_AUTHOR:Jane
DIFFYDUCK_EMAIL:jane@example.com
DIFFYDUCK_DATE:2024-01-10
DIFFYDUCK_SUBJECT:Merge commit
DIFFYDUCK_BODY_START
DIFFYDUCK_BODY_END
`

	commits := parseLogPathsOnly(input)

	require.Equal(t, 1, len(commits), "should parse 1 commit")
	assert.Equal(t, 0, len(commits[0].Files), "commit should have no files")
}

func TestMockGit_LogPathsOnly(t *testing.T) {
	mock := &MockGit{
		LogPaths: []CommitWithPaths{
			{
				Meta: &CommitMeta{
					SHA:     "aaa111",
					Author:  "Alice",
					Subject: "First",
				},
				Files: []FilePath{{Path: "file1.go"}, {Path: "file2.go"}},
			},
			{
				Meta: &CommitMeta{
					SHA:     "bbb222",
					Author:  "Bob",
					Subject: "Second",
				},
				Files: []FilePath{{Path: "file3.go"}},
			},
		},
	}

	commits, err := mock.LogPathsOnly(10)
	require.NoError(t, err)
	assert.Equal(t, 2, len(commits))
	assert.Equal(t, "aaa111", commits[0].Meta.SHA)
	assert.Equal(t, 2, len(commits[0].Files))
	assert.Equal(t, "bbb222", commits[1].Meta.SHA)
	assert.Equal(t, 1, len(commits[1].Files))
}

// =============================================================================
// Snapshot Ref Tests
// =============================================================================

func TestMockGit_SnapshotRefs(t *testing.T) {
	mock := &MockGit{}

	// CurrentBranch returns "main"
	branch, err := mock.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "main", branch)

	// UpdateSnapshotRef is a no-op
	err = mock.UpdateSnapshotRef("main", "base-sha", "abc123")
	require.NoError(t, err)

	// ListSnapshotRefs returns empty
	refs, err := mock.ListSnapshotRefs("main", "base-sha")
	require.NoError(t, err)
	assert.Nil(t, refs)

	// DeleteSnapshotRefs is a no-op
	err = mock.DeleteSnapshotRefs("main", "base-sha")
	require.NoError(t, err)

	// ExpireOldSnapshotRefs is a no-op
	deleted, err := mock.ExpireOldSnapshotRefs(14)
	require.NoError(t, err)
	assert.Equal(t, 0, deleted)
}

func TestPrependContextFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "empty args",
			args: []string{},
			want: []string{"-U8"},
		},
		{
			name: "no context flag",
			args: []string{"HEAD", "--", "file.go"},
			want: []string{"-U8", "HEAD", "--", "file.go"},
		},
		{
			name: "user specified -U3",
			args: []string{"-U3", "HEAD"},
			want: []string{"-U3", "HEAD"},
		},
		{
			name: "user specified -U20",
			args: []string{"HEAD", "-U20"},
			want: []string{"HEAD", "-U20"},
		},
		{
			name: "user specified --unified=5",
			args: []string{"--unified=5", "HEAD"},
			want: []string{"--unified=5", "HEAD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prependContextFlag(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Integration Tests for Snapshot Refs (requires real git repo)
// =============================================================================

func TestRealGit_SnapshotRefs_Integration(t *testing.T) {
	// Skip if no git available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// Create a temp directory with a git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	runGit(t, tmpDir, "init")

	// Create initial commit (this is our base)
	// Use --no-verify to skip inherited pre-commit hooks
	// Use environment variables for author/committer to avoid touching any git config
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "initial")
	baseSHA := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))

	g := NewWithDir(tmpDir)

	branch := "test-branch"

	// Test ListSnapshotRefs when no ref exists
	refs, err := g.ListSnapshotRefs(branch, baseSHA)
	require.NoError(t, err)
	assert.Nil(t, refs, "should have no refs initially")

	// Create a snapshot commit with baseSHA as parent
	snapshot1, err := g.CreateSnapshot(false, baseSHA, "dfd: test @ now")
	require.NoError(t, err)

	// Update the snapshot ref to point to snapshot1
	err = g.UpdateSnapshotRef(branch, baseSHA, snapshot1)
	require.NoError(t, err)

	// Verify ref was created and lists the snapshot
	refs, err = g.ListSnapshotRefs(branch, baseSHA)
	require.NoError(t, err)
	require.Equal(t, 1, len(refs), "should have 1 snapshot")
	assert.Equal(t, snapshot1, refs[0].SHA)
	assert.Equal(t, "dfd: test @ now", refs[0].Subject)

	// Create another snapshot with snapshot1 as parent
	snapshot2, err := g.CreateSnapshot(false, snapshot1, "dfd: test @ now")
	require.NoError(t, err)

	// Update ref to point to latest
	err = g.UpdateSnapshotRef(branch, baseSHA, snapshot2)
	require.NoError(t, err)

	// Should now list both snapshots (oldest first)
	refs, err = g.ListSnapshotRefs(branch, baseSHA)
	require.NoError(t, err)
	require.Equal(t, 2, len(refs), "should have 2 snapshots")
	assert.Equal(t, snapshot1, refs[0].SHA, "first should be oldest")
	assert.Equal(t, snapshot2, refs[1].SHA, "second should be newest")

	// Test DeleteSnapshotRefs
	err = g.DeleteSnapshotRefs(branch, baseSHA)
	require.NoError(t, err)

	// Verify ref deleted (ListSnapshotRefs returns nil when ref doesn't exist)
	refs, err = g.ListSnapshotRefs(branch, baseSHA)
	require.NoError(t, err)
	assert.Nil(t, refs, "should have no refs after delete")
}

func TestRealGit_ListSnapshotRefs_SubjectAndDate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	// Create initial commit (base)
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "initial")
	baseSHA := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))

	g := NewWithDir(tmpDir)

	// Create snapshot with a known message
	msg := "dfd: abc1234 @ Feb 5 09:15"
	sha, err := g.CreateSnapshot(false, baseSHA, msg)
	require.NoError(t, err)
	require.NotEmpty(t, sha)

	err = g.UpdateSnapshotRef("test-branch", baseSHA, sha)
	require.NoError(t, err)

	refs, err := g.ListSnapshotRefs("test-branch", baseSHA)
	require.NoError(t, err)
	require.Len(t, refs, 1)

	// Subject should match the commit message exactly
	assert.Equal(t, msg, refs[0].Subject)
	// SHA should be the full commit SHA
	assert.Equal(t, sha, refs[0].SHA)
	// Date should be reformatted to "Jan 2 15:04" (from git's ci format)
	assert.NotEmpty(t, refs[0].Date)
	// Verify the date parses back (it's in "Jan 2 15:04" format)
	_, parseErr := time.Parse("Jan 2 15:04", refs[0].Date)
	assert.NoError(t, parseErr, "Date %q should be in 'Jan 2 15:04' format", refs[0].Date)
}

func TestRealGit_ListSnapshotRefs_EmptyMessage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "initial")
	baseSHA := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))

	g := NewWithDir(tmpDir)

	// Create snapshot with empty message
	sha, err := g.CreateSnapshot(false, baseSHA, "")
	require.NoError(t, err)
	require.NotEmpty(t, sha)

	err = g.UpdateSnapshotRef("test-branch", baseSHA, sha)
	require.NoError(t, err)

	refs, err := g.ListSnapshotRefs("test-branch", baseSHA)
	require.NoError(t, err)
	require.Len(t, refs, 1)

	assert.Equal(t, sha, refs[0].SHA)
	assert.Empty(t, refs[0].Subject)
	assert.NotEmpty(t, refs[0].Date)
}

func TestRealGit_ListSnapshotRefs_MultipleWithDistinctSubjects(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "initial")
	baseSHA := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))

	g := NewWithDir(tmpDir)

	// Create 3 snapshots with distinct messages to verify ordering and parsing
	messages := []string{
		"dfd: abc1234 @ Feb 3 08:00",
		"dfd: abc1234 @ Feb 4 09:30",
		"dfd: abc1234 @ Feb 5 11:45",
	}

	parentSHA := baseSHA
	for _, msg := range messages {
		sha, err := g.CreateSnapshot(false, parentSHA, msg)
		require.NoError(t, err)
		err = g.UpdateSnapshotRef("test-branch", baseSHA, sha)
		require.NoError(t, err)
		parentSHA = sha
	}

	refs, err := g.ListSnapshotRefs("test-branch", baseSHA)
	require.NoError(t, err)
	require.Len(t, refs, 3)

	// Should be oldest first
	assert.Equal(t, messages[0], refs[0].Subject)
	assert.Equal(t, messages[1], refs[1].Subject)
	assert.Equal(t, messages[2], refs[2].Subject)

	// Each should have a distinct SHA
	assert.NotEqual(t, refs[0].SHA, refs[1].SHA)
	assert.NotEqual(t, refs[1].SHA, refs[2].SHA)

	// All dates should be valid "Jan 2 15:04" format
	for i, ref := range refs {
		_, parseErr := time.Parse("Jan 2 15:04", ref.Date)
		assert.NoError(t, parseErr, "refs[%d].Date %q should parse", i, ref.Date)
	}
}

func TestRealGit_DeleteSnapshotRefs_WhenEmpty(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	g := NewWithDir(tmpDir)

	// Should not error when there are no refs to delete
	// Empty strings means delete all refs across all branches/bases
	err := g.DeleteSnapshotRefs("", "")
	require.NoError(t, err)
}

// =============================================================================
// Integration Tests for Branch Methods
// =============================================================================

func TestRealGit_LocalBranches_Integration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	env := []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}

	// Create initial commit on main
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "initial")

	// Rename default branch to "main" for consistency
	runGit(t, tmpDir, "branch", "-M", "main")

	// Create a second branch with a commit
	runGit(t, tmpDir, "checkout", "-b", "feature")
	writeFile(t, tmpDir, "feature.txt", "feature work")
	runGit(t, tmpDir, "add", "feature.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "add feature")

	g := NewWithDir(tmpDir)
	branches, err := g.LocalBranches()
	require.NoError(t, err)
	require.Len(t, branches, 2)

	// Check that we got both branches
	names := map[string]bool{}
	for _, b := range branches {
		names[b.Name] = true
		assert.NotEmpty(t, b.SHA)
		assert.NotEmpty(t, b.Subject)
		assert.NotEmpty(t, b.Date)
	}
	assert.True(t, names["main"])
	assert.True(t, names["feature"])

	// Current branch (feature) should be marked as HEAD
	var headCount int
	for _, b := range branches {
		if b.IsHead {
			headCount++
			assert.Equal(t, "feature", b.Name)
		}
	}
	assert.Equal(t, 1, headCount)
}

func TestRealGit_MergeBase_Integration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	env := []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}

	// Create initial commit
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "initial")
	runGit(t, tmpDir, "branch", "-M", "main")
	baseSHA := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))

	// Create feature branch with a commit
	runGit(t, tmpDir, "checkout", "-b", "feature")
	writeFile(t, tmpDir, "feature.txt", "feature")
	runGit(t, tmpDir, "add", "feature.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "add feature")

	g := NewWithDir(tmpDir)

	// merge-base of main and feature should be the initial commit
	mb, err := g.MergeBase("main", "feature")
	require.NoError(t, err)
	assert.Equal(t, baseSHA, mb)
}

func TestRealGit_MergeBase_NoCommonAncestor(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	env := []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}

	// Create initial commit on main
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "initial")
	runGit(t, tmpDir, "branch", "-M", "main")

	// Create orphan branch
	runGit(t, tmpDir, "checkout", "--orphan", "orphan")
	runGit(t, tmpDir, "rm", "-rf", ".")
	writeFile(t, tmpDir, "orphan.txt", "orphan")
	runGit(t, tmpDir, "add", "orphan.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "orphan initial")

	g := NewWithDir(tmpDir)

	mb, err := g.MergeBase("main", "orphan")
	require.NoError(t, err)
	assert.Equal(t, "", mb)
}

func TestRealGit_AheadBehind_Integration(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")

	env := []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}

	// Create initial commit on main
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "initial")
	runGit(t, tmpDir, "branch", "-M", "main")

	// Create feature branch with 2 commits
	runGit(t, tmpDir, "checkout", "-b", "feature")
	writeFile(t, tmpDir, "f1.txt", "f1")
	runGit(t, tmpDir, "add", "f1.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "feat 1")
	writeFile(t, tmpDir, "f2.txt", "f2")
	runGit(t, tmpDir, "add", "f2.txt")
	runGitWithEnv(t, tmpDir, env, "commit", "--no-verify", "-m", "feat 2")

	g := NewWithDir(tmpDir)

	// feature is 2 ahead of main, 0 behind
	ahead, behind, err := g.AheadBehind("feature", "main")
	require.NoError(t, err)
	assert.Equal(t, 2, ahead)
	assert.Equal(t, 0, behind)

	// main is 0 ahead of feature, 2 behind
	ahead, behind, err = g.AheadBehind("main", "feature")
	require.NoError(t, err)
	assert.Equal(t, 0, ahead)
	assert.Equal(t, 2, behind)
}

func TestRealGit_CurrentBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "initial")

	g := NewWithDir(tmpDir)

	// Should return the default branch name
	branch, err := g.CurrentBranch()
	require.NoError(t, err)
	assert.NotEmpty(t, branch)

	// Create and switch to a new branch
	runGit(t, tmpDir, "checkout", "-b", "feature/test")
	branch, err = g.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "feature/test", branch)

	// Detached HEAD should return "HEAD"
	sha := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))
	runGit(t, tmpDir, "checkout", sha)
	branch, err = g.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "HEAD", branch)
}

func TestRealGit_SnapshotRefs_BranchIsolation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")
	writeFile(t, tmpDir, "test.txt", "hello")
	runGit(t, tmpDir, "add", "test.txt")
	runGitWithEnv(t, tmpDir, []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}, "commit", "--no-verify", "-m", "initial")
	baseSHA := strings.TrimSpace(runGit(t, tmpDir, "rev-parse", "HEAD"))

	g := NewWithDir(tmpDir)

	// Create snapshot on branch-a
	snap1, err := g.CreateSnapshot(false, baseSHA, "snapshot on branch-a")
	require.NoError(t, err)
	err = g.UpdateSnapshotRef("branch-a", baseSHA, snap1)
	require.NoError(t, err)

	// Create snapshot on branch-b (same baseSHA)
	snap2, err := g.CreateSnapshot(false, baseSHA, "snapshot on branch-b")
	require.NoError(t, err)
	err = g.UpdateSnapshotRef("branch-b", baseSHA, snap2)
	require.NoError(t, err)

	// branch-a should only see its snapshot
	refsA, err := g.ListSnapshotRefs("branch-a", baseSHA)
	require.NoError(t, err)
	require.Len(t, refsA, 1)
	assert.Equal(t, snap1, refsA[0].SHA)
	assert.Equal(t, "snapshot on branch-a", refsA[0].Subject)

	// branch-b should only see its snapshot
	refsB, err := g.ListSnapshotRefs("branch-b", baseSHA)
	require.NoError(t, err)
	require.Len(t, refsB, 1)
	assert.Equal(t, snap2, refsB[0].SHA)
	assert.Equal(t, "snapshot on branch-b", refsB[0].Subject)

	// Deleting branch-a's ref should not affect branch-b
	err = g.DeleteSnapshotRefs("branch-a", baseSHA)
	require.NoError(t, err)

	refsA, err = g.ListSnapshotRefs("branch-a", baseSHA)
	require.NoError(t, err)
	assert.Nil(t, refsA)

	refsB, err = g.ListSnapshotRefs("branch-b", baseSHA)
	require.NoError(t, err)
	require.Len(t, refsB, 1)
	assert.Equal(t, snap2, refsB[0].SHA)
}

// Helper functions for integration tests

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = cleanGitEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return string(out)
}

func runGitWithEnv(t *testing.T, dir string, env []string, args ...string) string {
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

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := dir + "/" + name
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func TestHasConflicts_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, dir, "file.txt", "hello\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "initial")

	g := NewWithDir(dir)
	assert.False(t, g.HasConflicts())
}

func TestHasConflicts_MergeConflict(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create a base commit
	writeFile(t, dir, "file.txt", "base\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "base")

	// Create a branch with a conflicting change
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "file.txt", "feature change\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "feature")

	// Go back to main and make a conflicting change
	runGit(t, dir, "checkout", "master")
	writeFile(t, dir, "file.txt", "master change\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "master")

	// Attempt merge — should conflict
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	cmd.Env = cleanGitEnv(os.Environ())
	_ = cmd.Run() // ignore error (expected: merge conflict)

	g := NewWithDir(dir)
	assert.True(t, g.HasConflicts())
}

func TestHasConflicts_RebaseConflict(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create a base commit
	writeFile(t, dir, "file.txt", "base\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "base")

	// Create a branch with a conflicting change
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "file.txt", "feature change\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "feature")

	// Go back to main and make a conflicting change
	runGit(t, dir, "checkout", "master")
	writeFile(t, dir, "file.txt", "master change\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "master")

	// Attempt rebase — should conflict
	cmd := exec.Command("git", "rebase", "feature")
	cmd.Dir = dir
	cmd.Env = cleanGitEnv(os.Environ())
	_ = cmd.Run() // ignore error (expected: rebase conflict)

	g := NewWithDir(dir)
	assert.True(t, g.HasConflicts())
}

func TestMockGit_HasConflicts(t *testing.T) {
	mock := &MockGit{}
	assert.False(t, mock.HasConflicts())

	mock.HasConflictsVal = true
	assert.True(t, mock.HasConflicts())
}
