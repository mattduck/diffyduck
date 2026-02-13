package tui

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/diffyduck/pkg/comments"
	"github.com/user/diffyduck/pkg/config"
	"github.com/user/diffyduck/pkg/content"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/git"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/inlinediff"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

// inlineDiffKey identifies a specific line pair for caching inline diffs.
type inlineDiffKey struct {
	fileIndex int
	oldNum    int
	newNum    int
}

// commentKey identifies a line that can have a comment.
// Comments attach to the new/left side line number.
type commentKey struct {
	fileIndex  int
	newLineNum int // line number on new/left side
}

// NarrowScope identifies the node to which the view is narrowed.
// When Active, buildRows only emits rows for this node, and all navigation,
// search, and fold operations are scoped accordingly.
type NarrowScope struct {
	Active         bool
	CommitIdx      int  // which commit (-1 if not applicable, e.g. single-diff mode)
	FileIdx        int  // which file (-1 if scoping to a whole commit)
	HunkIdx        int  // which hunk (-1 if scoping to a whole file or commit)
	CommitInfoOnly bool // if true, narrow to commit info section only (not files)
}

// IncludesCommit returns true if the given commit index is within the narrow scope.
// When inactive or scoped to a commit, this determines visibility.
func (ns NarrowScope) IncludesCommit(commitIdx int) bool {
	if !ns.Active {
		return true
	}
	// If we're scoped to a specific commit (or file within a commit), only that commit is visible
	if ns.CommitIdx >= 0 {
		return commitIdx == ns.CommitIdx
	}
	return true
}

// IncludesFile returns true if the given file index is within the narrow scope.
func (ns NarrowScope) IncludesFile(fileIdx int) bool {
	if !ns.Active {
		return true
	}
	// When narrowed to commit info only, no files are shown
	if ns.CommitInfoOnly {
		return false
	}
	// If we're scoped to a specific file, only that file is visible
	if ns.FileIdx >= 0 {
		return fileIdx == ns.FileIdx
	}
	return true
}

// IsFileLevelOrBelow returns true if narrowed to a specific file (or hunk within a file).
// Used to skip commit-level rows when scoped to file level.
func (ns NarrowScope) IsFileLevelOrBelow() bool {
	return ns.Active && ns.FileIdx >= 0
}

// IsCommitInfoOnly returns true if narrowed to just the commit info section.
// When true, files are not shown, only commit info rows.
func (ns NarrowScope) IsCommitInfoOnly() bool {
	return ns.Active && ns.CommitInfoOnly
}

// inlineDiffResult stores cached inline diff spans for a line pair.
type inlineDiffResult struct {
	oldSpans []inlinediff.Span
	newSpans []inlinediff.Span
}

// VisualSelection tracks visual line selection mode state.
type VisualSelection struct {
	Active    bool // true when in visual mode
	AnchorRow int  // row index where selection started
}

// Window represents a single view into the diff content.
// Multiple windows can exist showing different parts of the same data.
type Window struct {
	// Viewport state
	scroll  int // vertical scroll offset (line index at top of viewport)
	hscroll int // horizontal scroll offset (display columns)

	// Narrow mode - scopes this window to a single node
	narrow NarrowScope

	// Fold state - per-window so each window can have different fold levels
	fileFoldLevels     map[int]sidebyside.FoldLevel       // file index -> fold level
	commitFoldLevels   map[int]sidebyside.CommitFoldLevel // commit index -> fold level
	commitInfoExpanded map[int]bool                       // commit index -> info expanded override

	// Row cache - per-window since it depends on fold/narrow state
	cachedRows     []displayRow // cached result of buildRows()
	rowsCacheValid bool         // true if cachedRows is up to date
	totalLines     int          // total number of displayable lines

	// Search navigation state (per-window, though query is shared)
	searchMatchIdx  int // index of current match within cursor row on current side (0 = first)
	searchMatchSide int // which side the current match is on (0 = new/left, 1 = old/right)

	// Comment editing state (per-window so you can edit in one window while browsing in another)
	commentMode   bool       // true when editing a comment
	commentInput  string     // text being edited
	commentCursor int        // cursor position in commentInput (byte offset)
	commentScroll int        // vertical scroll offset in comment editor
	commentKey    commentKey // identifies which line the comment is attached to

	// Visual selection mode (per-window so each split can have independent selection)
	visualSelection VisualSelection
}

// Model represents the application state.
type Model struct {
	// Data - hierarchical: commits contain files
	commits []sidebyside.CommitSet

	// Flattened files from all commits for unified indexing
	// commitFileStarts[i] is the index in files where commit i's files start
	files            []sidebyside.FilePair
	commitFileStarts []int // start index in files for each commit

	fetcher            *content.Fetcher // for fetching full file contents (lazy)
	git                git.Git          // for creating on-demand fetchers in log mode
	truncatedFileCount int              // number of files omitted due to limit

	// Debug mode
	debugMode bool        // true when --debug flag is passed, shows memory/goroutine stats
	debugLog  *log.Logger // file-based debug logger (nil when not in debug mode)

	// Syntax highlighting
	highlighter         *highlight.Highlighter
	highlightSpans      map[int]*FileHighlight      // file index -> full content highlight spans
	pairsHighlightSpans map[int]*PairsFileHighlight // file index -> pairs-based highlight spans

	// Code structure for breadcrumbs
	structureMaps      map[int]*FileStructure // file index -> full content structure
	pairsStructureMaps map[int]*FileStructure // file index -> pairs-based structure

	// Windows - multiple views into the same data
	windows           []*Window // up to 2 windows
	activeWindowIdx   int       // index of focused window
	windowSplitRatio  float64   // vertical split: left window's share of width (0.2 to 0.8, default 0.5)
	windowSplitRatioH float64   // horizontal split: top window's share of height (0.2 to 0.8, default 0.5)
	windowSplitV      bool      // true = vertical (side-by-side), false = horizontal (stacked)

	// Terminal dimensions (shared across windows)
	width  int // terminal width
	height int // terminal height (viewport height)

	// Configuration
	keys        KeyMap
	hscrollStep int // columns to scroll horizontally per keypress

	// Search state (query is shared across windows, navigation is per-window)
	searchMode    bool   // true when in search input mode
	searchForward bool   // true for forward search (/), false for backward (?)
	searchInput   string // current input being typed
	searchQuery   string // executed search query

	// Multi-key sequence state
	pendingKey string // first key of a multi-key sequence (e.g., "g" waiting for second key)

	// Initial state tracking
	initialFoldSet bool // true once initial fold levels have been determined

	// Derived/cached (shared across windows)
	maxLineNumSeen     int // largest line number seen (for dynamic gutter width, only grows)
	maxLessWidth       int // max width of less indicator (never shrinks to prevent jittering)
	maxNewContentWidth int // display width of new-side content (left side); defaults to 90, updated on 'r' refresh

	// Cached column widths for commit headers - updated on 'r' refresh
	cachedCommitFileCount int // max commit file count width (e.g., "99" = 2)
	cachedCommitAddWidth  int // max commit "+N" width
	cachedCommitRemWidth  int // max commit "-N" width
	cachedCommitTimeWidth int // max relative time width (e.g., "12 months")
	cachedCommitSubjWidth int // max subject width (capped at 120)
	cachedStructDiffWidth int // max structural diff content width

	// Inline diff cache - avoids recomputing Myers diff on every render
	inlineDiffCache map[inlineDiffKey]inlineDiffResult

	// Loading indicator - shows spinner while files are being fetched/parsed
	spinner        spinner.Model     // animated spinner for loading state
	loadingFiles   map[int]time.Time // file index -> time loading started
	spinnerTicking bool              // true if a spinner tick chain is already running

	// Startup loading - prefetch content for all supported files on startup
	startupQueue      []int // file indices waiting to be loaded on startup
	startupInFlight   int   // number of files currently being fetched
	startupQueuedInit bool  // true once startup queue has been initialized

	// Focus state - true when terminal has focus
	focused bool

	// Focus colour mode - dims content outside current hunk to reduce visual clutter
	focusColour bool

	// Clipboard
	clipboard Clipboard // clipboard interface for copy/paste

	// Comment data - shared across windows (the actual stored comments)
	comments            map[commentKey]string // stored comments (text only, for display)
	commentStore        *comments.Store       // git-backed persistent storage
	persistedCommentIDs map[commentKey]string // maps in-memory comment to persisted comment ID
	commentIndex        *comments.Index       // full index loaded once at startup
	loadedCommentIDs    map[string]bool       // tracks fetched comment IDs to avoid re-reads

	// Conflict state
	hasConflicts bool // true when repo is in a merge/rebase/cherry-pick conflict state

	// Status message (echo area)
	statusMessage     string    // message to display in status bar
	statusMessageTime time.Time // when the message was set (for auto-clear)

	// Pagination state (log mode only)
	loadedCommitCount  int      // number of commits currently loaded
	totalCommitCount   int      // total commits in repo (0=unknown, -1=error)
	commitBatchSize    int      // commits per batch (default 100)
	loadingMoreCommits bool     // true when fetching next batch
	logArgs            []string // extra args for git log (ref ranges, pathspecs)

	// Snapshot state (diff mode only)
	autoSnapshots bool     // true when snapshot-taking is enabled
	showSnapshots bool     // true when snapshot view is currently displayed
	snapshots     []string // list of snapshot commit SHAs (index 0 is initial snapshot)
	snapshotCount int      // counter for "Diff N" naming
	allMode       bool     // true if --all flag was used (include untracked files in snapshots)
	baseSHA       string   // SHA of the base ref we're diffing against (for keying snapshot refs)
	branch        string   // current branch name (for scoping snapshot refs)

	// Cached view state for S-key toggle between normal and snapshot views
	normalViewCommits   []sidebyside.CommitSet // base→WT commit(s)
	snapshotViewCommits []sidebyside.CommitSet // snapshot view with R-key additions (nil = not yet built)

	// Help screen
	helpMode   bool     // true when help screen is displayed
	helpScroll int      // scroll offset in help content
	helpLines  []string // pre-rendered help content lines
}

// DefaultHScrollStep is the default number of columns to scroll horizontally.
const DefaultHScrollStep = 4

// DefaultCommitBatchSize is the number of commits to load per batch in log mode.
const DefaultCommitBatchSize = 100

// PaginationScrollThreshold is the number of rows from the end to trigger loading more commits.
const PaginationScrollThreshold = 20

// MaxWindows is the maximum number of windows allowed.
const MaxWindows = 2

// w returns the active window. This is the primary way to access per-window state.
// Use this for modifications (pointer receiver methods).
// Lazily initializes the windows slice if empty (for backwards compatibility with tests).
func (m *Model) w() *Window {
	if len(m.windows) == 0 {
		m.windows = []*Window{newWindow()}
	}
	return m.windows[m.activeWindowIdx]
}

// wv returns the active window for value receiver methods (read-only access).
func (m Model) wv() *Window {
	if len(m.windows) == 0 {
		// Can't modify the slice in a value receiver, so return a temporary window
		// This case should rarely happen in practice
		return newWindow()
	}
	return m.windows[m.activeWindowIdx]
}

