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
	assert.Equal(t, 29, modelCast.viewport.Height) // height - 1
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
		name        string
		key         string
		expectQuit  bool
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
		name            string
		filesWithLines  []FileWithLines
		width           int
		containsText    []string
	}{
		{
			name:            "empty files",
			filesWithLines:  []FileWithLines{},
			width:           100,
			containsText:    []string{},
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