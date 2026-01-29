package tui

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/user/diffyduck/pkg/sidebyside"
)

// renderTopBar renders the top bar showing file info with a divider line below.
func (m Model) renderTopBar() string {
	info := m.StatusInfo()

	var lines []string

	// Commit info line (only if we have commit metadata)
	if m.hasCommitInfo() {
		commitLine := m.renderCommitLine()
		lines = append(lines, commitLine)
	}

	// File info line - show when:
	// - No commit info (file line contains total stats)
	// - OR cursor is on a file (info.CurrentFile > 0)
	if !m.hasCommitInfo() || info.CurrentFile > 0 {
		fileLine := m.renderFileLine(info)
		lines = append(lines, fileLine)
	}

	// Divider line using upper 1/8 block (dim, faint when unfocused)
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	if !m.focused {
		dividerStyle = dividerStyle.Faint(true)
	}
	divider := dividerStyle.Render(strings.Repeat("▔", m.width))
	lines = append(lines, divider)

	return strings.Join(lines, "\n")
}

// renderCommitLine renders the commit info line for the top bar.
// Shows fold icon, SHA, subject, and file stats for a compact display.
func (m *Model) renderCommitLine() string {
	commit := m.currentCommit()
	if commit == nil {
		return ""
	}
	commitInfo := commit.Info

	// Style for SHA (yellow/gold)
	shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	// Build commit line: ▶ ◐ a1b2c3d Subject line    N files +X -Y
	// Arrow shows when cursor is on any commit section (header or body)
	var prefix string
	if m.isOnCommitSection() {
		if m.focused {
			prefix = cursorArrowStyle.Render("▶") + " "
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷") + " "
		}
	} else {
		prefix = "  " // Same width as arrow + space
	}

	// Fold level icon: ◯ = folded, ◐ = normal, ● = expanded
	var foldIcon string
	switch commit.FoldLevel {
	case sidebyside.CommitFolded:
		foldIcon = "◯"
	case sidebyside.CommitNormal:
		foldIcon = "◐"
	case sidebyside.CommitExpanded:
		foldIcon = "●"
	}
	// Fold icon - always fg=8, no faint
	foldIconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	foldIconRendered := foldIconStyle.Render(foldIcon) + " "

	sha := shaStyle.Render(commitInfo.ShortSHA())
	subject := commitInfo.Subject

	// Calculate stats for the current commit's files only
	commitIdx := m.currentCommitIndex()
	var startIdx, endIdx int
	if len(m.commits) > 0 && len(m.commitFileStarts) > 0 {
		startIdx = m.commitFileStarts[commitIdx]
		endIdx = len(m.files)
		if commitIdx+1 < len(m.commits) {
			endIdx = m.commitFileStarts[commitIdx+1]
		}
	} else {
		// Legacy mode: use all files
		startIdx = 0
		endIdx = len(m.files)
	}
	totalAdded := 0
	totalRemoved := 0
	for i := startIdx; i < endIdx; i++ {
		added, removed := countFileStats(m.files[i])
		totalAdded += added
		totalRemoved += removed
	}

	// Build right section: N files +X -Y
	var rightText string
	var rightSection string
	fileCount := endIdx - startIdx
	if fileCount == 1 {
		rightText = "1 file"
	} else {
		rightText = fmt.Sprintf("%d files", fileCount)
	}
	if totalAdded > 0 || totalRemoved > 0 {
		addedText := fmt.Sprintf("+%d", totalAdded)
		removedText := fmt.Sprintf("-%d", totalRemoved)
		// For zeros, show just +/- without the number
		displayAdded := addedText
		displayRemoved := removedText
		if totalAdded == 0 {
			displayAdded = "+"
		}
		if totalRemoved == 0 {
			displayRemoved = "-"
		}
		rightText += " " + displayAdded + " " + displayRemoved
		rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText[:len(rightText)-len(displayAdded)-len(displayRemoved)-2]) + " " + addedStyle.Render(displayAdded) + " " + removedStyle.Render(displayRemoved)
	} else {
		rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText)
	}
	rightWidth := len(rightText)

	// Calculate available width for subject
	// Layout: prefix(2) + foldIcon(2) + sha(7) + sep(1) + subject + padding(1+) + rightSection
	fixedWidth := 2 + 2 + 7 + 1 + 1 + rightWidth
	availableWidth := m.width - fixedWidth
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Truncate subject if needed
	if len(subject) > availableWidth {
		if availableWidth > 3 {
			subject = subject[:availableWidth-3] + "..."
		} else if availableWidth > 0 {
			subject = subject[:availableWidth]
		} else {
			subject = ""
		}
	}

	// Calculate padding between subject and right section
	padding := m.width - 2 - 2 - 7 - 1 - len(subject) - rightWidth
	if padding < 1 {
		padding = 1
	}

	return prefix + foldIconRendered + sha + " " + subject + strings.Repeat(" ", padding) + rightSection
}