// newWindow creates a new Window with default state.
func newWindow() *Window {
	return &Window{
		scroll:             0,
		hscroll:            0,
		narrow:             NarrowScope{},
		fileFoldLevels:     make(map[int]sidebyside.FoldLevel),
		commitFoldLevels:   make(map[int]sidebyside.CommitFoldLevel),
		commitInfoExpanded: make(map[int]bool),
		cachedRows:         nil,
		rowsCacheValid:     false,
		totalLines:         0,
	}
}

// fileFoldLevel returns the fold level for a file in the active window.
// Falls back to the data's default FoldLevel if no window override exists.
func (m *Model) fileFoldLevel(fileIdx int) sidebyside.FoldLevel {
	w := m.w()
	if w.fileFoldLevels == nil {
		w.fileFoldLevels = make(map[int]sidebyside.FoldLevel)
	}
	if level, ok := w.fileFoldLevels[fileIdx]; ok {
		return level
	}
	// Fall back to default from data
	if fileIdx >= 0 && fileIdx < len(m.files) {
		return m.files[fileIdx].FoldLevel
	}
	return sidebyside.FoldHeader
}

// setFileFoldLevel sets the fold level for a file in the active window.
func (m *Model) setFileFoldLevel(fileIdx int, level sidebyside.FoldLevel) {
	w := m.w()
	if w.fileFoldLevels == nil {
		w.fileFoldLevels = make(map[int]sidebyside.FoldLevel)
	}
	w.fileFoldLevels[fileIdx] = level
}

// commitFoldLevel returns the fold level for a commit in the active window.
// Falls back to the data's default FoldLevel if no window override exists.
func (m *Model) commitFoldLevel(commitIdx int) sidebyside.CommitFoldLevel {
	w := m.w()
	if w.commitFoldLevels == nil {
		w.commitFoldLevels = make(map[int]sidebyside.CommitFoldLevel)
	}
	if level, ok := w.commitFoldLevels[commitIdx]; ok {
		return level
	}
	// Fall back to default from data
	if commitIdx >= 0 && commitIdx < len(m.commits) {
		return m.commits[commitIdx].FoldLevel
	}
	return sidebyside.CommitFolded
}

// isCommitInfoExpanded returns whether commit info is expanded for a commit.
// Falls back to the default for the commit's fold level if no override exists.
func (m *Model) isCommitInfoExpanded(commitIdx int) bool {
	w := m.w()
	if w.commitInfoExpanded != nil {
		if expanded, ok := w.commitInfoExpanded[commitIdx]; ok {
			return expanded
		}
	}
	return sidebyside.CommitInfoExpandedAt[m.commitFoldLevel(commitIdx)]
}

// setCommitInfoExpanded sets the commit info expanded state for a commit.
func (m *Model) setCommitInfoExpanded(commitIdx int, expanded bool) {
	w := m.w()
	if w.commitInfoExpanded == nil {
		w.commitInfoExpanded = make(map[int]bool)
	}
	w.commitInfoExpanded[commitIdx] = expanded
}

// setCommitFoldLevel sets the fold level for a commit in the active window.
func (m *Model) setCommitFoldLevel(commitIdx int, level sidebyside.CommitFoldLevel) {
	w := m.w()
	if w.commitFoldLevels == nil {
		w.commitFoldLevels = make(map[int]sidebyside.CommitFoldLevel)
	}
	w.commitFoldLevels[commitIdx] = level
}

// FileHighlight stores syntax highlighting spans for a file's old and new content.
type FileHighlight struct {
	OldSpans []highlight.Span // spans for old content
	NewSpans []highlight.Span // spans for new content
}

// PairsFileHighlight stores syntax highlighting spans derived from diff Pairs.
// Unlike FileHighlight which has spans for full file content, this has spans
// for the concatenated lines from Pairs, with a mapping from line numbers back
// to positions in the concatenated content.
type PairsFileHighlight struct {
	OldSpans      []highlight.Span // spans for concatenated old lines
	NewSpans      []highlight.Span // spans for concatenated new lines
	OldLineStarts map[int]int      // line number -> byte offset in concatenated old content
	NewLineStarts map[int]int      // line number -> byte offset in concatenated new content
	OldLineLens   map[int]int      // line number -> length of line content
	NewLineLens   map[int]int      // line number -> length of line content
}

// FileStructure stores code structure maps for breadcrumbs and structural diff.
// NewStructure is used for breadcrumbs (current version of the file).
// OldStructure is used for structural diff (comparing old vs new).
type FileStructure struct {
	OldStructure   *structure.Map            // structure for old content (for structural diff)
	NewStructure   *structure.Map            // structure for new content (for breadcrumbs)
	StructuralDiff *structure.StructuralDiff // diff between old and new structure
}

// Option is a function that configures a Model.
type Option func(*Model)

// WithFetcher sets the content fetcher for lazy file content loading.
func WithFetcher(f *content.Fetcher) Option {
	return func(m *Model) {
		m.fetcher = f
	}
}

// WithGit sets the git interface for on-demand content fetching in log mode.
// This allows creating per-commit fetchers when expanding files.
func WithGit(g git.Git) Option {
	return func(m *Model) {
		m.git = g
	}
}

// WithConflicts marks the model as being in a merge/rebase conflict state,
// enabling conflict marker highlighting in the view.
func WithConflicts() Option {
	return func(m *Model) {
		m.hasConflicts = true
	}
}

// WithDebugMode enables debug mode, which displays memory and goroutine
// stats in the status bar and writes a debug log to /tmp/dfd-debug.log.
func WithDebugMode() Option {
	return func(m *Model) {
		m.debugMode = true
		f, err := os.OpenFile("/tmp/dfd-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err == nil {
			m.debugLog = log.New(f, "dfd: ", log.Ltime|log.Lmicroseconds)
			m.debugLog.Println("debug logging started")
		}
	}
}

// logf writes to the debug log file if debug mode is enabled. No-op otherwise.
func (m *Model) logf(format string, args ...interface{}) {
	if m.debugLog != nil {
		m.debugLog.Printf(format, args...)
	}
}

// WithPagination configures pagination state for log mode.
// loaded is the number of commits currently loaded, batchSize is commits per batch.
func WithPagination(loaded, batchSize int) Option {
	return func(m *Model) {
		m.loadedCommitCount = loaded
		m.commitBatchSize = batchSize
	}
}

// WithLogArgs sets extra arguments for git log commands (ref ranges, pathspecs).
// These are passed through to LogPathsOnlyRange and LogMetaOnlyRange for pagination.
func WithLogArgs(args []string) Option {
	return func(m *Model) {
		m.logArgs = args
	}
}

// WithClipboard sets the clipboard implementation.
func WithClipboard(c Clipboard) Option {
	return func(m *Model) {
		m.clipboard = c
	}
}

// WithTruncatedFileCount sets the number of files that were truncated.
func WithTruncatedFileCount(count int) Option {
	return func(m *Model) {
		m.truncatedFileCount = count
		// Also update the commit set if present
		if len(m.commits) > 0 {
			m.commits[0].TruncatedFileCount = count
		}
	}
}

// WithAllMode sets whether --all flag was used.
// This affects snapshot creation (include untracked files).
func WithAllMode(allMode bool) Option {
	return func(m *Model) {
		m.allMode = allMode
	}
}

// WithAutoSnapshots enables or disables automatic snapshot-taking.
// Snapshots are only available when the diff involves the working tree.
func WithAutoSnapshots(enabled bool) Option {
	return func(m *Model) {
		m.autoSnapshots = enabled
	}
}

// WithShowSnapshots sets the initial snapshot view state.
// When true, the TUI starts in snapshot view showing the snapshot timeline.
func WithShowSnapshots(enabled bool) Option {
	return func(m *Model) {
		m.showSnapshots = enabled
	}
}

// WithSnapshotViewCommits provides pre-built snapshot timeline commits.
// Used at startup when --snapshots flag or show_snapshots config is set.
func WithSnapshotViewCommits(commits []sidebyside.CommitSet) Option {
	return func(m *Model) {
		m.snapshotViewCommits = commits
	}
}

// WithBaseSHA sets the SHA of the base ref we're diffing against.
// This is used as part of the key for persisting snapshot refs.
func WithBaseSHA(sha string) Option {
	return func(m *Model) {
		m.baseSHA = sha
	}
}

// WithBranch sets the current branch name for scoping snapshot refs.
func WithBranch(branch string) Option {
	return func(m *Model) {
		m.branch = branch
	}
}

// WithPersistedSnapshots sets the initial snapshots loaded from persistence.
// This is used with continue mode to resume from a previous session.
func WithPersistedSnapshots(snapshots []string) Option {
	return func(m *Model) {
		m.snapshots = snapshots
		m.snapshotCount = len(snapshots)
	}
}

// WithCommentStore sets the git-backed comment store for persistence.
func WithCommentStore(store *comments.Store) Option {
	return func(m *Model) {
		m.commentStore = store
	}
}

// WithConfig applies user configuration. It should be placed before other
// Options so that CLI flags (applied via later Options) take precedence.
func WithConfig(cfg config.Config) Option {
	return func(m *Model) {
		m.keys = ApplyKeysConfig(cfg.Keys)
		if err := ValidateBindings(m.keys); err != nil {
			m.statusMessage = "config: " + err.Error()
			m.statusMessageTime = time.Now()
		}
		ApplyTheme(cfg.Theme)
		if cfg.Theme.Syntax != nil {
			m.highlighter = highlight.NewWithTheme(buildHighlightTheme(cfg.Theme.Syntax))
		}
		if cfg.Features.HScrollStep != nil {
			m.hscrollStep = *cfg.Features.HScrollStep
		}
		if cfg.Features.CommitBatchSize != nil {
			m.commitBatchSize = *cfg.Features.CommitBatchSize
		}
	}
}

// New creates a new Model with the given file pairs.
// This wraps files in a single CommitSet for backward compatibility.
func New(files []sidebyside.FilePair, opts ...Option) Model {
	// Wrap files in a CommitSet
	commit := sidebyside.CommitSet{
		Files:       files,
		FoldLevel:   sidebyside.CommitFileHeaders, // Start with files visible
		FilesLoaded: true,                         // Files are already provided
	}
	return NewWithCommits([]sidebyside.CommitSet{commit}, opts...)
}

