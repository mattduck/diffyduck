package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleHelpKey handles key events while the help screen is displayed.
func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keys := m.keys

	switch {
	case msg.String() == "q" || msg.String() == "esc" || msg.String() == "ctrl+g" || msg.String() == "ctrl+c":
		m.helpMode = false
		return m, nil

	case matchesKey(msg, keys.Down):
		m.helpScroll++
		m.clampHelpScroll()

	case matchesKey(msg, keys.Up):
		m.helpScroll--
		m.clampHelpScroll()

	case matchesKey(msg, keys.PageDown):
		m.helpScroll += m.helpViewportHeight()
		m.clampHelpScroll()

	case matchesKey(msg, keys.PageUp):
		m.helpScroll -= m.helpViewportHeight()
		m.clampHelpScroll()

	case matchesKey(msg, keys.HalfDown):
		m.helpScroll += m.helpViewportHeight() / 2
		m.clampHelpScroll()

	case matchesKey(msg, keys.HalfUp):
		m.helpScroll -= m.helpViewportHeight() / 2
		m.clampHelpScroll()

	case matchesKey(msg, keys.Bottom):
		m.helpScroll = m.maxHelpScroll()
		m.clampHelpScroll()

	case matchesKey(msg, keys.Top):
		m.helpScroll = 0

	case m.keys.prefixSet[msg.String()]:
		// Support sequences (e.g. gg to go to top)
		m.pendingKey = msg.String()
		return m, nil
	}

	return m, nil
}

// helpViewportHeight returns the number of content lines visible in the help screen.
// Accounts for the title bar (1 line) and bottom bar (1 line).
func (m Model) helpViewportHeight() int {
	h := m.height - 2 // title bar + bottom bar
	if h < 1 {
		return 1
	}
	return h
}

// clampHelpScroll ensures helpScroll is within valid bounds.
func (m *Model) clampHelpScroll() {
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
	if max := m.maxHelpScroll(); m.helpScroll > max {
		m.helpScroll = max
	}
}

// maxHelpScroll returns the maximum valid help scroll offset.
func (m Model) maxHelpScroll() int {
	max := len(m.helpLines) - m.helpViewportHeight()
	if max < 0 {
		return 0
	}
	return max
}

// helpGroupBlock holds the pre-rendered lines for a single binding group,
// both plain (for scroll counting) and styled (for display).
type helpGroupBlock struct {
	plain  []string // unstyled lines (for width measurement and helpLines)
	styled []string // ANSI-styled lines (for rendering)
}

// buildHelpGroupBlocks builds rendered blocks for each binding group.
func buildHelpGroupBlocks(groups []BindingGroup) []helpGroupBlock {
	slash := " / "
	styledSlash := " " + helpSlashStyle.Render("/") + " "

	blocks := make([]helpGroupBlock, len(groups))
	for i, group := range groups {
		var plain, styled []string

		// Section header
		plain = append(plain, "  "+group.Name)
		styled = append(styled, "  "+helpSectionStyle.Render(group.Name))

		// Pre-format all bindings and measure description widths
		type formattedBinding struct {
			descPlain  string // e.g. "Scroll down / up"
			descStyled string
			keysPlain  string // e.g. "↓  j / ↑  k"
			keysStyled string // same but with ANSI styles on "/"
		}

		formatted := make([]formattedBinding, len(group.Bindings))
		maxDescWidth := 0

		for j, b := range group.Bindings {
			kp := formatBindingKeys(b.Keys)
			ks := helpKeyStyle.Render(kp)
			dp := b.Desc
			ds := helpDescStyle.Render(b.Desc)

			if len(b.Keys2) > 0 {
				kp2 := formatBindingKeys(b.Keys2)
				kp = kp + slash + kp2
				ks = ks + styledSlash + helpKeyStyle.Render(kp2)
				dp = dp + slash + b.Desc2
				ds = ds + styledSlash + helpDescStyle.Render(b.Desc2)
			}

			formatted[j] = formattedBinding{dp, ds, kp, ks}
			if w := displayWidth(dp); w > maxDescWidth {
				maxDescWidth = w
			}
		}

		for _, fb := range formatted {
			padding := maxDescWidth - displayWidth(fb.descPlain)
			if padding < 0 {
				padding = 0
			}
			pad := strings.Repeat(" ", padding)
			plain = append(plain, fmt.Sprintf("    %s%s   %s", fb.descPlain, pad, fb.keysPlain))
			styled = append(styled, "    "+fb.descStyled+pad+"   "+fb.keysStyled)
		}

		blocks[i] = helpGroupBlock{plain: plain, styled: styled}
	}
	return blocks
}

