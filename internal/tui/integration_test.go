package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// stripANSI removes ANSI escape codes from a string for position-based testing
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// findRuneIndex returns the rune (character) position of substr in s, not byte position
func findRuneIndex(s, substr string) int {
	bytePos := strings.Index(s, substr)
	if bytePos < 0 {
		return -1
	}
	// Count runes up to bytePos
	return len([]rune(s[:bytePos]))
}

// Uses the 'update' flag from view_test.go

// TestFullPipeline_AddedLines tests the full pipeline with a diff containing added lines.
// This reproduces the alignment bug where the │ separator doesn't align when
// left side is empty (pure additions).
func TestFullPipeline_AddedLines(t *testing.T) {
	// This is a realistic diff with an added import line
	input := `diff --git a/internal/tui/view.go b/internal/tui/view.go
index abc123..def456 100644
--- a/internal/tui/view.go
+++ b/internal/tui/view.go
@@ -5,6 +5,7 @@
     "strings"

     "github.com/charmbracelet/lipgloss"
+    "github.com/mattn/go-runewidth"
     "github.com/user/diffyduck/pkg/sidebyside"
 )
`
	// Parse
	d, err := diff.Parse(input)
	require.NoError(t, err)

	// Transform
	files := sidebyside.TransformDiff(d)

	// Create model and render
	m := New(files)
	m.width = 100
	m.height = 20

	output := m.View()

	// Check that all lines have the separator at the same position
	lines := strings.Split(output, "\n")
	var separatorPositions []int
	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		pos := findRuneIndex(stripANSI(line), "│")
		if pos >= 0 {
			separatorPositions = append(separatorPositions, pos)
		}
	}

	// All separators should be at the same position
	if len(separatorPositions) > 0 {
		first := separatorPositions[0]
		for i, pos := range separatorPositions {
			assert.Equal(t, first, pos, "line %d has separator at position %d, expected %d", i+1, pos, first)
		}
	}

	// Golden file test
	goldenPath := filepath.Join("testdata", "integration_added_lines.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

// TestFullPipeline_MixedChanges tests added, removed, and modified lines together.
func TestFullPipeline_MixedChanges(t *testing.T) {
	input := `diff --git a/example.go b/example.go
index abc123..def456 100644
--- a/example.go
+++ b/example.go
@@ -1,7 +1,8 @@
 package main

-import "fmt"
+import (
+    "fmt"
+    "os"
+)

 func main() {
-    fmt.Println("hello")
+    fmt.Println(os.Args)
 }
`
	d, err := diff.Parse(input)
	require.NoError(t, err)

	files := sidebyside.TransformDiff(d)

	m := New(files)
	m.width = 80
	m.height = 20

	output := m.View()

	// Verify separator alignment
	lines := strings.Split(output, "\n")
	var separatorPositions []int
	for i, line := range lines {
		if i == 0 {
			continue
		}
		pos := findRuneIndex(stripANSI(line), "│")
		if pos >= 0 {
			separatorPositions = append(separatorPositions, pos)
		}
	}

	if len(separatorPositions) > 0 {
		first := separatorPositions[0]
		for i, pos := range separatorPositions {
			assert.Equal(t, first, pos, "line %d has separator at position %d, expected %d", i+1, pos, first)
		}
	}

	goldenPath := filepath.Join("testdata", "integration_mixed_changes.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

// TestFullPipeline_NewFile tests display of a completely new file.
func TestFullPipeline_NewFile(t *testing.T) {
	input := `diff --git a/new_file.go b/new_file.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new_file.go
@@ -0,0 +1,5 @@
+package main
+
+func hello() {
+    println("hello")
+}
`
	d, err := diff.Parse(input)
	require.NoError(t, err)

	files := sidebyside.TransformDiff(d)

	m := New(files)
	m.width = 80
	m.height = 20

	output := m.View()

	// All lines should have empty left side, content on right
	lines := strings.Split(output, "\n")
	var separatorPositions []int
	for i, line := range lines {
		if i == 0 {
			continue
		}
		pos := findRuneIndex(stripANSI(line), "│")
		if pos >= 0 {
			separatorPositions = append(separatorPositions, pos)
		}
	}

	if len(separatorPositions) > 0 {
		first := separatorPositions[0]
		for i, pos := range separatorPositions {
			assert.Equal(t, first, pos, "line %d has separator at position %d, expected %d", i+1, pos, first)
		}
	}

	goldenPath := filepath.Join("testdata", "integration_new_file.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}

// TestFullPipeline_TabsInContent tests that tabs are expanded correctly
// so that column alignment is preserved.
func TestFullPipeline_TabsInContent(t *testing.T) {
	// Use literal tabs in the diff content (common in Go code)
	input := "diff --git a/main.go b/main.go\n" +
		"index abc123..def456 100644\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,4 +1,5 @@\n" +
		" package main\n" +
		" \n" +
		"-\tfmt.Println(\"old\")\n" +
		"+\tfmt.Println(\"new\")\n" +
		"+\tfmt.Println(\"added\")\n" +
		" }\n"

	d, err := diff.Parse(input)
	require.NoError(t, err)

	files := sidebyside.TransformDiff(d)

	m := New(files)
	m.width = 80
	m.height = 20

	output := m.View()

	// Verify separator alignment
	lines := strings.Split(output, "\n")
	var separatorPositions []int
	for i, line := range lines {
		if i == 0 {
			continue
		}
		pos := findRuneIndex(stripANSI(line), "│")
		if pos >= 0 {
			separatorPositions = append(separatorPositions, pos)
		}
	}

	if len(separatorPositions) > 0 {
		first := separatorPositions[0]
		for i, pos := range separatorPositions {
			assert.Equal(t, first, pos, "line %d has separator at position %d, expected %d\nFull output:\n%s", i+1, pos, first, output)
		}
	}

	goldenPath := filepath.Join("testdata", "integration_tabs.golden")
	if *update {
		err := os.WriteFile(goldenPath, []byte(output), 0644)
		require.NoError(t, err)
		return
	}

	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Run with -update to create golden file")
	assert.Equal(t, string(expected), output)
}
