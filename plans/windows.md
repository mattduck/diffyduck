# Windows Feature Plan

## Overview

Introduce multiple windows (views) into the same underlying diff data, similar to vim/emacs split windows.

## Requirements

### Window Management
- **Create vertical**: `Ctrl+W %` creates a vertical split (50/50, side-by-side)
- **Create horizontal**: `Ctrl+W "` creates a horizontal split (50/50, stacked top/bottom)
- **Close**: `Ctrl+W x` closes the current window
- **Navigate vertical**: `Ctrl+W h` / `Ctrl+W l` move focus left/right (no wrapping, only for vertical splits)
- **Navigate horizontal**: `Ctrl+W j` / `Ctrl+W k` move focus down/up (no wrapping, only for horizontal splits)
- **Resize vertical**: `Ctrl+W Ctrl+H` shrinks left window, `Ctrl+W Ctrl+L` grows left window (8 chars per step, clamped 20%-80%)
- **Resize horizontal**: `Ctrl+W Ctrl+J` grows top window, `Ctrl+W Ctrl+K` shrinks top window (8 rows per step, clamped 20%-80%)
- **Limit**: Maximum 2 windows initially

### Per-Window State
- Scroll position (vertical)
- Horizontal scroll
- Fold state (independent per file per window)
- Narrow scope
- Cursor position
- Comment editing state (can edit in one window while browsing in other)
- Top bar content (reflects what that window is viewing)

### Shared State
- Files/commits data structures
- Search query and match highlighting (highlights in all windows)
- Search navigation (only in active window)
- Comments data (the stored text)
- Focus colour mode (global toggle)
- Syntax highlighting cache
- Content fetcher
- Loading state / spinner

## Current Architecture

The `Model` struct currently mixes data and view concerns:

```
Model
├── Data (should be shared)
│   ├── commits, files, commitFileStarts
│   ├── fetcher, git
│   ├── highlighter, highlightSpans, pairsHighlightSpans
│   ├── structureMaps, pairsStructureMaps
│   ├── inlineDiffCache
│   ├── comments
│   ├── searchQuery (shared for highlighting)
│   └── various caches (column widths, etc.)
│
├── View/Window State (should be per-window)
│   ├── scroll, hscroll
│   ├── width, height (though terminal size is shared?)
│   ├── cachedRows, rowsCacheValid, totalLines
│   ├── narrow (NarrowScope)
│   ├── Fold levels (currently on FilePair - need to decouple)
│   ├── commentMode, commentInput, commentCursor, commentScroll, commentKey
│   └── searchMatchIdx, searchMatchSide (navigation state)
│
├── Global UI State
│   ├── searchMode, searchInput, searchForward
│   ├── pendingKey
│   ├── focused, focusColour
│   ├── statusMessage, statusMessageTime
│   └── spinner, loadingFiles, spinnerTicking
│
└── Config
    ├── keys (KeyMap)
    ├── hscrollStep
    ├── pagerMode, debugMode
    └── pagination state
```

## Design Questions

### Q1: Fold State Storage

Currently fold levels are stored on `FilePair.FoldLevel` and `CommitSet.FoldLevel`. For per-window folding, options:

**Option A: Copy fold state into window**
- Each window has `map[int]FoldLevel` for files and `map[int]CommitFoldLevel` for commits
- Pros: Clean separation, windows fully independent
- Cons: Need to initialize from current state when splitting, more memory

**Option B: Multi-level fold state on data**
- `FilePair` gets `FoldLevels map[WindowID]FoldLevel`
- Pros: Keeps fold state near the data
- Cons: Data knows about windows, cleanup on window close

**Recommendation**: Option A - cleaner separation of concerns

### Q2: Row Cache

Currently `cachedRows` is rebuilt when fold/narrow changes. With windows:

**Option A: Per-window row cache**
- Each window builds its own `cachedRows` based on its fold/narrow state
- Pros: Simple, each window is self-contained
- Cons: Memory duplication, need to invalidate per-window

**Option B: Shared row cache with filtering**
- Build full rows once, windows filter/slice
- Pros: Less rebuilding
- Cons: Doesn't work well with folding (fold changes row count)

**Recommendation**: Option A - folding fundamentally changes what rows exist

### Q3: Refactoring Approach

**Option A: Big bang refactor**
- Extract SharedState and WindowState in one go
- Pros: Clean result
- Cons: Large change, harder to review/test incrementally

**Option B: Incremental extraction**
1. First: Add `Window` struct that wraps current view state, Model has `windows []Window` + `activeWindow int`
2. Keep shared state in Model initially
3. Gradually move shared state to explicit `SharedState` struct
- Pros: Smaller steps, easier to verify correctness
- Cons: Intermediate states may be awkward

**Option C: New parallel struct**
- Create new `WindowedModel` alongside existing `Model`
- Migrate functionality piece by piece
- Pros: Can compare old/new behavior
- Cons: Code duplication during transition

**Recommendation**: Option B - incremental is safer for a change this large

## Proposed Architecture

