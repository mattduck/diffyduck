package diff

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_SingleFileWithOneHunk(t *testing.T) {
	input := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+
 func main() {
 }
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/foo.go", file.OldPath)
	assert.Equal(t, "b/foo.go", file.NewPath)
	require.Len(t, file.Hunks, 1)

	hunk := file.Hunks[0]
	assert.Equal(t, 1, hunk.OldStart)
	assert.Equal(t, 3, hunk.OldCount)
	assert.Equal(t, 1, hunk.NewStart)
	assert.Equal(t, 4, hunk.NewCount)

	require.Len(t, hunk.Lines, 4)
	assert.Equal(t, Line{Type: Context, Content: "package main"}, hunk.Lines[0])
	assert.Equal(t, Line{Type: Added, Content: ""}, hunk.Lines[1])
	assert.Equal(t, Line{Type: Context, Content: "func main() {"}, hunk.Lines[2])
	assert.Equal(t, Line{Type: Context, Content: "}"}, hunk.Lines[3])
}

func TestParse_SingleFileWithDeletion(t *testing.T) {
	input := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,4 +1,3 @@
 package main
-
 func main() {
 }
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	hunk := diff.Files[0].Hunks[0]
	require.Len(t, hunk.Lines, 4)
	assert.Equal(t, Line{Type: Context, Content: "package main"}, hunk.Lines[0])
	assert.Equal(t, Line{Type: Removed, Content: ""}, hunk.Lines[1])
	assert.Equal(t, Line{Type: Context, Content: "func main() {"}, hunk.Lines[2])
	assert.Equal(t, Line{Type: Context, Content: "}"}, hunk.Lines[3])
}

func TestParse_MultipleFiles(t *testing.T) {
	input := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,2 +1,2 @@
-old
+new
diff --git a/bar.go b/bar.go
index 111111..222222 100644
--- a/bar.go
+++ b/bar.go
@@ -1 +1 @@
-bar old
+bar new
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 2)

	assert.Equal(t, "a/foo.go", diff.Files[0].OldPath)
	assert.Equal(t, "b/foo.go", diff.Files[0].NewPath)
	assert.Equal(t, "a/bar.go", diff.Files[1].OldPath)
	assert.Equal(t, "b/bar.go", diff.Files[1].NewPath)

	// Verify hunks are preserved for BOTH files (regression test)
	require.Len(t, diff.Files[0].Hunks, 1, "first file should have 1 hunk")
	require.Len(t, diff.Files[1].Hunks, 1, "second file should have 1 hunk")
	assert.Len(t, diff.Files[0].Hunks[0].Lines, 2, "first file's hunk should have 2 lines")
	assert.Len(t, diff.Files[1].Hunks[0].Lines, 2, "second file's hunk should have 2 lines")
}

func TestParse_MultipleHunks(t *testing.T) {
	input := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,3 @@
 line1
-old2
+new2
 line3
@@ -10,3 +10,3 @@
 line10
-old11
+new11
 line12
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	require.Len(t, diff.Files[0].Hunks, 2)

	assert.Equal(t, 1, diff.Files[0].Hunks[0].OldStart)
	assert.Equal(t, 10, diff.Files[0].Hunks[1].OldStart)
}

func TestParse_NewFile(t *testing.T) {
	input := `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.go
@@ -0,0 +1,3 @@
+package main
+
+func new() {}
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "/dev/null", file.OldPath)
	assert.Equal(t, "b/new.go", file.NewPath)
	require.Len(t, file.Hunks, 1)
	require.Len(t, file.Hunks[0].Lines, 3)
	assert.Equal(t, Added, file.Hunks[0].Lines[0].Type)
	assert.Equal(t, Added, file.Hunks[0].Lines[1].Type)
	assert.Equal(t, Added, file.Hunks[0].Lines[2].Type)
}

func TestParse_DeletedFile(t *testing.T) {
	input := `diff --git a/old.go b/old.go
deleted file mode 100644
index abc1234..0000000
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package main
-
-func old() {}
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/old.go", file.OldPath)
	assert.Equal(t, "/dev/null", file.NewPath)
	require.Len(t, file.Hunks, 1)
	require.Len(t, file.Hunks[0].Lines, 3)
	assert.Equal(t, Removed, file.Hunks[0].Lines[0].Type)
}