// NewWithCommits creates a new Model with the given commit sets.
// Use this for log view or when commit metadata is available.
func NewWithCommits(commits []sidebyside.CommitSet, opts ...Option) Model {
	// Initialize spinner with compact style and slower speed
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Spinner.FPS = time.Second / 6 // 6 fps

	m := Model{
		commits:             commits,
		windows:             []*Window{newWindow()}, // start with one window
		activeWindowIdx:     0,
		windowSplitRatio:    0.5,  // default 50/50 vertical split
		windowSplitRatioH:   0.5,  // default 50/50 horizontal split
		windowSplitV:        true, // default to vertical split when created
		keys:                DefaultKeyMap(),
		hscrollStep:         DefaultHScrollStep,
		highlighter:         highlight.New(),
		highlightSpans:      make(map[int]*FileHighlight),
		pairsHighlightSpans: make(map[int]*PairsFileHighlight),
		structureMaps:       make(map[int]*FileStructure),
		pairsStructureMaps:  make(map[int]*FileStructure),
		inlineDiffCache:     make(map[inlineDiffKey]inlineDiffResult),
		spinner:             s,
		loadingFiles:        make(map[int]time.Time),
		focused:             true,
		focusColour:         false,
		clipboard:           &SystemClipboard{},
		comments:            make(map[commentKey]string),
		persistedCommentIDs: make(map[commentKey]string),
		loadedCommentIDs:    make(map[string]bool),
		maxNewContentWidth:  90,   // sensible default; recalculated on 'r' refresh
		maxLineNumSeen:      9999, // default gives 4-digit gutter; recalculated on 'r' refresh
		// Column width defaults - recalculated on 'r' refresh
		cachedCommitFileCount: 2,   // "99" files
		cachedCommitAddWidth:  5,   // "+9999"
		cachedCommitRemWidth:  5,   // "-9999"
		cachedCommitTimeWidth: 3,   // "16h"
		cachedCommitSubjWidth: 100, // reasonable subject length
		cachedStructDiffWidth: 0,   // structural diff width; 0 until 'r' refresh
	}

	// Flatten files from all commits and track boundaries
	m.commitFileStarts = make([]int, len(commits))
	for i, c := range commits {
		m.commitFileStarts[i] = len(m.files)
		m.files = append(m.files, c.Files...)
		m.truncatedFileCount += c.TruncatedFileCount
	}

	for _, opt := range opts {
		opt(&m)
	}

	// If starting in snapshot view, cache the normal commits and swap to snapshot view.
	// This must happen here (not Init) because Init has a value receiver.
	if m.showSnapshots && m.snapshotViewCommits != nil {
		m.normalViewCommits = make([]sidebyside.CommitSet, len(m.commits))
		copy(m.normalViewCommits, m.commits)
		m.commits = m.snapshotViewCommits
		m.rebuildFilesFromCommits()
	}

	// Load comment index early (before Init, which has a value receiver).
	// The index is a pointer field, so it must be set here on the real model.
	m.loadCommentIndex()

	m.calculateTotalLines()

	// Synchronously highlight the first file so initial render has highlighting.
	// Skip if in log mode with folded commits (first file won't be visible anyway).
	// For no-metadata commits (diff view), files are always visible even when
	// CommitFolded, so we still need to highlight.
	// The rest will be highlighted async in Init().
	firstFileVisible := len(m.files) > 0 &&
		(len(m.commits) == 0 || m.commitFoldLevel(0) != sidebyside.CommitFolded ||
			!m.commits[0].Info.HasMetadata())
	if firstFileVisible {
		m.highlightPairsSync(0)
	}

	return m
}

// estimateNormalRows calculates how many rows would be displayed if all files
// were at FoldStructure level. Used to determine initial fold state.
func (m Model) estimateNormalRows() int {
	total := 0
	for i, fp := range m.files {
		// Header structure: top border (first file only) + header + spacer + trailing border
		if i == 0 {
			total++ // top border for first file
		}
		total += 3 // header + spacer + trailing border

		// Pairs plus hunk separators
		total += len(fp.Pairs)
		total += m.countHunkSeparators(fp)

		// 4 blank lines after content
		total += 4
	}
	// Summary row
	if len(m.files) > 0 {
		total++
	}
	return total
}

// countHunkSeparators counts the number of hunk boundaries in a file's pairs.
func (m Model) countHunkSeparators(fp sidebyside.FilePair) int {
	count := 0
	var prevOld, prevNew int
	for i, pair := range fp.Pairs {
		if i > 0 {
			// Check for gap in line numbers (hunk boundary)
			oldGap := prevOld > 0 && pair.Old.Num > 0 && pair.Old.Num > prevOld+1
			newGap := prevNew > 0 && pair.New.Num > 0 && pair.New.Num > prevNew+1
			if oldGap || newGap {
				count++
			}
		}
		if pair.Old.Num > 0 {
			prevOld = pair.Old.Num
		}
		if pair.New.Num > 0 {
			prevNew = pair.New.Num
		}
	}
	return count
}

// calculateTotalLines counts total lines including file headers and hunk separators.
// Also updates maxLineNumSeen based on visible line numbers and maxLessWidth for status bar.
func (m *Model) calculateTotalLines() {
	// Rebuild the rows cache, which also updates totalLines, maxLineNumSeen, and maxLessWidth
	m.rebuildRowsCache()
}

// updateMaxLessWidth updates maxLessWidth if the current totalLines would require more width.
// This is called from calculateTotalLines to ensure the less indicator never shrinks.
func (m *Model) updateMaxLessWidth() {
	if m.w().totalLines == 0 {
		return
	}
	// Calculate width for worst case: "line TOTAL/TOTAL (END)"
	maxIndicator := formatLessIndicator(m.w().totalLines, m.w().totalLines, 100, true)
	width := displayWidth(maxIndicator)
	if width > m.maxLessWidth {
		m.maxLessWidth = width
	}
}

// Init implements tea.Model.
// Triggers async highlighting for all files except the first (which is highlighted sync in New).
// Also triggers async stats loading if any commits don't have stats loaded.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Match comments for any files that already have content (single-commit mode).
	// In log mode, files are skeletons here — comments match later in loadCommitDiff.
	// Note: commentIndex is loaded in NewWithCommits (Init has a value receiver,
	// so pointer field assignments here would be lost).
	m.matchCommentsForFiles(0, len(m.files))

	// Fetch total commit count for pagination (if pagination is enabled)
	if m.git != nil && m.loadedCommitCount > 0 {
		cmds = append(cmds, m.fetchTotalCommitCount())
	}

	// Check if we need to load stats asynchronously
	if m.needsStatsLoad() {
		cmds = append(cmds, m.fetchCommitStats())
	}

	// Cache the normal view commits (base→WT diff) for toggle
	// Take initial snapshot for diff mode if enabled
	if m.autoSnapshots && m.git != nil {
		cmds = append(cmds, m.createSnapshot())
	}

	// Highlight remaining files async
	if len(m.files) > 1 {
		cmds = append(cmds, m.RequestHighlightFromPairsExcept(map[int]bool{0: true}))
	}

	return tea.Batch(cmds...)
}

// createSnapshot returns a command that creates a snapshot of the working tree.
// Parent is the last snapshot if any exist, otherwise baseSHA.
func (m Model) createSnapshot() tea.Cmd {
	if m.git == nil {
		return nil
	}

	gitClient := m.git
	allMode := m.allMode
	baseSHA := m.baseSHA

	// Parent is last snapshot if any exist, otherwise baseSHA
	parentSHA := baseSHA
	if len(m.snapshots) > 0 {
		parentSHA = m.snapshots[len(m.snapshots)-1]
	}

	// Format commit message: "dfd: <sha> @ <datetime>"
	baseShort := baseSHA
	if len(baseShort) > 7 {
		baseShort = baseShort[:7]
	}
	dateStr := time.Now().Format("Jan 2 15:04")
	message := fmt.Sprintf("dfd: %s @ %s", baseShort, dateStr)

	return func() tea.Msg {
		sha, err := gitClient.CreateSnapshot(allMode, parentSHA, message)
		return SnapshotCreatedMsg{SHA: sha, Subject: message, Date: dateStr, Err: err}
	}
}

// needsStatsLoad returns true if any commit needs stats to be loaded.
func (m Model) needsStatsLoad() bool {
	for _, commit := range m.commits {
		if commit.Info.HasMetadata() && !commit.StatsLoaded {
			return true
		}
	}
	return false
}

// fetchCommitStats returns a command that fetches all remaining commit stats asynchronously.
// The first page of stats is loaded synchronously in main.go, this handles the rest.
func (m Model) fetchCommitStats() tea.Cmd {
	if m.git == nil {
		return nil
	}

	// Find the first commit that needs stats
	startOffset := 0
	for i, commit := range m.commits {
		if !commit.StatsLoaded {
			startOffset = i
			break
		}
	}

	if startOffset >= len(m.commits) {
		return nil // All stats already loaded
	}

	// Capture git interface for closure
	gitClient := m.git
	logArgs := m.logArgs
	totalCommits := len(m.commits)

	return func() tea.Msg {
		// Fetch stats for remaining commits
		remaining := totalCommits - startOffset
		commits, err := gitClient.LogMetaOnlyRange(startOffset, remaining, logArgs...)
		if err != nil {
			return CommitStatsLoadedMsg{Stats: nil}
		}

		// Build stats map keyed by SHA
		stats := make(map[string]CommitStats)
		for _, c := range commits {
			totalAdded := 0
			totalRemoved := 0
			var fileStats []FileStats
			for _, f := range c.Files {
				added := f.Added
				removed := f.Removed
				if added < 0 {
					added = 0
				}
				if removed < 0 {
					removed = 0
				}
				totalAdded += added
				totalRemoved += removed
				fileStats = append(fileStats, FileStats{
					Added:   f.Added,
					Removed: f.Removed,
				})
			}
			stats[c.Meta.SHA] = CommitStats{
				TotalAdded:   totalAdded,
				TotalRemoved: totalRemoved,
				FileStats:    fileStats,
			}
		}

		return CommitStatsLoadedMsg{Stats: stats}
	}
}

// commentMaxVisibleLines returns the maximum number of input lines to show.
// This is max(10, 20% of viewport height).
func (m Model) commentMaxVisibleLines() int {
	twentyPercent := m.height * 20 / 100
	if twentyPercent < 10 {
		return 10
	}
	return twentyPercent
}

// commentPromptHeight returns the number of lines needed for the comment prompt.
// Returns 1 when not in comment mode (for normal status bar).
func (m Model) commentPromptHeight() int {
	if !m.w().commentMode {
		return 1
	}
	// Count visual (wrapped) lines
	totalLines := m.commentVisualLineCount()
	maxVisible := m.commentMaxVisibleLines()

	// Calculate visible content lines
	visibleLines := totalLines
	if visibleLines > maxVisible {
		visibleLines = maxVisible
	}

	// Add scroll indicators if content exceeds visible area
	extraLines := 0
	if totalLines > maxVisible {
		// Check if there's content above (scroll > 0)
		if m.w().commentScroll > 0 {
			extraLines++
		}
		// Check if there's content below
		if m.w().commentScroll+maxVisible < totalLines {
			extraLines++
		}
	}

	// Add 1 for the help line at the bottom
	return visibleLines + extraLines + 1
}

// baseContentHeight returns the height available for content without the comment prompt.
// This is used for cursor calculations to keep them stable when comment mode is active.
func (m Model) baseContentHeight() int {
	reserved := 5 // top bar (3 content lines + divider) + bottom bar
	h := m.height - reserved
	if h < 1 {
		return 1
	}
	return h
}

// contentHeight returns the height available for rendering content.
// This accounts for the comment prompt when in comment mode.
func (m Model) contentHeight() int {
	h := m.baseContentHeight()
	// Subtract space for comment prompt when in comment mode
	// (bottom bar is already counted in baseContentHeight)
	if m.w().commentMode {
		h -= m.commentPromptHeight() - 1 // -1 since bottom bar already counted
	}
	if h < 1 {
		return 1
	}
	return h
}

// cursorOffset returns the preferred offset from the top of the viewport where the cursor sits.
// This is 20% of the base content height (ignores comment prompt to keep cursor stable).
// Note: The actual cursor position may be higher when near the top of content - see cursorViewportRow().
func (m Model) cursorOffset() int {
	return m.baseContentHeight() * 20 / 100
}

