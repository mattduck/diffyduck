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
	LogOutput  []CommitWithDiff  // output for LogWithMeta
	LogStats   []CommitWithStats // output for LogMetaOnly
	LogPaths   []CommitWithPaths // output for LogPathsOnly
	LogError   error
	// FileContents maps "ref:path" to file content for GetFileContent.
	// Use empty ref for index, e.g., ":foo.go" for staged content.
	FileContents    map[string]string
	HasConflictsVal bool   // return value for HasConflicts
	RepoStateOp     string // operation for RepoState (e.g. "Rebasing")
	RepoStateDetail string // detail for RepoState (e.g. "3/5")

	// Branch-related mock data
	Branches              []BranchInfo
	MergeBases            map[string]string // key: "a\x00b" (sorted), value: SHA
	AheadBehinds          map[string][2]int // key: "a\x00b", value: [ahead, behind]
	DefaultBranchVal      string            // return value for DefaultBranch
	WorktreeBranchVal     []string          // return value for WorktreeBranches
	WorktreeDetailsVal    []WorktreeInfo    // return value for WorktreeDetails
	WorktreeDirtyVal      map[string]bool   // path → dirty for IsWorktreeDirty
	TagNames              []string          // return value for Tags
	MergeConflictFilesVal []string          // return value for MergeConflictFiles
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
func (m *MockGit) LogWithMeta(n int, args ...string) ([]CommitWithDiff, error) {
	if m.LogError != nil {
		return nil, m.LogError
	}
	// Return at most n commits
	if n >= len(m.LogOutput) {
		return m.LogOutput, nil
	}
	return m.LogOutput[:n], nil
}

// LogMetaOnly returns the preconfigured log stats output.
func (m *MockGit) LogMetaOnly(n int, args ...string) ([]CommitWithStats, error) {
	return m.LogMetaOnlyRange(0, n)
}

// LogMetaOnlyRange returns a range of the preconfigured log stats output.
func (m *MockGit) LogMetaOnlyRange(skip, limit int, args ...string) ([]CommitWithStats, error) {
	if m.LogError != nil {
		return nil, m.LogError
	}
	if skip >= len(m.LogStats) {
		return nil, nil
	}
	end := skip + limit
	if end > len(m.LogStats) {
		end = len(m.LogStats)
	}
	return m.LogStats[skip:end], nil
}

// LogPathsOnly returns the preconfigured log paths output.
func (m *MockGit) LogPathsOnly(n int, args ...string) ([]CommitWithPaths, error) {
	return m.LogPathsOnlyRange(0, n)
}

// LogPathsOnlyRange returns a range of the preconfigured log paths output.
func (m *MockGit) LogPathsOnlyRange(skip, limit int, args ...string) ([]CommitWithPaths, error) {
	if m.LogError != nil {
		return nil, m.LogError
	}
	if skip >= len(m.LogPaths) {
		return nil, nil
	}
	end := skip + limit
	if end > len(m.LogPaths) {
		end = len(m.LogPaths)
	}
	return m.LogPaths[skip:end], nil
}

// CommitCount returns the number of commits in LogPaths (for testing).
func (m *MockGit) CommitCount(args ...string) (int, error) {
	if m.LogError != nil {
		return -1, m.LogError
	}
	return len(m.LogPaths), nil
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

// ListUntrackedFiles returns an empty list (mock doesn't track untracked files).
func (m *MockGit) ListUntrackedFiles() ([]string, error) {
	return nil, nil
}

// DiffNewFile returns an empty diff (mock doesn't generate diffs for new files).
func (m *MockGit) DiffNewFile(path string) (string, error) {
	return "", nil
}

// CreateSnapshot returns a fake SHA for testing (mock doesn't actually create commits).
func (m *MockGit) CreateSnapshot(parentSHA string, message string) (string, error) {
	return "mock-snapshot-sha", nil
}

// DiffSnapshots returns the preconfigured diff output (mock treats all diffs the same).
func (m *MockGit) DiffSnapshots(sha1, sha2 string, args ...string) (string, error) {
	return m.DiffOutput, m.DiffError
}

// CurrentBranch returns "main" for the mock.
func (m *MockGit) CurrentBranch() (string, error) {
	return "main", nil
}

// UpdateSnapshotRef is a no-op for the mock (doesn't persist refs).
func (m *MockGit) UpdateSnapshotRef(branch, baseSHA, sha string) error {
	return nil
}

// ListSnapshotRefs returns an empty list (mock doesn't persist refs).
func (m *MockGit) ListSnapshotRefs(branch, baseSHA string) ([]SnapshotInfo, error) {
	return nil, nil
}

// DeleteSnapshotRefs is a no-op for the mock (nothing to delete).
func (m *MockGit) DeleteSnapshotRefs(branch, baseSHA string) error {
	return nil
}

// ExpireOldSnapshotRefs is a no-op for the mock (nothing to expire).
func (m *MockGit) ExpireOldSnapshotRefs(maxAgeDays int) (int, error) {
	return 0, nil
}

// HasConflicts returns the preconfigured value.
func (m *MockGit) HasConflicts() bool {
	return m.HasConflictsVal
}

// RepoState returns the preconfigured operation and detail.
func (m *MockGit) RepoState() (string, string) {
	return m.RepoStateOp, m.RepoStateDetail
}

// LocalBranches returns the preconfigured branch list.
func (m *MockGit) LocalBranches() ([]BranchInfo, error) {
	return m.Branches, nil
}

// MergeBase returns the preconfigured merge base for two refs.
func (m *MockGit) MergeBase(a, b string) (string, error) {
	if m.MergeBases == nil {
		return "", nil
	}
	// Try both orderings
	if sha, ok := m.MergeBases[a+"\x00"+b]; ok {
		return sha, nil
	}
	if sha, ok := m.MergeBases[b+"\x00"+a]; ok {
		return sha, nil
	}
	return "", nil
}

// AheadBehind returns the preconfigured ahead/behind counts.
func (m *MockGit) AheadBehind(a, b string) (int, int, error) {
	if m.AheadBehinds == nil {
		return 0, 0, nil
	}
	if counts, ok := m.AheadBehinds[a+"\x00"+b]; ok {
		return counts[0], counts[1], nil
	}
	return 0, 0, nil
}

// DefaultBranch returns the preconfigured default branch name.
func (m *MockGit) DefaultBranch() (string, error) {
	if m.DefaultBranchVal == "" {
		return "main", nil
	}
	return m.DefaultBranchVal, nil
}

// WorktreeBranches returns the preconfigured worktree branch names.
func (m *MockGit) WorktreeBranches() ([]string, error) {
	return m.WorktreeBranchVal, nil
}

// WorktreeDetails returns the preconfigured worktree details.
func (m *MockGit) WorktreeDetails() ([]WorktreeInfo, error) {
	return m.WorktreeDetailsVal, nil
}

// IsWorktreeDirty returns whether the given path is dirty per the mock config.
func (m *MockGit) IsWorktreeDirty(path string) (bool, error) {
	if m.WorktreeDirtyVal == nil {
		return false, nil
	}
	return m.WorktreeDirtyVal[path], nil
}

// Tags returns the preconfigured tag names.
func (m *MockGit) Tags() ([]string, error) {
	return m.TagNames, nil
}

// MergeConflictFiles returns the preconfigured conflict file list.
func (m *MockGit) MergeConflictFiles(sha string) ([]string, error) {
	return m.MergeConflictFilesVal, nil
}
