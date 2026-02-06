package comments

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

// ReadIndex reads the comment index from the git ref.
// Returns an empty index if the ref doesn't exist.
func (s *Store) ReadIndex() (*Index, error) {
	data, err := s.readBlob(RefPath + ":index")
	if err != nil {
		// Ref doesn't exist yet - return empty index
		return NewIndex(), nil
	}
	return ParseIndex(data), nil
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
func (s *Store) WriteComment(c *Comment) (string, error) {
	if c.ID == "" {
		c.ID = NewID()
	}

	// Compute anchor if not set
	if c.Anchor == "" {
		c.Anchor = c.Context.ComputeAnchor()
	}

	// Serialize the comment
	data := c.Serialize()

	// Read current index
	idx, err := s.ReadIndex()
	if err != nil {
		return "", fmt.Errorf("reading index: %w", err)
	}

	// Add to index
	idx.Add(c.File, c.ID)

	// Write the tree
	if err := s.writeTree(idx, c.ID, data); err != nil {
		return "", fmt.Errorf("writing tree: %w", err)
	}

	return c.ID, nil
}

// DeleteComment removes a comment from the git ref.
func (s *Store) DeleteComment(id string) error {
	// Read current index
	idx, err := s.ReadIndex()
	if err != nil {
		return fmt.Errorf("reading index: %w", err)
	}

	// Find and remove from index
	// We need to read the comment first to get the file path
	c, err := s.ReadComment(id)
	if err != nil {
		return fmt.Errorf("reading comment to delete: %w", err)
	}

	idx.Remove(c.File, id)

	// Rebuild tree without this comment
	if err := s.writeTreeWithoutComment(idx, id); err != nil {
		return fmt.Errorf("writing tree: %w", err)
	}

	return nil
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
func (s *Store) AllComments() ([]*Comment, error) {
	idx, err := s.ReadIndex()
	if err != nil {
		return nil, err
	}

	return s.ReadComments(idx.All())
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

// writeTree writes the index and comment data to a new tree and updates the ref.
func (s *Store) writeTree(idx *Index, commentID, commentData string) error {
	// First, write the comment blob
	commentSHA, err := s.writeBlob(commentData)
	if err != nil {
		return fmt.Errorf("writing comment blob: %w", err)
	}

	// Get existing data tree entries (if ref exists)
	existingData, err := s.listDataEntries()
	if err != nil {
		// Ref doesn't exist - start fresh
		existingData = make(map[string]string)
	}

	// Add/update the new comment
	existingData[commentID] = commentSHA

	// Build data tree
	dataSHA, err := s.buildDataTree(existingData)
	if err != nil {
		return fmt.Errorf("building data tree: %w", err)
	}

	// Write index blob
	indexSHA, err := s.writeBlob(idx.Serialize())
	if err != nil {
		return fmt.Errorf("writing index blob: %w", err)
	}

	// Build root tree
	rootSHA, err := s.buildRootTree(indexSHA, dataSHA)
	if err != nil {
		return fmt.Errorf("building root tree: %w", err)
	}

	// Update ref
	return s.updateRef(rootSHA)
}

// writeTreeWithoutComment rebuilds the tree excluding a specific comment.
func (s *Store) writeTreeWithoutComment(idx *Index, excludeID string) error {
	// Get existing data tree entries
	existingData, err := s.listDataEntries()
	if err != nil {
		return fmt.Errorf("listing data entries: %w", err)
	}

	// Remove the excluded comment
	delete(existingData, excludeID)

	// Build data tree
	dataSHA, err := s.buildDataTree(existingData)
	if err != nil {
		return fmt.Errorf("building data tree: %w", err)
	}

	// Write index blob
	indexSHA, err := s.writeBlob(idx.Serialize())
	if err != nil {
		return fmt.Errorf("writing index blob: %w", err)
	}

	// Build root tree
	rootSHA, err := s.buildRootTree(indexSHA, dataSHA)
	if err != nil {
		return fmt.Errorf("building root tree: %w", err)
	}

	// Update ref
	return s.updateRef(rootSHA)
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

// listDataEntries returns a map of comment ID -> blob SHA from the current data tree.
func (s *Store) listDataEntries() (map[string]string, error) {
	cmd := s.gitCmd("ls-tree", RefPath+":data")

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	entries := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
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

// updateRef updates the comments ref to point to a new tree.
func (s *Store) updateRef(treeSHA string) error {
	return s.gitCmd("update-ref", RefPath, treeSHA).Run()
}

// Exists checks if the comments ref exists.
func (s *Store) Exists() bool {
	return s.gitCmd("rev-parse", "--verify", RefPath).Run() == nil
}

// Clear removes all comments by deleting the ref.
func (s *Store) Clear() error {
	// Ignore error if ref doesn't exist
	_ = s.gitCmd("update-ref", "-d", RefPath).Run()
	return nil
}
