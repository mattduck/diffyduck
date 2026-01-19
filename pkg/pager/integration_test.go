package pager_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/pager"
)

// TestColoredDiffParsesIdentically verifies that stripping ANSI codes from
// colored git diff output produces a result that parses identically to
// the non-colored version.
func TestColoredDiffParsesIdentically(t *testing.T) {
	// Non-colored diff (what we'd get from git diff --no-color)
	plain := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,4 @@
 package main
+
 func main() {
 }
`

	// Colored version (simulating git diff --color=always)
	// Colors: 37=white (meta), 36=cyan (hunk header), 32=green (added)
	colored := "\x1b[37mdiff --git a/foo.go b/foo.go\x1b[m\n" +
		"\x1b[37mindex abc123..def456 100644\x1b[m\n" +
		"\x1b[37m--- a/foo.go\x1b[m\n" +
		"\x1b[37m+++ b/foo.go\x1b[m\n" +
		"\x1b[36m@@ -1,3 +1,4 @@\x1b[m\n" +
		" package main\n" +
		"\x1b[32m+\x1b[m\n" +
		" func main() {\n" +
		" }\n"

	// Strip colors and parse both
	stripped := pager.StripANSI(colored)

	plainDiff, err := diff.Parse(plain)
	require.NoError(t, err)

	strippedDiff, err := diff.Parse(stripped)
	require.NoError(t, err)

	// Verify they parse to the same structure
	require.Len(t, strippedDiff.Files, len(plainDiff.Files))

	for i, plainFile := range plainDiff.Files {
		strippedFile := strippedDiff.Files[i]
		assert.Equal(t, plainFile.OldPath, strippedFile.OldPath)
		assert.Equal(t, plainFile.NewPath, strippedFile.NewPath)
		require.Len(t, strippedFile.Hunks, len(plainFile.Hunks))

		for j, plainHunk := range plainFile.Hunks {
			strippedHunk := strippedFile.Hunks[j]
			assert.Equal(t, plainHunk.OldStart, strippedHunk.OldStart)
			assert.Equal(t, plainHunk.OldCount, strippedHunk.OldCount)
			assert.Equal(t, plainHunk.NewStart, strippedHunk.NewStart)
			assert.Equal(t, plainHunk.NewCount, strippedHunk.NewCount)
			require.Len(t, strippedHunk.Lines, len(plainHunk.Lines))

			for k, plainLine := range plainHunk.Lines {
				strippedLine := strippedHunk.Lines[k]
				assert.Equal(t, plainLine.Type, strippedLine.Type, "line %d type mismatch", k)
				assert.Equal(t, plainLine.Content, strippedLine.Content, "line %d content mismatch", k)
			}
		}
	}
}

func TestColoredDiffWithMultipleFiles(t *testing.T) {
	plain := `diff --git a/foo.go b/foo.go
index abc123..def456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,2 +1,2 @@
-old foo
+new foo
diff --git a/bar.go b/bar.go
index 111111..222222 100644
--- a/bar.go
+++ b/bar.go
@@ -1 +1 @@
-old bar
+new bar
`

	// Colored version with typical git colors
	colored := "\x1b[1;37mdiff --git a/foo.go b/foo.go\x1b[m\n" +
		"\x1b[37mindex abc123..def456 100644\x1b[m\n" +
		"\x1b[1;37m--- a/foo.go\x1b[m\n" +
		"\x1b[1;37m+++ b/foo.go\x1b[m\n" +
		"\x1b[36m@@ -1,2 +1,2 @@\x1b[m\n" +
		"\x1b[31m-old foo\x1b[m\n" +
		"\x1b[32m+new foo\x1b[m\n" +
		"\x1b[1;37mdiff --git a/bar.go b/bar.go\x1b[m\n" +
		"\x1b[37mindex 111111..222222 100644\x1b[m\n" +
		"\x1b[1;37m--- a/bar.go\x1b[m\n" +
		"\x1b[1;37m+++ b/bar.go\x1b[m\n" +
		"\x1b[36m@@ -1 +1 @@\x1b[m\n" +
		"\x1b[31m-old bar\x1b[m\n" +
		"\x1b[32m+new bar\x1b[m\n"

	stripped := pager.StripANSI(colored)

	plainDiff, err := diff.Parse(plain)
	require.NoError(t, err)

	strippedDiff, err := diff.Parse(stripped)
	require.NoError(t, err)

	require.Len(t, strippedDiff.Files, 2)
	assert.Equal(t, plainDiff.Files[0].OldPath, strippedDiff.Files[0].OldPath)
	assert.Equal(t, plainDiff.Files[1].OldPath, strippedDiff.Files[1].OldPath)
}

func TestColoredNewFile(t *testing.T) {
	plain := `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+package main
+func new() {}
`

	colored := "\x1b[1;37mdiff --git a/new.go b/new.go\x1b[m\n" +
		"\x1b[37mnew file mode 100644\x1b[m\n" +
		"\x1b[37mindex 0000000..abc1234\x1b[m\n" +
		"\x1b[1;37m--- /dev/null\x1b[m\n" +
		"\x1b[1;37m+++ b/new.go\x1b[m\n" +
		"\x1b[36m@@ -0,0 +1,2 @@\x1b[m\n" +
		"\x1b[32m+package main\x1b[m\n" +
		"\x1b[32m+func new() {}\x1b[m\n"

	stripped := pager.StripANSI(colored)

	plainDiff, err := diff.Parse(plain)
	require.NoError(t, err)

	strippedDiff, err := diff.Parse(stripped)
	require.NoError(t, err)

	require.Len(t, strippedDiff.Files, 1)
	assert.Equal(t, "/dev/null", strippedDiff.Files[0].OldPath)
	assert.Equal(t, "b/new.go", strippedDiff.Files[0].NewPath)
	assert.Equal(t, plainDiff.Files[0].Hunks[0].Lines, strippedDiff.Files[0].Hunks[0].Lines)
}

func TestColoredDeletedFile(t *testing.T) {
	plain := `diff --git a/old.go b/old.go
deleted file mode 100644
index abc1234..0000000
--- a/old.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package main
-func old() {}
`

	colored := "\x1b[1;37mdiff --git a/old.go b/old.go\x1b[m\n" +
		"\x1b[37mdeleted file mode 100644\x1b[m\n" +
		"\x1b[37mindex abc1234..0000000\x1b[m\n" +
		"\x1b[1;37m--- a/old.go\x1b[m\n" +
		"\x1b[1;37m+++ /dev/null\x1b[m\n" +
		"\x1b[36m@@ -1,2 +0,0 @@\x1b[m\n" +
		"\x1b[31m-package main\x1b[m\n" +
		"\x1b[31m-func old() {}\x1b[m\n"

	stripped := pager.StripANSI(colored)

	plainDiff, err := diff.Parse(plain)
	require.NoError(t, err)

	strippedDiff, err := diff.Parse(stripped)
	require.NoError(t, err)

	require.Len(t, strippedDiff.Files, 1)
	assert.Equal(t, "a/old.go", strippedDiff.Files[0].OldPath)
	assert.Equal(t, "/dev/null", strippedDiff.Files[0].NewPath)
	assert.Equal(t, plainDiff.Files[0].Hunks[0].Lines, strippedDiff.Files[0].Hunks[0].Lines)
}
