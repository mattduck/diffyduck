package git

// MockGit is a mock implementation of Git for testing.
type MockGit struct {
	ShowOutput string
	ShowError  error
}

// Show returns the preconfigured output or error.
func (m *MockGit) Show(ref string) (string, error) {
	return m.ShowOutput, m.ShowError
}
