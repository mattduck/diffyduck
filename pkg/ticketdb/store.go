package ticketdb

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// errCASConflict signals that update-ref rejected our compare-and-swap because
// the ref's current value didn't match the oldSHA we passed. Only this error
// triggers a retry in update(); any other error is fatal.
var errCASConflict = errors.New("comments ref: CAS conflict")

// Store handles reading and writing comments to git refs.
//
// Comments are stored in a tree structure under refs/dfd/comments:
//
//	refs/dfd/comments -> tree
//	                      ├── index (blob: file path -> comment IDs)
//	                      └── data/ (subtree)
//	                           ├── <id1> (blob: comment content)
//	                           └── <id2> (blob: comment content)
//
// Writes go through a read-modify-write loop with compare-and-swap on
// update-ref, so concurrent writers from multiple processes (e.g. a TUI
// and a CLI `dfd comment add`) can't clobber each other.
//
// TODO: Future support for remote fetch/merge of the comments ref for collaboration.
type Store struct {
	// dir is the working directory for git commands.
	dir string
}

// NewStore creates a new Store for the given repository directory.
// If dir is empty, uses the current working directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// maxUpdateAttempts caps how many times update() retries the read-modify-write
// cycle when CAS fails. Each retry re-reads the ref so a stale read doesn't
// keep failing forever. The bound prevents infinite loops on persistent
// failures (e.g. permission denied) and is generous enough to cover realistic
// contention from multiple writers — well above any sane multi-writer scenario.
const maxUpdateAttempts = 32

// ReadIndex reads the comment index from the git ref.
// Returns an empty index if the ref doesn't exist.
// Returns an error for any other failure (corrupt ref, transient git error,
// missing index blob) — silently treating those as "empty" would clobber every
// other comment on the next WriteComment.
func (s *Store) ReadIndex() (*Index, error) {
	treeSHA, err := s.resolveRef()
	if err != nil {
		return nil, err
	}
	if treeSHA == "" {
		return NewIndex(), nil
	}
	return s.readIndexFromTree(treeSHA)
}

// ReadComment reads a single comment by ID.
func (s *Store) ReadComment(id string) (*Comment, error) {
	data, err := s.readBlob(RefPath + ":data/" + id)
	if err != nil {
		return nil, fmt.Errorf("reading comment %s: %w", id, err)
	}
	return ParseComment(id, data)
}

// ReadComments reads multiple comments by IDs.
func (s *Store) ReadComments(ids []string) ([]*Comment, error) {
	comments := make([]*Comment, 0, len(ids))
	for _, id := range ids {
		c, err := s.ReadComment(id)
		if err != nil {
			// Skip comments that can't be read (may have been deleted)
			continue
		}
		comments = append(comments, c)
	}
	return comments, nil
}

// ReadCommentsBatch reads multiple comments in a single git cat-file --batch call.
// Missing or unparseable blobs are silently skipped.
func (s *Store) ReadCommentsBatch(ids []string) ([]*Comment, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	// Build input: one ref per line
	var input strings.Builder
	for _, id := range ids {
		fmt.Fprintf(&input, "%s:data/%s\n", RefPath, id)
	}

	cmd := s.gitCmd("cat-file", "--batch")
	cmd.Stdin = strings.NewReader(input.String())

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("batch read: %w", err)
	}

	// Parse batch output. Each object is either:
	//   <sha> blob <size>\n<content>\n
	// or for missing objects:
	//   <ref> missing\n
	result := make([]*Comment, 0, len(ids))
	data := out
	idIdx := 0

	for len(data) > 0 && idIdx < len(ids) {
		// Read header line
		nl := bytes.IndexByte(data, '\n')
		if nl == -1 {
			break
		}
		header := string(data[:nl])
		data = data[nl+1:]

		if strings.HasSuffix(header, " missing") {
			idIdx++
			continue
		}

		// Parse "<sha> blob <size>"
		parts := strings.Fields(header)
		if len(parts) < 3 || parts[1] != "blob" {
			idIdx++
			continue
		}
		size, err := strconv.Atoi(parts[2])
		if err != nil {
			idIdx++
			continue
		}

		if len(data) < size+1 { // +1 for trailing newline
			break
		}
		content := string(data[:size])
		data = data[size+1:] // skip content + trailing newline

		c, err := ParseComment(ids[idIdx], content)
		if err != nil {
			idIdx++
			continue
		}
		result = append(result, c)
		idIdx++
	}

	return result, nil
}

