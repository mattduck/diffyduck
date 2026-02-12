package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

const maxStructuralDiffItems = 10
const maxStructuralDiffSigWidth = 120

// statsTextWidth returns the display width of an unaligned stats string like " +3 -4".
// Returns 0 if both added and removed are zero.
func statsTextWidth(added, removed int) int {
	if added == 0 && removed == 0 {
		return 0
	}
	w := 0
	if added > 0 {
		w += 1 + len(fmt.Sprintf("+%d", added)) // space + "+N"
	}
	if removed > 0 {
		w += 1 + len(fmt.Sprintf("-%d", removed)) // space + "-N"
	}
	return w
}

// styleSig applies syntax-style highlighting to a structural diff signature.
// Input format from FormatSignature: "Name(params) -> ReturnType"
// or with receiver: "(m *Model) Name(params) -> ReturnType"
// or for types (no signature): just "Name"
// The changeKind controls identifier styling: added/deleted items get underline
// in the corresponding inline diff color; modified items use normal syntax colors.
func styleSig(sig string, changeKind structure.ChangeKind) string {
	// Theme-matching styles
	funcStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("4"))  // blue (dark, not bright)
	typeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))  // magenta (dark, not bright)
	punctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // white
	paramStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7")) // white (not default/bright)

	// nameStyle is used only for the identifier name (func name or type name).
	// For added/deleted items it gets underline in the inline diff color.
	// Otherwise the identifier uses fg7 (white) instead of syntax colors.
	plainStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	nameStyle := plainStyle
	switch changeKind {
	case structure.ChangeAdded:
		s := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("2"))
		funcStyle = s
		nameStyle = s
	case structure.ChangeDeleted:
		s := lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("1"))
		funcStyle = s
		nameStyle = s
	default:
		funcStyle = plainStyle
	}

	// No parens at all — plain type name
	if !strings.Contains(sig, "(") {
		return nameStyle.Render(sig)
	}

	var result strings.Builder

	// Handle optional receiver prefix: "(m *Model) "
	rest := sig
	if strings.HasPrefix(rest, "(") {
		// Could be receiver or params — receiver is followed by ") Name("
		closeParen := strings.Index(rest, ") ")
		if closeParen > 0 && closeParen < strings.Index(rest[1:], "(")+1 {
			// It's a receiver
			receiver := rest[:closeParen+1]
			result.WriteString(styleReceiver(receiver, punctStyle, typeStyle, paramStyle))
			result.WriteString(" ")
			rest = rest[closeParen+2:]
		}
	}

	// Now rest is "Name(params)" or "Name(params) -> ReturnType"
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		// No parens — just a name (shouldn't happen after receiver check, but safe)
		result.WriteString(funcStyle.Render(rest))
		return result.String()
	}

	// Function name
	name := rest[:parenIdx]
	result.WriteString(funcStyle.Render(name))
	rest = rest[parenIdx:]

	// Split on " -> " for return type
	arrowIdx := strings.Index(rest, " -> ")
	var paramsPart, returnPart string
	if arrowIdx >= 0 {
		paramsPart = rest[:arrowIdx]
		returnPart = rest[arrowIdx:]
	} else {
		paramsPart = rest
	}

	// Style the params part: "(param1 Type1, param2 Type2)" or "(...)"
	result.WriteString(styleParams(paramsPart, punctStyle, typeStyle, paramStyle))

	// Style " -> ReturnType"
	if returnPart != "" {
		result.WriteString(" ")
		result.WriteString(punctStyle.Render("->"))
		result.WriteString(" ")
		retType := returnPart[4:] // skip " -> "
		result.WriteString(typeStyle.Render(retType))
	}

	return result.String()
}

// styleReceiver highlights a Go receiver like "(m *Model)".
func styleReceiver(recv string, punctStyle, typeStyle, paramStyle lipgloss.Style) string {
	// recv is "(m *Model)" — strip parens, split on space
	inner := recv[1 : len(recv)-1] // "m *Model"
	parts := strings.SplitN(inner, " ", 2)

	var result strings.Builder
	result.WriteString(punctStyle.Render("("))
	if len(parts) == 2 {
		result.WriteString(paramStyle.Render(parts[0]))
		result.WriteString(" ")
		result.WriteString(typeStyle.Render(parts[1]))
	} else {
		result.WriteString(paramStyle.Render(inner))
	}
	result.WriteString(punctStyle.Render(")"))
	return result.String()
}

// styleParams highlights a parameter list like "(ctx context.Context, req *Request)".
func styleParams(params string, punctStyle, typeStyle, paramStyle lipgloss.Style) string {
	// Handle "(...)" or "()" or "(param Type, ...)"
	if params == "(...)" || params == "()" {
		return punctStyle.Render(params)
	}

	// Strip outer parens
	if !strings.HasPrefix(params, "(") || !strings.HasSuffix(params, ")") {
		return params
	}
	inner := params[1 : len(params)-1]

	// Check for trailing ", ...)" -> inner ends with ", ..."
	hasEllipsis := strings.HasSuffix(inner, ", ...")
	if hasEllipsis {
		inner = strings.TrimSuffix(inner, ", ...")
	}

	var result strings.Builder
	result.WriteString(punctStyle.Render("("))

	paramList := strings.Split(inner, ", ")
	for i, p := range paramList {
		if i > 0 {
			result.WriteString(punctStyle.Render(","))
			result.WriteString(" ")
		}
		// Each param is "name Type" or "...Type" (variadic) or just "self" (Python)
		spaceIdx := strings.Index(p, " ")
		if spaceIdx >= 0 {
			paramName := p[:spaceIdx]
			paramType := p[spaceIdx+1:]
			result.WriteString(paramStyle.Render(paramName))
			result.WriteString(" ")
			result.WriteString(typeStyle.Render(paramType))
		} else {
			// No space — could be "self", "...Option", etc.
			result.WriteString(paramStyle.Render(p))
		}
	}

	if hasEllipsis {
		result.WriteString(punctStyle.Render(","))
		result.WriteString(" ")
		result.WriteString(punctStyle.Render("..."))
	}

	result.WriteString(punctStyle.Render(")"))
	return result.String()
}

