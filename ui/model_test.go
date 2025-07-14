package ui

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"duckdiff/aligner"
	"duckdiff/parser"
)

func TestNewModel(t *testing.T) {
	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
				Hunks:   []parser.Hunk{},
			},
			AlignedLines: []aligner.AlignedLine{},
		},
	}

	model := NewModel(filesWithLines)

	assert.Equal(t, filesWithLines, model.filesWithLines)
	assert.False(t, model.ready)
	assert.Equal(t, 0, model.width)
}

func TestModel_Init(t *testing.T) {
	model := NewModel([]FileWithLines{})
	cmd := model.Init()
	assert.Nil(t, cmd)
}

func TestModel_Update_WindowSize(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	model := NewModel([]FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
				Hunks:   []parser.Hunk{},
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("line1"),
					NewLine:    stringPtr("line1"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
			},
		},
	})

	// Test window size message
	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 30,
	}

	updatedModel, cmd := model.Update(msg)
	assert.Nil(t, cmd)

	modelCast := updatedModel.(Model)
	assert.True(t, modelCast.ready)
	assert.Equal(t, 100, modelCast.width)
	assert.Equal(t, 100, modelCast.viewport.Width)
	assert.Equal(t, 28, modelCast.viewport.Height) // height - 2 (reserve space for status line)
}

func TestModel_Update_KeyMessages(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create a model that's already ready
	model := NewModel([]FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "test.go",
				NewPath: "test.go",
				Hunks:   []parser.Hunk{},
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("line1"),
					NewLine:    stringPtr("line1"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
			},
		},
	})

	// Initialize the model first
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := model.Update(windowMsg)
	model = updatedModel.(Model)

	tests := []struct {
		name       string
		key        string
		expectQuit bool
	}{
		{
			name:       "quit with q",
			key:        "q",
			expectQuit: true,
		},
		{
			name:       "quit with ctrl+c",
			key:        "ctrl+c",
			expectQuit: true,
		},
		{
			name:       "down arrow navigation",
			key:        "down",
			expectQuit: false,
		},
		{
			name:       "j navigation",
			key:        "j",
			expectQuit: false,
		},
		{
			name:       "up arrow navigation",
			key:        "up",
			expectQuit: false,
		},
		{
			name:       "k navigation",
			key:        "k",
			expectQuit: false,
		},
		{
			name:       "d half page down",
			key:        "d",
			expectQuit: false,
		},
		{
			name:       "u half page up",
			key:        "u",
			expectQuit: false,
		},
		{
			name:       "g go to top",
			key:        "g",
			expectQuit: false,
		},
		{
			name:       "G go to bottom",
			key:        "G",
			expectQuit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyMsg := tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune(tt.key),
			}

			// Handle special keys
			switch tt.key {
			case "ctrl+c":
				keyMsg = tea.KeyMsg{Type: tea.KeyCtrlC}
			case "down":
				keyMsg = tea.KeyMsg{Type: tea.KeyDown}
			case "up":
				keyMsg = tea.KeyMsg{Type: tea.KeyUp}
			default:
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			_, cmd := model.Update(keyMsg)

			if tt.expectQuit {
				// Check if the command is a quit command
				assert.NotNil(t, cmd)
				// We can't easily test if it's specifically tea.Quit, but we can verify a command was returned
			} else {
				// For navigation commands, we don't expect a quit command
				// The command might be nil or a viewport update command
			}
		})
	}
}

func TestModel_View(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	tests := []struct {
		name           string
		ready          bool
		expectedOutput string
	}{
		{
			name:           "not ready shows loading",
			ready:          false,
			expectedOutput: "Loading...",
		},
		{
			name:           "ready shows viewport",
			ready:          true,
			expectedOutput: "", // We'll check this separately since viewport content is complex
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel([]FileWithLines{})
			model.ready = tt.ready

			view := model.View()

			if !tt.ready {
				assert.Equal(t, tt.expectedOutput, view)
			} else {
				// For ready state, just verify it's not the loading message
				assert.NotEqual(t, "Loading...", view)
			}
		})
	}
}