// renderFileLine renders the file info line for the top bar.
func (m Model) renderFileLine(info StatusInfo) string {
	// Only show file info when cursor is on a file (not on commit header)
	var content string
	var foldIcon string
	var fileNum string
	var leftSectionWidth int
	if info.CurrentFile > 0 {
		content = m.formatStatusFileInfo(info)

		// Fold icon
		foldIcon = m.foldLevelIcon(info.FoldLevel)

		// File number with # prefix
		_, fileCounterStyle := fileStatusIndicator(FileStatus(info.FileStatus))
		totalWidth := len(fmt.Sprintf("%d", info.TotalFiles))
		fileNumText := fmt.Sprintf("#%0*d", totalWidth, info.CurrentFile)
		fileNum = fileCounterStyle.Render(fileNumText) + " "

		// Layout: indent(3) + icon(1) + space(1) + #fileNum + space(1)
		leftSectionWidth = 3 + 1 + 1 + 1 + totalWidth + 1
	}

	// Right section: N files +123 -123 (only when no commit info - stats move to commit line otherwise)
	var rightText string
	var rightSection string
	var rightWidth int
	if !m.hasCommitInfo() {
		totalAdded := 0
		totalRemoved := 0
		for _, fp := range m.files {
			added, removed := countFileStats(fp)
			totalAdded += added
			totalRemoved += removed
		}

		fileCount := len(m.files)
		if fileCount == 1 {
			rightText = "1 file"
		} else {
			rightText = fmt.Sprintf("%d files", fileCount)
		}
		if totalAdded > 0 || totalRemoved > 0 {
			addedText := fmt.Sprintf("+%d", totalAdded)
			removedText := fmt.Sprintf("-%d", totalRemoved)
			// For zeros, show just +/- without the number
			displayAdded := addedText
			displayRemoved := removedText
			if totalAdded == 0 {
				displayAdded = "+"
			}
			if totalRemoved == 0 {
				displayRemoved = "-"
			}
			rightText += " " + displayAdded + " " + displayRemoved
			rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText[:len(rightText)-len(displayAdded)-len(displayRemoved)-2]) + " " + addedStyle.Render(displayAdded) + " " + removedStyle.Render(displayRemoved)
		} else {
			rightSection = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(rightText)
		}
		rightWidth = len(rightText)
	}

	// Leading arrow indicator - only show when NOT on commit section (or no commit info)
	// When on commit header or body, the arrow shows on the commit line instead
	var prefix string
	showArrow := !m.hasCommitInfo() || !m.isOnCommitSection()
	if showArrow {
		if m.focused {
			prefix = cursorArrowStyle.Render("▶") + " "
		} else {
			prefix = unfocusedCursorArrowStyle.Render("▷") + " "
		}
	} else {
		prefix = "  " // Same width as arrow + space
	}

	// Calculate widths for padding
	// Layout: prefix(2) + leftSection + content + padding + rightSection
	prefixWidth := 2 // "▶ " or "  "
	contentWidth := lipgloss.Width(content)
	padding := m.width - prefixWidth - leftSectionWidth - contentWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	// Build the left section: indent(3) + icon + space + fileNum
	var leftSection string
	if info.CurrentFile > 0 {
		// Style fold icon with fg=8 to match commit header fold icon
		foldIconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		leftSection = "  " + foldIconStyle.Render(foldIcon) + " " + fileNum
	}

	return prefix + leftSection + content + strings.Repeat(" ", padding) + rightSection
}

