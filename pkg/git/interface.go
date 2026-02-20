package git

import "io"

// SnapshotInfo contains metadata about a snapshot commit.
type SnapshotInfo struct {
	SHA     string // full commit SHA
	Subject string // commit message (first line)
	Date    string // commit date in "Jan 2 15:04" format
}

// WorktreeInfo describes a single git worktree.
type WorktreeInfo struct {
	Path   string // filesystem path of the worktree
	Branch string // branch name (empty for detached HEAD)
}

// BranchInfo contains metadata about a local branch.
type BranchInfo struct {
	Name           string // branch name (e.g. "main", "feature/foo")
	SHA            string // tip commit SHA (full)
	Subject        string // first line of commit message
	Date           string // author date in ISO 8601 format
	Author         string // author name
	IsHead         bool   // true if this is the currently checked-out branch
	Upstream       string // upstream branch (e.g. "origin/main"), empty if none
	UpstreamAhead  int    // commits local is ahead of upstream
	UpstreamBehind int    // commits local is behind upstream
	UpstreamGone   bool   // true if upstream branch has been deleted
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
	// Extra args are appended to the git log command (e.g. ref ranges, pathspecs).
	LogWithMeta(n int, args ...string) ([]CommitWithDiff, error)

	// LogMetaOnly returns commit metadata with per-file stats (no patches).
	// Much faster than LogWithMeta for large histories.
	// Extra args are appended to the git log command (e.g. ref ranges, pathspecs).
	LogMetaOnly(n int, args ...string) ([]CommitWithStats, error)

	// LogMetaOnlyRange returns commit metadata with per-file stats for a range.
	// skip is commits to skip, limit is max commits to return.
	// Extra args are appended to the git log command (e.g. ref ranges, pathspecs).
	LogMetaOnlyRange(skip, limit int, args ...string) ([]CommitWithStats, error)

	// LogPathsOnly returns commit metadata with file paths only (no stats or patches).
	// This is the fastest option for large histories.
	// Extra args are appended to the git log command (e.g. ref ranges, pathspecs).
	LogPathsOnly(n int, args ...string) ([]CommitWithPaths, error)

	// LogPathsOnlyRange returns commit metadata with file paths for a range.
	// skip is commits to skip, limit is max commits to return.
	// Extra args are appended to the git log command (e.g. ref ranges, pathspecs).
	LogPathsOnlyRange(skip, limit int, args ...string) ([]CommitWithPaths, error)

	// CommitCount returns the total number of commits matching the given args.
	// Args are passed through to git rev-list (e.g. ref ranges, pathspecs).
	// Returns -1 if count cannot be determined.
	CommitCount(args ...string) (int, error)

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
	// Extra args are appended to the git diff command (e.g. pathspecs).
	DiffSnapshots(sha1, sha2 string, args ...string) (string, error)

	// CurrentBranch returns the short name of the current branch (e.g. "main").
	// Returns "HEAD" if in detached HEAD state.
	CurrentBranch() (string, error)

	// UpdateSnapshotRef updates refs/dfd/snapshots/<branch>/<baseSHA> to point to sha.
	// Uses a single ref per branch+base, with history traversed via parent chain.
	UpdateSnapshotRef(branch, baseSHA, sha string) error

	// ListSnapshotRefs returns snapshot info for all snapshots for a branch and base SHA.
	// Traverses the parent chain from refs/dfd/snapshots/<branch>/<baseSHA> via git log.
	// Returns oldest first (chronological order).
	ListSnapshotRefs(branch, baseSHA string) ([]SnapshotInfo, error)

	// DeleteSnapshotRefs deletes snapshot refs under refs/dfd/snapshots/.
	// If branch and baseSHA are non-empty, only deletes that ref; otherwise deletes all.
	DeleteSnapshotRefs(branch, baseSHA string) error

	// ExpireOldSnapshotRefs deletes snapshot refs older than maxAgeDays.
	// Returns the number of deleted refs.
	ExpireOldSnapshotRefs(maxAgeDays int) (int, error)

	// HasConflicts returns true if the repo is in a merge, rebase, or
	// cherry-pick state (i.e. sentinel files like MERGE_HEAD exist).
	HasConflicts() bool

	// RepoState returns the current in-progress operation (merge, rebase, etc.)
	// and any contextual detail (e.g. branch name, step progress).
	// Returns ("", "") when the working tree is in a normal state.
	RepoState() (operation, detail string)

	// LocalBranches returns all local branches with tip commit metadata.
	LocalBranches() ([]BranchInfo, error)

	// MergeBase returns the best common ancestor SHA of two refs.
	// Returns empty string and no error if there is no common ancestor.
	MergeBase(a, b string) (string, error)

	// AheadBehind returns how many commits a is ahead of and behind b.
	AheadBehind(a, b string) (ahead, behind int, err error)

	// DefaultBranch returns the name of the repo's default branch (e.g. "main").
	// Tries origin/HEAD first, then falls back to checking for main/master.
	DefaultBranch() (string, error)

	// WorktreeBranches returns branch names that have associated worktrees.
	WorktreeBranches() ([]string, error)

	// WorktreeDetails returns path and branch info for all worktrees.
	WorktreeDetails() ([]WorktreeInfo, error)

	// IsWorktreeDirty returns true if the worktree at the given path has
	// uncommitted changes (staged, unstaged, or untracked files).
	IsWorktreeDirty(path string) (bool, error)

	// Tags returns all tag names.
	Tags() ([]string, error)
}