// renderCommitHeaderRow renders a commit header row in the content area.
// This is shown when viewing a commit and can be folded/unfolded.
//
// Layout: [cursor] [fold] [sha] [+added] [-removed] [files] [time] [author] [subject]
// Fixed columns (left): sha, +added, -removed, files, time, author (max 15 chars)
// Dynamic column (right): subject (max 120 chars)
func (m Model) renderCommitHeaderRow(row displayRow, isCursorRow bool) string {
	// Use commitIndex from the row to get the correct commit
	if row.commitIndex < 0 || row.commitIndex >= len(m.commits) {
		return ""
	}
	commit := &m.commits[row.commitIndex]
	commitInfo := commit.Info

	// Styles
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	authorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))

	// Tree style and SHA style: snapshots use magenta (color 5), commits use yellow (color 3)
	treeStyle := commitTreeStyle
	if commit.IsSnapshot {
		treeStyle = snapshotTreeStyle
		shaStyle = snapshotTreeStyle
	}

	// Fold icon colour matches tree style, fg=15 when cursor is on row
	foldIconStyle := treeStyle
	if isCursorRow {
		foldIconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	}

	// Fold icon
	var foldIcon string
	switch row.commitFoldLevel {
	case sidebyside.CommitFolded:
		foldIcon = "◯"
	case sidebyside.CommitNormal:
		foldIcon = "●"
	case sidebyside.CommitExpanded:
		foldIcon = "●"
	}

	// Get file count and stats for this commit
	startIdx := m.commitFileStarts[row.commitIndex]
	endIdx := len(m.files)
	if row.commitIndex+1 < len(m.commits) {
		endIdx = m.commitFileStarts[row.commitIndex+1]
	}
	fileCount := endIdx - startIdx

	// Determine stats to display:
	// - If StatsLoaded, use cached commit-level stats
	// - Otherwise, compute from files (handles diff mode and tests)
	// - Show "?" only when neither source has stats (progressive loading initial state)
	var totalAdded, totalRemoved int
	var statsLoaded bool
	if commit.StatsLoaded {
		// Use cached commit-level stats
		totalAdded = commit.TotalAdded
		totalRemoved = commit.TotalRemoved
		statsLoaded = true
	} else {
		// Compute from files
		for i := startIdx; i < endIdx; i++ {
			added, removed := countFileStats(m.files[i])
			totalAdded += added
			totalRemoved += removed
		}
		// Stats are "loaded" if we computed non-zero values OR there are no files
		// (zero stats with files could mean progressive loading not yet complete)
		statsLoaded = totalAdded > 0 || totalRemoved > 0 || fileCount == 0
	}

	// Cursor prefix
	var prefix string
	if isCursorRow {
		if m.focused {
			prefix = cursorArrowStyle.Render("▌")
		} else {
			prefix = " "
		}
	} else {
		// Single space margin (fold icon aligns with tree branch column)
		prefix = " "
	}

	// Build fixed columns
	// Regular commits: [prefix][fold] [sha] [+added] [-removed] [files] [time] [author] [subject]
	// Snapshots:       [prefix][fold] [sha?] [+added] [-removed] [files] [time] [subject]
	//                  (SHA shown if available from background snapshot, otherwise omitted)

	isSnapshot := commit.IsSnapshot

	// For snapshots, show SHA only if available (may arrive async from background)
	shaText := commitInfo.ShortSHA()
	filesText := "-"
	if fileCount > 0 {
		filesText = fmt.Sprintf("%d", fileCount)
	}
	var addedText, removedText string
	if statsLoaded {
		addedText = fmt.Sprintf("+%d", totalAdded)
		removedText = fmt.Sprintf("-%d", totalRemoved)
	} else {
		addedText = "+?"
		removedText = "-?"
	}

	// For snapshots, show absolute time; for commits, show relative time
	var timeText string
	if isSnapshot {
		timeText = formatAbsoluteTime(commitInfo.Date)
	} else {
		timeText = formatShortRelativeDate(commitInfo.Date)
	}

	// Pad columns to max widths for alignment across commits (right-align numbers)
	if len(filesText) < row.maxCommitFilesWidth {
		filesText = strings.Repeat(" ", row.maxCommitFilesWidth-len(filesText)) + filesText
	}
	if len(addedText) < row.maxCommitAddWidth {
		addedText = strings.Repeat(" ", row.maxCommitAddWidth-len(addedText)) + addedText
	}
	if len(removedText) < row.maxCommitRemWidth {
		removedText = strings.Repeat(" ", row.maxCommitRemWidth-len(removedText)) + removedText
	}
	if len(timeText) < row.maxCommitTimeWidth {
		timeText = strings.Repeat(" ", row.maxCommitTimeWidth-len(timeText)) + timeText
	}

	// Author: max 15 display columns, truncate with "..." if longer
	// Use displayWidth for Unicode-aware width calculation
	// Skip author for snapshots
	author := ""
	authorWidth := 0
	if !isSnapshot {
		author = commitInfo.Author
		maxAuthorLen := 15
		authorWidth = displayWidth(author)
		if authorWidth > maxAuthorLen {
			author = runewidth.Truncate(author, maxAuthorLen, "...")
			authorWidth = maxAuthorLen
		}
	}

	// Build the fixed part with styling
	// For zero counts, show just +/- right-aligned (no number)
	var styledAdded, styledRemoved string
	if totalAdded == 0 {
		// Right-align just the + in dim grey (no additions)
		padding := strings.TrimSuffix(addedText, "+0")
		styledAdded = padding + " " + dimStyle.Render("+")
	} else {
		styledAdded = addedStyle.Render(addedText)
	}
	if totalRemoved == 0 {
		// Right-align just the - in dim grey (no removals)
		padding := strings.TrimSuffix(removedText, "-0")
		styledRemoved = padding + " " + dimStyle.Render("-")
	} else {
		styledRemoved = removedStyle.Render(removedText)
	}
	// Build fixed part based on whether this is a snapshot or regular commit
	var fixedPart string
	if isSnapshot {
		// Snapshots: SHA only if available, no author
		fixedPart = prefix + foldIconStyle.Render(foldIcon) + " "
		if shaText != "" {
			styledSHA := m.highlightSearchInVisibleStyled(shaText, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide(), func(text string, _ int) string {
				return shaStyle.Render(text)
			})
			fixedPart += styledSHA + " "
		}
		fixedPart += styledAdded + " " +
			styledRemoved + " " +
			dimStyle.Render(filesText) + " " +
			dimStyle.Render(timeText)
	} else {
		// Regular commits: include SHA and author
		styledSHA := m.highlightSearchInVisibleStyled(shaText, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide(), func(text string, _ int) string {
			return shaStyle.Render(text)
		})
		styledAuthor := m.highlightSearchInVisibleStyled(author, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide(), func(text string, _ int) string {
			return authorStyle.Render(text)
		})

		fixedPart = prefix +
			foldIconStyle.Render(foldIcon) + " " +
			styledSHA + " " +
			styledAdded + " " +
			styledRemoved + " " +
			dimStyle.Render(filesText) + " " +
			dimStyle.Render(timeText) + " " +
			styledAuthor
	}

	// Build styled ref decorations
	var refsStyled string
	refsWidth := commitInfo.RefsDisplayWidth()
	for _, ref := range commitInfo.Refs {
		style := localRefStyle
		if ref.Kind == sidebyside.RefRemote {
			style = remoteRefStyle
		}
		if ref.IsHead {
			style = style.Bold(true)
		}
		switch ref.Kind {
		case sidebyside.RefLocal:
			refsStyled += " " + style.Render("["+ref.Name+"]")
		case sidebyside.RefRemote:
			refsStyled += " " + style.Render("{"+ref.Name+"}")
		}
	}

	// Subject: truncate to subjectDisplayWidth with Unicode-aware width,
	// reserving space for ref decorations
	subject := commitInfo.Subject
	subjectDisplayWidth := row.maxCommitSubjectWidth
	availableSubjectWidth := subjectDisplayWidth - refsWidth
	if availableSubjectWidth < 0 {
		availableSubjectWidth = 0
	}
	subjectWidth := displayWidth(subject)

	// Truncate if needed (capped at 120 display columns, then to available width)
	if subjectWidth > 120 {
		subject = runewidth.Truncate(subject, 120, "...")
		subjectWidth = displayWidth(subject)
	}
	if subjectWidth > availableSubjectWidth {
		if availableSubjectWidth > 3 {
			subject = runewidth.Truncate(subject, availableSubjectWidth, "...")
		} else if availableSubjectWidth > 0 {
			subject = runewidth.Truncate(subject, availableSubjectWidth, "")
		} else {
			subject = ""
		}
		subjectWidth = displayWidth(subject)
	}

	// Apply search highlighting to the subject
	subject = m.highlightSearchInVisible(subject, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide())

	// Build the dynamic part: subject + ref decorations
	var dynamicPart string
	if subjectDisplayWidth > 0 {
		dynamicPart = " " + subject + refsStyled
	}

	result := fixedPart + dynamicPart

	// Trailing indicator based on commit fold level:
	// - Folded: no trailing indicator
	// - Normal/Expanded: corner turning down, border continues on next line
	switch row.commitFoldLevel {
	case sidebyside.CommitFolded:
		// No trailing indicator
	default:
		// CommitNormal and CommitExpanded: ════╗ after content
		result += " " + treeStyle.Render("════╗")
	}

	return result
}

