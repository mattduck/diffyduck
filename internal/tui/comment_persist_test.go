package tui

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// cleanGitEnv returns os.Environ() with git-specific variables removed,
// so test git commands use the temp repo (via cmd.Dir) instead of
// inheriting GIT_DIR/GIT_INDEX_FILE from pre-commit hooks.
func cleanGitEnv() []string {
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GIT_DIR=") ||
			strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_INDEX_FILE=") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// setupTestRepo creates a temporary git repository for testing.
// Uses t.TempDir() for auto-cleanup and avoids touching git config.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Env = cleanGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	return dir
}

func TestCommentPersistenceRoundTrip(t *testing.T) {
	dir := setupTestRepo(t)

	store := comments.NewStore(dir)

	// Create a model with some file content
	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 2, Content: "line 2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 2, Content: "line 2", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			New: sidebyside.Line{Num: 3, Content: "added line", Type: sidebyside.Added},
		},
		{
			Old: sidebyside.Line{Num: 3, Content: "line 4", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 4, Content: "line 4", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 4, Content: "line 5", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 5, Content: "line 5", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Add a comment via the persistence layer
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = "Test comment on added line"

	id := m.persistComment(key, "Test comment on added line")
	if id == "" {
		t.Fatal("persistComment returned empty ID")
	}
	m.persistedCommentIDs[key] = id

	// Verify it was stored
	if !store.Exists() {
		t.Fatal("comment store ref should exist after write")
	}

	// Create a new model and load the comments
	m2 := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m2.width = 80
	m2.height = 30

	loaded := m2.loadPersistedComments()
	if loaded != 1 {
		t.Errorf("expected 1 comment loaded, got %d", loaded)
	}

	// Verify the comment was loaded at the correct position
	loadedComment, ok := m2.comments[key]
	if !ok {
		t.Fatal("comment not found in loaded model")
	}
	if loadedComment != "Test comment on added line" {
		t.Errorf("comment text mismatch: got %q", loadedComment)
	}
}

func TestCommentPersistenceWithLineMoved(t *testing.T) {
	dir := setupTestRepo(t)

	store := comments.NewStore(dir)

	// Original file content
	originalPairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "context above 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "context above 1", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 2, Content: "context above 2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 2, Content: "context above 2", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 3, Content: "target line", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 3, Content: "target line", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 4, Content: "context below 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 4, Content: "context below 1", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 5, Content: "context below 2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 5, Content: "context below 2", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: originalPairs},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Add comment on line 3 ("target line")
	key := commentKey{fileIndex: 0, newLineNum: 3}
	m.comments[key] = "Comment on target"
	id := m.persistComment(key, "Comment on target")
	m.persistedCommentIDs[key] = id

	// New file content - line moved down by 2 (new lines inserted at top)
	newPairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			New: sidebyside.Line{Num: 1, Content: "new line A", Type: sidebyside.Added},
		},
		{
			Old: sidebyside.Line{Num: 0, Content: "", Type: sidebyside.Empty},
			New: sidebyside.Line{Num: 2, Content: "new line B", Type: sidebyside.Added},
		},
		{
			Old: sidebyside.Line{Num: 1, Content: "context above 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 3, Content: "context above 1", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 2, Content: "context above 2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 4, Content: "context above 2", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 3, Content: "target line", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 5, Content: "target line", Type: sidebyside.Context}, // Now at line 5
		},
		{
			Old: sidebyside.Line{Num: 4, Content: "context below 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 6, Content: "context below 1", Type: sidebyside.Context},
		},
		{
			Old: sidebyside.Line{Num: 5, Content: "context below 2", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 7, Content: "context below 2", Type: sidebyside.Context},
		},
	}

	// Create new model with moved content
	m2 := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: newPairs},
	}, WithCommentStore(store))
	m2.width = 80
	m2.height = 30

	loaded := m2.loadPersistedComments()
	if loaded != 1 {
		t.Errorf("expected 1 comment loaded, got %d", loaded)
	}

	// Comment should now be at line 5 (moved with content)
	newKey := commentKey{fileIndex: 0, newLineNum: 5}
	loadedComment, ok := m2.comments[newKey]
	if !ok {
		// Check if it's still at old position (shouldn't be)
		if _, foundAtOld := m2.comments[key]; foundAtOld {
			t.Error("comment found at old position instead of new")
		} else {
			t.Fatal("comment not found at new position")
		}
	}
	if loadedComment != "Comment on target" {
		t.Errorf("comment text mismatch: got %q", loadedComment)
	}
}