// formatRelativeDate converts an ISO 8601 date string to a relative format like "2d ago".
func formatRelativeDate(isoDate string) string {
	if isoDate == "" {
		return ""
	}

	// Try to parse ISO 8601 format
	t, err := time.Parse(time.RFC3339, isoDate)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", isoDate)
		if err != nil {
			return isoDate // Return as-is if can't parse
		}
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / 24 / 7)
		if weeks == 1 {
			return "1w ago"
		}
		return fmt.Sprintf("%dw ago", weeks)
	case diff < 365*24*time.Hour:
		months := int(diff.Hours() / 24 / 30)
		if months == 1 {
			return "1mo ago"
		}
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := int(diff.Hours() / 24 / 365)
		if years == 1 {
			return "1y ago"
		}
		return fmt.Sprintf("%dy ago", years)
	}
}

// formatShortRelativeDate returns abbreviated relative time without "ago".
// Used in commit header rows for compact display.
// Format: "now", "1m", "4h", "2d", "3w", "1mo", "1y"
func formatShortRelativeDate(isoDate string) string {
	if isoDate == "" {
		return ""
	}

	// Try to parse ISO 8601 format
	t, err := time.Parse(time.RFC3339, isoDate)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", isoDate)
		if err != nil {
			// Try git log format: "Mon Jan 15 10:30:00 2024 -0500"
			t, err = time.Parse("Mon Jan 2 15:04:05 2006 -0700", isoDate)
			if err != nil {
				return isoDate // Return as-is if can't parse
			}
		}
	}

	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh", int(diff.Hours()))
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(diff.Hours()/24))
	case diff < 30*24*time.Hour:
		return fmt.Sprintf("%dw", int(diff.Hours()/24/7))
	case diff < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(diff.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy", int(diff.Hours()/24/365))
	}
}

// renderStatusBar renders the status bar at the bottom of the screen.
// This now only contains the less-style indicator (file info is in top bar).
func (m Model) renderStatusBar() string {
	// In comment mode, show comment prompt
	if m.commentMode {
		return m.renderCommentPrompt()
	}

	// In search mode, show search prompt
	if m.searchMode {
		return m.renderSearchPrompt()
	}

	info := m.StatusInfo()

	// Build less-style line indicator (with reverse styling)
	lessIndicator := formatLessIndicator(info.CurrentLine, info.TotalLines, info.Percentage, info.AtEnd)

	// Pad to max width to prevent shrinking (maxLessWidth is computed in calculateTotalLines)
	lessWidth := displayWidth(lessIndicator)
	if lessWidth < m.maxLessWidth {
		lessIndicator += strings.Repeat(" ", m.maxLessWidth-lessWidth)
	}

	// Apply reverse style to the less indicator portion
	// Use unfocused style when terminal loses focus
	var styledLessIndicator string
	if m.focused {
		styledLessIndicator = statusStyle.Render(" " + lessIndicator)
	} else {
		styledLessIndicator = unfocusedStatusStyle.Render(" " + lessIndicator)
	}

	// Status message (echo area) - shown after less indicator
	var statusMsg string
	var statusMsgWidth int
	if m.statusMessage != "" {
		statusMsgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		statusMsg = " " + statusMsgStyle.Render(m.statusMessage)
		statusMsgWidth = 1 + displayWidth(m.statusMessage)
	}

	// Loading indicator (grey, shown when any files are loading)
	var loadingIndicator string
	var loadingWidth int
	if m.hasAnyLoadingFiles() {
		loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		loadingIndicator = " " + loadingStyle.Render(m.spinner.View()+" Loading...")
		loadingWidth = 1 + 1 + len(" Loading...") // space + spinner + text
	}

	// Pager mode indicator (right-aligned)
	var pagerIndicator string
	if m.pagerMode {
		pagerIndicator = "PAGER"
	}

	// Debug stats (right-aligned, before pager indicator)
	var debugStats string
	var debugWidth int
	if m.debugMode {
		debugStats, debugWidth = m.formatDebugStats()
	}

	// Combine: reversed_less_indicator + status_msg + loading + padding + debug_stats + pager_indicator
	content := styledLessIndicator + statusMsg + loadingIndicator
	contentWidth := displayWidth(" "+lessIndicator) + statusMsgWidth + loadingWidth
	pagerWidth := displayWidth(pagerIndicator)

	// Calculate padding between content and right-side indicators
	rightWidth := debugWidth + pagerWidth
	if debugWidth > 0 && pagerWidth > 0 {
		rightWidth++ // space between debug and pager
	}
	padding := m.width - contentWidth - rightWidth
	if padding < 0 {
		padding = 0
	}

	// Build right side
	var rightSide string
	if debugStats != "" && pagerIndicator != "" {
		rightSide = debugStats + " " + pagerIndicator
	} else {
		rightSide = debugStats + pagerIndicator
	}

	return content + strings.Repeat(" ", padding) + rightSide
}

