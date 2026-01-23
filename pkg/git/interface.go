package git

import "io"

// Git provides an interface for git operations.
// This allows mocking in tests.
type Git interface {
	// Show returns the diff output for git show.
	// Args are passed through to git show.
	Show(args ...string) (string, error)

	// ShowWithMeta returns commit metadata and diff output for git show.
	// The first return is parsed commit metadata, the second is the diff.
	ShowWithMeta(args ...string) (*CommitMeta, string, error)

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
}
