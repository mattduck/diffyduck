package ticketcli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattduck/diffyduck/pkg/diff"
	"github.com/mattduck/diffyduck/pkg/git"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
	"golang.org/x/term"
)

// runNote dispatches note sub-commands. Notes are standalone comments
// (no file attachment). This is shorthand for comment commands with --kind note.
func runNote(opts Options) error {
	switch opts.Sub {
	case "list":
		opts.Kind = "note"
		return runCommentList(opts)
	case "edit":
		return runNoteEdit(opts.ID, opts.Resolved)
	case "resolve":
		resolved := true
		return runNoteEdit(opts.ID, &resolved)
	case "unresolve":
		resolved := false
		return runNoteEdit(opts.ID, &resolved)
	case "add":
		return runCommentAddStandalone(opts)
	default:
		usage()
		return nil
	}
}

// runNoteEdit validates that the comment is standalone, then delegates to runCommentEdit.
func runNoteEdit(id string, resolved *bool) error {
	store := ticketdb.NewStore("")

	fullID, err := resolveCommentID(store, id)
	if err != nil {
		return err
	}

	c, err := store.ReadComment(fullID)
	if err != nil {
		return fmt.Errorf("comment %s not found: %w", fullID, err)
	}

	if !c.IsStandalone() {
		return fmt.Errorf("comment %s is attached to %s:%d (use 'comment edit' instead)", id, c.File, c.Line)
	}

	return runCommentEdit(fullID, resolved)
}

// runComment dispatches comment sub-commands.
func runComment(opts Options) error {
	switch opts.Sub {
	case "list":
		if opts.Kind == "" {
			opts.Kind = "comment"
		}
		return runCommentList(opts)
	case "edit":
		return runCommentEdit(opts.ID, opts.Resolved)
	case "resolve":
		resolved := true
		return runCommentEdit(opts.ID, &resolved)
	case "unresolve":
		resolved := false
		return runCommentEdit(opts.ID, &resolved)
	case "add":
		return runCommentAdd(opts)
	default:
		usage()
		return nil
	}
}