// formatDebugStats returns formatted memory and goroutine stats for debug mode.
// Returns (styled string, display width).
func (m Model) formatDebugStats() (string, int) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	heapMB := float64(mem.Alloc) / 1024 / 1024
	goroutines := runtime.NumGoroutine()

	// Build raw values for width calculation
	heapVal := fmt.Sprintf("%.1fMB", heapMB)
	grVal := fmt.Sprintf("%d", goroutines)
	// "Heap: XXX GR: YYY"
	rawWidth := displayWidth("Heap: " + heapVal + " GR: " + grVal)

	// Build styled output
	heapLabel := debugLabelStyle.Render("Heap:")
	heapValue := debugValueStyle.Render(heapVal)
	grLabel := debugLabelStyle.Render("GR:")
	grValue := debugValueStyle.Render(grVal)

	return heapLabel + " " + heapValue + " " + grLabel + " " + grValue, rawWidth
}

// formatStatusFileInfo formats the file info for the status bar.
// Format: statusIcon fileName +N -M (icon handled separately by caller)
func (m Model) formatStatusFileInfo(info StatusInfo) string {
	// Get status indicator - shows spinner if file is loading
	fileIndex := info.CurrentFile - 1 // CurrentFile is 1-based
	styledStatus := m.fileStatusSymbolStyled(fileIndex, FileStatus(info.FileStatus))

	// Format stats (only show if there are changes)
	var stats string
	var statsWidth int
	if info.Added > 0 || info.Removed > 0 {
		var parts []string
		if info.Added > 0 {
			addedText := fmt.Sprintf("+%d", info.Added)
			parts = append(parts, addedStyle.Render(addedText))
			statsWidth += len(addedText)
		}
		if info.Removed > 0 {
			removedText := fmt.Sprintf("-%d", info.Removed)
			parts = append(parts, removedStyle.Render(removedText))
			statsWidth += len(removedText)
		}
		stats = " " + strings.Join(parts, " ")
		statsWidth += 1 + len(parts) - 1 // leading space + spaces between parts
	}

	// Calculate available width for breadcrumbs
	// Layout: statusIcon(1) + space(1) + fileName + stats + "  " + breadcrumbs
	usedWidth := 1 + 1 + len(info.FileName) + statsWidth + 2
	availableWidth := m.width - usedWidth
	if availableWidth < 0 {
		availableWidth = 0
	}

	// Format breadcrumbs with syntax highlighting if we have entries and a highlighter
	var breadcrumbs string
	if len(info.BreadcrumbEntries) > 0 && m.highlighter != nil {
		theme := m.highlighter.Theme()
		breadcrumbs = "  " + formatBreadcrumbsStyled(info.BreadcrumbEntries, theme, availableWidth)
	} else if info.Breadcrumbs != "" {
		// Fallback to plain grey if no highlighter
		breadcrumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		breadcrumbs = "  " + breadcrumbStyle.Render(info.Breadcrumbs)
	}

	return styledStatus + " " + info.FileName + stats + breadcrumbs
}