// renderCommitInfoHeader renders the commit info header row (foldable child node).
// Uses yellow styling. When expanded, renders with vertical borders like file headers.
// Layout folded: [tree prefix] [fold icon] [header text]
// Layout expanded: [tree prefix] │ [fold icon] [header text] [padding] │
func (m Model) renderCommitInfoHeader(row displayRow, isCursorRow bool) string {
	// Fold icon: ◯ for CommitNormal (header only), ● for CommitExpanded (full content)
	var foldIcon string
	switch row.commitFoldLevel {
	case sidebyside.CommitExpanded:
		foldIcon = "●"
	default:
		foldIcon = "◯"
	}

	// Icon colour matches commit info border (grey/color 7), fg=15 when cursor is on row
	iconStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	if isCursorRow {
		iconStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	}
	styledIcon := iconStyle.Render(foldIcon)

	// Header text (formatted date) - dim parts in fg=7, date part in fg=15
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	brightStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	styledHeader := dimStyle.Render(row.dateParts.Day) +
		brightStyle.Render(row.dateParts.Date) +
		dimStyle.Render(row.dateParts.Offset) +
		dimStyle.Render(row.dateParts.Ago)

	// Tree prefix using the generic tree infrastructure
	treeLine := renderTreePrefix(row.treePath, true) // true = header row

	// Cursor handling: just replace first char with arrow, like commit headers
	if isCursorRow {
		var prefix string
		if m.focused {
			prefix = cursorArrowStyle.Render("▌")
		} else {
			prefix = " "
		}
		// Replace first char of treeLine with arrow
		if len(treeLine) > 0 {
			treeLine = prefix + treeLine[1:]
		} else {
			treeLine = prefix
		}
	}

	// Connect branch to fold icon with ━ (same style as horizontal branch)
	branchConnector := " "
	if row.treePath.Current != nil {
		branchConnector = row.treePath.Current.Style.Render("━")
	}
	result := treeLine + branchConnector + styledIcon + " " + styledHeader

	// Trailing indicator: nothing when folded, corner turning down when expanded
	greyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	if row.headerMode == HeaderSingleLine {
		// Folded commit info: no trailing indicator
	} else if m.width > 0 {
		// Expanded: ━━━━┓ after content, border continues on next line
		result += " " + greyStyle.Render("━━━━┓")
	}

	return result
}