// cursorViewportRow returns the actual row in the viewport where the cursor appears.
// Near the top of content, the cursor moves up to keep content at the top of screen.
// Once scroll reaches cursorOffset, the cursor stays at the fixed 20% position.
func (m Model) cursorViewportRow() int {
	offset := m.cursorOffset()
	if m.w().scroll < offset {
		return m.w().scroll
	}
	return offset
}

// contentStartLine returns which line of content appears at the top of the viewport.
// Near the top, content starts at line 0. Once scroll exceeds cursorOffset,
// content scrolls up and this returns a positive value.
func (m Model) contentStartLine() int {
	offset := m.cursorOffset()
	if m.w().scroll <= offset {
		return 0
	}
	return m.w().scroll - offset
}

// cursorLine returns the display row index that the cursor points to.
// In the new cursor model, scroll directly represents the cursor line.
func (m Model) cursorLine() int {
	return m.w().scroll
}

// minScroll returns the minimum valid scroll offset.
// The cursor can reach the first content row (row 0, the header).
func (m Model) minScroll() int {
	return 0
}

// maxScroll returns the maximum valid scroll offset.
// This allows the cursor to reach the last line of content.
func (m Model) maxScroll() int {
	if m.w().totalLines == 0 {
		return 0
	}
	return m.w().totalLines - 1
}

// clampScroll ensures scroll is within valid bounds.
func (m *Model) clampScroll() {
	if min := m.minScroll(); m.w().scroll < min {
		m.w().scroll = min
	}
	if max := m.maxScroll(); m.w().scroll > max {
		m.w().scroll = max
	}
}

// StatusInfo contains information for the status bar.
type StatusInfo struct {
	CurrentFile       int                  // 1-based index of current file
	TotalFiles        int                  // total number of files
	FileName          string               // name of current file
	CurrentLine       int                  // 1-based line position in viewport
	TotalLines        int                  // total lines in diff
	Percentage        int                  // 0-100 percentage through diff
	AtEnd             bool                 // true if scrolled to the end
	FoldLevel         sidebyside.FoldLevel // fold level of current file
	FileStatus        string               // file status (added, deleted, renamed, modified)
	Added             int                  // number of added lines in current file
	Removed           int                  // number of removed lines in current file
	Breadcrumbs       string               // code structure breadcrumb (e.g., "type MyStruct > func myMethod")
	BreadcrumbEntries []structure.Entry    // structure entries for styled breadcrumb rendering
}

// StatusInfo computes information for the status bar based on cursor position.
func (m Model) StatusInfo() StatusInfo {
	info := StatusInfo{
		TotalFiles: len(m.files),
		TotalLines: m.w().totalLines,
	}

	if m.w().totalLines == 0 || len(m.files) == 0 {
		return info
	}

	// Use cursor position (not scroll) to determine current file
	cursorPos := m.cursorLine()

	// Calculate current line (1-based, cursor position)
	info.CurrentLine = cursorPos + 1

	// Calculate percentage based on cursor position through the content
	if m.w().totalLines <= 1 {
		info.Percentage = 100
		info.AtEnd = true
	} else {
		maxCursor := m.w().totalLines - 1
		if cursorPos >= maxCursor {
			info.Percentage = 100
			info.AtEnd = true
		} else if cursorPos <= 0 {
			info.Percentage = 0
		} else {
			info.Percentage = (cursorPos * 100) / maxCursor
		}
	}

	// Find which file contains the cursor position
	fileIdx := m.currentFileIndex()
	if fileIdx >= 0 && fileIdx < len(m.files) {
		fp := m.files[fileIdx]

		// Calculate per-commit file number and total
		if len(m.commits) > 0 && len(m.commitFileStarts) > 0 {
			commitIdx := m.commitForFile(fileIdx)
			startIdx := m.commitFileStarts[commitIdx]
			endIdx := len(m.files)
			if commitIdx+1 < len(m.commits) {
				endIdx = m.commitFileStarts[commitIdx+1]
			}
			info.CurrentFile = fileIdx - startIdx + 1
			info.TotalFiles = endIdx - startIdx
		} else {
			// Legacy mode: use global file index
			info.CurrentFile = fileIdx + 1
		}

		info.FileName = formatFilePath(fp.OldPath, fp.NewPath)
		info.FoldLevel = m.fileFoldLevel(fileIdx)
		info.FileStatus = string(fileStatusFromPair(fp))
		info.Added, info.Removed = countFileStats(fp)

		// Get breadcrumbs for current source line
		info.BreadcrumbEntries = m.getEntriesForCursor(fileIdx, cursorPos)
		info.Breadcrumbs = formatBreadcrumbs(info.BreadcrumbEntries, 0)
	}
	// Summary row: leave file-specific fields at zero values (no file info shown)

	return info
}

// getEntriesForCursor returns structure entries for the cursor position.
// fileIdx is 0-based, cursorPos is the display row index.
func (m Model) getEntriesForCursor(fileIdx int, cursorPos int) []structure.Entry {
	// Get the display row at cursor position (use cache if valid)
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
		rows = m.buildRows()
	}
	if cursorPos < 0 || cursorPos >= len(rows) {
		return nil
	}

	row := rows[cursorPos]

	// Only show breadcrumbs for content rows and certain separator rows
	// (not headers, separator tops, structural diff rows, etc.)
	if row.isHeader || row.isSeparatorTop ||
		row.isBlank || row.isHeaderSpacer || row.isHeaderTopBorder ||
		row.isStructuralDiff {
		return nil
	}

	// For separator and separator bottom rows, use the chunk's start line for breadcrumbs
	// This shows the breadcrumb when cursor is on or below the breadcrumb line in the separator
	if row.isSeparator || row.isSeparatorBottom {
		if row.chunkStartLine <= 0 {
			return nil
		}
		return m.getStructureAtLine(row.fileIndex, row.chunkStartLine)
	}

	// For comment rows, use the line number the comment belongs to
	if row.kind == RowKindComment {
		if row.commentLineNum <= 0 {
			return nil
		}
		return m.getStructureAtLine(row.fileIndex, row.commentLineNum)
	}

	// Get source line number from the new side only
	// Don't show breadcrumbs for deleted lines to avoid confusion
	if row.pair.New.Num <= 0 {
		return nil
	}
	sourceLine := row.pair.New.Num

	// Look up structure for this file (new side only)
	return m.getStructureAtLine(fileIdx, sourceLine)
}

// getBreadcrumbsForCursor returns formatted breadcrumbs for the cursor position.
// fileIdx is 0-based, cursorPos is the display row index.
func (m Model) getBreadcrumbsForCursor(fileIdx int, cursorPos int) string {
	entries := m.getEntriesForCursor(fileIdx, cursorPos)
	if len(entries) == 0 {
		return ""
	}
	// Use 0 for compact default (status bar will truncate as needed)
	return formatBreadcrumbs(entries, 0)
}

// getStructureAtLine returns structure entries containing the given line.
// Only works when full file content has been loaded (expanded view).
// Returns nil for files that only have pairs-based content.
func (m Model) getStructureAtLine(fileIdx int, lineNum int) []structure.Entry {
	// Only full-content structure is available - pairs-based structure can't be used
	// because line numbers would be relative to concatenated hunk content, not source.
	fs, ok := m.structureMaps[fileIdx]
	if !ok || fs == nil || fs.NewStructure == nil {
		return nil
	}

	return fs.NewStructure.AtLine(lineNum)
}

// formatBreadcrumbs formats structure entries as a breadcrumb string.
// Entries are expected to be ordered from outermost to innermost.
// maxWidth controls signature truncation (0 = use compact default).
// Output format: "type MyStruct > func (m Model) myMethod(ctx) -> error"
func formatBreadcrumbs(entries []structure.Entry, maxWidth int) string {
	if len(entries) == 0 {
		return ""
	}

	// Calculate width budget for each entry's signature
	// Reserve space for kind prefix and separators
	separatorWidth := 3 // " > "
	totalSeparators := len(entries) - 1
	reservedWidth := totalSeparators * separatorWidth

	// Rough estimate: divide remaining width among entries
	// (In practice, outer entries like "class Foo" are short, inner ones need more space)
	sigWidth := 0
	if maxWidth > 0 && len(entries) > 0 {
		// Give most of the budget to the innermost entry (last one)
		sigWidth = maxWidth - reservedWidth
		for _, e := range entries[:len(entries)-1] {
			// Estimate width for outer entries (kind + name + some buffer)
			outerWidth := len(e.Kind) + 1 + len(e.Name) + 5
			sigWidth -= outerWidth
		}
		if sigWidth < 20 {
			sigWidth = 20 // minimum reasonable width
		}
	}

	var parts []string
	for i, e := range entries {
		var part string
		// Use full width budget for innermost entry, compact for others
		entryWidth := 0
		if i == len(entries)-1 {
			// Subtract the "kind " prefix from available width
			kindPrefixLen := len(e.Kind) + 1
			entryWidth = sigWidth - kindPrefixLen
			if entryWidth < 0 {
				entryWidth = 0
			}
		}
		sig := e.FormatSignature(entryWidth)
		if sig != "" {
			part = e.Kind + " " + sig
		} else {
			part = e.Kind + " " + e.Name
		}
		parts = append(parts, part)
	}

	// Join with separator
	result := ""
	for i, part := range parts {
		if i > 0 {
			result += " > "
		}
		result += part
	}
	return result
}

// fileAtLine returns the file index (1-based) and filename at the given display line.
// Blank separator lines between files belong to the file above them.
func (m Model) fileAtLine(line int) (int, string) {
	if len(m.files) == 0 {
		return 0, ""
	}

	// Clamp line to valid range
	if line < 0 {
		line = 0
	}
	if line >= m.w().totalLines {
		// Past all content - return last file
		lastFile := m.files[len(m.files)-1]
		return len(m.files), formatFilePath(lastFile.OldPath, lastFile.NewPath)
	}

	// Use cached rows if valid, otherwise rebuild
	rows := m.w().cachedRows
	if !m.w().rowsCacheValid {
		rows = m.buildRows()
	}
	if line >= len(rows) {
		// Shouldn't happen, but handle gracefully
		lastFile := m.files[len(m.files)-1]
		return len(m.files), formatFilePath(lastFile.OldPath, lastFile.NewPath)
	}

	row := rows[line]
	// Commit header or summary row has fileIndex = -1
	// Return 0 to indicate no specific file is selected
	if row.fileIndex < 0 {
		return 0, ""
	}
	// Top border rows have fileIndex set to the next file, but for status
	// bar purposes we don't want to switch to showing that file until the
	// cursor is actually on the header itself. Show the previous file instead.
	if row.isHeaderTopBorder {
		prevIdx := row.fileIndex - 1
		if prevIdx >= 0 && prevIdx < len(m.files) {
			return prevIdx + 1, formatFilePath(
				m.files[prevIdx].OldPath,
				m.files[prevIdx].NewPath,
			)
		}
		return 0, ""
	}
	return row.fileIndex + 1, formatFilePath(
		m.files[row.fileIndex].OldPath,
		m.files[row.fileIndex].NewPath,
	)
}

// formatFilePath returns a display-friendly file path.
func formatFilePath(oldPath, newPath string) string {
	// Prefer new path unless it's /dev/null (deleted file)
	var path string
	if newPath == "/dev/null" {
		path = oldPath
	} else {
		path = newPath
	}
	// Strip common prefixes
	if len(path) > 2 && (path[:2] == "a/" || path[:2] == "b/") {
		path = path[2:]
	}
	return path
}