```
Model
├── shared *SharedState
│   ├── commits, files, commitFileStarts
│   ├── fetcher, git
│   ├── highlighter, highlightSpans, ...
│   ├── comments
│   ├── searchQuery
│   └── ...
│
├── windows []*Window (max 2)
├── activeWindowIdx int
│
├── Global UI
│   ├── searchMode, searchInput
│   ├── pendingKey
│   ├── focusColour
│   └── ...
│
└── Config
    └── ...

Window
├── scroll, hscroll
├── width, height (portion of terminal)
├── narrow NarrowScope
├── foldLevels map[int]FoldLevel
├── commitFoldLevels map[int]CommitFoldLevel
├── cachedRows []displayRow
├── rowsCacheValid bool
├── totalLines int
├── commentMode, commentInput, ... (editing state)
├── searchMatchIdx, searchMatchSide (navigation within this window)
└── cursorRowIdentity (for stable cursor)
```

## Implementation Phases

### Phase 1: Window struct extraction [DONE]
- Created `Window` struct with scroll, hscroll, narrow, cachedRows, rowsCacheValid, totalLines, searchMatchIdx, searchMatchSide
- Model gets `windows []*Window`, `activeWindowIdx`
- Single window initially, all operations go through `m.w()`
- `w()` lazily initializes window if empty (for test compatibility)
- Added `wv()` for value receiver methods
- No user-visible changes

### Phase 2: Fold state per window [DONE]
- Added `fileFoldLevels` and `commitFoldLevels` maps to Window struct
- Added accessor methods: `fileFoldLevel(i)`, `setFileFoldLevel(i, level)`, `commitFoldLevel(i)`, `setCommitFoldLevel(i, level)`
- Accessor methods check window map first, fall back to data default if not set
- All source code updated to use accessor methods instead of direct FoldLevel access
- Tests updated to use accessor methods for assertions after mutations

### Phase 3: Row cache per window [DONE - via Phases 1 & 2]
- cachedRows, rowsCacheValid, totalLines already moved to Window in Phase 1
- buildRows() and related methods now use accessor methods from Phase 2
- Each window's row cache reflects its own fold/narrow state

### Phase 4: Window management keybindings [DONE]
- Added `Ctrl+W` prefix handling in `handleKeyMsg` using `pendingKey` mechanism
- Implemented `handlePendingCtrlW` for window commands:
  - `%` - `windowSplitVertical()`: creates 50/50 vertical split (side-by-side)
  - `"` - `windowSplitHorizontal()`: creates 50/50 horizontal split (stacked top/bottom)
  - `x` - `windowClose()`: closes current window (prevents closing last)
  - `h` - `windowFocusLeft()`: moves focus to left window (vertical split only)
  - `l` - `windowFocusRight()`: moves focus to right window (vertical split only)
  - `j` - `windowFocusDown()`: moves focus to bottom window (horizontal split only)
  - `k` - `windowFocusUp()`: moves focus to top window (horizontal split only)
  - `Ctrl+H` - `windowResizeLeft()`: shrinks left window (vertical split only)
  - `Ctrl+L` - `windowResizeRight()`: grows left window (vertical split only)
  - `Ctrl+J` - `windowResizeDown()`: grows top window (horizontal split only)
  - `Ctrl+K` - `windowResizeUp()`: shrinks top window (horizontal split only)
- Updated `View()` to detect multiple windows and branch on `windowSplitV`
- `renderVerticalSplitView()` renders windows side-by-side with full-block (█) divider
- `renderHorizontalSplitView()` renders windows stacked with half-block (▀) divider
- Inactive window renders with unfocused styling (gray cursor arrow)

### Phase 5: Comment editing per window [DONE]
- Moved `commentMode`, `commentInput`, `commentCursor`, `commentScroll`, `commentKey` from Model to Window
- Comment data (`comments` map) remains shared across windows
- Each window can independently edit comments while the other window is used for browsing

### Phase 6: Search navigation per window [DONE - via Phase 1]
- `searchQuery` remains on Model (shared for highlighting in all windows)
- `searchMatchIdx` and `searchMatchSide` already on Window from Phase 1
- Search highlights show in all windows; n/N navigation operates per-window

## Decisions

1. **Split initial position**: New window starts at same scroll position as current window
2. **Close last window**: Prevent closing, show a status message
3. **Active window indicator**: Use existing focus/unfocus styling - active window shows focused cursor/less-bar, inactive window shows unfocused styling. When terminal loses focus, both windows show unfocused. On re-entry, only active window goes back to focused.
4. **Quit behavior**: `q` quits the whole program (not just current window)

## Notes

- This is similar to narrow mode in that windows provide different views of the same data
- Vertical split divider uses full block (█) in dim gray for a clean visual separator
- Horizontal split divider uses upper half block (▀) repeated across the width
- Terminal resize redistributes space to windows based on split ratio
- Navigation and resize commands are split-type-aware: h/l/Ctrl+H/Ctrl+L only work for vertical splits, j/k/Ctrl+J/Ctrl+K only work for horizontal splits