// renderCommitInfoBottomBorder renders the bottom border of the commit info header.
// Positions ┗ under the ┓ on the header line, extends ━━━ to screen edge with ●.
func (m Model) renderCommitInfoBottomBorder(row displayRow, isCursorRow bool) string {
	greyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))

	// Build tree continuation from ancestors
	var treeCont string
	var treeContWidth int
	if len(row.treePath.Ancestors) > 0 {
		level := row.treePath.Ancestors[0]
		if !level.IsLast && !level.IsFolded {
			treeCont = treeContinuationStyle.Render("│")
		} else {
			treeCont = " "
		}
		treeContWidth = 1
	}

	margin := strings.Repeat(" ", TreeLeftMargin)

	// Compute column of ┓ on the header line (same formula as file headers + 4 for ━━━━)
	headerNumAncestors := len(row.treePath.Ancestors) - 1
	if headerNumAncestors < 0 {
		headerNumAncestors = 0
	}
	cornerColumn := treeWidth(headerNumAncestors, true) + row.headerBoxWidth - 2 + 4 // +4 for ━━━━ before ┓

	spacesBeforeCorner := cornerColumn - TreeLeftMargin - treeContWidth
	if spacesBeforeCorner < 0 {
		spacesBeforeCorner = 0
	}
	spacing := strings.Repeat(" ", spacesBeforeCorner)

	// Extend to screen edge with ● end cap
	borderFill := m.width - cornerColumn - 1 // -1 for corner char
	var content string
	if borderFill > 1 {
		content = "┗" + strings.Repeat("━", borderFill-1) + "●"
	} else if borderFill > 0 {
		content = "┗●"
	} else {
		content = "┗"
	}

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▌")
		} else {
			arrow = " "
		}
		if TreeLeftMargin > 0 {
			return arrow + margin[1:] + treeCont + spacing + greyStyle.Render(content)
		}
		return arrow + treeCont + spacing + greyStyle.Render(content)
	}

	return margin + treeCont + spacing + greyStyle.Render(content)
}

// renderCommitInfoBody renders a commit info body row (Author, Date, message content).
// Uses the same styling as the legacy commit body rows.
func (m Model) renderCommitInfoBody(row displayRow, isCursorRow bool) string {
	// Styles
	gutterStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	line := row.commitInfoLine

	// Build base styler for commit info body lines.
	// Preserves prefix colouring (dim label, yellow SHA) under search highlighting.
	var baseStyler SearchBaseStyler
	if strings.HasPrefix(line, "commit ") {
		prefixLen := 7 // len("commit ")
		baseStyler = commitInfoBodyStyler(prefixLen, gutterStyle, &shaStyle)
	} else if strings.HasPrefix(line, "Author: ") {
		prefixLen := 8 // len("Author: ")
		baseStyler = commitInfoBodyStyler(prefixLen, gutterStyle, nil)
	} else if strings.HasPrefix(line, "Date:   ") {
		prefixLen := 8 // len("Date:   ")
		baseStyler = commitInfoBodyStyler(prefixLen, gutterStyle, nil)
	}

	styledLine := m.highlightSearchInVisibleStyled(line, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide(), baseStyler)

	// Tree prefix with tight spacing (just 1 space after │)
	margin := strings.Repeat(" ", TreeLeftMargin)
	treePrefix := margin + renderTreeContinuationTight(row.treePath.Ancestors)

	if isCursorRow {
		var arrow string
		if m.focused {
			arrow = cursorArrowStyle.Render("▌")
		} else {
			arrow = " "
		}
		// Replace first char of margin with arrow
		if TreeLeftMargin > 0 {
			return arrow + margin[1:] + renderTreeContinuationTight(row.treePath.Ancestors) + styledLine
		}
		return arrow + renderTreeContinuationTight(row.treePath.Ancestors) + styledLine
	}

	return treePrefix + styledLine
}

