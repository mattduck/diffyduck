package git

// MockGit is a mock implementation of Git for testing.
type MockGit struct {
	ShowOutput string
	ShowError  error
	DiffOutput string
	DiffError  error
}

// Show returns the preconfigured output or error.
func (m *MockGit) Show(args ...string) (string, error) {
	return m.ShowOutput, m.ShowError
}

// Diff returns the preconfigured output or error.
func (m *MockGit) Diff(args ...string) (string, error) {
	return m.DiffOutput, m.DiffError
}
