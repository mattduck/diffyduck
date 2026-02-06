package comments

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setupTestRepo creates a temporary git repository for testing.
// Uses environment variables for author/committer to avoid touching any git config,
// and --no-verify to skip inherited pre-commit hooks.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	runGit(t, dir, nil, "init")

	// Create an initial commit so HEAD exists
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	runGit(t, dir, nil, "add", "test.txt")
	runGit(t, dir, gitAuthorEnv(), "commit", "--no-verify", "-m", "initial")

	return dir
}

// gitAuthorEnv returns environment variables for git author/committer
// so tests don't depend on or modify any git config.
func gitAuthorEnv() []string {
	return []string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	}
}

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

// runGit runs a git command in the given directory with optional extra env vars.
// Always filters out inherited GIT_DIR/GIT_INDEX_FILE to prevent test commands
// from operating on the project repo when run from pre-commit hooks.
func runGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(cleanGitEnv(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestStoreWriteAndReadComment(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	// Initially no comments
	if store.Exists() {
		t.Error("expected ref to not exist initially")
	}

	// Create a comment
	now := time.Now().Truncate(time.Second)
	c := &Comment{
		Text:      "Test comment",
		File:      "src/foo.go",
		Line:      42,
		Created:   now,
		Updated:   now,
		CommitSHA: "abc123",
		Branch:    "main",
		Context: LineContext{
			Above: []string{"line1", "line2"},
			Line:  "target line",
			Below: []string{"line3", "line4"},
		},
	}

	// Write
	id, err := store.WriteComment(c)
	if err != nil {
		t.Fatalf("WriteComment failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}

	// Ref should now exist
	if !store.Exists() {
		t.Error("expected ref to exist after write")
	}

	// Read back
	read, err := store.ReadComment(id)
	if err != nil {
		t.Fatalf("ReadComment failed: %v", err)
	}

	if read.Text != c.Text {
		t.Errorf("Text mismatch: got %q, want %q", read.Text, c.Text)
	}
	if read.File != c.File {
		t.Errorf("File mismatch: got %q, want %q", read.File, c.File)
	}
	if read.Line != c.Line {
		t.Errorf("Line mismatch: got %d, want %d", read.Line, c.Line)
	}
	if read.Anchor == "" {
		t.Error("expected anchor to be computed")
	}
}

func TestStoreReadIndex(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	// Empty index when ref doesn't exist
	idx, err := store.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}
	if len(idx.All()) != 0 {
		t.Errorf("expected empty index, got %v", idx.All())
	}

	// Add some comments
	now := time.Now()
	c1 := &Comment{Text: "c1", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	c2 := &Comment{Text: "c2", File: "foo.go", Line: 2, Created: now, Updated: now, Context: LineContext{Line: "b"}}
	c3 := &Comment{Text: "c3", File: "bar.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "c"}}

	id1, _ := store.WriteComment(c1)
	id2, _ := store.WriteComment(c2)
	id3, _ := store.WriteComment(c3)

	// Read index
	idx, err = store.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}

	fooIDs := idx.Get("foo.go")
	if len(fooIDs) != 2 {
		t.Errorf("expected 2 comments for foo.go, got %d", len(fooIDs))
	}

	barIDs := idx.Get("bar.go")
	if len(barIDs) != 1 {
		t.Errorf("expected 1 comment for bar.go, got %d", len(barIDs))
	}

	// Verify IDs are present
	all := idx.All()
	found := make(map[string]bool)
	for _, id := range all {
		found[id] = true
	}
	if !found[id1] || !found[id2] || !found[id3] {
		t.Errorf("missing IDs in index: %v", all)
	}
}

func TestStoreCommentsForFile(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	now := time.Now()
	c1 := &Comment{Text: "c1", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	c2 := &Comment{Text: "c2", File: "foo.go", Line: 2, Created: now, Updated: now, Context: LineContext{Line: "b"}}
	c3 := &Comment{Text: "c3", File: "bar.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "c"}}

	store.WriteComment(c1)
	store.WriteComment(c2)
	store.WriteComment(c3)

	// Get comments for foo.go
	comments, err := store.CommentsForFile("foo.go")
	if err != nil {
		t.Fatalf("CommentsForFile failed: %v", err)
	}
	if len(comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(comments))
	}

	// Get comments for bar.go
	comments, err = store.CommentsForFile("bar.go")
	if err != nil {
		t.Fatalf("CommentsForFile failed: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("expected 1 comment, got %d", len(comments))
	}

	// Get comments for unknown file
	comments, err = store.CommentsForFile("unknown.go")
	if err != nil {
		t.Fatalf("CommentsForFile failed: %v", err)
	}
	if comments != nil {
		t.Errorf("expected nil for unknown file, got %v", comments)
	}
}

func TestStoreDeleteComment(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	now := time.Now()
	c1 := &Comment{Text: "c1", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	c2 := &Comment{Text: "c2", File: "foo.go", Line: 2, Created: now, Updated: now, Context: LineContext{Line: "b"}}

	id1, _ := store.WriteComment(c1)
	id2, _ := store.WriteComment(c2)

	// Delete c1
	err := store.DeleteComment(id1)
	if err != nil {
		t.Fatalf("DeleteComment failed: %v", err)
	}

	// Index should only have c2
	idx, _ := store.ReadIndex()
	all := idx.All()
	if len(all) != 1 || all[0] != id2 {
		t.Errorf("expected only %s, got %v", id2, all)
	}

	// Reading deleted comment should fail
	_, err = store.ReadComment(id1)
	if err == nil {
		t.Error("expected error reading deleted comment")
	}

	// c2 should still be readable
	read, err := store.ReadComment(id2)
	if err != nil {
		t.Fatalf("ReadComment failed: %v", err)
	}
	if read.Text != "c2" {
		t.Errorf("wrong comment text: %q", read.Text)
	}
}

func TestStoreAllComments(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	// Empty initially
	all, err := store.AllComments()
	if err != nil {
		t.Fatalf("AllComments failed: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 comments, got %d", len(all))
	}

	// Add some comments
	now := time.Now()
	c1 := &Comment{Text: "c1", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	c2 := &Comment{Text: "c2", File: "bar.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "b"}}

	store.WriteComment(c1)
	store.WriteComment(c2)

	all, err = store.AllComments()
	if err != nil {
		t.Fatalf("AllComments failed: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 comments, got %d", len(all))
	}
}

func TestStoreClear(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	now := time.Now()
	c := &Comment{Text: "test", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	store.WriteComment(c)

	if !store.Exists() {
		t.Error("expected ref to exist")
	}

	err := store.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if store.Exists() {
		t.Error("expected ref to not exist after clear")
	}

	// Should be able to clear again without error
	err = store.Clear()
	if err != nil {
		t.Fatalf("Clear failed on already cleared: %v", err)
	}
}

func TestStoreUpdateComment(t *testing.T) {
	dir := setupTestRepo(t)

	store := NewStore(dir)

	now := time.Now()
	c := &Comment{
		Text:    "original",
		File:    "foo.go",
		Line:    1,
		Created: now,
		Updated: now,
		Context: LineContext{Line: "code"},
	}

	id, _ := store.WriteComment(c)

	// Update the comment
	c.ID = id
	c.Text = "updated"
	c.Updated = time.Now()

	_, err := store.WriteComment(c)
	if err != nil {
		t.Fatalf("WriteComment (update) failed: %v", err)
	}

	// Read back
	read, err := store.ReadComment(id)
	if err != nil {
		t.Fatalf("ReadComment failed: %v", err)
	}
	if read.Text != "updated" {
		t.Errorf("expected 'updated', got %q", read.Text)
	}

	// Should still only have one entry in index
	idx, _ := store.ReadIndex()
	if len(idx.All()) != 1 {
		t.Errorf("expected 1 entry in index, got %d", len(idx.All()))
	}
}