// buildCommitBodyRows creates display rows for the commit body (shown when expanded).
// Format is similar to git log: full SHA, author, date, then message body.
func (m Model) buildCommitBodyRows(commit *sidebyside.CommitSet, commitIdx int) []displayRow {
	var rows []displayRow
	info := commit.Info

	// Blank line after commit header row
	rows = append(rows, displayRow{
		kind:              RowKindCommitBody,
		fileIndex:         -1,
		isCommitBody:      true,
		commitBodyLine:    "",
		commitBodyIsBlank: true,
		commitIndex:       commitIdx,
	})

	// Line 1: "commit <full sha>" (indented 2 spaces)
	commitLine := "commit " + info.SHA
	rows = append(rows, displayRow{
		kind:           RowKindCommitBody,
		fileIndex:      -1,
		isCommitBody:   true,
		commitBodyLine: commitLine,
		commitIndex:    commitIdx,
	})

	// Line 2: "Author: <author> <email>"
	authorLine := "Author: " + info.Author
	if info.Email != "" {
		authorLine += " <" + info.Email + ">"
	}
	rows = append(rows, displayRow{
		kind:           RowKindCommitBody,
		fileIndex:      -1,
		isCommitBody:   true,
		commitBodyLine: authorLine,
		commitIndex:    commitIdx,
	})

	// Line 3: "Date:   <date>"
	dateLine := "Date:   " + info.Date
	rows = append(rows, displayRow{
		kind:           RowKindCommitBody,
		fileIndex:      -1,
		isCommitBody:   true,
		commitBodyLine: dateLine,
		commitIndex:    commitIdx,
	})

	// Blank line before message
	rows = append(rows, displayRow{
		kind:              RowKindCommitBody,
		fileIndex:         -1,
		isCommitBody:      true,
		commitBodyLine:    "",
		commitBodyIsBlank: true,
		commitIndex:       commitIdx,
	})

	// Subject line (first line of message, indented)
	if info.Subject != "" {
		rows = append(rows, displayRow{
			kind:           RowKindCommitBody,
			fileIndex:      -1,
			isCommitBody:   true,
			commitBodyLine: "    " + info.Subject,
			commitIndex:    commitIdx,
		})
	}

	// Body lines (rest of message, indented)
	if info.Body != "" {
		// Blank line between subject and body (traditional git log format)
		rows = append(rows, displayRow{
			kind:              RowKindCommitBody,
			fileIndex:         -1,
			isCommitBody:      true,
			commitBodyLine:    "",
			commitBodyIsBlank: true,
			commitIndex:       commitIdx,
		})
		bodyLines := strings.Split(info.Body, "\n")
		for _, line := range bodyLines {
			// Skip empty lines at the start of body
			if line == "" {
				rows = append(rows, displayRow{
					kind:              RowKindCommitBody,
					fileIndex:         -1,
					isCommitBody:      true,
					commitBodyLine:    "",
					commitBodyIsBlank: true,
					commitIndex:       commitIdx,
				})
			} else {
				rows = append(rows, displayRow{
					kind:           RowKindCommitBody,
					fileIndex:      -1,
					isCommitBody:   true,
					commitBodyLine: "    " + line,
					commitIndex:    commitIdx,
				})
			}
		}
	}

	// Trailing blank line for separation from files
	rows = append(rows, displayRow{
		kind:              RowKindCommitBody,
		fileIndex:         -1,
		isCommitBody:      true,
		commitBodyLine:    "",
		commitBodyIsBlank: true,
		commitIndex:       commitIdx,
	})

	return rows
}

// buildCommitBodyRowsSkipFirstBlank creates body rows but skips the first blank line.
// The first blank is replaced by the commit header bottom border.
func (m Model) buildCommitBodyRowsSkipFirstBlank(commit *sidebyside.CommitSet, commitIdx int) []displayRow {
	rows := m.buildCommitBodyRows(commit, commitIdx)
	if len(rows) > 0 && rows[0].commitBodyIsBlank {
		return rows[1:]
	}
	return rows
}

