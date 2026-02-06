package git

import "io"

// SnapshotInfo contains metadata about a snapshot commit.
type SnapshotInfo struct {
	SHA     string // full commit SHA
	Subject string // commit message (first line)
	Date    string // commit date in "Jan 2 15:04" format
}

// Git provides an interface for git operations.
// This allows mocking in tests.
type Git interface {
	// Show returns the diff output for git show.
	// Args are passed through to git show.
	Show(args ...string) (string, error)

	// ShowWithMeta returns commit metadata and diff output for git show.
	// The first return is parsed commit metadata, the second is the diff.
	ShowWithMeta(args ...string) (*CommitMeta, string, error)

	// LogWithMeta returns commit metadata and diff output for multiple commits.
	// The n parameter limits the number of commits returned.
	LogWithMeta(n int) ([]CommitWithDiff, error)

	// LogMetaOnly returns commit metadata with per-file stats (no patches).
	// Much faster than LogWithMeta for large histories.
	LogMetaOnly(n int) ([]CommitWithStats, error)

	// LogMetaOnlyRange returns commit metadata with per-file stats for a range.
	// skip is commits to skip, limit is max commits to return.
	LogMetaOnlyRange(skip, limit int) ([]CommitWithStats, error)

	// LogPathsOnly returns commit metadata with file paths only (no stats or patches).
	// This is the fastest option for large histories.
	LogPathsOnly(n int) ([]CommitWithPaths, error)

	// LogPathsOnlyRange returns commit metadata with file paths for a range.
	// skip is commits to skip, limit is max commits to return.
	LogPathsOnlyRange(skip, limit int) ([]CommitWithPaths, error)

	// CommitCount returns the total number of commits in the repository.
	// Returns -1 if count cannot be determined.
	CommitCount() (int, error)

	// Diff returns the diff output for git diff.
	// Args are passed through to git diff.
	Diff(args ...string) (string, error)

	// GetFileContent returns the content of a file at a given ref.
	// The ref can be a commit, branch, tag, or empty string for the index.
	// Uses git show <ref>:<path> internally.
	GetFileContent(ref, path string) (string, error)

	// GetFileContentReader returns a reader for streaming file content at a given ref.
	// The caller must close the returned ReadCloser when done.
	// The cleanup function must be called after closing the reader to wait for the git process.
	GetFileContentReader(ref, path string) (io.ReadCloser, func() error, error)

	// ListUntrackedFiles returns a list of untracked files (excluding ignored files).
	// Uses git ls-files --others --exclude-standard.
	ListUntrackedFiles() ([]string, error)

	// DiffNewFile generates a diff showing a file as entirely new.
	// This is used for untracked files that have no previous version.
	DiffNewFile(path string) (string, error)

	// CreateSnapshot creates a commit representing the current working tree state.
	// If allMode is true, includes untracked files; otherwise only tracked files.
	// If parentSHA is non-empty, the commit will have that as its parent, forming a chain.
	// The message is used as the commit message.
	// Returns the commit SHA. The commit is not attached to any ref.
	CreateSnapshot(allMode bool, parentSHA string, message string) (string, error)

	// DiffSnapshots returns the diff between two snapshot commits.
	DiffSnapshots(sha1, sha2 string) (string, error)

	// UpdateSnapshotRef updates refs/dfd/snapshots/<baseSHA> to point to sha.
	// Uses a single ref per base, with history traversed via parent chain.
	UpdateSnapshotRef(baseSHA string, sha string) error

	// ListSnapshotRefs returns snapshot info for all snapshots for a base SHA.
	// Traverses the parent chain from refs/dfd/snapshots/<baseSHA> via git log.
	// Returns oldest first (chronological order).
	ListSnapshotRefs(baseSHA string) ([]SnapshotInfo, error)

	// DeleteSnapshotRefs deletes snapshot refs under refs/dfd/snapshots/.
	// If baseSHA is non-empty, only deletes that ref; otherwise deletes all.
	DeleteSnapshotRefs(baseSHA string) error

	// ExpireOldSnapshotRefs deletes snapshot refs older than maxAgeDays.
	// Returns the number of deleted refs.
	ExpireOldSnapshotRefs(maxAgeDays int) (int, error)
}
