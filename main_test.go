package main

import (
	"io"
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"duckdiff/aligner"
	"duckdiff/git"
	"duckdiff/parser"
	"duckdiff/ui"
)

func TestReadStdin(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectedError  bool
	}{
		{
			name:           "simple input",
			input:          "hello world",
			expectedOutput: "hello world",
			expectedError:  false,
		},
		{
			name:           "multiline input",
			input:          "line1\nline2\nline3",
			expectedOutput: "line1\nline2\nline3",
			expectedError:  false,
		},
		{
			name:           "empty input",
			input:          "",
			expectedOutput: "",
			expectedError:  false,
		},
		{
			name:           "input with trailing newline",
			input:          "content\n",
			expectedOutput: "content\n",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Redirect stdin
			originalStdin := os.Stdin
			defer func() { os.Stdin = originalStdin }()

			r, w, err := os.Pipe()
			require.NoError(t, err)

			os.Stdin = r

			// Write test input
			go func() {
				defer w.Close()
				w.WriteString(tt.input)
			}()

			// Test readStdin function
			result, err := readStdin()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedOutput, result)
			}
		})
	}
}

func TestIntegration_DiffParsing(t *testing.T) {
	// Test the complete diff parsing pipeline
	diffContent := `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }`

	// Parse the diff
	diffParser := parser.NewDiffParser()
	fileDiffs, err := diffParser.Parse(diffContent)
	require.NoError(t, err)
	require.Len(t, fileDiffs, 1)

	// Verify parsed content
	fileDiff := fileDiffs[0]
	assert.Equal(t, "a/test.go", fileDiff.OldPath)
	assert.Equal(t, "test.go", fileDiff.NewPath)
	require.Len(t, fileDiff.Hunks, 1)

	hunk := fileDiff.Hunks[0]
	assert.Equal(t, 1, hunk.OldStart)
	assert.Equal(t, 3, hunk.OldCount)
	assert.Equal(t, 1, hunk.NewStart)
	assert.Equal(t, 3, hunk.NewCount)

	expectedLines := []string{
		" func main() {",
		`-	fmt.Println("hello")`,
		`+	fmt.Println("world")`,
		" }",
	}
	assert.Equal(t, expectedLines, hunk.Lines)
}

func TestIntegration_DiffAlignment(t *testing.T) {
	// Test the complete alignment pipeline
	oldLines := []string{
		"func main() {",
		`	fmt.Println("hello")`,
		"}",
	}

	newLines := []string{
		"func main() {",
		`	fmt.Println("world")`,
		"}",
	}

	hunks := []parser.Hunk{
		{
			OldStart: 1,
			OldCount: 3,
			NewStart: 1,
			NewCount: 3,
			Lines: []string{
				" func main() {",
				`-	fmt.Println("hello")`,
				`+	fmt.Println("world")`,
				" }",
			},
		},
	}

	// Align the file
	diffAligner := aligner.NewDiffAligner()
	alignedLines := diffAligner.AlignFile(oldLines, newLines, hunks)

	require.Len(t, alignedLines, 3)

	// Verify alignment
	assert.Equal(t, aligner.Unchanged, alignedLines[0].LineType)
	assert.Equal(t, "func main() {", *alignedLines[0].OldLine)
	assert.Equal(t, "func main() {", *alignedLines[0].NewLine)

	assert.Equal(t, aligner.Modified, alignedLines[1].LineType)
	assert.Equal(t, `	fmt.Println("hello")`, *alignedLines[1].OldLine)
	assert.Equal(t, `	fmt.Println("world")`, *alignedLines[1].NewLine)

	assert.Equal(t, aligner.Unchanged, alignedLines[2].LineType)
	assert.Equal(t, "}", *alignedLines[2].OldLine)
	assert.Equal(t, "}", *alignedLines[2].NewLine)
}

func TestIntegration_UIRendering(t *testing.T) {
	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Test the complete UI rendering pipeline
	filesWithLines := []ui.FileWithLines{
		{
			FileDiff: parser.FileDiff{
				OldPath: "a/test.go",
				NewPath: "test.go",
				Hunks:   []parser.Hunk{},
			},
			OldFileType: git.TextFile,
			NewFileType: git.TextFile,
			AlignedLines: []aligner.AlignedLine{
				{
					OldLine:    stringPtr("func main() {"),
					NewLine:    stringPtr("func main() {"),
					LineType:   aligner.Unchanged,
					OldLineNum: 1,
					NewLineNum: 1,
				},
				{
					OldLine:    stringPtr(`	fmt.Println("hello")`),
					NewLine:    stringPtr(`	fmt.Println("world")`),
					LineType:   aligner.Modified,
					OldLineNum: 2,
					NewLineNum: 2,
				},
				{
					OldLine:    stringPtr("}"),
					NewLine:    stringPtr("}"),
					LineType:   aligner.Unchanged,
					OldLineNum: 3,
					NewLineNum: 3,
				},
			},
		},
	}

	// Create and test the model
	model := ui.NewModel(filesWithLines)
	assert.NotNil(t, model)

	// Initialize the model with a window size
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, cmd := model.Update(windowMsg)
	assert.Nil(t, cmd)

	// Get the view
	view := updatedModel.View()
	assert.NotEmpty(t, view)

	// Check that the view contains expected content
	assert.Contains(t, view, "test.go")
	// Check for content with or without ANSI color codes
	assert.Regexp(t, `func.*main\(\)`, view)
	assert.Contains(t, view, "hello")
	assert.Contains(t, view, "world")
}