// buildCommitInfoRows creates the foldable commit info node rows.
// This node appears as the first child under a commit, before any files.
// - CommitFolded: returns empty (node hidden)
// - CommitNormal: returns only header row ("commit abc1234")
// - CommitExpanded: returns header + body rows (Author, Date, message)
// - IsSnapshot: returns empty (snapshots have no details node)
func (m Model) buildCommitInfoRows(commit *sidebyside.CommitSet, commitIdx int) []displayRow {
	var rows []displayRow
	info := commit.Info

	// No info rows for snapshots (they have no commit metadata to show)
	if commit.IsSnapshot {
		return rows
	}

	// No info rows if commit has no metadata or is folded
	commitFoldLevel := m.commitFoldLevel(commitIdx)
	if !info.HasMetadata() || commitFoldLevel == sidebyside.CommitFolded {
		return rows
	}

	// Build tree path for the details node.
	// Details is a sibling to the files under this commit, not their parent.
	// Note: Commits are tree roots with no siblings, so we don't include commit
	// level in Ancestors (no continuation line needed above details/files).

	// Check if this commit has any files - if so, details is not the last child
	startIdx := 0
	if commitIdx < len(m.commitFileStarts) {
		startIdx = m.commitFileStarts[commitIdx]
	}
	endIdx := len(m.files)
	if commitIdx+1 < len(m.commitFileStarts) {
		endIdx = m.commitFileStarts[commitIdx+1]
	}
	hasFiles := startIdx < endIdx

	// Details level - grey style (Color 7) for subdued appearance
	detailsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	detailsLevel := TreeLevel{
		IsLast:   !hasFiles,                                  // details is last only if no files follow
		IsFolded: commitFoldLevel == sidebyside.CommitNormal, // folded when just showing header
		Style:    detailsStyle,
		Depth:    0, // depth 0 since commit is the root (not in tree)
	}

	detailsTreePath := TreePath{
		Ancestors: []TreeLevel{}, // no ancestors - commit is root, not a tree node
		Current:   &detailsLevel,
	}

	// Header text for the commit-info node - show formatted date if available
	now := time.Now()
	dateParts := info.FormattedDateParts(now)
	headerText := dateParts.Plain()

	// Calculate header box width for borders
	// treePrefixWidth includes the space before the icon (+1), matching file headers.
	// headerBoxWidth uses the same tree-overlap offset (3) as fileHeaderBoxWidth,
	// ensuring the render formula (treePrefixWidth + headerBoxWidth - 2) gives the
	// correct content width and the bottom border aligns with ┏.
	treePrefixWidth := treeWidth(0, true) + 1
	headerBoxWidth := 3 + 1 + 1 + displayWidth(headerText) + 1 // overlap(3) + icon(1) + space(1) + text + gap(1)

	// Determine header mode: expanded shows borders, normal shows single line
	infoHeaderMode := HeaderSingleLine
	if commitFoldLevel == sidebyside.CommitExpanded {
		infoHeaderMode = HeaderThreeLine
	}

	// Commit info header (child node showing commit metadata)
	rows = append(rows, displayRow{
		kind:               RowKindCommitInfoHeader,
		fileIndex:          -1,
		isCommitInfoHeader: true,
		header:             headerText,
		dateParts:          dateParts,
		headerMode:         infoHeaderMode,
		commitFoldLevel:    commitFoldLevel,
		commitIndex:        commitIdx,
		treePath:           detailsTreePath,
		headerBoxWidth:     headerBoxWidth,
		treePrefixWidth:    treePrefixWidth,
	})

	// If parent is CommitNormal, only show header (info folded)
	if commitFoldLevel == sidebyside.CommitNormal {
		return rows
	}

	// CommitExpanded: show full info body

	// Body rows have detailsLevel as ancestor (shows continuation │)
	bodyTreePath := TreePath{
		Ancestors: []TreeLevel{detailsLevel},
		Current:   nil,
	}

	// Bottom border after header - uses bodyTreePath for tree continuation
	rows = append(rows, displayRow{
		kind:                     RowKindCommitInfoBottomBorder,
		fileIndex:                -1,
		isCommitInfoBottomBorder: true,
		headerMode:               infoHeaderMode,
		commitIndex:              commitIdx,
		headerBoxWidth:           headerBoxWidth,
		treePrefixWidth:          treePrefixWidth,
		treePath:                 bodyTreePath,
	})

	// Blank line after header border
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "",
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// commit <full sha>
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "commit " + info.SHA,
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Author line
	authorLine := "Author: " + info.Author
	if info.Email != "" {
		authorLine += " <" + info.Email + ">"
	}
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   authorLine,
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Date line
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "Date:   " + info.Date,
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Blank line before message
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "",
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	// Subject line (indented)
	if info.Subject != "" {
		rows = append(rows, displayRow{
			kind:             RowKindCommitInfoBody,
			fileIndex:        -1,
			isCommitInfoBody: true,
			commitInfoLine:   "    " + info.Subject,
			commitIndex:      commitIdx,
			treePath:         bodyTreePath,
		})
	}

	// Body lines (if present)
	if info.Body != "" {
		// Blank line between subject and body
		rows = append(rows, displayRow{
			kind:             RowKindCommitInfoBody,
			fileIndex:        -1,
			isCommitInfoBody: true,
			commitInfoLine:   "",
			commitIndex:      commitIdx,
			treePath:         bodyTreePath,
		})
		bodyLines := strings.Split(info.Body, "\n")
		for _, line := range bodyLines {
			indentedLine := ""
			if line != "" {
				indentedLine = "    " + line
			}
			rows = append(rows, displayRow{
				kind:             RowKindCommitInfoBody,
				fileIndex:        -1,
				isCommitInfoBody: true,
				commitInfoLine:   indentedLine,
				commitIndex:      commitIdx,
				treePath:         bodyTreePath,
			})
		}
	}

	// Trailing blank line for the file border to draw into
	rows = append(rows, displayRow{
		kind:             RowKindCommitInfoBody,
		fileIndex:        -1,
		isCommitInfoBody: true,
		commitInfoLine:   "",
		commitIndex:      commitIdx,
		treePath:         bodyTreePath,
	})

	return rows
}

// structuralDiffMaxContentWidth calculates the maximum content width needed for
// structural diff lines for a file. This is used to expand the header box width
// if structural diff entries are wider than the filename. Returns 0 if no
// structural diff or no changes. The width is the content after the icon prefix:
// kind + space(1) + name/sig + stats.
// For child items, add 2 more for child indent.
func (m Model) structuralDiffMaxContentWidth(fileIdx int) int {
	fs := m.structureMaps[fileIdx]
	if fs == nil || fs.StructuralDiff == nil {
		return 0
	}

	topLevel, _ := structure.TopChanges(fs.StructuralDiff, maxStructuralDiffItems)
	if len(topLevel) == 0 {
		return 0
	}

	// Calculate max stats width across all entries
	maxStatsWidth := 0
	for _, node := range topLevel {
		w := statsTextWidth(node.Change.LinesAdded, node.Change.LinesRemoved)
		if w > maxStatsWidth {
			maxStatsWidth = w
		}
		for _, child := range node.Children {
			w := statsTextWidth(child.LinesAdded, child.LinesRemoved)
			if w > maxStatsWidth {
				maxStatsWidth = w
			}
		}
	}

	// Calculate max width available for signatures (80% of terminal minus overhead)
	maxSignatureWidth := 0
	if m.width > 0 {
		totalFiles := len(m.files)
		numDigits := len(fmt.Sprintf("%d", totalFiles))
		iconPartWidth := 9 + numDigits
		maxBoxWidth := m.width * 80 / 100
		overhead := 5 + 1 + maxStatsWidth
		maxSignatureWidth = maxBoxWidth - iconPartWidth - overhead
		if maxSignatureWidth < 20 {
			maxSignatureWidth = 20
		}
		if maxSignatureWidth > maxStructuralDiffSigWidth {
			maxSignatureWidth = maxStructuralDiffSigWidth
		}
	}

	entryDisplayWidth := func(entry *structure.Entry) int {
		sig := entry.FormatSignature(maxSignatureWidth)
		if sig == "" {
			return runewidth.StringWidth(entry.Name)
		}
		return runewidth.StringWidth(sig)
	}

	maxWidth := 0
	for _, node := range topLevel {
		entry := node.Change.Entry()
		if entry == nil {
			continue
		}
		width := runewidth.StringWidth(entry.Kind) + 1 + entryDisplayWidth(entry) + statsTextWidth(node.Change.LinesAdded, node.Change.LinesRemoved)
		if width > maxWidth {
			maxWidth = width
		}
		for _, child := range node.Children {
			childEntry := child.Entry()
			if childEntry == nil {
				continue
			}
			childWidth := 2 + runewidth.StringWidth(childEntry.Kind) + 1 + entryDisplayWidth(childEntry) + statsTextWidth(child.LinesAdded, child.LinesRemoved)
			if childWidth > maxWidth {
				maxWidth = childWidth
			}
		}
	}

	return maxWidth
}