// updateMaxLineNum updates maxLineNumSeen if n is larger (never shrinks).
func (m *Model) updateMaxLineNum(n int) {
	if n > m.maxLineNumSeen {
		m.maxLineNumSeen = n
	}
}

// updateMaxLineNumSeen scans all file pairs to find the largest line number.
// Called on 'r' refresh to adjust gutter width for large files.
func (m *Model) updateMaxLineNumSeen() {
	for _, fp := range m.files {
		for _, pair := range fp.Pairs {
			m.updateMaxLineNum(pair.Old.Num)
			m.updateMaxLineNum(pair.New.Num)
		}
	}
}

// commentContentWidth returns the text width available inside a comment box.
// This mirrors the layout math in renderCommentRow and getVisibleRows.
func (m *Model) commentContentWidth() int {
	lineNumW := m.lineNumWidth()
	gutterWidth := 2 + lineNumW // arrow(1) + space(1) + lineNum

	// Compute leftHalfWidth the same way getVisibleRows does.
	gutterOverhead := 1 + 1 + lineNumW + 1 + 4
	targetLeftContent := 90
	if m.maxNewContentWidth < targetLeftContent {
		targetLeftContent = m.maxNewContentWidth
	}
	defaultHalf := (m.width - 3) / 2
	leftContentAt50 := defaultHalf - gutterOverhead
	minRightWidth := 1 + 1 + lineNumW + 1 + 2

	var leftHalfWidth int
	if leftContentAt50 >= targetLeftContent {
		leftHalfWidth = defaultHalf
	} else {
		targetLeftWidth := gutterOverhead + targetLeftContent
		maxLeftWidth := m.width - 3 - minRightWidth
		leftHalfWidth = targetLeftWidth
		if leftHalfWidth > maxLeftWidth {
			leftHalfWidth = maxLeftWidth
		}
	}

	boxWidth := leftHalfWidth - gutterWidth
	if boxWidth < 6 {
		boxWidth = 6
	}
	contentWidth := boxWidth - 4 // │ + space + space + │
	if contentWidth < 1 {
		contentWidth = 1
	}
	return contentWidth
}

// lineNumWidth returns the width needed for line numbers based on the largest seen.
// Minimum width is 4 to handle typical files up to 9999 lines.
func (m *Model) lineNumWidth() int {
	// Fall back to scanning if not initialized (e.g., in tests)
	if m.maxLineNumSeen == 0 {
		m.updateMaxLineNumSeen()
	}
	width := 4 // minimum
	n := m.maxLineNumSeen
	for n >= 10000 {
		width++
		n /= 10
	}
	return width
}

// updateMaxNewContentWidth scans visible content to update maxNewContentWidth.
// Only grows, never shrinks (prevents jitter when divider position changes).
// Measures new-side content (displayed on the left).
func (m *Model) updateMaxNewContentWidth() {
	for fileIdx, fp := range m.files {
		if m.fileFoldLevel(fileIdx) != sidebyside.FoldHunks {
			continue // only measure content width for hunk view (FoldHunks)
		}

		// In full-file view, measure full content lines instead of pairs
		if fp.ShowFullFile && fp.HasContent() {
			for _, line := range fp.NewContent {
				if line != "" {
					w := displayWidth(expandTabs(line))
					if w > m.maxNewContentWidth {
						m.maxNewContentWidth = w
					}
				}
			}
			continue
		}

		for _, pair := range fp.Pairs {
			if pair.New.Content != "" {
				w := displayWidth(expandTabs(pair.New.Content))
				if w > m.maxNewContentWidth {
					m.maxNewContentWidth = w
				}
			}
		}
	}
}

// updateColumnWidths recalculates cached column widths for file and commit headers.
// Called on 'r' refresh to align columns based on actual content.
func (m *Model) updateColumnWidths() {
	// Reset to allow shrinking on manual refresh
	m.cachedCommitFileCount = 0
	m.cachedCommitAddWidth = 0
	m.cachedCommitRemWidth = 0
	m.cachedCommitTimeWidth = 0
	m.cachedCommitSubjWidth = 0

	// Calculate commit header widths
	for commitIdx := range m.commits {
		commit := m.commits[commitIdx]
		startIdx := m.commitFileStarts[commitIdx]
		endIdx := len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
		commitFileCount := endIdx - startIdx

		// Calculate stats column widths (matching renderCommitHeaderRow logic)
		var commitAdded, commitRemoved int
		var statsKnown bool
		if commit.StatsLoaded {
			commitAdded = commit.TotalAdded
			commitRemoved = commit.TotalRemoved
			statsKnown = true
		} else {
			// Compute from files (same as render code)
			for i := startIdx; i < endIdx; i++ {
				added, removed := countFileStats(m.files[i])
				commitAdded += added
				commitRemoved += removed
			}
			statsKnown = commitAdded > 0 || commitRemoved > 0 || commitFileCount == 0
		}

		var aw, rw int
		if statsKnown {
			aw = len(fmt.Sprintf("+%d", commitAdded))
			rw = len(fmt.Sprintf("-%d", commitRemoved))
		} else {
			// Stats not loaded yet, use placeholder width ("+?" = 2 chars)
			aw = 2
			rw = 2
		}

		fw := len(fmt.Sprintf("%d", commitFileCount))
		if fw > m.cachedCommitFileCount {
			m.cachedCommitFileCount = fw
		}
		if aw > m.cachedCommitAddWidth {
			m.cachedCommitAddWidth = aw
		}
		if rw > m.cachedCommitRemWidth {
			m.cachedCommitRemWidth = rw
		}
		tw := len(formatShortRelativeDate(commit.Info.Date))
		if tw > m.cachedCommitTimeWidth {
			m.cachedCommitTimeWidth = tw
		}
		sw := displayWidth(commit.Info.Subject) + commit.Info.RefsDisplayWidth()
		if sw > 120 {
			sw = 120
		}
		if sw > m.cachedCommitSubjWidth {
			m.cachedCommitSubjWidth = sw
		}
	}
}

// updateStructuralDiffWidth scans all files to find the max structural diff content width.
// Called on 'r' refresh to expand header box if structural diffs are wider than filenames.
func (m *Model) updateStructuralDiffWidth() {
	m.cachedStructDiffWidth = 0
	for fileIdx := range m.files {
		w := m.structuralDiffMaxContentWidth(fileIdx)
		if w > m.cachedStructDiffWidth {
			m.cachedStructDiffWidth = w
		}
	}
}

// RefreshLayout recalculates all dynamic layout metrics based on actual content.
// Called by 'r' key and useful in tests to get properly aligned output.
func (m *Model) RefreshLayout() {
	m.updateMaxLineNumSeen()
	m.updateMaxNewContentWidth()
	m.updateColumnWidths()
	m.updateStructuralDiffWidth()
	m.w().rowsCacheValid = false
}

// rebuildRowsCache unconditionally rebuilds the cached rows.
// This also updates totalLines. Width metrics are updated on 'r' refresh.
func (m *Model) rebuildRowsCache() {
	// NOTE: maxLineNumSeen, maxNewContentWidth, and column widths are NOT updated
	// automatically here for performance. Press 'r' to refresh layout.

	m.w().cachedRows = m.buildRows()
	m.w().rowsCacheValid = true
	m.w().totalLines = len(m.w().cachedRows)
	m.updateMaxLessWidth()
}

// invalidateAllRowCaches marks all windows' row caches as invalid.
// Use this when display data changes but row structure stays the same
// (e.g., stats loaded, highlighting applied).
func (m *Model) invalidateAllRowCaches() {
	for _, w := range m.windows {
		w.rowsCacheValid = false
	}
}

// rebuildAllRowCachesPreservingCursor rebuilds row caches for ALL windows,
// preserving each window's cursor position on the same logical row.
// Use this when shared state changes (e.g., comments added, content loaded).
func (m *Model) rebuildAllRowCachesPreservingCursor() {
	if len(m.windows) == 0 {
		return
	}

	savedActiveIdx := m.activeWindowIdx

	// Phase 1: Capture cursor identity for each window before invalidating caches
	identities := make([]cursorRowIdentity, len(m.windows))
	for i := range m.windows {
		m.activeWindowIdx = i
		identities[i] = m.getCursorRowIdentity()
	}

	// Phase 2: Invalidate all caches
	for _, w := range m.windows {
		w.rowsCacheValid = false
	}

	// Phase 3: Rebuild each window and restore cursor position
	for i := range m.windows {
		m.activeWindowIdx = i
		m.rebuildRowsCache()
		newRowIdx := m.findRowOrNearestAbove(identities[i])
		m.adjustScrollToRow(newRowIdx)
	}

	m.activeWindowIdx = savedActiveIdx
}

// getRows returns the cached rows, rebuilding if necessary.
func (m *Model) getRows() []displayRow {
	if !m.w().rowsCacheValid {
		m.rebuildRowsCache()
	}
	return m.w().cachedRows
}

// currentCommitIndex returns the index of the commit the cursor is currently in.
// Returns -1 if the cursor is not on any commit (e.g. on a blank separator
// between commits, or on a top border that hasn't reached the header yet).
func (m *Model) currentCommitIndex() int {
	if len(m.commits) == 0 {
		return -1
	}

	// Get the row at the cursor position
	rows := m.getRows()
	cursorPos := m.cursorLine()

	// Past all content — return last commit
	if cursorPos >= len(rows) {
		return len(m.commits) - 1
	}

	if cursorPos >= 0 {
		row := rows[cursorPos]
		// For file rows, use fileIndex to determine the commit.
		// For top border rows, fileIndex points to the next file — but that's
		// the correct commit (the top border sits between files, and the file
		// line in the status bar shows the previous file while the commit line
		// should show whichever commit the next file belongs to).
		if row.fileIndex >= 0 {
			return m.commitForFile(row.fileIndex)
		}
		// For commit header/body rows, use commitIndex directly
		if row.commitIndex >= 0 && row.commitIndex < len(m.commits) {
			// Commit top border points to the next commit; don't switch yet.
			if row.isCommitHeaderTopBorder {
				if row.commitIndex > 0 {
					return row.commitIndex - 1
				}
				return -1
			}
			// Blank separator between commits is not "on" any commit.
			if row.commitBodyIsBlank {
				return -1
			}
			return row.commitIndex
		}
	}

	return -1
}

// currentCommit returns the commit set the cursor is currently in.
// Uses the cursor position to determine which commit is displayed.
func (m *Model) currentCommit() *sidebyside.CommitSet {
	idx := m.currentCommitIndex()
	if idx < 0 || idx >= len(m.commits) {
		return nil
	}
	return &m.commits[idx]
}

