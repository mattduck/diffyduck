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

func TestStoreReadCommentsBatch(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewStore(dir)

	now := time.Now()
	c1 := &Comment{Text: "first", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	c2 := &Comment{Text: "second", File: "foo.go", Line: 2, Created: now, Updated: now, Context: LineContext{Line: "b"}}
	c3 := &Comment{Text: "third", File: "bar.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "c"}}

	id1, _ := store.WriteComment(c1)
	id2, _ := store.WriteComment(c2)
	id3, _ := store.WriteComment(c3)

	// Batch read all three
	results, err := store.ReadCommentsBatch([]string{id1, id2, id3})
	if err != nil {
		t.Fatalf("ReadCommentsBatch failed: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(results))
	}

	texts := map[string]bool{}
	for _, c := range results {
		texts[c.Text] = true
	}
	if !texts["first"] || !texts["second"] || !texts["third"] {
		t.Errorf("missing comments in batch result: %v", texts)
	}
}

func TestStoreReadCommentsBatchPartialMissing(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewStore(dir)

	now := time.Now()
	c1 := &Comment{Text: "exists", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	id1, _ := store.WriteComment(c1)

	// Batch read with one valid and one nonexistent ID
	results, err := store.ReadCommentsBatch([]string{id1, "nonexistent-id"})
	if err != nil {
		t.Fatalf("ReadCommentsBatch failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(results))
	}
	if results[0].Text != "exists" {
		t.Errorf("wrong text: %q", results[0].Text)
	}
}

func TestStoreReadCommentsBatchEmpty(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewStore(dir)

	results, err := store.ReadCommentsBatch(nil)
	if err != nil {
		t.Fatalf("ReadCommentsBatch failed: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil, got %v", results)
	}

	results, err = store.ReadCommentsBatch([]string{})
	if err != nil {
		t.Fatalf("ReadCommentsBatch failed: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil, got %v", results)
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

// runGitOutput runs a git command and returns trimmed stdout.
// Fatals on failure. Like runGit but returns the output.
func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = cleanGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// runGitInput runs a git command with stdin and returns trimmed stdout.
func runGitInput(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = cleanGitEnv()
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// rewriteRefWithoutIndex re-points the comments ref to a tree that contains
// only the data subtree — no index blob. This simulates the corruption mode
// that previously caused silent data loss: a ref whose index is unreadable
// must NOT be treated as an empty index.
func rewriteRefWithoutIndex(t *testing.T, dir string) {
	t.Helper()

	// Get the data subtree SHA from the current ref.
	lsOut := runGitOutput(t, dir, "ls-tree", RefPath, "data")
	// Format: "<mode> tree <sha>\tdata"
	fields := strings.Fields(lsOut)
	if len(fields) < 3 || fields[1] != "tree" {
		t.Fatalf("unexpected ls-tree output: %q", lsOut)
	}
	dataSHA := fields[2]

	// Build a new root tree with only the data entry.
	mktreeInput := "040000 tree " + dataSHA + "\tdata\n"
	newTree := runGitInput(t, dir, mktreeInput, "mktree")

	// Update the ref to point to the corrupted tree.
	cmd := exec.Command("git", "update-ref", RefPath, newTree)
	cmd.Dir = dir
	cmd.Env = cleanGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("update-ref failed: %v\n%s", err, out)
	}
}

// TestReadIndexErrorsWhenRefCorrupted verifies that ReadIndex distinguishes
// "ref doesn't exist" (legitimate first-write) from "ref exists but the
// index blob is unreadable" (corruption). The latter must surface as an
// error rather than silently returning an empty index, because callers
// would otherwise rewrite the ref with only the new comment and orphan
// every other one.
func TestReadIndexErrorsWhenRefCorrupted(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewStore(dir)

	now := time.Now()
	c := &Comment{Text: "c", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	if _, err := store.WriteComment(c); err != nil {
		t.Fatalf("WriteComment failed: %v", err)
	}

	rewriteRefWithoutIndex(t, dir)

	idx, err := store.ReadIndex()
	if err == nil {
		t.Fatalf("expected error reading index from corrupted ref, got idx=%v", idx.All())
	}
}

// TestWriteCommentRefusesToClobberOnCorruptedRef verifies that writing a new
// comment against a corrupted ref (index blob missing) fails fast instead of
// silently rewriting the ref with an index that contains only the new
// comment — the failure mode that caused "all comments disappear after
// resolve". After the failed write, the existing data blobs must still be
// reachable from the ref's data subtree.
func TestWriteCommentRefusesToClobberOnCorruptedRef(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewStore(dir)

	now := time.Now()
	c1 := &Comment{Text: "c1", File: "foo.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}}
	c2 := &Comment{Text: "c2", File: "foo.go", Line: 2, Created: now, Updated: now, Context: LineContext{Line: "b"}}
	id1, _ := store.WriteComment(c1)
	id2, _ := store.WriteComment(c2)

	rewriteRefWithoutIndex(t, dir)

	// Attempting to write must now fail rather than silently dropping c1+c2.
	c3 := &Comment{Text: "c3", File: "foo.go", Line: 3, Created: now, Updated: now, Context: LineContext{Line: "c"}}
	if _, err := store.WriteComment(c3); err == nil {
		t.Fatal("expected WriteComment to fail on corrupted ref, got nil")
	}

	// Data blobs for the original two comments must still be reachable
	// through the data subtree. The index is gone (that's the corruption),
	// but ReadComment goes directly through :data/<id>.
	for _, id := range []string{id1, id2} {
		if _, err := store.ReadComment(id); err != nil {
			t.Errorf("comment %s should still be readable after failed write: %v", id, err)
		}
	}
}

// TestResolveDoesNotClobberOtherComments is a regression test for the bug
// where flipping Resolved on one comment via WriteComment (the path taken by
// `dfd comment resolve <id>`) caused every other comment to vanish. It
// exercises the happy path end-to-end: many comments exist, one is
// "resolved" by re-writing with Resolved=true, and the rest must remain
// readable through both ReadComment and the index.
func TestResolveDoesNotClobberOtherComments(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewStore(dir)

	now := time.Now()
	want := []*Comment{
		{Text: "c1", File: "a.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "a"}},
		{Text: "c2", File: "a.go", Line: 2, Created: now, Updated: now, Context: LineContext{Line: "b"}},
		{Text: "c3", File: "b.go", Line: 1, Created: now, Updated: now, Context: LineContext{Line: "c"}},
		{Text: "c4", File: "", Line: 0, Created: now, Updated: now, Context: LineContext{}}, // standalone note
	}
	ids := make([]string, len(want))
	for i, c := range want {
		id, err := store.WriteComment(c)
		if err != nil {
			t.Fatalf("WriteComment %d failed: %v", i, err)
		}
		ids[i] = id
	}

	// Simulate `dfd comment resolve <id1>`: read, flip Resolved, write back.
	target, err := store.ReadComment(ids[0])
	if err != nil {
		t.Fatalf("ReadComment failed: %v", err)
	}
	target.Resolved = true
	target.Updated = time.Now()
	if _, err := store.WriteComment(target); err != nil {
		t.Fatalf("WriteComment (resolve) failed: %v", err)
	}

	// Index must still list every comment.
	idx, err := store.ReadIndex()
	if err != nil {
		t.Fatalf("ReadIndex failed: %v", err)
	}
	got := make(map[string]bool)
	for _, id := range idx.All() {
		got[id] = true
	}
	for _, id := range ids {
		if !got[id] {
			t.Errorf("comment %s missing from index after resolve", id)
		}
	}

	// Each comment blob must still be readable, and the resolved flag must
	// have stuck only on the target.
	for i, id := range ids {
		c, err := store.ReadComment(id)
		if err != nil {
			t.Errorf("comment %s unreadable after resolve: %v", id, err)
			continue
		}
		wantResolved := i == 0
		if c.Resolved != wantResolved {
			t.Errorf("comment %s Resolved=%v, want %v", id, c.Resolved, wantResolved)
		}
	}
}