// buildStructuralDiffRows creates display rows for the structural diff summary.
// Shows which functions, methods, and types were added, modified, or deleted.
// The rows are rendered inside the file header box, so they receive the same
// headerBoxWidth setting as the header line.
func (m Model) buildStructuralDiffRows(fileIdx int, headerBoxWidth int, isLastFileInCommit bool, isFileFolded bool) []displayRow {
	fs := m.structureMaps[fileIdx]
	if fs == nil || fs.StructuralDiff == nil {
		return nil
	}

	topLevel, truncatedCount := structure.TopChanges(fs.StructuralDiff, maxStructuralDiffItems)
	if len(topLevel) == 0 {
		return nil
	}

	// Build tree path for structural diff rows (content-level, inside file)
	structuralTreePath := m.buildFileTreePath(fileIdx, isLastFileInCommit, isFileFolded, TreeRowPreview)

	var rows []displayRow

	// Children get 2 extra spaces for indent
	childPrefix := "  "

	// Calculate max stats width across all entries for signature narrowing.
	maxStatsWidth := 0
	for _, node := range topLevel {
		w := statsTextWidth(node.Change.LinesAdded, node.Change.LinesRemoved)
		if w > maxStatsWidth {
			maxStatsWidth = w
		}
		for _, child := range node.Children {
			w := statsTextWidth(child.LinesAdded, child.LinesRemoved)
			if w > maxStatsWidth {
				maxStatsWidth = w
			}
		}
	}

	// Calculate max width available for signatures (80% of terminal minus overhead)
	maxSignatureWidth := 0
	if m.width > 0 {
		totalFiles := len(m.files)
		numDigits := len(fmt.Sprintf("%d", totalFiles))
		iconPartWidth := 9 + numDigits
		maxBoxWidth := m.width * 80 / 100
		overhead := 5 + 1 + maxStatsWidth
		maxSignatureWidth = maxBoxWidth - iconPartWidth - overhead
		if maxSignatureWidth < 20 {
			maxSignatureWidth = 20
		}
		if maxSignatureWidth > maxStructuralDiffSigWidth {
			maxSignatureWidth = maxStructuralDiffSigWidth
		}
	}

	formatEntry := func(entry *structure.Entry) string {
		sig := entry.FormatSignature(0)
		if sig == "" {
			return entry.Name
		}
		return entry.FormatSignature(maxSignatureWidth)
	}

	// Render tree
	for _, node := range topLevel {
		c := node.Change
		entry := c.Entry()
		if entry == nil {
			continue
		}

		nameOrSig := formatEntry(entry)
		line := entry.Kind + " " + nameOrSig

		rows = append(rows, displayRow{
			kind:                     RowKindStructuralDiff,
			fileIndex:                fileIdx,
			isStructuralDiff:         true,
			structuralDiffLine:       line,
			structuralDiffChangeKind: c.Kind,
			structuralDiffAdded:      c.LinesAdded,
			structuralDiffRemoved:    c.LinesRemoved,
			headerBoxWidth:           headerBoxWidth,
			isLastFileInCommit:       isLastFileInCommit,
			isFileFolded:             isFileFolded,
			treePath:                 structuralTreePath,
		})

		// Add children (methods within types) with extra indentation.
		// If the parent is added/deleted, children inherit that styling
		// so their identifiers also show bold+underline in the diff color.
		for _, child := range node.Children {
			childEntry := child.Entry()
			if childEntry == nil {
				continue
			}
			childNameOrSig := formatEntry(childEntry)
			childLine := childPrefix + childEntry.Kind + " " + childNameOrSig

			childChangeKind := child.Kind
			if c.Kind == structure.ChangeAdded || c.Kind == structure.ChangeDeleted {
				childChangeKind = c.Kind
			}

			rows = append(rows, displayRow{
				kind:                     RowKindStructuralDiff,
				fileIndex:                fileIdx,
				isStructuralDiff:         true,
				structuralDiffLine:       childLine,
				structuralDiffChangeKind: childChangeKind,
				structuralDiffAdded:      child.LinesAdded,
				structuralDiffRemoved:    child.LinesRemoved,
				headerBoxWidth:           headerBoxWidth,
				isLastFileInCommit:       isLastFileInCommit,
				isFileFolded:             isFileFolded,
				treePath:                 structuralTreePath,
			})
		}
	}

	// Add "...N more" row if we truncated
	if truncatedCount > 0 {
		moreLine := fmt.Sprintf("...(%d more)", truncatedCount)
		rows = append(rows, displayRow{
			kind:                      RowKindStructuralDiff,
			fileIndex:                 fileIdx,
			isStructuralDiff:          true,
			structuralDiffLine:        moreLine,
			structuralDiffIsTruncated: true,
			headerBoxWidth:            headerBoxWidth,
			isLastFileInCommit:        isLastFileInCommit,
			isFileFolded:              isFileFolded,
			treePath:                  structuralTreePath,
		})
	}

	// No trailing blank here - margins are handled at the file level in buildFileRows

	return rows
}