// runCommentList lists comments filtered by status and limited by -n.
// runStateList renders git-state tickets using ListOptions. It is the shared
// implementation for `tdb list --source state` and `tdb comment list`.
func runStateList(o ListOptions) error {
	cs := o.Styles
	store := ticketdb.NewStore("")
	all, err := store.AllComments()
	if err != nil {
		return fmt.Errorf("reading comments: %w", err)
	}

	allIDs := make([]string, len(all))
	for i, c := range all {
		allIDs[i] = c.ID
	}
	shortIDs := shortSuffixes(allIDs)

	if o.ID != "" {
		var matched []*ticketdb.Comment
		for _, c := range all {
			if strings.HasSuffix(c.ID, o.ID) {
				matched = append(matched, c)
			}
		}
		all = matched
	}

	if o.ID == "" {
		status := o.Status
		if status == "" {
			status = "unresolved"
		}
		if status != "all" {
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if status == "resolved" && c.Resolved {
					filtered = append(filtered, c)
				} else if status == "unresolved" && !c.Resolved {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}

		switch o.Kind {
		case "comment":
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if !c.IsStandalone() {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		case "note":
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if c.IsStandalone() {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}

		if o.Since != "" {
			dur, err := parseSinceDuration(o.Since)
			if err != nil {
				return fmt.Errorf("invalid --since value: %w", err)
			}
			if dur > 0 {
				cutoff := time.Now().Add(-dur)
				var filtered []*ticketdb.Comment
				for _, c := range all {
					if c.Created.After(cutoff) {
						filtered = append(filtered, c)
					}
				}
				all = filtered
			}
		}

		if o.Branch != "" {
			branch := o.Branch
			if branch == "." {
				if cb, err := store.CurrentBranch(); err == nil && cb != "" {
					branch = cb
				} else {
					return fmt.Errorf("could not determine current branch")
				}
			}
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if c.Branch == branch {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		} else if !o.AllBranches {
			currentBranch, _ := store.CurrentBranch()
			if currentBranch != "" {
				var filtered []*ticketdb.Comment
				for _, c := range all {
					if c.Branch == currentBranch {
						filtered = append(filtered, c)
					}
				}
				all = filtered
			} else {
				fmt.Fprintln(os.Stderr, "warning: detached HEAD — showing comments from all branches")
			}
		}

		if o.AuthorSet {
			if o.Author == "" {
				var filtered []*ticketdb.Comment
				for _, c := range all {
					if c.Author == "" {
						filtered = append(filtered, c)
					}
				}
				all = filtered
			} else {
				needle := strings.ToLower(o.Author)
				var filtered []*ticketdb.Comment
				for _, c := range all {
					if strings.Contains(strings.ToLower(c.Author), needle) {
						filtered = append(filtered, c)
					}
				}
				all = filtered
			}
		}

		if o.File != "" {
			isPrefix := strings.HasSuffix(o.File, "/")
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if isPrefix && strings.HasPrefix(c.File, o.File) {
					filtered = append(filtered, c)
				} else if !isPrefix && c.File == o.File {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}

		if o.Grep != "" {
			needle := strings.ToLower(o.Grep)
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if strings.Contains(strings.ToLower(c.Text), needle) {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}

		if len(o.Markers) > 0 {
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if markerMatches(o.Markers, c.Prefix) {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}

		if o.Type != "" {
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if strings.EqualFold(o.Type, c.Type) {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}

		if o.Scope != "" {
			var filtered []*ticketdb.Comment
			for _, c := range all {
				if strings.EqualFold(o.Scope, c.Scope) {
					filtered = append(filtered, c)
				}
			}
			all = filtered
		}
	}

	if len(all) == 0 {
		if o.ID != "" {
			fmt.Printf("No comment matching suffix %q\n", o.ID)
		} else {
			fmt.Println("No comments")
		}
		return exitCodeResult(o.ExitCode, false)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Created.After(all[j].Created)
	})

	showBlock := o.Verbose || o.ID != ""

	totalCount := len(all)
	truncated := false
	if o.ID == "" {
		if !o.NSet {
			defaultN := 20
			if showBlock {
				defaultN = 5
			}
			if len(all) > defaultN {
				all = all[:defaultN]
				truncated = true
			}
		} else if o.N == 0 {
			// uncapped
		} else if o.N > 0 {
			if o.N < len(all) {
				all = all[:o.N]
				truncated = true
			}
		} else {
			count := -o.N
			if count < len(all) {
				all = all[len(all)-count:]
				truncated = true
			}
		}
	}

	slices.Reverse(all)

	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}

	if !o.Raw && !showBlock {
		now := time.Now()
		cols := computeOnelineCols(all, shortIDs, now)
		for _, c := range all {
			fmt.Println(formatCommentOneline(c, shortIDs[c.ID], termWidth, now, cols, cs))
		}
		if truncated {
			fmt.Printf("%s\n", cs.Label.Render(fmt.Sprintf("%d/%d", len(all), totalCount)))
		}
		return exitCodeResult(o.ExitCode, true)
	}

	for i, c := range all {
		if o.Raw {
			if i > 0 {
				fmt.Print("\n")
			}
			fmt.Print(c.Serialize())
		} else {
			if i > 0 {
				fmt.Print("\n\n")
			}
			fmt.Print(formatCommentBlock(c, o.Highlighter, termWidth, shortIDs[c.ID], time.Now(), cs))
		}
	}
	if truncated && !o.Raw {
		fmt.Printf("\n%s\n", cs.Label.Render(fmt.Sprintf("%d/%d", len(all), totalCount)))
	}
	return exitCodeResult(o.ExitCode, true)
}

func runCommentList(opts Options) error {
	return runStateList(ListOptions{
		Source:      SourceState,
		Kind:        opts.Kind,
		Status:      opts.Status,
		Since:       opts.Since,
		Author:      opts.Author,
		AuthorSet:   opts.AuthorSet,
		File:        opts.File,
		Grep:        opts.Grep,
		Markers:     markerList(opts.Marker),
		Type:        opts.Type,
		Scope:       opts.Scope,
		N:           opts.N,
		NSet:        opts.NSet,
		AllBranches: opts.AllBranches,
		Branch:      opts.Branch,
		Verbose:     opts.Verbose,
		Raw:         opts.Raw,
		ID:          opts.ID,
		Styles:      opts.Styles,
		Highlighter: opts.Highlighter,
	})
}

// onelineCols holds the computed column widths for oneline output,
// derived from the actual content of all comments being displayed.
type onelineCols struct {
	id     int // width of ID column
	date   int // width of date column
	commit int // width of commit SHA column
	branch int // width of branch column (0 if no comments have branches)
	file   int // width of file column (0 if no comments have files)
}

// computeOnelineCols scans all comments to determine the column widths.
// Each column is sized to fit the widest value, capped at a maximum.
func computeOnelineCols(all []*ticketdb.Comment, shortIDs map[string]string, now time.Time) onelineCols {
	var cols onelineCols
	for _, c := range all {
		id := shortIDs[c.ID]
		if id == "" {
			id = c.ID
		}
		if w := len(id); w > cols.id {
			cols.id = w
		}

		dateStr := c.Created.Format("Jan 02 15:04")
		age := FormatRelativeAge(now, c.Created)
		if w := len(dateStr) + 1 + len(age); w > cols.date {
			cols.date = w
		}

		commitShort := c.CommitSHA
		if len(commitShort) > 7 {
			commitShort = commitShort[:7]
		}
		if commitShort == "" && c.BranchHead != "" {
			commitShort = c.BranchHead
			if len(commitShort) > 7 {
				commitShort = commitShort[:7]
			}
			commitShort = "(" + commitShort + ")"
		}
		if commitShort == "" {
			commitShort = "-"
		}
		if w := len(commitShort); w > cols.commit {
			cols.commit = w
		}

		if w := len(c.Branch); w > cols.branch {
			cols.branch = w
		}

		if !c.IsStandalone() {
			raw := fmt.Sprintf("%s:%d", c.File, c.Line)
			if w := len(raw); w > cols.file {
				cols.file = w
			}
		}
	}
	// Apply max caps
	if cols.id > 8 {
		cols.id = 8
	}
	if cols.date > 18 {
		cols.date = 18
	}
	if cols.commit > 9 { // 7-char hash + 2 parens for "(abcdef0)"
		cols.commit = 9
	}
	if cols.branch > 20 {
		cols.branch = 20
	}
	if cols.file > 30 {
		cols.file = 30
	}
	return cols
}

// formatCommentOneline formats a single comment as a compact one-liner with
// dynamically-sized columns: ID, date, commit, branch, file, title.
// The title column fills remaining terminal width and is truncated to fit.
func formatCommentOneline(c *ticketdb.Comment, displayID string, termWidth int, now time.Time, cols onelineCols, cs CommentListStyles) string {
	if displayID == "" {
		displayID = c.ID
	}

	strike := func(s lipgloss.Style) lipgloss.Style { return s.Strikethrough(true) }

	// Column 1: ID (bold; strikethrough if resolved)
	idCol := displayID
	if len(idCol) > cols.id {
		idCol = idCol[:cols.id]
	}
	idStyle := cs.Header.Bold(true)
	if c.Resolved {
		idStyle = strike(idStyle)
	}
	idStyled := idStyle.Render(idCol)

	// Column 2: date with relative age (strikethrough if resolved)
	dateStr := c.Created.Format("Jan 02 15:04")
	age := FormatRelativeAge(now, c.Created)
	dateCol := fmt.Sprintf("%s %s", dateStr, age)
	if len(dateCol) > cols.date {
		dateCol = dateCol[:cols.date]
	}
	dateStyle := cs.Label
	if c.Resolved {
		dateStyle = strike(dateStyle)
	}
	dateStyled := dateStyle.Render(dateCol)

	// Column 3: commit SHA (strikethrough if resolved)
	// Falls back to BranchHead when no explicit commit ref.
	commitShort := c.CommitSHA
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}
	if commitShort == "" && c.BranchHead != "" {
		commitShort = c.BranchHead
		if len(commitShort) > 7 {
			commitShort = commitShort[:7]
		}
		commitShort = "(" + commitShort + ")"
	}
	if commitShort == "" {
		commitShort = "-"
	}
	commitPlainWidth := len(commitShort)
	commitStyle := cs.Commit
	if c.Resolved {
		commitStyle = strike(commitStyle)
	}
	commitStyled := commitStyle.Render(commitShort)

	// Column 4: branch (strikethrough if resolved)
	branchPart := c.Branch
	if cols.branch > 0 && len(branchPart) > cols.branch {
		branchPart = branchPart[:cols.branch-1] + "…"
	}
	var branchStyled string
	branchPlainWidth := len(branchPart)
	if branchPart != "" {
		branchStyle := cs.Branch
		if c.Resolved {
			branchStyle = strike(branchStyle)
		}
		branchStyled = branchStyle.Render(branchPart)
	}

	// Column 5: file (optional, strikethrough if resolved)
	var fileStyled string
	var filePlainWidth int
	if !c.IsStandalone() && cols.file > 0 {
		var filePart string
		if c.Resolved {
			raw := fmt.Sprintf("%s:%d", c.File, c.Line)
			if len(raw) > cols.file {
				raw = raw[:cols.file-1] + "…"
			}
			filePart = strike(cs.DirPart).Render(raw)
			filePlainWidth = len(raw)
		} else {
			filePart = styleCommentPath(c.File, c.Line, cs)
			filePlainWidth = lipgloss.Width(filePart)
			if filePlainWidth > cols.file {
				raw := fmt.Sprintf("%s:%d", c.File, c.Line)
				if len(raw) > cols.file-1 {
					raw = raw[:cols.file-2] + "…"
				}
				filePart = cs.DirPart.Render(raw)
				filePlainWidth = len(raw)
			}
		}
		fileStyled = filePart
	}

	// Last column: author + title (fills remaining width; gray text if resolved)
	authorPart := ""
	authorWidth := 0
	if c.Author != "" {
		authorPart = cs.Label.Render(fmt.Sprintf("[%s]", c.Author)) + " "
		authorWidth = len(c.Author) + 3 // "[author] "
	}

	// Calculate remaining width for title
	usedWidth := cols.id + 1 + cols.date + 1 + cols.commit + 1
	if cols.branch > 0 {
		usedWidth += cols.branch + 1
	}
	if cols.file > 0 {
		usedWidth += cols.file + 1
	}
	usedWidth += authorWidth

	text := c.Text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	titleMax := termWidth - usedWidth
	if titleMax < 10 {
		titleMax = 10
	}
	if len(text) > titleMax {
		if titleMax > 3 {
			text = text[:titleMax-3] + "..."
		} else {
			text = text[:titleMax]
		}
	}
	if c.Resolved {
		text = cs.Label.Render(text)
	}

	// Build the line with padded columns
	var b strings.Builder
	b.WriteString(idStyled)
	b.WriteString(strings.Repeat(" ", cols.id-len(idCol)))
	b.WriteByte(' ')
	b.WriteString(dateStyled)
	b.WriteString(strings.Repeat(" ", cols.date-len(dateCol)))
	b.WriteByte(' ')
	b.WriteString(commitStyled)
	b.WriteString(strings.Repeat(" ", cols.commit-commitPlainWidth))
	b.WriteByte(' ')
	if cols.branch > 0 {
		b.WriteString(branchStyled)
		if branchPlainWidth < cols.branch {
			b.WriteString(strings.Repeat(" ", cols.branch-branchPlainWidth))
		}
		b.WriteByte(' ')
	}
	if cols.file > 0 {
		b.WriteString(fileStyled)
		if filePlainWidth < cols.file {
			b.WriteString(strings.Repeat(" ", cols.file-filePlainWidth))
		}
		b.WriteByte(' ')
	}
	b.WriteString(authorPart)
	b.WriteString(text)

	return b.String()
}

// formatCommentBlock formats a comment as a multiline block showing the full
// serialized patch context and metadata. If h is non-nil, context lines are
// syntax-highlighted based on the comment's file extension. When termWidth is
// wide enough, metadata is rendered in two columns.
func formatCommentBlock(c *ticketdb.Comment, ch ContextHighlighter, termWidth int, suffix string, now time.Time, cs CommentListStyles) string {
	var b strings.Builder

	// Helper to format "Label:   value" with styled label
	labelVal := func(label, value string) string {
		return cs.Label.Render(label) + value
	}

	// Header line
	commitShort := c.CommitSHA
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}

	// Build left column: Date, Status, ID
	dateVal := c.Created.Format("Jan 02 15:04") + " " + FormatRelativeAge(now, c.Created)
	var statusLine string
	if c.Resolved {
		statusLine = labelVal("Status:", " "+cs.Label.Render("resolved"))
	} else {
		statusLine = labelVal("Status:", " unresolved")
	}
	var idLine string
	if suffix != "" && strings.HasSuffix(c.ID, suffix) {
		prefix := c.ID[:len(c.ID)-len(suffix)]
		idLine = labelVal("ID:", "     "+cs.Label.Render(prefix)+cs.Header.Render(suffix))
	} else {
		idLine = labelVal("ID:", "     "+cs.Label.Render(c.ID))
	}

	// Build right column: File (if attached), Ref
	var fileLine string
	if !c.IsStandalone() {
		fileLine = labelVal("File:", "   "+styleCommentPath(c.File, c.Line, cs))
	}
	var refLine string
	if commitShort != "" && c.Branch != "" {
		refLine = labelVal("Ref:", "    "+cs.Commit.Render(commitShort)+" on "+cs.Branch.Render(c.Branch))
	} else if commitShort != "" {
		refLine = labelVal("Ref:", "    "+cs.Commit.Render(commitShort))
	} else if c.BranchHead != "" && c.Branch != "" {
		bh := c.BranchHead
		if len(bh) > 7 {
			bh = bh[:7]
		}
		refLine = labelVal("Ref:", "    "+cs.Commit.Render("("+bh+")")+" on "+cs.Branch.Render(c.Branch))
	} else if c.BranchHead != "" {
		bh := c.BranchHead
		if len(bh) > 7 {
			bh = bh[:7]
		}
		refLine = labelVal("Ref:", "    "+cs.Commit.Render("("+bh+")"))
	} else if c.Branch != "" {
		refLine = labelVal("Ref:", "    "+cs.Branch.Render(c.Branch))
	}

	// Left column width anchored on Date line (the longest fixed-length field).
	// "Date:   " = 8 chars + date value + 2 char gap
	leftColW := 8 + len(dateVal) + 2

	// Determine if two-column layout fits (account for bar prefix "┃ " = 2 chars)
	twoCol := termWidth >= leftColW+20+2 // 20 = minimum useful right column

	if twoCol {
		left := []string{
			labelVal("Date:", "   "+dateVal),
			statusLine,
			idLine,
		}
		var right []string
		if fileLine != "" {
			right = append(right, fileLine)
		}
		if refLine != "" {
			right = append(right, refLine)
		}
		if c.Author != "" {
			right = append(right, labelVal("Author:", " "+cs.Branch.Render(c.Author)))
		}
		rows := max(len(left), len(right))
		for i := range rows {
			var l, r string
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			if r != "" {
				// Pad left column to fixed width using visible length
				visLen := lipgloss.Width(l)
				pad := leftColW - visLen
				if pad < 0 {
					pad = 0
				}
				fmt.Fprintf(&b, "%s%s%s\n", l, strings.Repeat(" ", pad), r)
			} else {
				fmt.Fprintf(&b, "%s\n", l)
			}
		}
	} else {
		// Single column fallback
		fmt.Fprintf(&b, "%s\n", labelVal("Date:", "   "+dateVal))
		fmt.Fprintf(&b, "%s\n", statusLine)
		fmt.Fprintf(&b, "%s\n", idLine)
		if fileLine != "" {
			fmt.Fprintf(&b, "%s\n", fileLine)
		}
		if refLine != "" {
			fmt.Fprintf(&b, "%s\n", refLine)
		}
		if c.Author != "" {
			fmt.Fprintf(&b, "%s\n", labelVal("Author:", " "+cs.Branch.Render(c.Author)))
		}
	}

	// Diff context (with optional syntax highlighting) — skip for standalone comments
	if !c.IsStandalone() {
		targetLineStyle := cs.Header.Bold(true)
		contextLineStyle := cs.Label.Faint(true)
		b.WriteString("\n")
		contextLines := highlightedContext(c, ch)
		targetIdx := len(c.Context.Above)
		startLine := c.Line - len(c.Context.Above)
		// Determine width for line number gutter
		lastLine := startLine + len(contextLines) - 1
		gutterW := len(strconv.Itoa(lastLine))
		for i, hl := range contextLines {
			lineNo := startLine + i
			lineNumStr := fmt.Sprintf("%*d", gutterW, lineNo)
			if i == targetIdx {
				fmt.Fprintf(&b, "%s %s %s\n", targetLineStyle.Render(">"), targetLineStyle.Render(lineNumStr), hl)
			} else {
				fmt.Fprintf(&b, "  %s %s\n", contextLineStyle.Render(lineNumStr), hl)
			}
		}
	}

	// Comment text — word-wrap long lines to fit within terminal width.
	// Subtract 2 for the "┃ " left margin bar added below.
	wrapWidth := termWidth - 2
	if wrapWidth > 70 {
		wrapWidth = 70
	}
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	b.WriteString("\n")
	inCodeBlock := false
	for _, line := range strings.Split(c.Text, "\n") {
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
		}
		out := line
		if !inCodeBlock && !strings.HasPrefix(line, "```") {
			out = ansi.Wordwrap(line, wrapWidth, "")
		}
		for _, wl := range strings.Split(out, "\n") {
			if c.Resolved {
				fmt.Fprintf(&b, "%s\n", cs.Label.Render(wl))
			} else {
				fmt.Fprintf(&b, "%s\n", wl)
			}
		}
	}

	// Add grey left margin bar to every line
	bar := cs.Label.Render("┃") + " "
	raw := b.String()
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	var out strings.Builder
	for _, line := range lines {
		out.WriteString(bar)
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}

// shortSuffixes forwards to ticketdb.ShortSuffixes.
func shortSuffixes(ids []string) map[string]string {
	return ticketdb.ShortSuffixes(ids)
}

// resolveCommentID resolves a (possibly short) suffix to a full comment ID.
// Returns an error if the suffix matches zero or multiple IDs.
func resolveCommentID(store *ticketdb.Store, suffix string) (string, error) {
	idx, err := store.ReadIndex()
	if err != nil {
		return "", fmt.Errorf("reading index: %w", err)
	}

	allIDs := idx.All()
	var matches []string
	for _, id := range allIDs {
		if strings.HasSuffix(id, suffix) {
			matches = append(matches, id)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no comment matching suffix %q", suffix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous suffix %q matches %d comments: %s",
			suffix, len(matches), strings.Join(matches, ", "))
	}
}

// styleCommentPath formats a file:line with dir in dim, basename in bright white.
func styleCommentPath(path string, line int, cs CommentListStyles) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	lineStr := strconv.Itoa(line)
	if dir == "." {
		return cs.Header.Render(base) + cs.Label.Render(":") + lineStr
	}
	return cs.DirPart.Render(dir+"/") + cs.Header.Render(base) + cs.Label.Render(":") + lineStr
}

// highlightedContext returns a comment's context lines (above + target + below),
// syntax-highlighted via the injected ContextHighlighter when one is present and
// plain otherwise. Keeping highlighting behind the interface lets the cgo-free tdb
// binary render context without importing the tree-sitter-backed highlighter.
func highlightedContext(c *ticketdb.Comment, ch ContextHighlighter) []string {
	if ch != nil {
		return ch.HighlightContext(c)
	}
	lines := make([]string, 0, len(c.Context.Above)+1+len(c.Context.Below))
	lines = append(lines, c.Context.Above...)
	lines = append(lines, c.Context.Line)
	lines = append(lines, c.Context.Below...)
	return lines
}

// splitFileLines splits file content into lines, trimming the trailing empty
// element that strings.Split produces for newline-terminated files.
func splitFileLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// parseCommentTarget parses a "file:line" string into path and line number.
func parseCommentTarget(target string) (string, int, error) {
	lastColon := strings.LastIndex(target, ":")
	if lastColon < 0 {
		return "", 0, fmt.Errorf("expected file:line format, got %q (missing colon)", target)
	}
	filePart := target[:lastColon]
	linePart := target[lastColon+1:]
	if filePart == "" {
		return "", 0, fmt.Errorf("expected file:line format, got %q (empty file path)", target)
	}
	lineNum, err := strconv.Atoi(linePart)
	if err != nil || lineNum < 1 {
		return "", 0, fmt.Errorf("expected file:line format with positive line number, got %q", target)
	}
	return filePart, lineNum, nil
}

// normalizeFilePath converts a file path (absolute or relative) to a repo-relative path.
// Also returns the repo root so the caller can reuse it without another git call.
func normalizeFilePath(g *git.RealGit, path string) (relPath, repoRoot string, err error) {
	topLevel, err := g.TopLevel()
	if err != nil {
		return "", "", fmt.Errorf("cannot determine repo root: %w", err)
	}

	absPath := path
	if !filepath.IsAbs(absPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		absPath = filepath.Join(cwd, absPath)
	}
	absPath = filepath.Clean(absPath)

	// Resolve symlinks so we can compare against topLevel, which is already
	// resolved by git rev-parse --show-toplevel.
	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", "", fmt.Errorf("cannot resolve path %s: %w", path, err)
	}

	rel, err := filepath.Rel(topLevel, absPath)
	if err != nil {
		return "", "", fmt.Errorf("cannot make path relative to repo root: %w", err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("file %s is outside the repository", path)
	}
	return rel, topLevel, nil
}

// isLineInDiff checks whether the given new-side line number appears as a
// changed (added) line in the diff for the given file.
func isLineInDiff(diffOutput string, filePath string, lineNum int) bool {
	d, err := diff.Parse(diffOutput)
	if err != nil || d == nil {
		return false
	}
	for _, f := range d.Files {
		// Match file path (strip a/ b/ prefixes)
		newPath := strings.TrimPrefix(f.NewPath, "b/")
		if newPath != filePath {
			continue
		}
		for _, h := range f.Hunks {
			newLine := h.NewStart
			for _, l := range h.Lines {
				switch l.Type {
				case diff.Added:
					if newLine == lineNum {
						return true
					}
					newLine++
				case diff.Removed:
					// Removed lines don't advance new line counter
				case diff.Context:
					newLine++
				}
			}
		}
	}
	return false
}

// runCommentAdd creates a new comment on a specific file and line.
func runCommentAdd(opts Options) error {
	if opts.AddTarget == "" {
		return runCommentAddStandalone(opts)
	}
	return runCommentAddFile(opts)
}

// runCommentAddStandalone creates a standalone comment with no file attachment.
func runCommentAddStandalone(opts Options) error {
	g := git.New()

	// Get comment text from -m or stdin
	text, err := readCommentText(opts.AddMessage)
	if err != nil {
		return err
	}

	var commitSHA string
	var branch string

	if opts.AddRef != "" {
		sha, err := g.RevParse(opts.AddRef)
		if err != nil {
			return fmt.Errorf("cannot resolve ref %q: %w", opts.AddRef, err)
		}
		commitSHA = sha
		if cb, err := g.CurrentBranch(); err == nil && cb != "HEAD" {
			isAnc, err := g.IsAncestor(sha, "HEAD")
			if err == nil && isAnc {
				branch = cb
			}
		}
	} else {
		if cb, err := g.CurrentBranch(); err == nil {
			branch = cb
		}
	}

	// Record branch tip when not viewing a specific commit
	var branchHead string
	if commitSHA == "" {
		if head, err := g.RevParse("HEAD"); err == nil {
			branchHead = head
		}
	}

	now := time.Now()
	c := &ticketdb.Comment{
		Text:       text,
		Created:    now,
		Updated:    now,
		CommitSHA:  commitSHA,
		Branch:     branch,
		BranchHead: branchHead,
		Author:     opts.Author,
		Prefix:     opts.Marker,
		Type:       opts.Type,
		Scope:      opts.Scope,
	}

	store := ticketdb.NewStore("")
	id, err := store.WriteComment(c)
	if err != nil {
		return fmt.Errorf("saving comment: %w", err)
	}

	fmt.Printf("Created standalone comment %s\n", id)
	return nil
}

// runCommentAddFile creates a comment attached to a specific file and line.
func runCommentAddFile(opts Options) error {
	// Parse file:line target
	filePath, lineNum, err := parseCommentTarget(opts.AddTarget)
	if err != nil {
		return err
	}

	g := git.New()

	// Normalize file path to repo-relative
	relPath, repoRoot, err := normalizeFilePath(g, filePath)
	if err != nil {
		return err
	}

	// Get comment text from -m or stdin
	text, err := readCommentText(opts.AddMessage)
	if err != nil {
		return err
	}

	// Read file content (working tree or from ref)
	var fileLines []string
	var commitSHA string
	var branch string

	if opts.AddRef != "" {
		// --ref provided: resolve to commit SHA and read file from that commit
		sha, err := g.RevParse(opts.AddRef)
		if err != nil {
			return fmt.Errorf("cannot resolve ref %q: %w", opts.AddRef, err)
		}
		commitSHA = sha

		content, err := g.GetFileContent(sha, relPath)
		if err != nil {
			return fmt.Errorf("cannot read %s at %s: %w", relPath, opts.AddRef, err)
		}
		fileLines = splitFileLines(content)

		// Check if this commit is on our current branch
		if cb, err := g.CurrentBranch(); err == nil && cb != "HEAD" {
			isAnc, err := g.IsAncestor(sha, "HEAD")
			if err == nil && isAnc {
				branch = cb
			}
		}
	} else {
		// No ref: read from working tree
		absFile := filepath.Join(repoRoot, relPath)
		data, err := os.ReadFile(absFile)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", relPath, err)
		}
		fileLines = splitFileLines(string(data))

		// Record branch only — no commit SHA without explicit --ref
		if cb, err := g.CurrentBranch(); err == nil {
			branch = cb
		}
	}

	// Validate line number
	if lineNum > len(fileLines) {
		return fmt.Errorf("line %d is out of range (file has %d lines)", lineNum, len(fileLines))
	}

	// Build comment
	store := ticketdb.NewStore("")
	ctx := ticketdb.ExtractContextForLine(fileLines, lineNum)

	// Record branch tip when not viewing a specific commit
	var branchHead string
	if commitSHA == "" {
		if head, err := g.RevParse("HEAD"); err == nil {
			branchHead = head
		}
	}

	now := time.Now()
	c := &ticketdb.Comment{
		Text:       text,
		File:       relPath,
		Line:       lineNum,
		Context:    ctx,
		Anchor:     ctx.ComputeAnchor(),
		Created:    now,
		Updated:    now,
		CommitSHA:  commitSHA,
		Branch:     branch,
		BranchHead: branchHead,
		Author:     opts.Author,
		Prefix:     opts.Marker,
		Type:       opts.Type,
		Scope:      opts.Scope,
	}

	// Write to store
	id, err := store.WriteComment(c)
	if err != nil {
		return fmt.Errorf("saving comment: %w", err)
	}

	fmt.Printf("Created comment %s on %s:%d\n", id, relPath, lineNum)
	return nil
}

// readCommentText reads comment text from -m flag or stdin.
func readCommentText(message string) (string, error) {
	text := message
	if text == "" {
		stat, err := os.Stdin.Stat()
		if err != nil {
			return "", fmt.Errorf("cannot read stdin: %w", err)
		}
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return "", fmt.Errorf("no message provided: use -m or pipe text to stdin")
		}
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		text = strings.TrimRight(string(data), "\n")
	}
	if text == "" {
		return "", fmt.Errorf("comment text cannot be empty")
	}
	return text, nil
}

// runCommentEdit opens a comment in $EDITOR for editing, or toggles
// the resolved state when --resolved is provided.
func runCommentEdit(id string, resolved *bool) error {
	store := ticketdb.NewStore("")

	// Resolve suffix to full ID
	fullID, err := resolveCommentID(store, id)
	if err != nil {
		return err
	}

	// Read the existing comment
	original, err := store.ReadComment(fullID)
	if err != nil {
		return fmt.Errorf("comment %s not found: %w", fullID, err)
	}

	// --resolved flag: toggle resolved state without opening editor
	if resolved != nil {
		original.Resolved = *resolved
		original.Updated = time.Now()
		if _, err := store.WriteComment(original); err != nil {
			return fmt.Errorf("saving comment: %w", err)
		}
		state := "unresolved"
		if *resolved {
			state = "resolved"
		}
		fmt.Printf("Marked comment %s as %s\n", id, state)
		return nil
	}

	editor := editorCmd()
	if editor == "" {
		return fmt.Errorf("$VISUAL or $EDITOR must be set")
	}

	// Serialize to temp file
	serialized := original.Serialize()
	tmpFile, err := os.CreateTemp("", "comment-*.patch")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(serialized); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	// Open in editor
	cmd := exec.Command("sh", "-c", editor+` "$@"`, "--", tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	// Read back edited content
	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("read edited file: %w", err)
	}

	// Parse and validate
	parsed, err := ticketdb.ParseComment(fullID, string(edited))
	if err != nil {
		return fmt.Errorf("invalid comment format: %w", err)
	}
	if parsed.File != "" && parsed.Line <= 0 {
		return fmt.Errorf("invalid edit: LINE metadata must be positive when FILE is set")
	}

	// Update timestamp and write back
	parsed.Updated = time.Now()
	if _, err := store.WriteComment(parsed); err != nil {
		return fmt.Errorf("saving comment: %w", err)
	}

	fmt.Printf("Updated comment %s\n", id)
	return nil
}
