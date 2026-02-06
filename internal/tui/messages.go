package tui

import (
	"time"

	"github.com/user/diffyduck/pkg/sidebyside"
)

// ClearStatusMsg is sent to clear the status message after a delay.
type ClearStatusMsg struct {
	SetTime time.Time // time when the message was set, to avoid clearing newer messages
}

// FileContentLoadedMsg is sent when file content has been fetched for a single file.
type FileContentLoadedMsg struct {
	FileIndex        int
	OldContent       []string
	NewContent       []string
	ContentTruncated bool // true if content was truncated due to limits (legacy, use per-side)
	OldTruncated     bool // true if old content was truncated
	NewTruncated     bool // true if new content was truncated
	Err              error
}

// AllContentLoadedMsg is sent when content for all files has been fetched.
type AllContentLoadedMsg struct {
	Contents []FileContent
}

// FileContent holds the fetched content for a single file.
type FileContent struct {
	FileIndex        int
	OldContent       []string
	NewContent       []string
	ContentTruncated bool // true if content was truncated due to limits (legacy, use per-side)
	OldTruncated     bool // true if old content was truncated
	NewTruncated     bool // true if new content was truncated
	Err              error
}

// HighlightReadyMsg is sent when syntax highlighting spans are ready for a file.
type HighlightReadyMsg struct {
	FileIndex    int
	OldSpans     []HighlightSpan  // spans for old file content
	NewSpans     []HighlightSpan  // spans for new file content
	OldStructure []StructureEntry // structure for old file content (for structural diff)
	NewStructure []StructureEntry // structure for new file content (for breadcrumbs)
}

// HighlightSpan represents a highlighted range with a category.
// This is a copy of highlight.Span to avoid import cycles.
type HighlightSpan struct {
	Start    int // byte offset
	End      int // byte offset (exclusive)
	Category int // highlight.Category value
}

// PairsHighlightReadyMsg is sent when syntax highlighting from Pairs is ready for a file.
type PairsHighlightReadyMsg struct {
	FileIndex     int
	OldSpans      []HighlightSpan // spans for concatenated old lines
	NewSpans      []HighlightSpan // spans for concatenated new lines
	OldLineStarts map[int]int     // line number -> byte offset
	NewLineStarts map[int]int     // line number -> byte offset
	OldLineLens   map[int]int     // line number -> line length
	NewLineLens   map[int]int     // line number -> line length
}

// StructureEntry represents a structural code element for breadcrumbs.
// This mirrors structure.Entry to avoid import in messages.
type StructureEntry struct {
	StartLine  int      // 1-based line number
	EndLine    int      // 1-based line number (inclusive)
	Name       string   // e.g., "MyStruct", "myMethod"
	Kind       string   // e.g., "type", "func", "def", "class"
	Receiver   string   // Go only: e.g., "(m *Model)"
	Params     []string // Function parameters
	ReturnType string   // Return type annotation
}

// CommitStatsLoadedMsg is sent when stats for commits have been fetched asynchronously.
type CommitStatsLoadedMsg struct {
	// Stats maps commit SHA to per-file stats
	Stats map[string]CommitStats
}

// CommitStats holds the stats for a single commit.
type CommitStats struct {
	TotalAdded   int
	TotalRemoved int
	FileStats    []FileStats // per-file stats, indexed same as CommitSet.Files
}

// FileStats holds stats for a single file.
type FileStats struct {
	Added   int
	Removed int
}

// MoreCommitsLoadedMsg is sent when additional commits have been fetched for pagination.
type MoreCommitsLoadedMsg struct {
	Commits []sidebyside.CommitSet // newly loaded commits
	Err     error                  // error if loading failed
}

// TotalCommitCountMsg is sent when the total commit count has been fetched.
type TotalCommitCountMsg struct {
	Count int // total commits in repo, -1 if error
}

// SnapshotCreatedMsg is sent when a snapshot has been created.
type SnapshotCreatedMsg struct {
	SHA     string // commit SHA of the snapshot
	Subject string // commit message (for display)
	Date    string // formatted date (for display)
	Err     error  // error if snapshot creation failed
}

// SnapshotDiffReadyMsg is sent when a snapshot diff has been computed.
type SnapshotDiffReadyMsg struct {
	CommitSet   sidebyside.CommitSet // the diff as a commit set (with "Diff N" subject)
	SnapshotSHA string               // full SHA of the new snapshot (for storing in snapshots list)
	Err         error                // error if diff failed
}