// commitInfoBodyStyler returns a SearchBaseStyler that applies prefixStyle to
// the first prefixLen bytes and valueStyle to the rest. If valueStyle is nil,
// the value portion is left unstyled. Used for "commit ", "Author: ", "Date:   " lines.
func commitInfoBodyStyler(prefixLen int, prefixStyle lipgloss.Style, valueStyle *lipgloss.Style) SearchBaseStyler {
	return func(text string, byteOffset int) string {
		// Entirely within value region
		if byteOffset >= prefixLen {
			if valueStyle != nil {
				return valueStyle.Render(text)
			}
			return text
		}
		// Entirely within prefix region
		if byteOffset+len(text) <= prefixLen {
			return prefixStyle.Render(text)
		}
		// Spans the boundary
		splitAt := prefixLen - byteOffset
		result := prefixStyle.Render(text[:splitAt])
		if valueStyle != nil {
			result += valueStyle.Render(text[splitAt:])
		} else {
			result += text[splitAt:]
		}
		return result
	}
}

// renderCommitBodyRow renders a single line of the commit body.
func (m Model) renderCommitBodyRow(row displayRow, isCursorRow bool) string {
	line := row.commitBodyLine

	// Build base styler to preserve colours under search highlighting
	var baseStyler SearchBaseStyler
	if strings.HasPrefix(line, "commit ") {
		shaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		baseStyler = commitInfoBodyStyler(7, lipgloss.NewStyle(), &shaStyle) // "commit " prefix unstyled, SHA in yellow
	}

	content := m.highlightSearchInVisibleStyled(line, isCursorRow, m.currentMatchIdx(), 0, m.currentMatchSide(), baseStyler)

	// Cursor handling with 1-char bg highlight (like file headers)
	if isCursorRow && m.focused {
		// Format: arrow + space + [1 char bg] + space + content
		styledGutter := cursorStyle.Render(" ")
		return cursorArrowStyle.Render("▌") + " " + styledGutter + " " + content
	}

	if isCursorRow && !m.focused {
		// Unfocused: outline arrow, no background highlight
		return " " + "   " + content
	}

	// Non-cursor: 2-space prefix + 2-space indent
	return "    " + content
}

// renderStructuralDiffRow renders a single line of the structural diff summary.
// The row is rendered inside the header box with proper padding and border.
// Structural diff rows are always shown under folded files, so borders are always dark.
func (m Model) renderStructuralDiffRow(row displayRow, isCursorRow bool) string {
	content := row.structuralDiffLine
	headerBoxWidth := row.headerBoxWidth

	// Tree prefix using tight spacing for compact content
	treeContinuation := renderTreePrefixTight(row.treePath)

	// Handle truncated "...N more" row - plain fg7 text, no stats
	if row.structuralDiffIsTruncated {
		moreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
		styledContent := moreStyle.Render(content)
		contentWidth := runewidth.StringWidth(content)
		paddingNeeded := headerBoxWidth - contentWidth + 2
		padding := ""
		if paddingNeeded > 0 {
			padding = strings.Repeat(" ", paddingNeeded)
		}
		borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
		var result string
		if isCursorRow && m.focused {
			result = cursorArrowStyle.Render("▌") + treeContinuation[1:] + styledContent + padding
		} else if isCursorRow && !m.focused {
			result = " " + treeContinuation[1:] + styledContent + padding
		} else {
			result = treeContinuation + styledContent + padding
		}
		result += " " + borderStyle.Render("│")
		return result
	}

	// Extract parts: prefix (spaces for child indent) + "kind signature"
	// Format: "kind name" (top-level) or "  kind name" (child)
	var prefix, rest string
	// Find first non-space character (start of kind)
	kindPos := 0
	for i, c := range content {
		if c != ' ' {
			kindPos = i
			break
		}
	}
	if kindPos < len(content) {
		prefix = content[:kindPos]
		rest = content[kindPos:]
	} else {
		prefix = content
		rest = ""
	}

	// Dark colors for line count stats
	darkAddedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	darkRemovedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// Style the "kind signature" with syntax-style highlighting and change-kind styling
	kindStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	changeKind := row.structuralDiffChangeKind
	var styledContent string
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) == 2 {
		styledContent = kindStyle.Render(parts[0]) + " " + styleSig(parts[1], changeKind)
	} else {
		styledContent = rest // fallback
	}

	// Format line count stats (e.g., " +3 -4") - placed straight after signature
	var statsStr string
	var statsWidth int
	if row.structuralDiffAdded > 0 || row.structuralDiffRemoved > 0 {
		var parts []string
		var partsWidth int
		if row.structuralDiffAdded > 0 {
			raw := fmt.Sprintf("+%d", row.structuralDiffAdded)
			parts = append(parts, darkAddedStyle.Render(raw))
			partsWidth += len(raw)
		}
		if row.structuralDiffRemoved > 0 {
			raw := fmt.Sprintf("-%d", row.structuralDiffRemoved)
			parts = append(parts, darkRemovedStyle.Render(raw))
			partsWidth += len(raw)
		}
		statsStr = " " + strings.Join(parts, " ")
		statsWidth = 1 + partsWidth + len(parts) - 1 // leading space + parts + separating spaces
	}

	// Calculate padding: signature + stats must fill to headerBoxWidth + 2
	originalWidth := runewidth.StringWidth(content) + statsWidth
	paddingNeeded := headerBoxWidth - originalWidth + 2
	padding := ""
	if paddingNeeded > 0 {
		padding = strings.Repeat(" ", paddingNeeded)
	}

	// Border style - always dark for structural diff rows (shown under folded files)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("0"))

	// Build: prefix + kind + signature + padding + stats + border
	var result string
	if isCursorRow && m.focused {
		result = cursorArrowStyle.Render("▌") + treeContinuation[1:] + prefix + styledContent + statsStr + padding
	} else if isCursorRow && !m.focused {
		result = " " + treeContinuation[1:] + prefix + styledContent + statsStr + padding
	} else {
		result = treeContinuation + prefix + styledContent + statsStr + padding
	}

	// Add border (│)
	result += " " + borderStyle.Render("│")

	return result
}
