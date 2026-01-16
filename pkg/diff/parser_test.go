package diff

import (
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