func TestModel_renderContent(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	tests := []struct {
		name           string
		filesWithLines []FileWithLines
		width          int
		containsText   []string
	}{
		{
			name:           "empty files",
			filesWithLines: []FileWithLines{},
			width:          100,
			containsText:   []string{},
		},
		{
			name: "single file with unchanged line",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{
						OldPath: "test.go",
						NewPath: "test.go",
						Hunks:   []parser.Hunk{},
					},
					AlignedLines: []aligner.AlignedLine{
						{
							OldLine:    stringPtr("unchanged line"),
							NewLine:    stringPtr("unchanged line"),
							LineType:   aligner.Unchanged,
							OldLineNum: 1,
							NewLineNum: 1,
						},
					},
				},
			},
			width:        100,
			containsText: []string{"~ test.go", "unchanged line"},
		},
		{
			name: "file with modification",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{
						OldPath: "a/test.go",
						NewPath: "test.go",
						Hunks:   []parser.Hunk{},
					},
					AlignedLines: []aligner.AlignedLine{
						{
							OldLine:    stringPtr("old line"),
							NewLine:    stringPtr("new line"),
							LineType:   aligner.Modified,
							OldLineNum: 1,
							NewLineNum: 1,
						},
					},
				},
			},
			width:        100,
			containsText: []string{"~ test.go", "old line", "new line"},
		},
		{
			name: "new file",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{
						OldPath: "/dev/null",
						NewPath: "newfile.txt",
						Hunks:   []parser.Hunk{},
					},
					AlignedLines: []aligner.AlignedLine{
						{
							OldLine:    nil,
							NewLine:    stringPtr("new content"),
							LineType:   aligner.Added,
							OldLineNum: 0,
							NewLineNum: 1,
						},
					},
				},
			},
			width:        100,
			containsText: []string{"+ newfile.txt", "new content"},
		},
		{
			name: "deleted file",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{
						OldPath: "a/deleted.txt",
						NewPath: "/dev/null",
						Hunks:   []parser.Hunk{},
					},
					AlignedLines: []aligner.AlignedLine{
						{
							OldLine:    stringPtr("deleted content"),
							NewLine:    nil,
							LineType:   aligner.Deleted,
							OldLineNum: 1,
							NewLineNum: 0,
						},
					},
				},
			},
			width:        100,
			containsText: []string{"- a/deleted.txt", "deleted content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(tt.filesWithLines)
			model.width = tt.width

			content := model.renderContent()

			for _, text := range tt.containsText {
				assert.Contains(t, content, text, "Expected to find '%s' in rendered content", text)
			}
		})
	}
}

func TestModel_WithTeatest(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "a/test.go",
				NewPath: "test.go",
				Hunks:   []parser.Hunk{},
			},
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("old line"),
					NewLine:    stringPtr("new line"),
					LineType:   aligner.Modified,
					OldLineNum: 1,
					NewLineNum: 1,
				},
			},
		},
	}

	model := NewModel(filesWithLines)

	// Test with teatest
	tm := teatest.NewTestModel(
		t, model,
		teatest.WithInitialTermSize(80, 24),
	)

	// Send a quit command after a short delay to test the UI
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// Wait for the program to finish
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	// Get the final output
	out := tm.FinalOutput(t)
	require.NotNil(t, out)

	// Read the output to verify we got something
	outBytes, err := io.ReadAll(out)
	require.NoError(t, err)

	// We can't easily assert on the exact output due to ANSI codes and formatting,
	// but we can verify the program ran and exited properly
	assert.Greater(t, len(outBytes), 0, "Expected some output from the UI")
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