// columnGap is the space between two columns.
const columnGap = 4

// leftColumnMaxWidth returns the widest plain-text line across the left-column blocks.
func leftColumnMaxWidth(blocks []helpGroupBlock) int {
	max := 0
	for _, b := range blocks {
		for _, line := range b.plain {
			if w := displayWidth(line); w > max {
				max = w
			}
		}
	}
	return max
}

// canFitTwoColumns returns true if the left-column content plus gap plus a
// reasonable minimum right column (20 chars) fits within termWidth.
func canFitTwoColumns(leftWidth, termWidth int) bool {
	return leftWidth+columnGap+20 <= termWidth
}

// buildHelpLines generates the plain help content lines (for scroll counting).
// Called when entering help mode and on resize.
func (m Model) buildHelpLines() []string {
	groups := AllBindingGroups(m.keys)
	blocks := buildHelpGroupBlocks(groups)
	left, right := splitBlocksIntoColumns(blocks)
	leftWidth := leftColumnMaxWidth(left)

	if len(right) > 0 && canFitTwoColumns(leftWidth, m.width) {
		return mergePlainColumns(flattenPlain(left), flattenPlain(right), leftWidth)
	}
	return buildSingleColumnPlain(blocks)
}

func buildSingleColumnPlain(blocks []helpGroupBlock) []string {
	var lines []string
	lines = append(lines, "") // blank after title

	for i, block := range blocks {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block.plain...)
	}
	lines = append(lines, "")
	return lines
}

// splitBlocksIntoColumns distributes blocks into two columns, roughly balancing
// total line count. Groups are kept intact (never split across columns).
func splitBlocksIntoColumns(blocks []helpGroupBlock) ([]helpGroupBlock, []helpGroupBlock) {
	// Count total lines (including 1 blank separator between groups)
	totalLines := 0
	for i, b := range blocks {
		totalLines += len(b.plain)
		if i > 0 {
			totalLines++ // blank between groups
		}
	}

	half := totalLines / 2
	running := 0
	splitAt := len(blocks) // default: all in left column

	for i, b := range blocks {
		extra := len(b.plain)
		if i > 0 {
			extra++ // blank separator
		}
		if running+extra > half && i > 0 {
			// Check if splitting here or including this block is closer to half
			withThis := running + extra
			if withThis-half < half-running {
				splitAt = i + 1
			} else {
				splitAt = i
			}
			break
		}
		running += extra
	}

	if splitAt >= len(blocks) {
		splitAt = len(blocks)
	}
	return blocks[:splitAt], blocks[splitAt:]
}

func flattenPlain(blocks []helpGroupBlock) []string {
	var lines []string
	for i, b := range blocks {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, b.plain...)
	}
	return lines
}

func flattenStyled(blocks []helpGroupBlock) []string {
	var lines []string
	for i, b := range blocks {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, b.styled...)
	}
	return lines
}

// mergePlainColumns merges two plain-text column slices side by side.
// leftColWidth is the actual max content width of the left column.
func mergePlainColumns(left, right []string, leftColWidth int) []string {
	height := len(left)
	if len(right) > height {
		height = len(right)
	}

	lines := make([]string, 0, height+2)
	lines = append(lines, "") // blank after title

	gap := strings.Repeat(" ", columnGap)
	for i := 0; i < height; i++ {
		l := ""
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}

		padding := leftColWidth - displayWidth(l)
		if padding < 0 {
			padding = 0
		}
		lines = append(lines, l+strings.Repeat(" ", padding)+gap+r)
	}

	lines = append(lines, "")
	return lines
}