func TestParse_NewEmptyFile(t *testing.T) {
	// Empty new file has no --- or +++ lines, no hunks
	// Must detect "new file mode" to set OldPath correctly
	input := `diff --git a/empty.txt b/empty.txt
new file mode 100644
index 0000000..e69de29
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "/dev/null", file.OldPath, "new file should have /dev/null as OldPath")
	assert.Equal(t, "b/empty.txt", file.NewPath)
	assert.Len(t, file.Hunks, 0, "empty file should have no hunks")
}

func TestParse_DeletedEmptyFile(t *testing.T) {
	// Empty deleted file has no --- or +++ lines, no hunks
	// Must detect "deleted file mode" to set NewPath correctly
	input := `diff --git a/empty.txt b/empty.txt
deleted file mode 100644
index e69de29..0000000
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/empty.txt", file.OldPath)
	assert.Equal(t, "/dev/null", file.NewPath, "deleted file should have /dev/null as NewPath")
	assert.Len(t, file.Hunks, 0, "empty file should have no hunks")
}

func TestParse_RenamedFile_PureRename(t *testing.T) {
	// Pure rename with 100% similarity - no --- or +++ lines, no hunks
	input := `diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/old.go", file.OldPath)
	assert.Equal(t, "b/new.go", file.NewPath)
	assert.True(t, file.IsRename)
	assert.False(t, file.IsCopy)
	assert.Equal(t, 100, file.Similarity)
	assert.Len(t, file.Hunks, 0)
}

func TestParse_RenamedFile_WithChanges(t *testing.T) {
	// Rename with content changes
	input := `diff --git a/old.go b/new.go
similarity index 80%
rename from old.go
rename to new.go
index abc123..def456 100644
--- a/old.go
+++ b/new.go
@@ -1,2 +1,3 @@
 package main
+
+func new() {}
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/old.go", file.OldPath)
	assert.Equal(t, "b/new.go", file.NewPath)
	assert.True(t, file.IsRename)
	assert.False(t, file.IsCopy)
	assert.Equal(t, 80, file.Similarity)
	require.Len(t, file.Hunks, 1)
	assert.Equal(t, 2, file.TotalAdded)
}

func TestParse_CopiedFile(t *testing.T) {
	// Copy with modifications
	input := `diff --git a/original.go b/copy.go
similarity index 90%
copy from original.go
copy to copy.go
index abc123..def456 100644
--- a/original.go
+++ b/copy.go
@@ -1 +1,2 @@
 package main
+func copied() {}
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/original.go", file.OldPath)
	assert.Equal(t, "b/copy.go", file.NewPath)
	assert.False(t, file.IsRename)
	assert.True(t, file.IsCopy)
	assert.Equal(t, 90, file.Similarity)
}

func TestParse_BinaryFile_New(t *testing.T) {
	// New binary file
	input := `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..abc1234
Binary files /dev/null and b/image.png differ
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	// Paths come from "Binary files" line, not "diff --git"
	assert.Equal(t, "/dev/null", file.OldPath)
	assert.Equal(t, "b/image.png", file.NewPath)
	assert.True(t, file.IsBinary)
	assert.Len(t, file.Hunks, 0) // Binary files have no hunks
}

func TestParse_BinaryFile_Deleted(t *testing.T) {
	// Deleted binary file
	input := `diff --git a/image.png b/image.png
deleted file mode 100644
index abc1234..0000000
Binary files a/image.png and /dev/null differ
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	// Paths come from "Binary files" line
	assert.Equal(t, "a/image.png", file.OldPath)
	assert.Equal(t, "/dev/null", file.NewPath)
	assert.True(t, file.IsBinary)
	assert.Len(t, file.Hunks, 0)
}

func TestParse_BinaryFile_Modified(t *testing.T) {
	// Modified binary file
	input := `diff --git a/image.png b/image.png
index abc1234..def5678 100644
Binary files a/image.png and b/image.png differ
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/image.png", file.OldPath)
	assert.Equal(t, "b/image.png", file.NewPath)
	assert.True(t, file.IsBinary)
	assert.Len(t, file.Hunks, 0)
}

func TestParse_EmptyInput(t *testing.T) {
	diff, err := Parse("")
	require.NoError(t, err)
	assert.Len(t, diff.Files, 0)
}