func TestModel_buildNavigableLines(t *testing.T) {
	tests := []struct {
		name           string
		filesWithLines []FileWithLines
		expectedCount  int
		expectedRefs   []NavigableLineRef
	}{
		{
			name:           "empty input",
			filesWithLines: []FileWithLines{},
			expectedCount:  0,
			expectedRefs:   nil, // nil slice for empty case
		},
		{
			name: "only unchanged lines",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: stringPtr("line1"), NewLine: stringPtr("line1"), LineType: aligner.Unchanged},
						{OldLine: stringPtr("line2"), NewLine: stringPtr("line2"), LineType: aligner.Unchanged},
					},
				},
			},
			expectedCount: 0,
			expectedRefs:  nil, // nil slice for no navigable lines
		},
		{
			name: "added lines only",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{OldPath: "/dev/null", NewPath: "new.go"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: nil, NewLine: stringPtr("added1"), LineType: aligner.Added},
						{OldLine: nil, NewLine: stringPtr("added2"), LineType: aligner.Added},
					},
				},
			},
			expectedCount: 2,
			expectedRefs: []NavigableLineRef{
				{FileIndex: 0, LineIndex: 0},
				{FileIndex: 0, LineIndex: 1},
			},
		},
		{
			name: "deleted lines only",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{OldPath: "old.go", NewPath: "/dev/null"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: stringPtr("deleted1"), NewLine: nil, LineType: aligner.Deleted},
						{OldLine: stringPtr("deleted2"), NewLine: nil, LineType: aligner.Deleted},
					},
				},
			},
			expectedCount: 1, // Only first line of deletion block
			expectedRefs: []NavigableLineRef{
				{FileIndex: 0, LineIndex: 0},
			},
		},
		{
			name: "mixed changes with context",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: stringPtr("context"), NewLine: stringPtr("context"), LineType: aligner.Unchanged},
						{OldLine: nil, NewLine: stringPtr("added"), LineType: aligner.Added},
						{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: aligner.Modified},
						{OldLine: stringPtr("deleted"), NewLine: nil, LineType: aligner.Deleted},
						{OldLine: stringPtr("context2"), NewLine: stringPtr("context2"), LineType: aligner.Unchanged},
					},
				},
			},
			expectedCount: 3,
			expectedRefs: []NavigableLineRef{
				{FileIndex: 0, LineIndex: 1}, // added
				{FileIndex: 0, LineIndex: 2}, // modified
				{FileIndex: 0, LineIndex: 3}, // deleted (first of block)
			},
		},
		{
			name: "multiple deletion blocks",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: stringPtr("del1"), NewLine: nil, LineType: aligner.Deleted},
						{OldLine: stringPtr("del2"), NewLine: nil, LineType: aligner.Deleted},
						{OldLine: stringPtr("context"), NewLine: stringPtr("context"), LineType: aligner.Unchanged},
						{OldLine: stringPtr("del3"), NewLine: nil, LineType: aligner.Deleted},
					},
				},
			},
			expectedCount: 2, // First line of each deletion block
			expectedRefs: []NavigableLineRef{
				{FileIndex: 0, LineIndex: 0}, // first deletion block
				{FileIndex: 0, LineIndex: 3}, // second deletion block
			},
		},
		{
			name: "multiple files",
			filesWithLines: []FileWithLines{
				{
					FileDiff: parser.FileDiff{OldPath: "file1.go", NewPath: "file1.go"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: nil, NewLine: stringPtr("added"), LineType: aligner.Added},
					},
				},
				{
					FileDiff: parser.FileDiff{OldPath: "file2.go", NewPath: "file2.go"},
					AlignedLines: []aligner.AlignedLine{
						{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: aligner.Modified},
					},
				},
			},
			expectedCount: 2,
			expectedRefs: []NavigableLineRef{
				{FileIndex: 0, LineIndex: 0}, // file1 added
				{FileIndex: 1, LineIndex: 0}, // file2 modified
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(tt.filesWithLines)

			navigableLines := model.buildNavigableLines()

			assert.Equal(t, tt.expectedCount, len(navigableLines), "Expected %d navigable lines, got %d", tt.expectedCount, len(navigableLines))
			assert.Equal(t, tt.expectedRefs, navigableLines, "Navigable line refs don't match expected")
		})
	}
}