// renderSearchPrompt renders the status bar as a search input prompt.
// Uses normal styling (not reversed) so the search input is visible.
func (m Model) renderSearchPrompt() string {
	// Show / for forward, ? for backward
	prefix := "/"
	if !m.searchForward {
		prefix = "?"
	}

	left := " " + prefix + m.searchInput

	// Calculate padding to fill the width
	leftWidth := displayWidth(left)
	padding := m.width - leftWidth
	if padding < 0 {
		padding = 0
	}

	// No reverse styling for search prompt - just return plain text with padding
	return left + strings.Repeat(" ", padding)
}

// renderCommentPrompt renders the comment input as a multi-line prompt.
func (m Model) renderCommentPrompt() string {
	// Split input into lines
	lines := strings.Split(m.commentInput, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	// Find which line the cursor is on and the position within that line
	cursorLine := 0
	cursorCol := m.commentCursor
	pos := 0
	for i, line := range lines {
		lineEnd := pos + len(line)
		if i < len(lines)-1 {
			lineEnd++ // account for newline
		}
		// Use < so cursor right after newline is on the next line
		if m.commentCursor < lineEnd || i == len(lines)-1 {
			cursorLine = i
			cursorCol = m.commentCursor - pos
			break
		}
		pos = lineEnd
	}

	// Calculate visible range based on scroll
	maxVisible := m.commentMaxVisibleLines()
	startLine := m.commentScroll
	endLine := startLine + maxVisible
	if endLine > len(lines) {
		endLine = len(lines)
	}

	// Check if there's content above/below the visible area
	hasMoreAbove := startLine > 0
	hasMoreBelow := endLine < len(lines)

	var result []string

	// Show scroll indicator if there's content above
	if hasMoreAbove {
		indicator := fmt.Sprintf(" ↑ %d more line", startLine)
		if startLine > 1 {
			indicator += "s"
		}
		indicatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		indicatorPadding := m.width - lipgloss.Width(indicator)
		if indicatorPadding < 0 {
			indicatorPadding = 0
		}
		result = append(result, indicatorStyle.Render(indicator)+strings.Repeat(" ", indicatorPadding))
	}

	// Render visible lines of input
	for i := startLine; i < endLine; i++ {
		line := lines[i]
		var prefix string
		if i == cursorLine {
			prefix = " > " // cursor line gets the main prompt
		} else {
			prefix = " . " // other lines get continuation indicator
		}

		var renderedLine string
		if i == cursorLine {
			// This line has the cursor
			if cursorCol > len(line) {
				cursorCol = len(line)
			}

			beforeCursor := line[:cursorCol]
			var cursorChar string
			var afterCursor string

			if cursorCol < len(line) {
				runes := []rune(line[cursorCol:])
				cursorChar = string(runes[0])
				afterCursor = string(runes[1:])
			} else {
				cursorChar = " "
				afterCursor = ""
			}

			styledCursor := statusStyle.Render(cursorChar)
			renderedLine = prefix + beforeCursor + styledCursor + afterCursor
		} else {
			renderedLine = prefix + line
		}

		// Pad to full width (use lipgloss.Width to handle ANSI escape codes correctly)
		lineWidth := lipgloss.Width(renderedLine)
		padding := m.width - lineWidth
		if padding < 0 {
			padding = 0
		}
		result = append(result, renderedLine+strings.Repeat(" ", padding))
	}

	// Show scroll indicator if there's content below
	if hasMoreBelow {
		remaining := len(lines) - endLine
		indicator := fmt.Sprintf(" ↓ %d more line", remaining)
		if remaining > 1 {
			indicator += "s"
		}
		indicatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		indicatorPadding := m.width - lipgloss.Width(indicator)
		if indicatorPadding < 0 {
			indicatorPadding = 0
		}
		result = append(result, indicatorStyle.Render(indicator)+strings.Repeat(" ", indicatorPadding))
	}

	// Add help line at the bottom
	help := " (C-j to submit, C-c to cancel)"
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpPadding := m.width - lipgloss.Width(help)
	if helpPadding < 0 {
		helpPadding = 0
	}
	result = append(result, helpStyle.Render(help)+strings.Repeat(" ", helpPadding))

	return strings.Join(result, "\n")
}

// highlightSearchInVisible highlights search matches in visible text.
// Searches on-demand in the visible text and highlights matches.
// isCursorRow indicates if this row is at the cursor position.
// currentIdx is the index of the current match (0 = first match).