// toggleNarrow toggles narrow mode on/off.
// When entering narrow mode, determines the scope from the current cursor position.
// When exiting, tries to keep the cursor at its current content position in the widened view.
func (m *Model) toggleNarrow() {
	if m.w().narrow.Active {
		// Capture current position BEFORE clearing narrow mode
		currentIdentity := m.getCursorRowIdentity()
		narrowedFileIdx := m.w().narrow.FileIdx
		narrowedCommitIdx := m.w().narrow.CommitIdx

		// Clear narrow scope and rebuild rows
		m.w().narrow = NarrowScope{}
		m.rebuildRowsCache()

		// Strategy: try to find the current row in the widened view.
		// This keeps the user at the same content they were viewing.
		newRow := m.findRowOrNearestAbove(currentIdentity)
		if newRow >= 0 {
			m.w().scroll = newRow
			m.clampScroll()
			return
		}

		// Fallback: position at the header of the narrowed node.
		// This ensures the user sees what they were narrowed to.
		if narrowedFileIdx >= 0 {
			headerRow := m.findFileHeaderRow(narrowedFileIdx)
			if headerRow >= 0 {
				m.w().scroll = headerRow
				m.clampScroll()
				return
			}
		} else if narrowedCommitIdx >= 0 {
			headerRow := m.findCommitHeaderRow(narrowedCommitIdx)
			if headerRow >= 0 {
				m.w().scroll = headerRow
				m.clampScroll()
				return
			}
		}

		// Last resort: go to top
		m.w().scroll = m.minScroll()
		m.clampScroll()
		return
	}

	// Enter narrow mode: determine scope from cursor position
	rows := m.getRows()
	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return // nothing to narrow to
	}

	row := rows[cursorPos]

	// Capture current position for preserving cursor after layout change
	currentIdentity := m.getCursorRowIdentity()

	// Determine scope based on what the cursor is on
	switch {
	case row.fileIndex >= 0:
		// On a file row: narrow to this file
		m.w().narrow.Active = true
		m.w().narrow.CommitIdx = m.commitForFile(row.fileIndex)
		m.w().narrow.FileIdx = row.fileIndex
		m.w().narrow.HunkIdx = -1
		m.w().narrow.CommitInfoOnly = false

	case row.kind == RowKindCommitInfoHeader || row.kind == RowKindCommitInfoBody ||
		row.kind == RowKindCommitInfoTopBorder || row.kind == RowKindCommitInfoBottomBorder:
		// On a commit info row: narrow to just the commit info section
		m.w().narrow.Active = true
		m.w().narrow.CommitIdx = row.commitIndex
		m.w().narrow.FileIdx = -1
		m.w().narrow.HunkIdx = -1
		m.w().narrow.CommitInfoOnly = true

	case row.commitIndex >= 0 && !row.commitBodyIsBlank:
		// On a commit row (header, body): narrow to this commit (including files)
		m.w().narrow.Active = true
		m.w().narrow.CommitIdx = row.commitIndex
		m.w().narrow.FileIdx = -1
		m.w().narrow.HunkIdx = -1
		m.w().narrow.CommitInfoOnly = false

	default:
		// On a separator or other non-scoped row: don't enter narrow mode
		return
	}

	// Rebuild rows with the new scope
	m.rebuildRowsCache()

	// Preserve cursor position: find the same row in the narrowed view
	newRow := m.findRowOrNearestAbove(currentIdentity)
	if newRow >= 0 {
		m.w().scroll = newRow
	} else {
		m.w().scroll = 0
	}
	m.clampScroll()
}

// findFileHeaderRow returns the row index of the header for the given file.
// Returns -1 if not found.
func (m Model) findFileHeaderRow(fileIdx int) int {
	rows := m.getRows()
	for i, row := range rows {
		if row.fileIndex == fileIdx && row.kind == RowKindHeader {
			return i
		}
	}
	return -1
}

// findCommitHeaderRow returns the row index of the header for the given commit.
// Returns -1 if not found.
func (m Model) findCommitHeaderRow(commitIdx int) int {
	rows := m.getRows()
	for i, row := range rows {
		if row.commitIndex == commitIdx && row.kind == RowKindCommitHeader {
			return i
		}
	}
	return -1
}

// commitForFile returns the commit index that contains the given file index.
func (m Model) commitForFile(fileIdx int) int {
	for i := len(m.commitFileStarts) - 1; i >= 0; i-- {
		if fileIdx >= m.commitFileStarts[i] {
			return i
		}
	}
	return 0
}

// isFirstFileInCommit returns true if fileIdx is the first file in its commit.
func (m Model) isFirstFileInCommit(fileIdx int) bool {
	commitIdx := m.commitForFile(fileIdx)
	return fileIdx == m.commitFileStarts[commitIdx]
}

// hasCommitInfo returns true if any commit in the view has metadata.
// This is used to determine whether to reserve space for the commit line in the top bar.
func (m Model) hasCommitInfo() bool {
	for i := range m.commits {
		if m.commits[i].Info.HasMetadata() {
			return true
		}
	}
	return false
}

// getFocusPredicate returns a predicate function that determines if a row should be in focus.
// Returns nil if focus mode shouldn't apply (focusColour disabled or cursor not on focusable row).
// When non-nil, rows where the predicate returns false will be dimmed.
func (m Model) getFocusPredicate() func(rowIdx int, row displayRow) bool {
	if !m.focusColour {
		return nil
	}

	rows := m.w().cachedRows
	if !m.w().rowsCacheValid || len(rows) == 0 {
		return nil
	}

	cursorPos := m.cursorLine()
	if cursorPos < 0 || cursorPos >= len(rows) {
		return nil
	}

	cursorRow := rows[cursorPos]

	switch cursorRow.kind {
	case RowKindContent, RowKindSeparatorTop, RowKindSeparator, RowKindSeparatorBottom:
		// Focus on current hunk + file header + all hunk separators for navigation
		start, end, fileIdx := m.getHunkBounds(cursorPos, cursorRow.fileIndex)
		return func(rowIdx int, row displayRow) bool {
			// Only consider rows in the current file
			if row.fileIndex != fileIdx {
				return false
			}

			switch row.kind {
			// File header rows stay in focus
			case RowKindHeader, RowKindHeaderSpacer, RowKindHeaderTopBorder:
				return true
			// Structural diff rows (under file header) stay in focus
			case RowKindStructuralDiff:
				return true
			// All hunk separators in the file stay in focus for navigation
			case RowKindSeparatorTop, RowKindSeparator, RowKindSeparatorBottom:
				return true
			}

			// Current hunk content
			return rowIdx >= start && rowIdx < end
		}

	default:
		// No focus mode for other row types (yet)
		return nil
	}
}

// focusProximityThreshold is the number of lines within which nearby hunks
// are also included in the focus area.
const focusProximityThreshold = 15

// getHunkBounds returns the range of row indices for the current hunk.
// Returns (start, end, fileIdx) where start is inclusive and end is exclusive.
// The range includes the hunk's content and its preceding separator rows.
// Special case: SeparatorTop is treated as the bottom border of the hunk ABOVE.
// Also expands to include nearby hunks within focusProximityThreshold lines.
func (m Model) getHunkBounds(cursorPos, fileIdx int) (int, int, int) {
	rows := m.w().cachedRows
	cursorKind := rows[cursorPos].kind

	var start, end int

	// Special case: SeparatorTop belongs to the hunk above, not below.
	// It acts as the bottom border of the previous hunk.
	if cursorKind == RowKindSeparatorTop {
		// Scan backward through content to find the start of the hunk above
		start = cursorPos
		for start > 0 {
			prevRow := rows[start-1]
			if prevRow.fileIndex != fileIdx {
				break
			}
			if prevRow.kind == RowKindHeader || prevRow.kind == RowKindHeaderSpacer || prevRow.kind == RowKindHeaderTopBorder {
				break
			}
			if prevRow.kind == RowKindContent {
				start--
				continue
			}
			// Stop at the previous separator block
			if prevRow.kind == RowKindSeparatorTop {
				start-- // include the previous SeparatorTop
				break
			}
			if prevRow.kind == RowKindSeparator || prevRow.kind == RowKindSeparatorBottom {
				// Skip past the separator block to find its SeparatorTop
				start--
				continue
			}
			break
		}
		// End is just after the SeparatorTop we're on
		end = cursorPos + 1
	} else {
		// Normal case: scan backward to find the start of the current hunk.
		// The hunk includes Separator and SeparatorBottom above it, but NOT SeparatorTop
		// (since SeparatorTop belongs to the hunk above).
		start = cursorPos
		for start > 0 {
			prevRow := rows[start-1]
			// Stop if we hit a different file
			if prevRow.fileIndex != fileIdx {
				break
			}
			// Stop if we hit a header row (we're at the first hunk)
			if prevRow.kind == RowKindHeader || prevRow.kind == RowKindHeaderSpacer || prevRow.kind == RowKindHeaderTopBorder {
				break
			}
			// Include content rows
			if prevRow.kind == RowKindContent {
				start--
				continue
			}
			// Include Separator and SeparatorBottom (they belong to this hunk)
			if prevRow.kind == RowKindSeparatorBottom || prevRow.kind == RowKindSeparator {
				start--
				continue
			}
			// Stop at SeparatorTop (it belongs to the hunk above)
			if prevRow.kind == RowKindSeparatorTop {
				break
			}
			break
		}

		// Scan forward to find the end of the current hunk.
		// If cursor is on Separator or SeparatorBottom, skip past them to reach content.
		// The hunk ends when we hit SeparatorTop (which belongs to the next hunk's above).
		end = cursorPos + 1
		inSeparatorBlock := cursorKind == RowKindSeparator || cursorKind == RowKindSeparatorBottom

		for end < len(rows) {
			nextRow := rows[end]
			// Stop if we hit a different file
			if nextRow.fileIndex != fileIdx {
				break
			}
			// Stop if we hit blank rows (inter-file spacing)
			if nextRow.kind == RowKindBlank {
				break
			}
			// SeparatorTop belongs to the hunk above, so stop here
			if nextRow.kind == RowKindSeparatorTop {
				break
			}
			// Handle Separator and SeparatorBottom
			if nextRow.kind == RowKindSeparator || nextRow.kind == RowKindSeparatorBottom {
				if inSeparatorBlock {
					// Still in the initial separator block, keep going
					end++
					continue
				}
				// Hit a new separator block, include Separator and SeparatorBottom
				// (they're the bottom border of current hunk)
				end++
				continue
			}
			// Content row - we've exited any initial separator block
			if nextRow.kind == RowKindContent {
				inSeparatorBlock = false
				end++
			} else {
				break
			}
		}
	}

	// Expand to include nearby hunks within the proximity threshold
	start, end = m.expandToNearbyHunks(start, end, fileIdx)

	return start, end, fileIdx
}