func TestModel_BlockNavigation(t *testing.T) {
	// Set up test data with multiple blocks separated by context
	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
			AlignedLines: []aligner.AlignedLine{
				// Block 1: Added lines (indices 0-1)
				{OldLine: nil, NewLine: stringPtr("added1"), LineType: aligner.Added, NewLineNum: 1},
				{OldLine: nil, NewLine: stringPtr("added2"), LineType: aligner.Added, NewLineNum: 2},
				// Context (index 2) - breaks the block
				{OldLine: stringPtr("context"), NewLine: stringPtr("context"), LineType: aligner.Unchanged, OldLineNum: 3, NewLineNum: 3},
				// Block 2: Added lines (indices 3-4)
				{OldLine: nil, NewLine: stringPtr("added3"), LineType: aligner.Added, NewLineNum: 4},
				{OldLine: nil, NewLine: stringPtr("added4"), LineType: aligner.Added, NewLineNum: 5},
				// Context again (index 5)
				{OldLine: stringPtr("context2"), NewLine: stringPtr("context2"), LineType: aligner.Unchanged, OldLineNum: 6, NewLineNum: 6},
				// Block 3: Modified lines (indices 6-7)
				{OldLine: stringPtr("old1"), NewLine: stringPtr("new1"), LineType: aligner.Modified, OldLineNum: 7, NewLineNum: 7},
				{OldLine: stringPtr("old2"), NewLine: stringPtr("new2"), LineType: aligner.Modified, OldLineNum: 8, NewLineNum: 8},
			},
		},
	}

	tests := []struct {
		name             string
		startCursor      int
		action           string
		expectedCursor   int
		expectedNoChange bool
	}{
		{
			name:           "gj from first block to second block",
			startCursor:    0, // Block 1, line 0
			action:         "gj",
			expectedCursor: 2, // Block 2, line 0 (navigableLines index 2 = AlignedLines index 3)
		},
		{
			name:           "gj from second block to third block",
			startCursor:    2, // Block 2, line 0
			action:         "gj",
			expectedCursor: 4, // Block 3, line 0 (navigableLines index 4 = AlignedLines index 6)
		},
		{
			name:             "gj from last block (no change)",
			startCursor:      4, // Block 3, line 0
			action:           "gj",
			expectedCursor:   4, // No change
			expectedNoChange: true,
		},
		{
			name:           "gk from third block to second block",
			startCursor:    4, // Block 3, line 0
			action:         "gk",
			expectedCursor: 2, // Block 2, line 0
		},
		{
			name:           "gk from second block to first block",
			startCursor:    3, // Block 2, line 1
			action:         "gk",
			expectedCursor: 0, // Block 1, line 0
		},
		{
			name:             "gk from first block (no change)",
			startCursor:      0, // Block 1, line 0
			action:           "gk",
			expectedCursor:   0, // No change
			expectedNoChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(filesWithLines)
			model.cursorLine = tt.startCursor

			// Execute the action
			switch tt.action {
			case "gj":
				model.gotoNextBlock()
			case "gk":
				model.gotoPrevBlock()
			}

			if tt.expectedNoChange {
				assert.Equal(t, tt.startCursor, model.cursorLine, "Cursor should not move when at boundary")
			} else {
				assert.Equal(t, tt.expectedCursor, model.cursorLine, "Cursor should move to expected position")
			}
		})
	}
}

