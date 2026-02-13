package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func makeCommitSet(sha, subject string) sidebyside.CommitSet {
	return sidebyside.CommitSet{
		Info: sidebyside.CommitInfo{
			SHA:     sha,
			Subject: subject,
		},
		FoldLevel:   sidebyside.CommitFileHeaders,
		FilesLoaded: true,
		Files: []sidebyside.FilePair{
			{OldPath: "file.go", NewPath: "file.go", FoldLevel: sidebyside.FoldStructure},
		},
	}
}

// --- hasMoreCommitsToLoad ---

func TestHasMoreCommitsToLoad_RespectsCommitMaxCount(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.loadedCommitCount = 1
	m.totalCommitCount = 100 // repo has lots of commits

	// Without limit: should want more
	assert.True(t, m.hasMoreCommitsToLoad())

	// With limit equal to loaded: should stop
	m.commitMaxCount = 1
	assert.False(t, m.hasMoreCommitsToLoad())

	// With limit above loaded: should want more
	m.commitMaxCount = 5
	assert.True(t, m.hasMoreCommitsToLoad())
}

func TestHasMoreCommitsToLoad_UnknownTotal(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.loadedCommitCount = 1
	m.totalCommitCount = 0 // unknown

	// Unknown total → assume more available
	assert.True(t, m.hasMoreCommitsToLoad())

	// But not if -n limit reached
	m.commitMaxCount = 1
	assert.False(t, m.hasMoreCommitsToLoad())
}

func TestHasMoreCommitsToLoad_PaginationDisabled(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.loadedCommitCount = 0 // pagination not enabled
	assert.False(t, m.hasMoreCommitsToLoad())
}

func TestHasMoreCommitsToLoad_AllLoaded(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.loadedCommitCount = 5
	m.totalCommitCount = 5
	assert.False(t, m.hasMoreCommitsToLoad())
}

// --- shouldLoadMoreCommits ---

func TestShouldLoadMoreCommits_RespectsCommitMaxCount(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 1
	m.totalCommitCount = 100
	m.git = &git.MockGit{}
	m.w().scroll = m.maxScroll() // scroll to end

	// Without limit: should load more
	assert.True(t, m.shouldLoadMoreCommits())

	// With limit reached: should not load more
	m.commitMaxCount = 1
	assert.False(t, m.shouldLoadMoreCommits())
}

// --- fetchMoreCommits ---

func TestFetchMoreCommits_ClampsToMaxCount(t *testing.T) {
	mock := &git.MockGit{
		LogPaths: []git.CommitWithPaths{
			{Meta: &git.CommitMeta{SHA: "aaa"}, Files: []git.FilePath{{Path: "a.go"}}},
			{Meta: &git.CommitMeta{SHA: "bbb"}, Files: []git.FilePath{{Path: "b.go"}}},
			{Meta: &git.CommitMeta{SHA: "ccc"}, Files: []git.FilePath{{Path: "c.go"}}},
			{Meta: &git.CommitMeta{SHA: "ddd"}, Files: []git.FilePath{{Path: "d.go"}}},
			{Meta: &git.CommitMeta{SHA: "eee"}, Files: []git.FilePath{{Path: "e.go"}}},
		},
	}

	m := NewWithCommits(
		[]sidebyside.CommitSet{makeCommitSet("aaa", "first")},
		WithGit(mock),
		WithPagination(1, 100),
		WithCommitLimit(3),
	)
	m.spinnerTicking = true // prevent spinner from wrapping cmd in tea.Batch

	// Fetch should be clamped: max=3, loaded=1, so only fetch 2 more
	cmd := m.fetchMoreCommits()
	assert.NotNil(t, cmd)

	msg := cmd()
	loaded, ok := msg.(MoreCommitsLoadedMsg)
	assert.True(t, ok)
	assert.NoError(t, loaded.Err)
	assert.Len(t, loaded.Commits, 2, "should fetch exactly 2 commits (clamped by -n 3)")
}

func TestFetchMoreCommits_NilWhenAtLimit(t *testing.T) {
	mock2 := &git.MockGit{
		LogPaths: []git.CommitWithPaths{
			{Meta: &git.CommitMeta{SHA: "aaa"}, Files: []git.FilePath{{Path: "a.go"}}},
		},
	}

	m2 := NewWithCommits(
		[]sidebyside.CommitSet{makeCommitSet("aaa", "first")},
		WithGit(mock2),
		WithPagination(1, 100),
		WithCommitLimit(1),
	)

	// Already at limit → should return nil
	cmd2 := m2.fetchMoreCommits()
	assert.Nil(t, cmd2)
	assert.False(t, m2.loadingMoreCommits, "should not be in loading state")
}

// --- MoreCommitsLoadedMsg with empty batch ---

func TestMoreCommitsLoaded_EmptyBatchFixesTotalCount(t *testing.T) {
	// Simulates pathspec-filtered log: totalCommitCount was set to a high
	// value by rev-list, but actual matching commits are fewer.
	m := NewWithCommits([]sidebyside.CommitSet{
		makeCommitSet("aaa", "one"),
		makeCommitSet("bbb", "two"),
	})
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 2
	m.totalCommitCount = 500 // Wrong — set by unfiltered rev-list

	// Process an empty MoreCommitsLoadedMsg
	msg := MoreCommitsLoadedMsg{Commits: nil}
	newM, _ := m.Update(msg)
	updated := newM.(Model)

	assert.Equal(t, 2, updated.totalCommitCount, "totalCommitCount should be corrected to loadedCommitCount")
	assert.False(t, updated.hasMoreCommitsToLoad(), "should not have more commits after empty batch")
}

// --- Pagination indicator in buildRows ---

func TestPaginationIndicator_HiddenWhenAtMaxCount(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 1
	m.totalCommitCount = 100 // repo has many commits

	// Without limit: indicator should appear
	rows := m.buildRows()
	assert.True(t, hasPaginationRow(rows), "indicator should appear without limit")

	// With limit reached: indicator should disappear
	m.commitMaxCount = 1
	m.rebuildRowsCache()
	rows = m.buildRows()
	assert.False(t, hasPaginationRow(rows), "indicator should not appear when -n limit reached")
}

func TestPaginationIndicator_ShownWhenBelowMaxCount(t *testing.T) {
	m := NewWithCommits([]sidebyside.CommitSet{makeCommitSet("aaa", "one")})
	m.width = 120
	m.height = 40
	m.loadedCommitCount = 1
	m.totalCommitCount = 100
	m.commitMaxCount = 5 // limit not yet reached

	rows := m.buildRows()
	assert.True(t, hasPaginationRow(rows), "indicator should appear when below -n limit")
}

// --- WithCommitLimit option ---

func TestWithCommitLimit(t *testing.T) {
	m := NewWithCommits(
		[]sidebyside.CommitSet{makeCommitSet("aaa", "one")},
		WithCommitLimit(42),
	)
	assert.Equal(t, 42, m.commitMaxCount)
}

func hasPaginationRow(rows []displayRow) bool {
	for _, r := range rows {
		if r.kind == RowKindPaginationIndicator {
			return true
		}
	}
	return false
}