// expandToNearbyHunks expands the focus bounds to include hunks within
// focusProximityThreshold SOURCE lines of the current bounds.
// Uses actual line numbers from the code, not display row indices.
func (m Model) expandToNearbyHunks(start, end, fileIdx int) (int, int) {
	rows := m.w().cachedRows

	// Get the source line range of current focus area
	minLine, maxLine := m.getSourceLineRange(start, end)
	if minLine == 0 && maxLine == 0 {
		return start, end
	}

	// Keep expanding until no more nearby hunks are found
	for {
		expanded := false

		// Check for hunk above within threshold (by source line numbers)
		if start > 0 {
			// Find the last content row of the hunk above
			scanPos := start - 1
			// Skip past separator rows
			for scanPos >= 0 && rows[scanPos].fileIndex == fileIdx {
				row := rows[scanPos]
				if row.kind == RowKindSeparatorTop || row.kind == RowKindSeparator || row.kind == RowKindSeparatorBottom {
					scanPos--
				} else {
					break
				}
			}
			// Check if there's content above
			if scanPos >= 0 && rows[scanPos].fileIndex == fileIdx && rows[scanPos].kind == RowKindContent {
				// Get the max line number of the hunk above
				hunkAboveEnd := m.findHunkEnd(scanPos, fileIdx)
				_, hunkAboveMaxLine := m.getSourceLineRange(scanPos, hunkAboveEnd)
				// Check if it's within threshold of our min line
				if hunkAboveMaxLine > 0 && minLine-hunkAboveMaxLine <= focusProximityThreshold {
					newStart := m.findHunkStart(scanPos, fileIdx)
					if newStart < start {
						start = newStart
						// Update minLine for next iteration
						newMinLine, _ := m.getSourceLineRange(start, end)
						if newMinLine > 0 {
							minLine = newMinLine
						}
						expanded = true
					}
				}
			}
		}

		// Check for hunk below within threshold (by source line numbers)
		if end < len(rows) {
			// Find the first content row of the hunk below
			scanPos := end
			// Skip past separator rows
			for scanPos < len(rows) && rows[scanPos].fileIndex == fileIdx {
				row := rows[scanPos]
				if row.kind == RowKindSeparatorTop || row.kind == RowKindSeparator || row.kind == RowKindSeparatorBottom {
					scanPos++
				} else {
					break
				}
			}
			// Check if there's content below
			if scanPos < len(rows) && rows[scanPos].fileIndex == fileIdx && rows[scanPos].kind == RowKindContent {
				// Get the min line number of the hunk below
				hunkBelowStart := m.findHunkStart(scanPos, fileIdx)
				hunkBelowMinLine, _ := m.getSourceLineRange(hunkBelowStart, scanPos+1)
				// Check if it's within threshold of our max line
				if hunkBelowMinLine > 0 && hunkBelowMinLine-maxLine <= focusProximityThreshold {
					newEnd := m.findHunkEnd(scanPos, fileIdx)
					if newEnd > end {
						end = newEnd
						// Update maxLine for next iteration
						_, newMaxLine := m.getSourceLineRange(start, end)
						if newMaxLine > 0 {
							maxLine = newMaxLine
						}
						expanded = true
					}
				}
			}
		}

		if !expanded {
			break
		}
	}

	return start, end
}

// getSourceLineRange returns the min and max source line numbers in the given row range.
// Uses the New side line numbers (left side in the UI).
func (m Model) getSourceLineRange(start, end int) (minLine, maxLine int) {
	rows := m.w().cachedRows
	for i := start; i < end && i < len(rows); i++ {
		row := rows[i]
		if row.kind == RowKindContent {
			if row.pair.New.Num > 0 {
				if minLine == 0 || row.pair.New.Num < minLine {
					minLine = row.pair.New.Num
				}
				if row.pair.New.Num > maxLine {
					maxLine = row.pair.New.Num
				}
			}
			// Also check Old side for deleted-only lines
			if row.pair.Old.Num > 0 {
				if minLine == 0 || row.pair.Old.Num < minLine {
					minLine = row.pair.Old.Num
				}
				if row.pair.Old.Num > maxLine {
					maxLine = row.pair.Old.Num
				}
			}
		}
	}
	return minLine, maxLine
}

// findHunkStart finds the start of the hunk containing the given position.
func (m Model) findHunkStart(pos, fileIdx int) int {
	rows := m.w().cachedRows
	start := pos
	for start > 0 {
		prevRow := rows[start-1]
		if prevRow.fileIndex != fileIdx {
			break
		}
		if prevRow.kind == RowKindHeader || prevRow.kind == RowKindHeaderSpacer || prevRow.kind == RowKindHeaderTopBorder {
			break
		}
		if prevRow.kind == RowKindContent {
			start--
			continue
		}
		if prevRow.kind == RowKindSeparatorBottom || prevRow.kind == RowKindSeparator {
			start--
			continue
		}
		if prevRow.kind == RowKindSeparatorTop {
			break
		}
		break
	}
	return start
}

// findHunkEnd finds the end of the hunk containing the given position.
func (m Model) findHunkEnd(pos, fileIdx int) int {
	rows := m.w().cachedRows
	end := pos + 1
	for end < len(rows) {
		nextRow := rows[end]
		if nextRow.fileIndex != fileIdx {
			break
		}
		if nextRow.kind == RowKindBlank {
			break
		}
		if nextRow.kind == RowKindSeparatorTop {
			break
		}
		if nextRow.kind == RowKindContent {
			end++
		} else {
			break
		}
	}
	return end
}

// insertSnapshotCommit inserts a snapshot diff at the beginning of the commits list.
// This updates commits, files, commitFileStarts, and scrolls to the top.
func (m *Model) insertSnapshotCommit(commit sidebyside.CommitSet) {
	// Insert at position 0
	m.commits = append([]sidebyside.CommitSet{commit}, m.commits...)

	// Update files: prepend the new commit's files
	newFileCount := len(commit.Files)
	m.files = append(commit.Files, m.files...)

	// Update commitFileStarts: shift all indices by newFileCount
	newStarts := make([]int, len(m.commits))
	newStarts[0] = 0 // new commit starts at index 0
	for i := 1; i < len(m.commits); i++ {
		newStarts[i] = m.commitFileStarts[i-1] + newFileCount
	}
	m.commitFileStarts = newStarts

	// Shift file-indexed maps to account for prepended files
	m.shiftFileIndexMaps(newFileCount)

	// Match persisted comments for the new snapshot files
	if m.commentIndex != nil {
		m.matchCommentsForFiles(0, newFileCount)
	}

	// Invalidate caches for all windows
	m.invalidateAllRowCaches()
	m.calculateTotalLines()

	// Scroll to top to show the new snapshot
	m.w().scroll = 0

	// Highlight the new files asynchronously (they'll be highlighted via the normal flow)
}

// rebuildFilesFromCommits rebuilds m.files and m.commitFileStarts from m.commits.
func (m *Model) rebuildFilesFromCommits() {
	m.files = nil
	m.commitFileStarts = make([]int, len(m.commits))
	m.truncatedFileCount = 0
	for i, c := range m.commits {
		m.commitFileStarts[i] = len(m.files)
		m.files = append(m.files, c.Files...)
		m.truncatedFileCount += c.TruncatedFileCount
	}
}

// swapToView replaces the current view with the given commits.
// Clears all file-indexed caches and resets window state.
func (m *Model) swapToView(commits []sidebyside.CommitSet) tea.Cmd {
	m.commits = commits
	m.rebuildFilesFromCommits()

	// Clear file-indexed maps (indices have changed completely)
	m.highlightSpans = make(map[int]*FileHighlight)
	m.pairsHighlightSpans = make(map[int]*PairsFileHighlight)
	m.structureMaps = make(map[int]*FileStructure)
	m.pairsStructureMaps = make(map[int]*FileStructure)
	m.inlineDiffCache = make(map[inlineDiffKey]inlineDiffResult)

	// Reset all windows
	for _, w := range m.windows {
		w.scroll = 0
		w.hscroll = 0
		w.narrow = NarrowScope{}
		w.fileFoldLevels = make(map[int]sidebyside.FoldLevel)
		w.commitFoldLevels = make(map[int]sidebyside.CommitFoldLevel)
		w.commitInfoExpanded = make(map[int]bool)
		w.rowsCacheValid = false
		w.cachedRows = nil
	}

	// Re-apply auto-fold logic (same as initial WindowSizeMsg handler)
	if len(m.files) > 0 && m.width > 0 {
		if len(m.files) == 1 || m.estimateNormalRows() <= m.contentHeight() {
			for i := range m.files {
				m.setFileFoldLevel(i, sidebyside.FoldHunks)
			}
		} else {
			for i := range m.files {
				m.setFileFoldLevel(i, sidebyside.FoldHeader)
			}
		}
	}

	m.calculateTotalLines()

	// Re-match persisted comments for the new file set
	if m.commentIndex != nil {
		m.matchCommentsForFiles(0, len(m.files))
	}

	// Synchronously highlight first file, then async the rest
	if len(m.files) > 0 {
		m.highlightPairsSync(0)
		if len(m.files) > 1 {
			return m.RequestHighlightFromPairsExcept(map[int]bool{0: true})
		}
	}
	return nil
}

// handleSnapshotToggle handles the S key to toggle between normal and snapshot view.
// When toggling ON: builds the snapshot timeline (lastSnapshot→WT + historical diffs)
// via an async command, replicating the old continueMode behavior. When toggling OFF:
// caches the snapshot view and restores the normal base→WT view.
func (m *Model) handleSnapshotToggle() tea.Cmd {
	if !m.autoSnapshots {
		now := time.Now()
		m.statusMessage = "Snapshots not enabled"
		m.statusMessageTime = now
		return m.clearStatusAfter(now)
	}

	m.showSnapshots = !m.showSnapshots
	m.logf("handleSnapshotToggle: showSnapshots=%v git=%v baseSHA=%q branch=%q snapshots=%d snapshotViewCommits=%v",
		m.showSnapshots, m.git != nil, m.baseSHA, m.branch, len(m.snapshots), m.snapshotViewCommits != nil)

	if m.showSnapshots {
		// Switching TO snapshot view — cache current normal view
		m.normalViewCommits = make([]sidebyside.CommitSet, len(m.commits))
		copy(m.normalViewCommits, m.commits)

		if m.snapshotViewCommits != nil {
			m.logf("handleSnapshotToggle: restoring cached snapshot view (%d commits)", len(m.snapshotViewCommits))
			return m.swapToView(m.snapshotViewCommits)
		}

		// No snapshots taken yet — view will stay blank until SnapshotCreatedMsg
		// arrives and triggers buildSnapshotHistoryCmd.
		if len(m.snapshots) == 0 {
			m.logf("handleSnapshotToggle: no snapshots yet, waiting for initial snapshot")
			return nil
		}

		// No cached snapshot view — build the timeline asynchronously.
		// SnapshotHistoryReadyMsg handler will swap the view when ready.
		cmd := m.buildSnapshotHistoryCmd()
		m.logf("handleSnapshotToggle: buildSnapshotHistoryCmd returned cmd=%v", cmd != nil)
		return cmd
	}

	// Switching TO normal view — cache current commits (may include R-key snapshots)
	m.snapshotViewCommits = make([]sidebyside.CommitSet, len(m.commits))
	copy(m.snapshotViewCommits, m.commits)

	if m.normalViewCommits != nil {
		m.logf("handleSnapshotToggle: restoring normal view (%d commits)", len(m.normalViewCommits))
		return m.swapToView(m.normalViewCommits)
	}
	return nil
}