// WriteComment writes a comment to the git ref.
// If the comment ID already exists, it is overwritten.
// Returns the comment ID.
//
// Concurrent-safe: the read-modify-write happens under CAS on update-ref,
// so racing writers retry instead of stomping each other.
func (s *Store) WriteComment(c *Comment) (string, error) {
	if c.ID == "" {
		c.ID = NewID()
	}

	if c.Anchor == "" {
		c.Anchor = c.Context.ComputeAnchor()
	}

	// Write the comment blob outside the CAS loop. Blobs are content-addressed,
	// so even if update() retries, the same blob SHA pops out — no work wasted,
	// and the inner loop stays minimal.
	commentSHA, err := s.writeBlob(c.Serialize())
	if err != nil {
		return "", fmt.Errorf("writing comment blob: %w", err)
	}

	err = s.update(func(idx *Index, dataEntries map[string]string) error {
		idx.Add(c.File, c.ID)
		dataEntries[c.ID] = commentSHA
		return nil
	})
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

// DeleteComment removes a comment from the git ref.
func (s *Store) DeleteComment(id string) error {
	return s.update(func(idx *Index, dataEntries map[string]string) error {
		sha, ok := dataEntries[id]
		if !ok {
			return fmt.Errorf("comment %s not found", id)
		}
		// Recover the comment's file path so idx.Remove knows which bucket
		// to clear. The blob lookup is by content-addressed SHA, so it
		// returns the same content regardless of any concurrent ref churn
		// — no snapshot pinning involved.
		data, err := s.readBlob(sha)
		if err != nil {
			return fmt.Errorf("reading comment %s: %w", id, err)
		}
		c, err := ParseComment(id, data)
		if err != nil {
			return fmt.Errorf("parsing comment %s: %w", id, err)
		}
		idx.Remove(c.File, id)
		delete(dataEntries, id)
		return nil
	})
}

// CommentsForFile returns all comments for a given file path.
func (s *Store) CommentsForFile(filePath string) ([]*Comment, error) {
	idx, err := s.ReadIndex()
	if err != nil {
		return nil, err
	}

	ids := idx.Get(filePath)
	if len(ids) == 0 {
		return nil, nil
	}

	return s.ReadComments(ids)
}

// AllComments returns all comments in the store.
// Uses batch reading for efficiency (single git cat-file --batch call).
func (s *Store) AllComments() ([]*Comment, error) {
	idx, err := s.ReadIndex()
	if err != nil {
		return nil, err
	}

	return s.ReadCommentsBatch(idx.All())
}

// gitCmd creates an exec.Command for git with a clean environment.
// Filters out GIT_DIR, GIT_WORK_TREE, and GIT_INDEX_FILE so that cmd.Dir
// is respected even when running inside git hooks (e.g. pre-commit).
func (s *Store) gitCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if s.dir != "" {
		cmd.Dir = s.dir
	}
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GIT_DIR=") ||
			strings.HasPrefix(e, "GIT_WORK_TREE=") ||
			strings.HasPrefix(e, "GIT_INDEX_FILE=") {
			continue
		}
		env = append(env, e)
	}
	cmd.Env = env
	return cmd
}

