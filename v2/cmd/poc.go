package cmd

import (
	"fmt"

	"github.com/mattduck/diffyduck/aligner"
	"github.com/mattduck/diffyduck/git"
	"github.com/mattduck/diffyduck/parser"
	"github.com/mattduck/diffyduck/v2/app"
	"github.com/mattduck/diffyduck/v2/models"
)

// RunPOC runs the proof of concept virtual viewport demo
func RunPOC() error {
	// Get git diff output
	input, err := getGitDiffForPOC()
	if err != nil {
		return fmt.Errorf("failed to get git diff: %v", err)
	}

	if input == "" {
		// Create a synthetic large diff for testing
		input = createSyntheticDiff()
	}

	// Parse the diff
	diffParser := parser.NewDiffParser()
	fileDiffs, err := diffParser.Parse(input)
	if err != nil {
		return fmt.Errorf("failed to parse diff: %v", err)
	}

	// Convert to models.FileWithLines format
	var filesWithLines []models.FileWithLines
	retriever := git.NewFileRetriever()
	diffAligner := aligner.NewDiffAligner()

	for _, fileDiff := range fileDiffs {
		oldFileInfo, err := retriever.GetFileInfo(fileDiff.OldPath, true)
		if err != nil {
			return fmt.Errorf("error getting old file info: %v", err)
		}

		newFileInfo, err := retriever.GetFileInfo(fileDiff.NewPath, false)
		if err != nil {
			return fmt.Errorf("error getting new file info: %v", err)
		}

		var alignedLines []aligner.AlignedLine
		isBinaryFile := oldFileInfo.Type == git.BinaryFile || newFileInfo.Type == git.BinaryFile
		if !isBinaryFile {
			alignedLines = diffAligner.AlignFile(oldFileInfo.Lines, newFileInfo.Lines, fileDiff.Hunks)
		}

		filesWithLines = append(filesWithLines, models.FileWithLines{
			FileDiff:     fileDiff,
			AlignedLines: alignedLines,
			OldFileType:  oldFileInfo.Type,
			NewFileType:  newFileInfo.Type,
		})
	}

	// For POC, always use large synthetic data to demonstrate performance
	filesWithLines = createSyntheticFileData()

	// Create and run the POC app
	pocApp, err := app.NewPOCApp(filesWithLines)
	if err != nil {
		return fmt.Errorf("failed to create POC app: %v", err)
	}

	return pocApp.Run()
}

// getGitDiffForPOC gets git diff output for the POC
func getGitDiffForPOC() (string, error) {
	// Use a simple approach since we're in POC mode
	return "", nil // For now, we'll use synthetic data
}

// createSyntheticDiff creates a large synthetic diff for performance testing
func createSyntheticDiff() string {
	return `diff --git a/large_file.go b/large_file.go
index 1234567..8901234 100644
--- a/large_file.go
+++ b/large_file.go
@@ -1,1000 +1,1000 @@
 package main

 import (
-	"fmt"
+	"fmt"
	"os"
+	"time"
 )

 func main() {
-	fmt.Println("Hello World")
+	fmt.Println("Hello POC World")
+	time.Sleep(1 * time.Second)
 }`
}

// createSyntheticFileData creates synthetic file data for performance testing
func createSyntheticFileData() []models.FileWithLines {
	// Create a large synthetic file with many lines for testing
	var alignedLines []aligner.AlignedLine

	// Generate 2000 lines of synthetic diff content
	for i := 0; i < 2000; i++ {
		var lineType aligner.LineType
		var oldLine, newLine *string

		if i%10 == 0 {
			// Every 10th line is deleted
			lineType = aligner.Deleted
			content := fmt.Sprintf("// This is old line %d with some code content that makes it long enough to test horizontal scrolling functionality", i)
			oldLine = &content
		} else if i%10 == 1 {
			// Every 10th+1 line is added
			lineType = aligner.Added
			content := fmt.Sprintf("// This is new line %d with some updated code content that makes it long enough to test horizontal scrolling functionality", i)
			newLine = &content
		} else if i%10 == 2 {
			// Every 10th+2 line is modified
			lineType = aligner.Modified
			oldContent := fmt.Sprintf("func oldFunction%d() { return fmt.Sprintf(\"old implementation %%d\", %d) }", i, i)
			newContent := fmt.Sprintf("func newFunction%d() { return fmt.Sprintf(\"new implementation %%d\", %d) }", i, i)
			oldLine = &oldContent
			newLine = &newContent
		} else {
			// Other lines are unchanged
			lineType = aligner.Unchanged
			content := fmt.Sprintf("    unchanged line %d with regular content that also needs to be long enough for horizontal scroll testing", i)
			oldLine = &content
			newLine = &content
		}

		alignedLines = append(alignedLines, aligner.AlignedLine{
			OldLine:    oldLine,
			NewLine:    newLine,
			LineType:   lineType,
			OldLineNum: i + 1,
			NewLineNum: i + 1,
		})
	}

	// Create a file diff
	fileDiff := parser.FileDiff{
		OldPath:   "test/large_file.go",
		NewPath:   "test/large_file.go",
		Additions: 500,
		Deletions: 300,
	}

	return []models.FileWithLines{
		{
			FileDiff:     fileDiff,
			AlignedLines: alignedLines,
			OldFileType:  git.TextFile,
			NewFileType:  git.TextFile,
		},
	}
}