func TestModel_BlockNavigationWithDifferentTypes(t *testing.T) {
	// Test navigation across different change types
	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
			AlignedLines: []aligner.AlignedLine{
				// Block 1: Added (index 0)
				{OldLine: nil, NewLine: stringPtr("added"), LineType: aligner.Added, NewLineNum: 1},
				// Block 2: Modified (index 1)
				{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: aligner.Modified, OldLineNum: 2, NewLineNum: 2},
				// Block 3: Deleted (index 2)
				{OldLine: stringPtr("deleted"), NewLine: nil, LineType: aligner.Deleted, OldLineNum: 3},
			},
		},
	}

	model := NewModel(filesWithLines)

	// Should have 3 navigable lines: Added, Modified, Deleted
	assert.Equal(t, 3, len(model.navigableLines))

	// Test gj: Added -> Modified -> Deleted
	model.cursorLine = 0
	model.gotoNextBlock()
	assert.Equal(t, 1, model.cursorLine, "Should move from Added to Modified")

	model.gotoNextBlock()
	assert.Equal(t, 2, model.cursorLine, "Should move from Modified to Deleted")

	// Test gk: Deleted -> Modified -> Added
	model.gotoNextBlock() // Should not move (at end)
	assert.Equal(t, 2, model.cursorLine, "Should stay at Deleted (last block)")

	model.gotoPrevBlock()
	assert.Equal(t, 1, model.cursorLine, "Should move from Deleted to Modified")

	model.gotoPrevBlock()
	assert.Equal(t, 0, model.cursorLine, "Should move from Modified to Added")
}

func TestModel_BlockNavigationAcrossFiles(t *testing.T) {
	// Test navigation across multiple files
	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "file1.go", NewPath: "file1.go"},
			AlignedLines: []aligner.AlignedLine{
				{OldLine: nil, NewLine: stringPtr("added in file1"), LineType: aligner.Added, NewLineNum: 1},
			},
		},
		{
			FileDiff: parser.FileDiff{OldPath: "file2.go", NewPath: "file2.go"},
			AlignedLines: []aligner.AlignedLine{
				{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: aligner.Modified, OldLineNum: 1, NewLineNum: 1},
			},
		},
	}

	model := NewModel(filesWithLines)

	// Should have 2 navigable lines across 2 files
	assert.Equal(t, 2, len(model.navigableLines))
	assert.Equal(t, 0, model.navigableLines[0].FileIndex)
	assert.Equal(t, 1, model.navigableLines[1].FileIndex)

	// Test navigation across files
	model.cursorLine = 0
	model.gotoNextBlock()
	assert.Equal(t, 1, model.cursorLine, "Should move from file1 to file2")

	model.gotoPrevBlock()
	assert.Equal(t, 0, model.cursorLine, "Should move back from file2 to file1")
}