func TestIntegration_EndToEnd_WithTeatest(t *testing.T) {
	// Skip if running in CI or if display is not available
	if os.Getenv("CI") != "" {
		t.Skip("Skipping teatest in CI environment")
	}

	// Set consistent color profile for testing
	lipgloss.SetColorProfile(termenv.TrueColor)

	// Create a complete end-to-end test data
	diffContent := `diff --git a/example.txt b/example.txt
index abc123..def456 100644
--- a/example.txt
+++ b/example.txt
@@ -1,2 +1,2 @@
 line 1
-old line 2
+new line 2`

	// Mock the file content retrieval (since we can't rely on actual git/files in tests)
	oldLines := []string{"line 1", "old line 2"}
	newLines := []string{"line 1", "new line 2"}

	// Parse diff
	diffParser := parser.NewDiffParser()
	fileDiffs, err := diffParser.Parse(diffContent)
	require.NoError(t, err)
	require.Len(t, fileDiffs, 1)

	// Align files
	diffAligner := aligner.NewDiffAligner()
	alignedLines := diffAligner.AlignFile(oldLines, newLines, fileDiffs[0].Hunks)

	filesWithLines := []ui.FileWithLines{
		{
			FileDiff:     fileDiffs[0],
			AlignedLines: alignedLines,
		},
	}

	// Create UI model
	model := ui.NewModel(filesWithLines)

	// Test with teatest
	tm := teatest.NewTestModel(
		t, model,
		teatest.WithInitialTermSize(80, 24),
	)

	// Send a quit command to exit gracefully
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// Wait for the program to finish
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))

	// Get and verify final output
	out := tm.FinalOutput(t)
	require.NotNil(t, out)

	outBytes, err := io.ReadAll(out)
	require.NoError(t, err)
	assert.Greater(t, len(outBytes), 0, "Expected some output from the complete pipeline")
}

func TestIntegration_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		diffContent string
		expectError bool
	}{
		{
			name:        "empty diff content",
			diffContent: "",
			expectError: false, // Parser should handle empty content gracefully
		},
		{
			name:        "malformed diff",
			diffContent: "not a valid diff",
			expectError: false, // Parser should handle invalid content gracefully
		},
		{
			name: "diff with no hunks",
			diffContent: `diff --git a/test.txt b/test.txt
index abc123..def456 100644
--- a/test.txt
+++ b/test.txt`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the pipeline doesn't crash with various inputs
			diffParser := parser.NewDiffParser()
			fileDiffs, err := diffParser.Parse(tt.diffContent)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Even with empty or malformed diffs, we should be able to create a UI
			var filesWithLines []ui.FileWithLines

			for _, fileDiff := range fileDiffs {
				// Mock empty file content for this test
				oldLines := []string{}
				newLines := []string{}

				diffAligner := aligner.NewDiffAligner()
				alignedLines := diffAligner.AlignFile(oldLines, newLines, fileDiff.Hunks)

				filesWithLines = append(filesWithLines, ui.FileWithLines{
					FileDiff:     fileDiff,
					AlignedLines: alignedLines,
				})
			}

			// Should not panic when creating model
			model := ui.NewModel(filesWithLines)
			assert.NotNil(t, model)

			// Should not panic when rendering
			cmd := model.Init()
			assert.Nil(t, cmd)
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests for the complete pipeline
func BenchmarkIntegration_CompletePipeline(b *testing.B) {
	diffContent := `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -1,3 +1,3 @@
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }`

	oldLines := []string{
		"func main() {",
		`	fmt.Println("hello")`,
		"}",
	}

	newLines := []string{
		"func main() {",
		`	fmt.Println("world")`,
		"}",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Parse
		diffParser := parser.NewDiffParser()
		fileDiffs, err := diffParser.Parse(diffContent)
		if err != nil {
			b.Fatal(err)
		}

		// Align
		diffAligner := aligner.NewDiffAligner()
		alignedLines := diffAligner.AlignFile(oldLines, newLines, fileDiffs[0].Hunks)

		// Create UI model
		filesWithLines := []ui.FileWithLines{
			{
				FileDiff:     fileDiffs[0],
				AlignedLines: alignedLines,
			},
		}

		model := ui.NewModel(filesWithLines)

		// Initialize (this triggers content rendering)
		windowMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
		model.Update(windowMsg)
	}
}