// mergeStyledColumns merges two styled column slices side by side.
// leftPlain is used for width measurement (ANSI codes don't count).
func mergeStyledColumns(left, right, leftPlain []string, leftColWidth int) []string {
	height := len(left)
	if len(right) > height {
		height = len(right)
	}

	lines := make([]string, 0, height+2)
	lines = append(lines, "") // blank after title

	gap := strings.Repeat(" ", columnGap)
	for i := 0; i < height; i++ {
		l := ""
		lp := ""
		if i < len(left) {
			l = left[i]
			lp = leftPlain[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}

		padding := leftColWidth - displayWidth(lp)
		if padding < 0 {
			padding = 0
		}
		lines = append(lines, l+strings.Repeat(" ", padding)+gap+r)
	}

	lines = append(lines, "")
	return lines
}

// formatBindingKeys formats a binding's key list for display.
// Applies display transformations: "up" → "↑", "ctrl+x" → "C-x", etc.
func formatBindingKeys(keys []string) string {
	formatted := make([]string, len(keys))
	for i, k := range keys {
		formatted[i] = formatKeyForDisplay(k)
	}
	return strings.Join(formatted, "  ")
}

// formatKeyForDisplay converts an internal key name to a human-readable form.
// For sequences (space-separated tokens like "g g"), each token is formatted
// individually and rejoined without spaces (e.g. "g g" → "gg", "ctrl+w %" → "C-w%").
func formatKeyForDisplay(key string) string {
	// Handle sequences (2+ tokens separated by spaces)
	if parts := strings.Fields(key); len(parts) > 1 {
		formatted := make([]string, len(parts))
		for i, p := range parts {
			formatted[i] = formatKeyForDisplay(p)
		}
		return strings.Join(formatted, "")
	}

	switch key {
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	case "pgup":
		return "PgUp"
	case "pgdown":
		return "PgDn"
	case "home":
		return "Home"
	case "end":
		return "End"
	case "tab":
		return "Tab"
	case "shift+tab":
		return "S-Tab"
	case "enter":
		return "Enter"
	case " ":
		return "Space"
	case "esc":
		return "Esc"
	case "backspace":
		return "Backspace"
	case "delete":
		return "Delete"
	}

	// ctrl+x → C-x
	if strings.HasPrefix(key, "ctrl+") {
		return "C-" + key[5:]
	}

	return key
}

var (
	helpTitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	helpKeyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	helpDescStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	helpDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpSlashStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)

// renderHelp renders the help screen content.
// Rebuilt on each frame so column layout adapts to terminal width.
func (m Model) renderHelp() string {
	// Title bar
	title := helpTitleStyle.Render("diffyduck help")
	titleLine := "  " + title

	// Bottom bar
	bottomText := helpDimStyle.Render(" q, Esc, C-g or C-h to go back")
	bottomPad := m.width - displayWidth(bottomText)
	if bottomPad < 0 {
		bottomPad = 0
	}
	bottomLine := bottomText + strings.Repeat(" ", bottomPad)

	// Build styled content
	groups := AllBindingGroups(m.keys)
	blocks := buildHelpGroupBlocks(groups)
	left, right := splitBlocksIntoColumns(blocks)
	leftWidth := leftColumnMaxWidth(left)

	var styledLines []string
	if len(right) > 0 && canFitTwoColumns(leftWidth, m.width) {
		leftStyled := flattenStyled(left)
		rightStyled := flattenStyled(right)
		leftPlain := flattenPlain(left)
		styledLines = mergeStyledColumns(leftStyled, rightStyled, leftPlain, leftWidth)
	} else {
		styledLines = make([]string, 0)
		styledLines = append(styledLines, "") // blank after title
		for i, block := range blocks {
			if i > 0 {
				styledLines = append(styledLines, "")
			}
			styledLines = append(styledLines, block.styled...)
		}
		styledLines = append(styledLines, "")
	}

	// Apply scroll
	viewportHeight := m.helpViewportHeight()
	start := m.helpScroll
	if start > len(styledLines) {
		start = len(styledLines)
	}
	end := start + viewportHeight
	if end > len(styledLines) {
		end = len(styledLines)
	}

	visible := styledLines[start:end]

	// Pad remaining lines to fill viewport
	for len(visible) < viewportHeight {
		visible = append(visible, "")
	}

	return titleLine + "\n" + strings.Join(visible, "\n") + "\n" + bottomLine
}