func TestParse_NoNewlineAtEndOfFile(t *testing.T) {
	input := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1 +1 @@
-old
\ No newline at end of file
+new
\ No newline at end of file
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	require.Len(t, diff.Files[0].Hunks, 1)

	hunk := diff.Files[0].Hunks[0]
	// The "\ No newline" lines should be ignored or handled gracefully
	assert.Equal(t, Removed, hunk.Lines[0].Type)
	assert.Equal(t, "old", hunk.Lines[0].Content)
}

func TestParse_StandardUnifiedDiff_NoDiffGitHeader(t *testing.T) {
	// Standard unified diff format (from diff -u) without "diff --git" header
	input := `--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+
 func main() {
 }
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "a/foo.go", file.OldPath)
	assert.Equal(t, "b/foo.go", file.NewPath)
	require.Len(t, file.Hunks, 1)

	hunk := file.Hunks[0]
	assert.Equal(t, 1, hunk.OldStart)
	assert.Equal(t, 3, hunk.OldCount)
	assert.Equal(t, 1, hunk.NewStart)
	assert.Equal(t, 4, hunk.NewCount)

	require.Len(t, hunk.Lines, 4)
	assert.Equal(t, Line{Type: Context, Content: "package main"}, hunk.Lines[0])
	assert.Equal(t, Line{Type: Added, Content: ""}, hunk.Lines[1])
}

func TestParse_StandardUnifiedDiff_MultipleFiles(t *testing.T) {
	// Multiple files in standard unified diff format
	input := `--- a/foo.go
+++ b/foo.go
@@ -1 +1 @@
-old foo
+new foo
--- a/bar.go
+++ b/bar.go
@@ -1 +1 @@
-old bar
+new bar
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 2)

	assert.Equal(t, "a/foo.go", diff.Files[0].OldPath)
	assert.Equal(t, "b/foo.go", diff.Files[0].NewPath)
	assert.Equal(t, "a/bar.go", diff.Files[1].OldPath)
	assert.Equal(t, "b/bar.go", diff.Files[1].NewPath)

	require.Len(t, diff.Files[0].Hunks, 1)
	require.Len(t, diff.Files[1].Hunks, 1)
}

func TestParse_StandardUnifiedDiff_FullPaths(t *testing.T) {
	// Standard diff with full paths (no a/ b/ prefix)
	input := `--- /Users/matt/project/foo.py
+++ /Users/matt/project/foo.py
@@ -202 +202 @@
-    old line
+    new line
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.Equal(t, "/Users/matt/project/foo.py", file.OldPath)
	assert.Equal(t, "/Users/matt/project/foo.py", file.NewPath)
}

func TestParse_LongLineTruncation(t *testing.T) {
	// Create a line that exceeds MaxLineLength
	longContent := strings.Repeat("x", MaxLineLength+100)
	expectedTruncated := strings.Repeat("x", MaxLineLength-len(LineTruncationText)) + LineTruncationText

	input := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,3 @@
 short line
-` + longContent + `
+` + longContent + `
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)
	require.Len(t, diff.Files[0].Hunks, 1)

	hunk := diff.Files[0].Hunks[0]
	require.Len(t, hunk.Lines, 3)

	// First line should not be truncated (it's short)
	assert.Equal(t, "short line", hunk.Lines[0].Content)
	assert.Equal(t, Context, hunk.Lines[0].Type)

	// Second line (removed) should be truncated
	assert.Equal(t, expectedTruncated, hunk.Lines[1].Content)
	assert.Equal(t, Removed, hunk.Lines[1].Type)
	assert.Len(t, hunk.Lines[1].Content, MaxLineLength)

	// Third line (added) should be truncated
	assert.Equal(t, expectedTruncated, hunk.Lines[2].Content)
	assert.Equal(t, Added, hunk.Lines[2].Type)
	assert.Len(t, hunk.Lines[2].Content, MaxLineLength)
}

func TestParse_LineExactlyAtLimit(t *testing.T) {
	// A line exactly at MaxLineLength should NOT be truncated
	exactContent := strings.Repeat("y", MaxLineLength)

	input := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1 +1 @@
-` + exactContent + `
+short
`
	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files[0].Hunks[0].Lines, 2)

	// Line exactly at limit should be unchanged
	assert.Equal(t, exactContent, diff.Files[0].Hunks[0].Lines[0].Content)
	assert.Len(t, diff.Files[0].Hunks[0].Lines[0].Content, MaxLineLength)
}

