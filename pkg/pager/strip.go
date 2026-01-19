package pager

import "regexp"

// ansiRegex matches ANSI SGR (Select Graphic Rendition) escape sequences.
// These are the color/style codes that git diff --color produces.
// Pattern: ESC [ <params> m
// Where params can be numbers separated by semicolons (e.g., 1;31 for bold red).
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripANSI removes ANSI color/style escape sequences from the input string.
// This is used to clean git diff output that may contain color codes when
// the user has color.ui=always or uses --color=always.
//
// Only SGR sequences (ending in 'm') are stripped. Other ANSI sequences
// like cursor movement are preserved, though these are rare in git output.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
