package git

import (
	"fmt"
	"io"
	"strings"
)

// MockGit is a mock implementation of Git for testing.
type MockGit struct {
	ShowOutput string
	ShowError  error
	ShowMeta   *CommitMeta // optional metadata for ShowWithMeta
	DiffOutput string
	DiffError  error
	LogOutput  []CommitWithDiff // output for LogWithMeta
	LogError   error
	// FileContents maps "ref:path" to file content for GetFileContent.
	// Use empty ref for index, e.g., ":foo.go" for staged content.
	FileContents map[string]string
}

// Show returns the preconfigured output or error.
func (m *MockGit) Show(args ...string) (string, error) {
	return m.ShowOutput, m.ShowError
}

// ShowWithMeta returns the preconfigured metadata and output.
func (m *MockGit) ShowWithMeta(args ...string) (*CommitMeta, string, error) {
	if m.ShowError != nil {
		return nil, "", m.ShowError
	}
	meta := m.ShowMeta
	if meta == nil {
		meta = &CommitMeta{} // return empty metadata if not set
	}
	return meta, m.ShowOutput, nil
}

// LogWithMeta returns the preconfigured log output.
func (m *MockGit) LogWithMeta(n int) ([]CommitWithDiff, error) {
	if m.LogError != nil {
		return nil, m.LogError
	}
	// Return at most n commits
	if n >= len(m.LogOutput) {
		return m.LogOutput, nil
	}
	return m.LogOutput[:n], nil
}

// Diff returns the preconfigured output or error.
func (m *MockGit) Diff(args ...string) (string, error) {
	return m.DiffOutput, m.DiffError
}

// GetFileContent returns file content from the FileContents map.
func (m *MockGit) GetFileContent(ref, path string) (string, error) {
	key := ref + ":" + path
	if content, ok := m.FileContents[key]; ok {
		return content, nil
	}
	return "", fmt.Errorf("file not found: %s at %s", path, ref)
}

// GetFileContentReader returns a reader for file content from the FileContents map.
func (m *MockGit) GetFileContentReader(ref, path string) (io.ReadCloser, func() error, error) {
	key := ref + ":" + path
	if content, ok := m.FileContents[key]; ok {
		reader := io.NopCloser(strings.NewReader(content))
		cleanup := func() error { return nil }
		return reader, cleanup, nil
	}
	return nil, nil, fmt.Errorf("file not found: %s at %s", path, ref)
}
