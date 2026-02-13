package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestInitStartupQueue_ShowModePreloadsFoldedCommit(t *testing.T) {
	// In show mode (fetcher set), files should be preloaded even when the
	// commit is folded with metadata — the user will expand it.
	files := []sidebyside.FilePair{
		{NewPath: "a.go", FoldLevel: sidebyside.FoldHunks},
		{NewPath: "b.go", FoldLevel: sidebyside.FoldHunks},
	}
	commit := sidebyside.CommitSet{
		Info:      sidebyside.CommitInfo{SHA: "abc123", Subject: "feat: something"},
		Files:     files,
		FoldLevel: sidebyside.CommitFolded,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit},
		WithFetcher(content.NewFetcher(nil, content.ModeShow, "abc123", "")),
	)
	defer m.highlighter.Close()

	assert.True(t, m.commits[0].Info.HasMetadata(), "commit should have metadata")
	assert.Equal(t, sidebyside.CommitFolded, m.commitFoldLevel(0), "commit should start folded")

	cmd := m.initStartupQueue()

	assert.NotNil(t, cmd, "should return a command to start preloading")
	assert.True(t, m.startupQueuedInit, "should mark queue as initialized")
}

func TestInitStartupQueue_LogModeSkipsFoldedCommit(t *testing.T) {
	// In log mode (no fetcher, uses git), files in folded commits with
	// metadata should NOT be preloaded — most commits will stay folded.
	files := []sidebyside.FilePair{
		{NewPath: "a.go", FoldLevel: sidebyside.FoldHunks},
	}
	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123", Subject: "feat: something"},
		Files:       files,
		FoldLevel:   sidebyside.CommitFolded,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	defer m.highlighter.Close()

	// No fetcher set — log mode uses git + per-commit fetchers
	assert.Nil(t, m.fetcher, "log mode should not have a persistent fetcher")

	cmd := m.initStartupQueue()

	// No git set either, so even if files were queued, processStartupQueue returns nil.
	// Verify via the queue itself: it should be empty because files were skipped.
	assert.Nil(t, cmd, "should not preload files in folded log-mode commits")
	assert.Empty(t, m.startupQueue, "queue should be empty for folded commit in log mode")
}
