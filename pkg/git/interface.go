package git

// Git provides an interface for git operations.
// This allows mocking in tests.
type Git interface {
	// Show returns the diff output for git show.
	// Args are passed through to git show.
	Show(args ...string) (string, error)

	// Diff returns the diff output for git diff.
	// Args are passed through to git diff.
	Diff(args ...string) (string, error)
}