// readBlob reads a blob from git.
func (s *Store) readBlob(ref string) (string, error) {
	cmd := s.gitCmd("cat-file", "blob", ref)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// resolveRef returns the SHA the comments ref currently points to, or ""
// if the ref doesn't exist. Errors are returned for any other failure
// (corrupt repo, transient git, permission). Callers MUST treat a non-nil
// error as "I don't know the state" — never as "ref is missing" — otherwise
// they'd build a fresh tree and clobber every existing comment.
//
// Implementation: `git rev-parse --verify --quiet` prints the SHA on success
// and exits non-zero with empty stderr when the ref is missing. Any non-zero
// exit with non-empty stderr is a real error (e.g. broken repo).
func (s *Store) resolveRef() (string, error) {
	cmd := s.gitCmd("rev-parse", "--verify", "--quiet", RefPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if stderr.Len() == 0 {
			// --quiet swallows the "not a valid object" message; empty
			// stderr means the ref simply doesn't exist.
			return "", nil
		}
		return "", fmt.Errorf("resolving %s: %s: %w", RefPath, strings.TrimSpace(stderr.String()), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// readIndexFromTree reads the index blob from a specific tree SHA, so the
// caller can read index and data entries from the same point-in-time view.
func (s *Store) readIndexFromTree(treeSHA string) (*Index, error) {
	data, err := s.readBlob(treeSHA + ":index")
	if err != nil {
		return nil, fmt.Errorf("reading index from tree %s: %w", treeSHA, err)
	}
	return ParseIndex(data), nil
}

// listDataEntriesFromTree returns the comment ID -> blob SHA map from a
// specific tree SHA, paired with readIndexFromTree to give a consistent
// snapshot independent of any concurrent ref updates.
func (s *Store) listDataEntriesFromTree(treeSHA string) (map[string]string, error) {
	cmd := s.gitCmd("ls-tree", treeSHA+":data")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("listing data entries from tree %s: %s: %w", treeSHA, strings.TrimSpace(stderr.String()), err)
	}

	entries := make(map[string]string)
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <mode> <type> <sha>\t<name>
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			sha := parts[2]
			name := parts[3]
			entries[name] = sha
		}
	}

	return entries, nil
}

// update runs a read-modify-write cycle against the comments ref, retrying
// on CAS conflict. The mutate callback receives the index and data-tree
// entries read from the same pinned tree SHA, so the snapshot is internally
// consistent even when other writers are racing.
//
// mutate MUST be idempotent: on a CAS conflict it will be invoked again
// with a freshly-read snapshot, and any work it did against the previous
// snapshot is discarded. Don't perform external side effects (logging
// progress, appending to slices, incrementing counters owned by the caller,
// I/O) inside it — restrict mutations to the supplied idx and dataEntries.
func (s *Store) update(mutate func(*Index, map[string]string) error) error {
	var lastErr error
	for attempt := 0; attempt < maxUpdateAttempts; attempt++ {
		oldSHA, err := s.resolveRef()
		if err != nil {
			return fmt.Errorf("resolving ref: %w", err)
		}

		var idx *Index
		dataEntries := make(map[string]string)
		if oldSHA != "" {
			idx, err = s.readIndexFromTree(oldSHA)
			if err != nil {
				return err
			}
			dataEntries, err = s.listDataEntriesFromTree(oldSHA)
			if err != nil {
				return err
			}
		} else {
			idx = NewIndex()
		}

		if err := mutate(idx, dataEntries); err != nil {
			return err
		}

		dataTreeSHA, err := s.buildDataTree(dataEntries)
		if err != nil {
			return fmt.Errorf("building data tree: %w", err)
		}
		indexBlobSHA, err := s.writeBlob(idx.Serialize())
		if err != nil {
			return fmt.Errorf("writing index blob: %w", err)
		}
		rootTreeSHA, err := s.buildRootTree(indexBlobSHA, dataTreeSHA)
		if err != nil {
			return fmt.Errorf("building root tree: %w", err)
		}

		err = s.casUpdateRef(rootTreeSHA, oldSHA)
		if err == nil {
			return nil
		}
		// Only CAS conflicts justify a retry — a real failure (disk full,
		// permission, broken repo) would otherwise burn through the entire
		// retry budget before surfacing.
		if !errors.Is(err, errCASConflict) {
			return err
		}
		lastErr = err

		// Jitter so synchronized retries don't deadlock-march in lockstep.
		// Tiny by design — git's ref lock is held briefly and we want to
		// converge fast under contention.
		time.Sleep(time.Duration(rand.IntN(5)+1) * time.Millisecond)
	}
	return fmt.Errorf("update-ref failed after %d attempts: %w", maxUpdateAttempts, lastErr)
}

// writeBlob writes data to a blob and returns the SHA.
func (s *Store) writeBlob(data string) (string, error) {
	cmd := s.gitCmd("hash-object", "-w", "--stdin")
	cmd.Stdin = strings.NewReader(data)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// buildDataTree creates a tree object from comment ID -> SHA mappings.
func (s *Store) buildDataTree(entries map[string]string) (string, error) {
	var treeInput bytes.Buffer
	for name, sha := range entries {
		fmt.Fprintf(&treeInput, "100644 blob %s\t%s\n", sha, name)
	}

	cmd := s.gitCmd("mktree")
	cmd.Stdin = &treeInput

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// buildRootTree creates the root tree with index and data subtree.
func (s *Store) buildRootTree(indexSHA, dataSHA string) (string, error) {
	var treeInput bytes.Buffer
	fmt.Fprintf(&treeInput, "100644 blob %s\tindex\n", indexSHA)
	fmt.Fprintf(&treeInput, "040000 tree %s\tdata\n", dataSHA)

	cmd := s.gitCmd("mktree")
	cmd.Stdin = &treeInput

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// casUpdateRef updates the comments ref with a compare-and-swap on its
// current value. oldSHA="" means "ref must not exist" (first-write case).
// Returns errCASConflict if the ref doesn't currently match oldSHA (the
// retry loop in update() catches that and re-reads); any other error is
// a real failure (broken repo, disk full, permission denied) and must
// not trigger a retry.
//
// CAS failures from update-ref are identifiable by their stderr signature:
//   - "cannot lock ref ...: reference already exists" (oldSHA="" but ref exists)
//   - "cannot lock ref ...: is at X but expected Y" (oldSHA mismatch)
//
// Anything else — non-zero exit without one of those messages — is treated
// as a real error so callers see it immediately instead of burning through
// the retry budget.
func (s *Store) casUpdateRef(newSHA, oldSHA string) error {
	cmd := s.gitCmd("update-ref", RefPath, newSHA, oldSHA)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if isCASConflict(msg) {
			return errCASConflict
		}
		return fmt.Errorf("update-ref: %s: %w", msg, err)
	}
	return nil
}

// isCASConflict matches update-ref's stderr for the two CAS-rejection cases.
// Conservative on purpose: any unrecognised failure is treated as a real
// error rather than silently retried.
func isCASConflict(stderr string) bool {
	return strings.Contains(stderr, "reference already exists") ||
		strings.Contains(stderr, "but expected")
}

// ReachableCommits returns the set of commit SHAs reachable from ref.
// Uses a single git rev-list call for efficiency.
func (s *Store) ReachableCommits(ref string) (map[string]bool, error) {
	cmd := s.gitCmd("rev-list", ref)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rev-list %s: %w", ref, err)
	}

	result := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result[line] = true
		}
	}
	return result, nil
}

// CurrentBranch returns the current branch name.
// Returns "HEAD" if in detached HEAD state.
func (s *Store) CurrentBranch() (string, error) {
	cmd := s.gitCmd("symbolic-ref", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "HEAD", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// Clear removes all comments by deleting the ref.
func (s *Store) Clear() error {
	// Ignore error if ref doesn't exist
	_ = s.gitCmd("update-ref", "-d", RefPath).Run()
	return nil
}