func TestCommentPersistenceDelete(t *testing.T) {
	dir := setupTestRepo(t)

	store := comments.NewStore(dir)

	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Add and persist a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "To be deleted"
	id := m.persistComment(key, "To be deleted")
	m.persistedCommentIDs[key] = id

	// Delete it
	delete(m.comments, key)
	m.deletePersistedComment(key)

	// Verify deletion
	_, idStillTracked := m.persistedCommentIDs[key]
	if idStillTracked {
		t.Error("persisted comment ID should be removed after delete")
	}

	// Create new model and verify comment is gone
	m2 := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m2.width = 80
	m2.height = 30

	loaded := m2.loadPersistedComments()
	if loaded != 0 {
		t.Errorf("expected 0 comments after delete, got %d", loaded)
	}
}

func TestCommentPersistenceOrphaned(t *testing.T) {
	dir := setupTestRepo(t)

	store := comments.NewStore(dir)

	// Original file content
	originalPairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "unique line that will be removed", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "unique line that will be removed", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: originalPairs},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Add comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Orphaned comment"
	id := m.persistComment(key, "Orphaned comment")
	m.persistedCommentIDs[key] = id

	// New file with completely different content
	newPairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "completely different content", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "completely different content", Type: sidebyside.Context},
		},
	}

	m2 := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: newPairs},
	}, WithCommentStore(store))
	m2.width = 80
	m2.height = 30

	// Comment should be orphaned (not loaded) because content changed
	loaded := m2.loadPersistedComments()
	if loaded != 0 {
		t.Errorf("expected 0 comments (orphaned), got %d", loaded)
	}

	// Comment should still exist in store (not deleted)
	allComments, _ := store.AllComments()
	if len(allComments) != 1 {
		t.Errorf("expected 1 comment in store (orphaned but kept), got %d", len(allComments))
	}
}

func TestCleanFilePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"b/cmd/dfd/main.go", "cmd/dfd/main.go"},
		{"a/cmd/dfd/main.go", "cmd/dfd/main.go"},
		{"cmd/dfd/main.go", "cmd/dfd/main.go"},
		{"b/file.go", "file.go"},
		{"a/file.go", "file.go"},
		{"file.go", "file.go"},
		{"", ""},
		{"b/", ""},
		{"a/", ""},
	}

	for _, tt := range tests {
		result := cleanFilePath(tt.input)
		if result != tt.expected {
			t.Errorf("cleanFilePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCommentPersistenceWithPrefixedPaths(t *testing.T) {
	dir := setupTestRepo(t)

	store := comments.NewStore(dir)

	// Create model with b/ prefixed paths (as they come from diff output)
	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Add a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Test comment"
	id := m.persistComment(key, "Test comment")
	m.persistedCommentIDs[key] = id

	// Verify the stored path is clean (no b/ prefix)
	idx, _ := store.ReadIndex()
	files := idx.Files()
	if len(files) != 1 {
		t.Fatalf("expected 1 file in index, got %d", len(files))
	}
	if files[0] != "test.go" {
		t.Errorf("expected clean path 'test.go', got %q", files[0])
	}

	// Verify we can load it back with the same prefixed paths
	m2 := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m2.width = 80
	m2.height = 30

	loaded := m2.loadPersistedComments()
	if loaded != 1 {
		t.Errorf("expected 1 comment loaded, got %d", loaded)
	}
}

func TestMatchCommentsSkipsSkeletonFiles(t *testing.T) {
	dir := setupTestRepo(t)
	store := comments.NewStore(dir)

	// Write a comment to the store for a file
	c := &comments.Comment{
		Text:    "should not match",
		File:    "test.go",
		Line:    1,
		Context: comments.LineContext{Line: "line 1"},
	}
	c.Anchor = c.Context.ComputeAnchor()
	store.WriteComment(c)

	// Create model with a skeleton file (no Pairs)
	skeleton := sidebyside.SkeletonFilePairNoStats("test.go")
	m := New([]sidebyside.FilePair{skeleton}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Load index then try to match — should skip skeleton (no Pairs)
	m.loadCommentIndex()
	loaded := m.matchCommentsForFiles(0, len(m.files))
	if loaded != 0 {
		t.Errorf("expected 0 comments matched for skeleton file, got %d", loaded)
	}
}

func TestMatchCommentsDeduplicatesIDs(t *testing.T) {
	dir := setupTestRepo(t)
	store := comments.NewStore(dir)

	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Add and persist a comment
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "Test comment"
	id := m.persistComment(key, "Test comment")
	m.persistedCommentIDs[key] = id

	// Clear in-memory state and force-reload index (it was loaded empty at construction
	// time before the comment was persisted)
	m.comments = make(map[commentKey]string)
	m.persistedCommentIDs = make(map[commentKey]string)
	m.commentIndex = nil
	m.loadCommentIndex()

	// First match should load the comment
	loaded := m.matchCommentsForFiles(0, len(m.files))
	if loaded != 1 {
		t.Errorf("expected 1 comment on first match, got %d", loaded)
	}

	// Clear in-memory again but keep loadedCommentIDs
	m.comments = make(map[commentKey]string)
	m.persistedCommentIDs = make(map[commentKey]string)

	// Second match should not re-fetch (ID already in loadedCommentIDs)
	loaded = m.matchCommentsForFiles(0, len(m.files))
	if loaded != 0 {
		t.Errorf("expected 0 comments on second match (dedup), got %d", loaded)
	}
}

func TestPersistCommentUpdatesIndex(t *testing.T) {
	dir := setupTestRepo(t)
	store := comments.NewStore(dir)

	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
	}

	// Two copies of the same file (simulates same file in two commits)
	m := NewWithCommits([]sidebyside.CommitSet{
		{Files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
		}, FilesLoaded: true, FoldLevel: sidebyside.CommitFileHeaders},
		{Files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
		}, FilesLoaded: true, FoldLevel: sidebyside.CommitFileHeaders},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Persist a comment on file 0 (commit A's copy)
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "cross-commit comment"
	id := m.persistComment(key, "cross-commit comment")
	m.persistedCommentIDs[key] = id

	// matchCommentsForFiles on file 1 (commit B's copy) should find it
	// via the updated in-memory index
	loaded := m.matchCommentsForFiles(1, 2)
	if loaded != 1 {
		t.Errorf("expected comment to match on second commit's file, got %d", loaded)
	}

	key2 := commentKey{fileIndex: 1, newLineNum: 1}
	if m.comments[key2] != "cross-commit comment" {
		t.Errorf("comment text mismatch on second file: %q", m.comments[key2])
	}
}

func TestDeleteCommentUpdatesIndex(t *testing.T) {
	dir := setupTestRepo(t)
	store := comments.NewStore(dir)

	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
	}

	m := NewWithCommits([]sidebyside.CommitSet{
		{Files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
		}, FilesLoaded: true, FoldLevel: sidebyside.CommitFileHeaders},
		{Files: []sidebyside.FilePair{
			{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
		}, FilesLoaded: true, FoldLevel: sidebyside.CommitFileHeaders},
	}, WithCommentStore(store))
	m.width = 80
	m.height = 30

	// Persist then delete
	key := commentKey{fileIndex: 0, newLineNum: 1}
	m.comments[key] = "will be deleted"
	id := m.persistComment(key, "will be deleted")
	m.persistedCommentIDs[key] = id

	delete(m.comments, key)
	m.deletePersistedComment(key)

	// Index should be empty — matchCommentsForFiles on the second file should find nothing
	loaded := m.matchCommentsForFiles(1, 2)
	if loaded != 0 {
		t.Errorf("expected 0 after delete, got %d", loaded)
	}
}

func TestCommentPersistenceNoStore(t *testing.T) {
	// Model without store should work without errors
	pairs := []sidebyside.LinePair{
		{
			Old: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
			New: sidebyside.Line{Num: 1, Content: "line 1", Type: sidebyside.Context},
		},
	}

	m := New([]sidebyside.FilePair{
		{OldPath: "a/test.go", NewPath: "b/test.go", FoldLevel: sidebyside.FoldHunks, Pairs: pairs},
	}) // No WithCommentStore
	m.width = 80
	m.height = 30

	// These should not panic
	loaded := m.loadPersistedComments()
	if loaded != 0 {
		t.Errorf("expected 0 loaded without store, got %d", loaded)
	}

	key := commentKey{fileIndex: 0, newLineNum: 1}
	id := m.persistComment(key, "test")
	if id != "" {
		t.Error("persistComment should return empty ID without store")
	}

	m.deletePersistedComment(key) // Should not panic
}
