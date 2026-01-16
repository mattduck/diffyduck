package git

// Git provides an interface for git operations.
// This allows mocking in tests.
type Git interface {
	// Show returns the diff output for a given commit reference.
	Show(ref string) (string, error)
}