func TestParse_FileLineCountTruncation(t *testing.T) {
	// Build a diff with more lines than MaxLinesPerFile
	var sb strings.Builder
	sb.WriteString("diff --git a/large.go b/large.go\n")
	sb.WriteString("--- a/large.go\n")
	sb.WriteString("+++ b/large.go\n")
	sb.WriteString("@@ -1,100000 +1,100000 @@\n")

	// Add more lines than the limit
	for i := 0; i < MaxLinesPerFile+500; i++ {
		sb.WriteString("+line " + strconv.Itoa(i) + "\n")
	}

	diff, err := Parse(sb.String())
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.True(t, file.Truncated, "file should be marked as truncated")

	// Count total lines across all hunks
	totalLines := 0
	for _, h := range file.Hunks {
		totalLines += len(h.Lines)
	}
	assert.Equal(t, MaxLinesPerFile, totalLines, "should have exactly MaxLinesPerFile lines")
}

func TestParse_FileLineCountExactlyAtLimit(t *testing.T) {
	// Build a diff with exactly MaxLinesPerFile lines
	var sb strings.Builder
	sb.WriteString("diff --git a/exact.go b/exact.go\n")
	sb.WriteString("--- a/exact.go\n")
	sb.WriteString("+++ b/exact.go\n")
	sb.WriteString("@@ -1,10000 +1,10000 @@\n")

	for i := 0; i < MaxLinesPerFile; i++ {
		sb.WriteString("+line " + strconv.Itoa(i) + "\n")
	}

	diff, err := Parse(sb.String())
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.False(t, file.Truncated, "file at exact limit should NOT be marked as truncated")

	totalLines := 0
	for _, h := range file.Hunks {
		totalLines += len(h.Lines)
	}
	assert.Equal(t, MaxLinesPerFile, totalLines)
}

func TestParse_MultipleFilesWithTruncation(t *testing.T) {
	// First file has too many lines, second file is small
	var sb strings.Builder

	// Large file
	sb.WriteString("diff --git a/large.go b/large.go\n")
	sb.WriteString("--- a/large.go\n")
	sb.WriteString("+++ b/large.go\n")
	sb.WriteString("@@ -1,100000 +1,100000 @@\n")
	for i := 0; i < MaxLinesPerFile+100; i++ {
		sb.WriteString("+line " + strconv.Itoa(i) + "\n")
	}

	// Small file
	sb.WriteString("diff --git a/small.go b/small.go\n")
	sb.WriteString("--- a/small.go\n")
	sb.WriteString("+++ b/small.go\n")
	sb.WriteString("@@ -1 +1 @@\n")
	sb.WriteString("-old\n")
	sb.WriteString("+new\n")

	diff, err := Parse(sb.String())
	require.NoError(t, err)
	require.Len(t, diff.Files, 2)

	// First file should be truncated
	assert.True(t, diff.Files[0].Truncated)

	// Second file should NOT be truncated (line count resets per file)
	assert.False(t, diff.Files[1].Truncated)
	assert.Len(t, diff.Files[1].Hunks[0].Lines, 2)
}

func TestParse_TotalAddedRemovedAccurateWhenTruncated(t *testing.T) {
	// Build a diff with more lines than MaxLinesPerFile
	// TotalAdded and TotalRemoved should count ALL lines, not just stored ones
	var sb strings.Builder
	sb.WriteString("diff --git a/large.go b/large.go\n")
	sb.WriteString("--- a/large.go\n")
	sb.WriteString("+++ b/large.go\n")
	sb.WriteString("@@ -1,15000 +1,15000 @@\n")

	totalAdded := 12000
	totalRemoved := 8000

	// Add lines (more than MaxLinesPerFile total)
	for i := 0; i < totalAdded; i++ {
		sb.WriteString("+added line " + strconv.Itoa(i) + "\n")
	}
	for i := 0; i < totalRemoved; i++ {
		sb.WriteString("-removed line " + strconv.Itoa(i) + "\n")
	}

	diff, err := Parse(sb.String())
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.True(t, file.Truncated, "file should be truncated")
	assert.Equal(t, totalAdded, file.TotalAdded, "TotalAdded should count all added lines, not just stored ones")
	assert.Equal(t, totalRemoved, file.TotalRemoved, "TotalRemoved should count all removed lines, not just stored ones")

	// Verify stored lines are limited
	storedLines := 0
	for _, h := range file.Hunks {
		storedLines += len(h.Lines)
	}
	assert.Equal(t, MaxLinesPerFile, storedLines, "stored lines should be limited to MaxLinesPerFile")
}

