package pager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripANSI_Empty(t *testing.T) {
	assert.Equal(t, "", StripANSI(""))
}

func TestStripANSI_NoANSI(t *testing.T) {
	input := "plain text without any escape sequences"
	assert.Equal(t, input, StripANSI(input))
}

func TestStripANSI_SimpleColor(t *testing.T) {
	// \x1b[32m = green, \x1b[m = reset
	input := "\x1b[32mgreen text\x1b[m"
	assert.Equal(t, "green text", StripANSI(input))
}

func TestStripANSI_MultipleColors(t *testing.T) {
	// Red then green then reset
	input := "\x1b[31mred\x1b[32mgreen\x1b[m"
	assert.Equal(t, "redgreen", StripANSI(input))
}

func TestStripANSI_ColorWithSemicolon(t *testing.T) {
	// Bold red: \x1b[1;31m
	input := "\x1b[1;31mbold red\x1b[0m"
	assert.Equal(t, "bold red", StripANSI(input))
}

func TestStripANSI_256Color(t *testing.T) {
	// 256-color: \x1b[38;5;196m (foreground color 196)
	input := "\x1b[38;5;196mcolor 196\x1b[0m"
	assert.Equal(t, "color 196", StripANSI(input))
}

func TestStripANSI_TrueColor(t *testing.T) {
	// True color RGB: \x1b[38;2;255;100;50m
	input := "\x1b[38;2;255;100;50mtrue color\x1b[0m"
	assert.Equal(t, "true color", StripANSI(input))
}

func TestStripANSI_GitDiffHeader(t *testing.T) {
	// Real git diff header with color
	input := "\x1b[37mdiff --git a/file.go b/file.go\x1b[m"
	assert.Equal(t, "diff --git a/file.go b/file.go", StripANSI(input))
}

func TestStripANSI_GitDiffAddedLine(t *testing.T) {
	// Git colors the + separately from the content
	input := "\x1b[32m+\x1b[m\x1b[32mpackage main\x1b[m"
	assert.Equal(t, "+package main", StripANSI(input))
}

func TestStripANSI_GitDiffRemovedLine(t *testing.T) {
	input := "\x1b[31m-\x1b[m\x1b[31mold line\x1b[m"
	assert.Equal(t, "-old line", StripANSI(input))
}

func TestStripANSI_GitDiffHunkHeader(t *testing.T) {
	input := "\x1b[36m@@ -1,3 +1,4 @@\x1b[m"
	assert.Equal(t, "@@ -1,3 +1,4 @@", StripANSI(input))
}

func TestStripANSI_PreservesNewlines(t *testing.T) {
	input := "\x1b[32mline1\x1b[m\n\x1b[31mline2\x1b[m\n"
	assert.Equal(t, "line1\nline2\n", StripANSI(input))
}

func TestStripANSI_PreservesTabs(t *testing.T) {
	input := "\x1b[32m\tindented\x1b[m"
	assert.Equal(t, "\tindented", StripANSI(input))
}

func TestStripANSI_CursorMovement(t *testing.T) {
	// While less common in git output, ensure we handle other CSI sequences
	// Cursor up: \x1b[A, Cursor down: \x1b[B
	input := "text\x1b[Amore"
	// We only strip color/SGR sequences (ending in 'm'), not cursor movement
	// This test documents current behavior
	assert.Equal(t, "text\x1b[Amore", StripANSI(input))
}

func TestStripANSI_RealGitDiff(t *testing.T) {
	// A more complete example of what git diff --color=always produces
	colored := "\x1b[37mdiff --git a/foo.go b/foo.go\x1b[m\n" +
		"\x1b[37mindex abc123..def456 100644\x1b[m\n" +
		"\x1b[37m--- a/foo.go\x1b[m\n" +
		"\x1b[37m+++ b/foo.go\x1b[m\n" +
		"\x1b[36m@@ -1,3 +1,4 @@\x1b[m\n" +
		" package main\n" +
		"\x1b[32m+\x1b[m\n" +
		" func main() {\n" +
		" }\n"

	expected := "diff --git a/foo.go b/foo.go\n" +
		"index abc123..def456 100644\n" +
		"--- a/foo.go\n" +
		"+++ b/foo.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+\n" +
		" func main() {\n" +
		" }\n"

	assert.Equal(t, expected, StripANSI(colored))
}

func TestStripANSI_BoldAndUnderline(t *testing.T) {
	// Bold: \x1b[1m, Underline: \x1b[4m
	input := "\x1b[1mbold\x1b[0m \x1b[4munderline\x1b[0m"
	assert.Equal(t, "bold underline", StripANSI(input))
}

func TestStripANSI_IncompleteSequence(t *testing.T) {
	// Malformed/incomplete sequence - should be preserved as-is
	input := "text\x1b[incomplete"
	// The incomplete sequence should remain since it doesn't end with 'm'
	assert.Equal(t, "text\x1b[incomplete", StripANSI(input))
}
