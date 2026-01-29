package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// displayWidth returns the display width of a string, accounting for
// wide characters (CJK, emoji) that take 2 cells.
func displayWidth(s string) int {
	return runewidth.StringWidth(s)
}

// truncateLeft removes the first width display columns from a string.
func truncateLeft(s string, width int) string {
	if width <= 0 {
		return s
	}
	currentWidth := 0
	for i, r := range s {
		w := runewidth.RuneWidth(r)
		currentWidth += w
		if currentWidth >= width {
			return s[i+len(string(r)):]
		}
	}
	return ""
}

// truncateOrPad truncates or pads a string to exactly the given display width.
// It properly handles multi-byte and wide characters.
func truncateOrPad(s string, width int) string {
	sw := displayWidth(s)

	if sw <= width {
		// Pad with spaces
		return s + strings.Repeat(" ", width-sw)
	}

	// Need to truncate
	if width <= 3 {
		// Too narrow for ellipsis, just truncate
		return runewidth.Truncate(s, width, "")
	}

	// Truncate with ellipsis
	return runewidth.Truncate(s, width, "...")
}

// horizontalSlice returns a slice of a string starting at the given display
// column offset and spanning the given width. It handles wide characters
// (CJK, emoji) properly - if the offset lands in the middle of a wide char,
// that position is replaced with a space. The result is always exactly `width`
// display columns, padded with spaces if needed.
func horizontalSlice(s string, offset, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	col := 0             // current display column
	resultWidth := 0     // width of result so far
	skippedHalf := false // true if we skipped half of a wide char at offset

	for _, r := range s {
		rw := runewidth.RuneWidth(r)

		// Still in the skip zone (before offset)
		if col < offset {
			// Check if this wide char spans the offset boundary
			if col+rw > offset && rw > 1 {
				// Wide char straddles the offset - we're cutting it in half
				skippedHalf = true
			}
			col += rw
			continue
		}

		// We're at or past the offset - start collecting

		// If we just started and skipped half a wide char, emit a space
		if col == offset && skippedHalf {
			result.WriteRune(' ')
			resultWidth++
			skippedHalf = false
		}

		// Check if this rune fits in remaining width
		if resultWidth+rw > width {
			break
		}

		result.WriteRune(r)
		resultWidth += rw
		col += rw
	}

	// Handle case where offset was past content, or we skipped into empty
	if col <= offset && skippedHalf {
		// We never emitted anything but owe a space for half-wide-char
		result.WriteRune(' ')
		resultWidth++
	}

	// Pad to exact width
	if resultWidth < width {
		result.WriteString(strings.Repeat(" ", width-resultWidth))
	}

	return result.String()
}

// applyColumnIndicators wraps lines with gutter columns:
// - Added/removed: ░ + space + content + space + ░
// - Context/empty: space + space + content + space + space