func TestParse_TotalAddedRemovedNormalCase(t *testing.T) {
	// Small diff - TotalAdded/TotalRemoved should match stored line counts
	input := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 context
-removed1
-removed2
+added1
+added2
+added3
 more context`

	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 1)

	file := diff.Files[0]
	assert.False(t, file.Truncated)
	assert.Equal(t, 3, file.TotalAdded)
	assert.Equal(t, 2, file.TotalRemoved)
}

func TestParse_FileCountTruncation(t *testing.T) {
	// Build a diff with more files than MaxFiles
	var sb strings.Builder

	extraFiles := 15
	totalFiles := MaxFiles + extraFiles

	for i := 0; i < totalFiles; i++ {
		sb.WriteString("diff --git a/file" + strconv.Itoa(i) + ".go b/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("--- a/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("+++ b/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("@@ -1 +1 @@\n")
		sb.WriteString("-old\n")
		sb.WriteString("+new\n")
	}

	diff, err := Parse(sb.String())
	require.NoError(t, err)

	assert.Len(t, diff.Files, MaxFiles, "should have exactly MaxFiles files")
	assert.Equal(t, extraFiles, diff.TruncatedFileCount, "should report correct truncated file count")
}

func TestParse_FileCountExactlyAtLimit(t *testing.T) {
	// Build a diff with exactly MaxFiles files
	var sb strings.Builder

	for i := 0; i < MaxFiles; i++ {
		sb.WriteString("diff --git a/file" + strconv.Itoa(i) + ".go b/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("--- a/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("+++ b/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("@@ -1 +1 @@\n")
		sb.WriteString("-old\n")
		sb.WriteString("+new\n")
	}

	diff, err := Parse(sb.String())
	require.NoError(t, err)

	assert.Len(t, diff.Files, MaxFiles)
	assert.Equal(t, 0, diff.TruncatedFileCount, "should not truncate when exactly at limit")
}

func TestParse_FileCountTruncation_StandardUnifiedDiff(t *testing.T) {
	// Test with standard unified diff format (no "diff --git" header)
	var sb strings.Builder

	extraFiles := 5
	totalFiles := MaxFiles + extraFiles

	for i := 0; i < totalFiles; i++ {
		sb.WriteString("--- a/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("+++ b/file" + strconv.Itoa(i) + ".go\n")
		sb.WriteString("@@ -1 +1 @@\n")
		sb.WriteString("-old\n")
		sb.WriteString("+new\n")
	}

	diff, err := Parse(sb.String())
	require.NoError(t, err)

	assert.Len(t, diff.Files, MaxFiles, "should have exactly MaxFiles files")
	assert.Equal(t, extraFiles, diff.TruncatedFileCount, "should report correct truncated file count")
}

func TestParse_CombinedDiffWithUntrackedFiles(t *testing.T) {
	// This simulates the output of getDiffAll: git diff HEAD + git diff --no-index for untracked files
	// The untracked file diff from "git diff --no-index /dev/null <file>" has a slightly different format
	input := `diff --git a/existing.go b/existing.go
index abc123..def456 100644
--- a/existing.go
+++ b/existing.go
@@ -1,3 +1,4 @@
 package main

+// new comment
 func main() {}
diff --git a/untracked.txt b/untracked.txt
new file mode 100644
index 0000000..d670460
--- /dev/null
+++ b/untracked.txt
@@ -0,0 +1,3 @@
+line 1
+line 2
+line 3
`

	diff, err := Parse(input)
	require.NoError(t, err)
	require.Len(t, diff.Files, 2)

	// First file: modified existing file
	existing := diff.Files[0]
	assert.Equal(t, "a/existing.go", existing.OldPath)
	assert.Equal(t, "b/existing.go", existing.NewPath)
	assert.Equal(t, 1, existing.TotalAdded)
	assert.Equal(t, 0, existing.TotalRemoved)

	// Second file: new untracked file
	untracked := diff.Files[1]
	assert.Equal(t, "/dev/null", untracked.OldPath)
	assert.Equal(t, "b/untracked.txt", untracked.NewPath)
	assert.Equal(t, 3, untracked.TotalAdded)
	assert.Equal(t, 0, untracked.TotalRemoved)
}
