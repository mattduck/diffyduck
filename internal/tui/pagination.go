package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// shouldLoadMoreCommits returns true if we should fetch more commits.
// This checks if user is near the end of loaded content and more commits are available.
func (m Model) shouldLoadMoreCommits() bool {
	// Don't load if already loading
	if m.loadingMoreCommits {
		return false
	}
	// Don't load if no git interface (e.g., pager mode)
	if m.git == nil {
		return false
	}
	// Don't load if pagination not enabled (loadedCommitCount == 0)
	if m.loadedCommitCount == 0 {
		return false
	}
	// Don't load if narrowed - the narrowed view is complete and new commits wouldn't appear
	if m.w().narrow.Active {
		return false
	}
	// Don't load if we've loaded all commits
	if !m.hasMoreCommitsToLoad() {
		return false
	}
	// Check if cursor is within threshold of end
	return m.maxScroll()-m.w().scroll < PaginationScrollThreshold
}

// hasMoreCommitsToLoad returns true if there are more commits available to load.
func (m Model) hasMoreCommitsToLoad() bool {
	// Pagination not enabled (no WithPagination option set)
	if m.loadedCommitCount == 0 {
		return false
	}
	// If total count is unknown (0), assume there could be more
	if m.totalCommitCount == 0 {
		return true
	}
	// If error getting count (-1), assume there could be more
	if m.totalCommitCount < 0 {
		return true
	}
	return m.loadedCommitCount < m.totalCommitCount
}

// fetchMoreCommits returns a command to load the next batch of commits.
func (m *Model) fetchMoreCommits() tea.Cmd {
	if m.git == nil {
		return nil
	}

	m.loadingMoreCommits = true
	skip := m.loadedCommitCount
	limit := m.commitBatchSize
	if limit == 0 {
		limit = DefaultCommitBatchSize
	}
	gitClient := m.git
	logArgs := m.logArgs

	fetchCmd := func() tea.Msg {
		commits, err := gitClient.LogPathsOnlyRange(skip, limit, logArgs...)
		if err != nil {
			return MoreCommitsLoadedMsg{Err: err}
		}

		// Convert git commits to CommitSets (skeleton files, no stats)
		commitSets := convertToCommitSets(commits)
		return MoreCommitsLoadedMsg{Commits: commitSets}
	}

	// Start spinner alongside the fetch
	spinnerCmd := m.startSpinnerIfNeeded()
	if spinnerCmd != nil {
		return tea.Batch(fetchCmd, spinnerCmd)
	}
	return fetchCmd
}

// fetchTotalCommitCount returns a command to get the total commit count.
func (m Model) fetchTotalCommitCount() tea.Cmd {
	if m.git == nil {
		return nil
	}
	gitClient := m.git

	return func() tea.Msg {
		count, err := gitClient.CommitCount()
		if err != nil {
			return TotalCommitCountMsg{Count: -1}
		}
		return TotalCommitCountMsg{Count: count}
	}
}

// appendCommits adds new commits to the model.
// This updates commits, files, commitFileStarts, and invalidates the row cache.
func (m *Model) appendCommits(commits []sidebyside.CommitSet) {
	for _, c := range commits {
		m.commitFileStarts = append(m.commitFileStarts, len(m.files))
		m.commits = append(m.commits, c)
		m.files = append(m.files, c.Files...)
	}
	m.loadedCommitCount = len(m.commits)
	// Invalidate all windows since new commits affect all views
	m.invalidateAllRowCaches()
	m.calculateTotalLines()
}

// convertToCommitSets converts git commits to sidebyside CommitSets.
// Creates skeleton files without stats (stats are loaded separately).
func convertToCommitSets(commits []git.CommitWithPaths) []sidebyside.CommitSet {
	var result []sidebyside.CommitSet
	for _, c := range commits {
		var files []sidebyside.FilePair
		for _, f := range c.Files {
			files = append(files, sidebyside.SkeletonFilePairNoStats(f.Path))
		}

		commitSet := sidebyside.CommitSet{
			Info: sidebyside.CommitInfo{
				SHA:     c.Meta.SHA,
				Author:  c.Meta.Author,
				Email:   c.Meta.Email,
				Date:    c.Meta.Date,
				Subject: c.Meta.Subject,
				Body:    c.Meta.Body,
				Refs:    sidebyside.ParseRefs(c.Meta.Refs),
			},
			Files:       files,
			FoldLevel:   sidebyside.CommitFolded, // Start folded
			FilesLoaded: false,
			StatsLoaded: false,
		}
		result = append(result, commitSet)
	}
	return result
}