// buildSnapshotHistoryCmd returns an async command that computes snapshot history.
func (m *Model) buildSnapshotHistoryCmd() tea.Cmd {
	gitClient := m.git
	if gitClient == nil {
		m.logf("buildSnapshotHistoryCmd: git client is nil")
		return nil
	}

	baseSHA := m.baseSHA
	branch := m.branch
	snapshots := make([]string, len(m.snapshots))
	copy(snapshots, m.snapshots)
	debugLog := m.debugLog // capture for closure

	logf := func(format string, args ...interface{}) {
		if debugLog != nil {
			debugLog.Printf(format, args...)
		}
	}

	return func() tea.Msg {
		logf("buildSnapshotHistoryCmd: starting (baseSHA=%q branch=%q snapshots=%d)", baseSHA, branch, len(snapshots))
		infos, err := gitClient.ListSnapshotRefs(branch, baseSHA)
		if err != nil {
			logf("buildSnapshotHistoryCmd: ListSnapshotRefs error: %v", err)
			return SnapshotHistoryReadyMsg{Err: err}
		}
		logf("buildSnapshotHistoryCmd: ListSnapshotRefs returned %d infos", len(infos))
		// Fall back to in-memory snapshots if git refs not found
		// (handles race with initial snapshot or ref persistence failure)
		if len(infos) == 0 {
			if len(snapshots) == 0 {
				logf("buildSnapshotHistoryCmd: no refs and no in-memory snapshots, returning empty")
				return SnapshotHistoryReadyMsg{}
			}
			logf("buildSnapshotHistoryCmd: falling back to %d in-memory snapshots", len(snapshots))
			infos = make([]git.SnapshotInfo, len(snapshots))
			for i, sha := range snapshots {
				infos[i] = git.SnapshotInfo{SHA: sha}
			}
		}

		persistedSHAs := make([]string, len(infos))
		for i, info := range infos {
			persistedSHAs[i] = info.SHA
		}

		var commits []sidebyside.CommitSet

		// Build lastSnapshot→WT diff
		lastSnapshot := persistedSHAs[len(persistedSHAs)-1]
		logf("buildSnapshotHistoryCmd: diffing lastSnapshot=%q → WT", lastSnapshot[:min(len(lastSnapshot), 7)])
		wtDiff, err := gitClient.Diff(lastSnapshot)
		logf("buildSnapshotHistoryCmd: WT diff err=%v len=%d", err, len(wtDiff))
		if err == nil && wtDiff != "" {
			if wtParsed, err := diff.Parse(wtDiff); err == nil && len(wtParsed.Files) > 0 {
				wtFiles, _ := sidebyside.TransformDiff(wtParsed)
				wtCommit := sidebyside.CommitSet{
					Info:           sidebyside.CommitInfo{Subject: "Working tree changes"},
					Files:          wtFiles,
					FoldLevel:      sidebyside.CommitFileHeaders,
					FilesLoaded:    true,
					StatsLoaded:    true,
					IsSnapshot:     true,
					SnapshotOldRef: lastSnapshot,
				}
				for _, f := range wtFiles {
					for _, lp := range f.Pairs {
						if lp.New.Type == sidebyside.Added {
							wtCommit.TotalAdded++
						}
						if lp.Old.Type == sidebyside.Removed {
							wtCommit.TotalRemoved++
						}
					}
				}
				commits = append(commits, wtCommit)
			}
		}

		// Build S(n-1)→S(n) diffs (newest first)
		for i := len(infos) - 1; i >= 1; i-- {
			oldRef := infos[i-1].SHA
			newRef := infos[i].SHA
			histDiff, err := gitClient.DiffSnapshots(oldRef, newRef)
			if err != nil || histDiff == "" {
				continue
			}
			histParsed, err := diff.Parse(histDiff)
			if err != nil {
				continue
			}
			info := infos[i]
			snapshotShort := info.SHA
			if len(snapshotShort) > 7 {
				snapshotShort = snapshotShort[:7]
			}
			histFiles, _ := sidebyside.TransformDiff(histParsed)
			histCommit := sidebyside.CommitSet{
				Info: sidebyside.CommitInfo{
					SHA: snapshotShort, Date: info.Date, Subject: info.Subject,
				},
				Files: histFiles, FoldLevel: sidebyside.CommitFolded,
				FilesLoaded: true, StatsLoaded: true, IsSnapshot: true,
				SnapshotOldRef: oldRef, SnapshotNewRef: newRef,
			}
			for _, f := range histFiles {
				for _, lp := range f.Pairs {
					if lp.New.Type == sidebyside.Added {
						histCommit.TotalAdded++
					}
					if lp.Old.Type == sidebyside.Removed {
						histCommit.TotalRemoved++
					}
				}
			}
			commits = append(commits, histCommit)
		}

		// Build base→S0 diff
		firstInfo := infos[0]
		histDiff, err := gitClient.DiffSnapshots(baseSHA, firstInfo.SHA)
		if err == nil && histDiff != "" {
			histParsed, err := diff.Parse(histDiff)
			if err == nil {
				snapshotShort := firstInfo.SHA
				if len(snapshotShort) > 7 {
					snapshotShort = snapshotShort[:7]
				}
				histFiles, _ := sidebyside.TransformDiff(histParsed)
				histCommit := sidebyside.CommitSet{
					Info: sidebyside.CommitInfo{
						SHA: snapshotShort, Date: firstInfo.Date, Subject: firstInfo.Subject,
					},
					Files: histFiles, FoldLevel: sidebyside.CommitFolded,
					FilesLoaded: true, StatsLoaded: true, IsSnapshot: true,
					SnapshotOldRef: baseSHA, SnapshotNewRef: firstInfo.SHA,
				}
				for _, f := range histFiles {
					for _, lp := range f.Pairs {
						if lp.New.Type == sidebyside.Added {
							histCommit.TotalAdded++
						}
						if lp.Old.Type == sidebyside.Removed {
							histCommit.TotalRemoved++
						}
					}
				}
				commits = append(commits, histCommit)
			}
		}

		logf("buildSnapshotHistoryCmd: returning %d commits", len(commits))
		for i, c := range commits {
			logf("  commit[%d]: subject=%q files=%d isSnapshot=%v", i, c.Info.Subject, len(c.Files), c.IsSnapshot)
		}
		return SnapshotHistoryReadyMsg{Commits: commits}
	}
}

// shiftFileIndexMapsFrom shifts file-indexed map keys >= fromIdx by delta.
// Keys in [fromIdx-delta, fromIdx) after a negative delta are removed (their
// files no longer exist). Used by loadCommitDiff when file counts change.
func (m *Model) shiftFileIndexMapsFrom(fromIdx, delta int) {
	// Shift highlightSpans
	if len(m.highlightSpans) > 0 {
		newMap := make(map[int]*FileHighlight, len(m.highlightSpans))
		for k, v := range m.highlightSpans {
			if k >= fromIdx {
				newMap[k+delta] = v
			} else {
				newMap[k] = v
			}
		}
		m.highlightSpans = newMap
	}

	// Shift pairsHighlightSpans
	if len(m.pairsHighlightSpans) > 0 {
		newMap := make(map[int]*PairsFileHighlight, len(m.pairsHighlightSpans))
		for k, v := range m.pairsHighlightSpans {
			if k >= fromIdx {
				newMap[k+delta] = v
			} else {
				newMap[k] = v
			}
		}
		m.pairsHighlightSpans = newMap
	}

	// Shift structureMaps
	if len(m.structureMaps) > 0 {
		newMap := make(map[int]*FileStructure, len(m.structureMaps))
		for k, v := range m.structureMaps {
			if k >= fromIdx {
				newMap[k+delta] = v
			} else {
				newMap[k] = v
			}
		}
		m.structureMaps = newMap
	}

	// Shift pairsStructureMaps
	if len(m.pairsStructureMaps) > 0 {
		newMap := make(map[int]*FileStructure, len(m.pairsStructureMaps))
		for k, v := range m.pairsStructureMaps {
			if k >= fromIdx {
				newMap[k+delta] = v
			} else {
				newMap[k] = v
			}
		}
		m.pairsStructureMaps = newMap
	}

	// Shift loadingFiles
	if len(m.loadingFiles) > 0 {
		newMap := make(map[int]time.Time, len(m.loadingFiles))
		for k, v := range m.loadingFiles {
			if k >= fromIdx {
				newMap[k+delta] = v
			} else {
				newMap[k] = v
			}
		}
		m.loadingFiles = newMap
	}

	// Shift comments
	if len(m.comments) > 0 {
		newMap := make(map[commentKey]string, len(m.comments))
		for k, v := range m.comments {
			if k.fileIndex >= fromIdx {
				newMap[commentKey{fileIndex: k.fileIndex + delta, newLineNum: k.newLineNum}] = v
			} else {
				newMap[k] = v
			}
		}
		m.comments = newMap
	}
	if len(m.persistedCommentIDs) > 0 {
		newMap := make(map[commentKey]string, len(m.persistedCommentIDs))
		for k, v := range m.persistedCommentIDs {
			if k.fileIndex >= fromIdx {
				newMap[commentKey{fileIndex: k.fileIndex + delta, newLineNum: k.newLineNum}] = v
			} else {
				newMap[k] = v
			}
		}
		m.persistedCommentIDs = newMap
	}

	// Clear inlineDiffCache — it uses fileIndex in keys; clearing is safe since it's
	// just a performance cache that will be repopulated on next render.
	if len(m.inlineDiffCache) > 0 {
		m.inlineDiffCache = make(map[inlineDiffKey]inlineDiffResult)
	}
}

// shiftFileIndexMaps shifts all file-indexed map keys by the given offset.
// This is needed when files are prepended, changing all existing file indices.
func (m *Model) shiftFileIndexMaps(offset int) {
	// Shift highlightSpans
	if len(m.highlightSpans) > 0 {
		newMap := make(map[int]*FileHighlight, len(m.highlightSpans))
		for k, v := range m.highlightSpans {
			newMap[k+offset] = v
		}
		m.highlightSpans = newMap
	}

	// Shift pairsHighlightSpans
	if len(m.pairsHighlightSpans) > 0 {
		newMap := make(map[int]*PairsFileHighlight, len(m.pairsHighlightSpans))
		for k, v := range m.pairsHighlightSpans {
			newMap[k+offset] = v
		}
		m.pairsHighlightSpans = newMap
	}

	// Shift structureMaps
	if len(m.structureMaps) > 0 {
		newMap := make(map[int]*FileStructure, len(m.structureMaps))
		for k, v := range m.structureMaps {
			newMap[k+offset] = v
		}
		m.structureMaps = newMap
	}

	// Shift pairsStructureMaps
	if len(m.pairsStructureMaps) > 0 {
		newMap := make(map[int]*FileStructure, len(m.pairsStructureMaps))
		for k, v := range m.pairsStructureMaps {
			newMap[k+offset] = v
		}
		m.pairsStructureMaps = newMap
	}

	// Shift loadingFiles
	if len(m.loadingFiles) > 0 {
		newMap := make(map[int]time.Time, len(m.loadingFiles))
		for k, v := range m.loadingFiles {
			newMap[k+offset] = v
		}
		m.loadingFiles = newMap
	}

	// Shift comments and persistedCommentIDs (commentKey includes fileIndex)
	if len(m.comments) > 0 {
		newMap := make(map[commentKey]string, len(m.comments))
		for k, v := range m.comments {
			newMap[commentKey{fileIndex: k.fileIndex + offset, newLineNum: k.newLineNum}] = v
		}
		m.comments = newMap
	}
	if len(m.persistedCommentIDs) > 0 {
		newMap := make(map[commentKey]string, len(m.persistedCommentIDs))
		for k, v := range m.persistedCommentIDs {
			newMap[commentKey{fileIndex: k.fileIndex + offset, newLineNum: k.newLineNum}] = v
		}
		m.persistedCommentIDs = newMap
	}

	// Clear inlineDiffCache - it uses fileIndex in its keys and shifting
	// would be complex; clearing is safe since it's just a performance cache
	if len(m.inlineDiffCache) > 0 {
		m.inlineDiffCache = make(map[inlineDiffKey]inlineDiffResult)
	}
}