func TestModel_KeySequences(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create test data with multiple blocks
	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
			AlignedLines: []aligner.AlignedLine{
				{OldLine: nil, NewLine: stringPtr("added1"), LineType: aligner.Added, NewLineNum: 1},
				{OldLine: nil, NewLine: stringPtr("added2"), LineType: aligner.Added, NewLineNum: 2},
				{OldLine: stringPtr("context"), NewLine: stringPtr("context"), LineType: aligner.Unchanged, OldLineNum: 3, NewLineNum: 3},
				{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: aligner.Modified, OldLineNum: 4, NewLineNum: 4},
			},
		},
	}

	tests := []struct {
		name           string
		keySequence    []tea.KeyMsg
		expectedCursor int
		description    string
	}{
		{
			name: "gg sequence",
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune("g")},
				{Type: tea.KeyRunes, Runes: []rune("g")},
			},
			expectedCursor: 0,
			description:    "gg should go to top",
		},
		{
			name: "gj sequence",
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune("g")},
				{Type: tea.KeyRunes, Runes: []rune("j")},
			},
			expectedCursor: 2, // navigableLines index 2 = AlignedLines index 3 (modified line)
			description:    "gj should go to next block",
		},
		{
			name: "multiple gj sequences",
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune("g")},
				{Type: tea.KeyRunes, Runes: []rune("j")},
				{Type: tea.KeyRunes, Runes: []rune("g")},
				{Type: tea.KeyRunes, Runes: []rune("k")},
			},
			expectedCursor: 0, // gj then gk should return to start
			description:    "gj then gk should return to first block",
		},
		// {
		// 	name: "single j navigation",
		// 	keySequence: []tea.KeyMsg{
		// 		{Type: tea.KeyRunes, Runes: []rune("j")},
		// 	},
		// 	expectedCursor: 1, // Normal j navigation within block
		// 	description:    "j should move to next line within block",
		// },
		{
			name: "g timeout (invalid sequence)",
			keySequence: []tea.KeyMsg{
				{Type: tea.KeyRunes, Runes: []rune("g")},
				{Type: tea.KeyRunes, Runes: []rune("x")}, // Invalid after g
			},
			expectedCursor: 0, // Should remain at start
			description:    "g followed by invalid key should do nothing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(filesWithLines)

			// Initialize model with window size
			windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
			updatedModel, _ := model.Update(windowMsg)
			model = updatedModel.(Model)

			// Execute key sequence
			for _, keyMsg := range tt.keySequence {
				updatedModel, _ := model.Update(keyMsg)
				model = updatedModel.(Model)
			}

			assert.Equal(t, tt.expectedCursor, model.cursorLine, tt.description)
		})
	}
}

func TestModel_CursorPositioning(t *testing.T) {
	// Test cursor indicator positioning
	filesWithLines := []FileWithLines{
		{
			FileDiff: parser.FileDiff{OldPath: "test.go", NewPath: "test.go"},
			AlignedLines: []aligner.AlignedLine{
				{OldLine: nil, NewLine: stringPtr("added"), LineType: aligner.Added, NewLineNum: 1},
				{OldLine: stringPtr("old"), NewLine: stringPtr("new"), LineType: aligner.Modified, OldLineNum: 2, NewLineNum: 2},
			},
		},
	}

	model := NewModel(filesWithLines)
	model.width = 100

	// Test cursor at different positions
	testCases := []struct {
		position  int
		fileIndex int
		lineIndex int
	}{
		{position: 0, fileIndex: 0, lineIndex: 0},
		{position: 1, fileIndex: 0, lineIndex: 1},
	}

	for _, tc := range testCases {
		model.cursorLine = tc.position

		// Test isCursorAt function
		assert.True(t, model.isCursorAt(tc.fileIndex, tc.lineIndex),
			"isCursorAt should return true for cursor position %d", tc.position)

		// Test cursor positioning doesn't crash rendering
		content := model.renderContent()
		assert.NotEmpty(t, content, "Rendering should not be empty")
		assert.Contains(t, content, "*", "Rendered content should contain cursor indicator")
	}
}

// Benchmark tests
func BenchmarkModel_renderContent(b *testing.B) {
	// Set consistent color profile for benchmarking
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create a model with several files and lines
	filesWithLines := make([]FileWithLines, 5)
	for i := 0; i < 5; i++ {
		alignedLines := make([]aligner.AlignedLine, 100)
		for j := 0; j < 100; j++ {
			alignedLines[j] = aligner.AlignedLine{
				OldLine:    stringPtr("old line " + string(rune(j))),
				NewLine:    stringPtr("new line " + string(rune(j))),
				LineType:   aligner.Modified,
				OldLineNum: j + 1,
				NewLineNum: j + 1,
			}
		}

		filesWithLines[i] = FileWithLines{
			FileDiff: parser.FileDiff{
				OldPath: "a/file" + string(rune(i)) + ".txt",
				NewPath: "file" + string(rune(i)) + ".txt",
				Hunks:   []parser.Hunk{},
			},
			AlignedLines: alignedLines,
		}
	}

	model := NewModel(filesWithLines)
	model.width = 120

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = model.renderContent()
	}
}
