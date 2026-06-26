package ticketcli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattduck/diffyduck/pkg/git"
	"github.com/mattduck/diffyduck/pkg/scanner"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
	"golang.org/x/term"
)

// Source values for the unified list.
const (
	SourceAll   = "all"
	SourceState = "state"
	SourceCode  = "code"
)

// ListOptions holds the parsed inputs for the unified `tdb list` command, which
// merges git-state tickets and in-code markers into one view.
type ListOptions struct {
	Source         string   // all (default), state, code
	Markers        []string // restrict code markers to these keywords (empty = defaults)
	ExcludeMarkers []string // exclude these marker keywords from results
	Type           string   // --type filter (code markers only)
	File           string   // --file filter (trailing / = prefix match)
	Grep           string   // --grep filter (case-insensitive)
	Status         string   // ticket filter: unresolved (default), resolved, all
	Rule           string   // --rule filter (tickets carrying this rule code)
	Kind           string   // ticket subtype filter: comment, note, all (state/all source only)
	Since          string   // --since duration filter (state/all source only)
	Author         string   // --author value (state/all source only)
	AuthorSet      bool     // true if --author was explicitly passed
	Verbose        bool     // -v: block output (state source only)
	Raw            bool     // --raw: serialized blob output (state source only)
	ID             string   // positional ID lookup (state source only)
	N              int      // -n cap on combined rows (0 = uncapped)
	NSet           bool

	AllBranches bool   // --all-branches
	Branch      string // --branch / -b ("." = current branch)

	Styles      CommentListStyles
	Highlighter ContextHighlighter // syntax highlighter injected by host binary (nil = plain)
}

// ParseListArgs parses `list` command-line arguments. argv[0] must be "list".
func ParseListArgs(argv []string) (ListOptions, error) {
	var o ListOptions
	if len(argv) == 0 || argv[0] != "list" {
		return o, fmt.Errorf("not a list command")
	}
	o.Source = SourceAll
	rest := argv[1:]

	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		next := func() (string, bool) {
			if i+1 < len(rest) {
				i++
				return rest[i], true
			}
			return "", false
		}
		switch {
		case arg == "-h" || arg == "--help":
			return o, errHelp

		case arg == "--source":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--source requires a value (all, state, code)")
			}
			if err := setSource(&o, v); err != nil {
				return o, err
			}
		case strings.HasPrefix(arg, "--source="):
			if err := setSource(&o, strings.TrimPrefix(arg, "--source=")); err != nil {
				return o, err
			}

		case arg == "--marker":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--marker requires a keyword (e.g. TODO,FIXME)")
			}
			o.Markers = append(o.Markers, splitList(v)...)
		case strings.HasPrefix(arg, "--marker="):
			o.Markers = append(o.Markers, splitList(strings.TrimPrefix(arg, "--marker="))...)

		case arg == "--exclude-marker":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--exclude-marker requires a keyword (e.g. RPT)")
			}
			o.ExcludeMarkers = append(o.ExcludeMarkers, splitList(v)...)
		case strings.HasPrefix(arg, "--exclude-marker="):
			o.ExcludeMarkers = append(o.ExcludeMarkers, splitList(strings.TrimPrefix(arg, "--exclude-marker="))...)

		case arg == "--type":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--type requires a value")
			}
			o.Type = v
		case strings.HasPrefix(arg, "--type="):
			o.Type = strings.TrimPrefix(arg, "--type=")

		case arg == "--file":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--file requires a path")
			}
			o.File = v
		case strings.HasPrefix(arg, "--file="):
			o.File = strings.TrimPrefix(arg, "--file=")

		case arg == "--grep":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--grep requires a pattern")
			}
			o.Grep = v
		case strings.HasPrefix(arg, "--grep="):
			o.Grep = strings.TrimPrefix(arg, "--grep=")

		case arg == "--status":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--status requires a value (unresolved, resolved, all)")
			}
			if err := setListStatus(&o, v); err != nil {
				return o, err
			}
		case strings.HasPrefix(arg, "--status="):
			if err := setListStatus(&o, strings.TrimPrefix(arg, "--status=")); err != nil {
				return o, err
			}

		case arg == "--rule":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--rule requires a value")
			}
			o.Rule = v
		case strings.HasPrefix(arg, "--rule="):
			o.Rule = strings.TrimPrefix(arg, "--rule=")

		case arg == "-n":
			if i+1 < len(rest) {
				if n, err := strconv.Atoi(rest[i+1]); err == nil {
					o.N = n
					o.NSet = true
					i++
					continue
				}
			}
			o.N = 0
			o.NSet = true
		case strings.HasPrefix(arg, "-n"):
			n, err := strconv.Atoi(strings.TrimPrefix(arg, "-n"))
			if err != nil {
				return o, fmt.Errorf("-n requires an integer, got %q", strings.TrimPrefix(arg, "-n"))
			}
			o.N = n
			o.NSet = true

		case arg == "--all-branches":
			o.AllBranches = true
		case arg == "-b" || arg == "--branch":
			if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
				o.Branch = rest[i+1]
				i++
			} else {
				o.AllBranches = true
			}
		case strings.HasPrefix(arg, "--branch="):
			o.Branch = strings.TrimPrefix(arg, "--branch=")

		case arg == "--kind":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--kind requires a value (comment, note, all)")
			}
			if err := setListKind(&o, v); err != nil {
				return o, err
			}
		case strings.HasPrefix(arg, "--kind="):
			if err := setListKind(&o, strings.TrimPrefix(arg, "--kind=")); err != nil {
				return o, err
			}

		case arg == "--since":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--since requires a duration (e.g. 30m, 6h, 30d, 2w, 3M, 1y, all)")
			}
			o.Since = v
		case strings.HasPrefix(arg, "--since="):
			o.Since = strings.TrimPrefix(arg, "--since=")

		case arg == "--author":
			if i+1 < len(rest) && !strings.HasPrefix(rest[i+1], "-") {
				o.Author = rest[i+1]
				o.AuthorSet = true
				i++
			} else {
				o.AuthorSet = true
			}
		case strings.HasPrefix(arg, "--author="):
			o.Author = strings.TrimPrefix(arg, "--author=")
			if o.Author == "" {
				return o, fmt.Errorf("--author requires a value when using = syntax")
			}
			o.AuthorSet = true

		case arg == "-v" || arg == "--verbose":
			o.Verbose = true

		case arg == "--raw":
			o.Raw = true

		default:
			if strings.HasPrefix(arg, "-") {
				return o, fmt.Errorf("unknown flag: %s", arg)
			}
			if o.ID != "" {
				return o, fmt.Errorf("unexpected argument: %s", arg)
			}
			o.ID = arg
		}
	}

	if o.AllBranches && o.Branch != "" {
		return o, fmt.Errorf("--all-branches and --branch cannot be used together")
	}
	if len(o.Markers) > 0 && o.Source == SourceState {
		return o, fmt.Errorf("--marker is only valid when listing code markers")
	}
	if len(o.ExcludeMarkers) > 0 && o.Source == SourceState {
		return o, fmt.Errorf("--exclude-marker is only valid when listing code markers")
	}
	if o.Type != "" && o.Source == SourceState {
		return o, fmt.Errorf("--type is only valid when listing code markers")
	}
	if o.Rule != "" && o.Source == SourceCode {
		return o, fmt.Errorf("--rule is only valid when listing tickets")
	}
	if o.Source == SourceCode {
		if o.Kind != "" {
			return o, fmt.Errorf("--kind is only valid when listing tickets")
		}
		if o.Since != "" {
			return o, fmt.Errorf("--since is only valid when listing tickets")
		}
		if o.AuthorSet {
			return o, fmt.Errorf("--author is only valid when listing tickets")
		}
		if o.Verbose {
			return o, fmt.Errorf("-v/--verbose is only valid when listing tickets")
		}
		if o.Raw {
			return o, fmt.Errorf("--raw is only valid when listing tickets")
		}
		if o.ID != "" {
			return o, fmt.Errorf("ID lookup is only valid when listing tickets")
		}
	}
	if o.Source == SourceAll {
		if o.ID != "" {
			return o, fmt.Errorf("ID lookup requires --source state")
		}
		if o.Verbose || o.Raw {
			return o, fmt.Errorf("-v/--raw require --source state")
		}
	}
	return o, nil
}

func setListKind(o *ListOptions, v string) error {
	switch v {
	case "comment", "note", "all":
		o.Kind = v
		return nil
	default:
		return fmt.Errorf("--kind must be comment, note, or all; got %q", v)
	}
}

func setSource(o *ListOptions, v string) error {
	switch v {
	case "all":
		o.Source = SourceAll
	case "state", "tickets":
		o.Source = SourceState
	case "code", "markers":
		o.Source = SourceCode
	default:
		return fmt.Errorf("--source must be all, state, or code; got %q", v)
	}
	return nil
}

func setListStatus(o *ListOptions, v string) error {
	switch v {
	case "unresolved", "resolved", "all":
		o.Status = v
		return nil
	default:
		return fmt.Errorf("--status must be unresolved, resolved, or all; got %q", v)
	}
}

// splitList splits a comma-separated flag value into trimmed, non-empty parts.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// RunList executes the unified `tdb list` command, merging git-state tickets and
// in-code markers into a single view.
func RunList(argv []string, cfg Options) error {
	o, err := ParseListArgs(argv)
	if err != nil {
		return err
	}
	o.Styles = cfg.Styles
	return runList(o)
}

func runList(o ListOptions) error {
	// State-only mode: use the richer ticket renderer.
	if o.Source == SourceState {
		return runStateList(o)
	}

	var rows []listRow

	if o.Source != SourceCode {
		ticketRows, err := gatherTickets(o)
		if err != nil {
			return err
		}
		rows = append(rows, ticketRows...)
	}

	if o.Source != SourceState {
		markerRows, err := gatherMarkers(o)
		if err != nil {
			return err
		}
		rows = append(rows, markerRows...)
	}

	if len(rows) == 0 {
		fmt.Println("No tickets or code markers found")
		return nil
	}

	// Tickets first (newest first), then code markers (by file/line).
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].code != rows[j].code {
			return !rows[i].code
		}
		if !rows[i].code {
			return rows[i].created.After(rows[j].created)
		}
		if rows[i].file != rows[j].file {
			return rows[i].file < rows[j].file
		}
		return rows[i].line < rows[j].line
	})

	totalCount := len(rows)
	truncated := false
	if o.NSet && o.N > 0 && o.N < len(rows) {
		rows = rows[:o.N]
		truncated = true
	}

	renderRows(rows, o.Styles)
	if truncated {
		fmt.Println(o.Styles.Label.Render(fmt.Sprintf("%d/%d", len(rows), totalCount)))
	}
	return nil
}

// listRow is a single line in the unified list, normalized across both sources.
type listRow struct {
	kind    string    // "comment", "note", or a lowercased marker keyword
	file    string    // path for path-style location ("" when using id)
	line    int       // line number for path location
	id      string    // short id (standalone notes)
	text    string    // title or first line of text/message
	created time.Time // ticket creation time (zero for code markers)
	dim     bool      // resolved/closed → render dim
	code    bool      // true for code markers
}

func gatherTickets(o ListOptions) ([]listRow, error) {
	store := ticketdb.NewStore("")
	all, err := store.AllComments()
	if err != nil {
		return nil, fmt.Errorf("reading tickets: %w", err)
	}

	// Stable short suffixes computed over every ticket in the store.
	ids := make([]string, len(all))
	for i, c := range all {
		ids[i] = c.ID
	}
	shortIDs := shortSuffixes(ids)

	// Branch scoping mirrors `comment list`: default to the current branch
	// unless --all-branches or an explicit --branch is given.
	branch := o.Branch
	if branch == "." {
		cb, err := store.CurrentBranch()
		if err != nil || cb == "" {
			return nil, fmt.Errorf("could not determine current branch")
		}
		branch = cb
	}
	if branch == "" && !o.AllBranches {
		if cb, _ := store.CurrentBranch(); cb != "" {
			branch = cb
		} else {
			fmt.Fprintln(os.Stderr, "warning: detached HEAD — showing tickets from all branches")
		}
	}

	status := o.Status
	if status == "" {
		status = "unresolved"
	}

	now := time.Now()
	var sinceFilter time.Duration
	if o.Since != "" {
		if d, err := parseSinceDuration(o.Since); err == nil {
			sinceFilter = d
		}
	}

	var rows []listRow
	for _, c := range all {
		if branch != "" && c.Branch != branch {
			continue
		}
		if status == "resolved" && !c.Resolved {
			continue
		}
		if status == "unresolved" && c.Resolved {
			continue
		}
		switch o.Kind {
		case "comment":
			if c.IsStandalone() {
				continue
			}
		case "note":
			if !c.IsStandalone() {
				continue
			}
		}
		if sinceFilter > 0 && !c.Created.After(now.Add(-sinceFilter)) {
			continue
		}
		if o.AuthorSet {
			if o.Author == "" {
				if c.Author != "" {
					continue
				}
			} else {
				if !strings.Contains(strings.ToLower(c.Author), strings.ToLower(o.Author)) {
					continue
				}
			}
		}
		if !fileMatches(o.File, c.File) {
			continue
		}
		if !grepMatches(o.Grep, c.Text, c.Title) {
			continue
		}
		if o.Rule != "" && !strings.EqualFold(o.Rule, c.Rule) {
			continue
		}

		kind := "comment"
		if c.IsStandalone() {
			kind = "note"
		}
		if st := c.EffectiveStatus(); st == ticketdb.StatusInProgress {
			kind = kind + "*" // in-progress marker
		}

		rows = append(rows, listRow{
			kind:    kind,
			file:    c.File,
			line:    c.Line,
			id:      shortIDs[c.ID],
			text:    ticketText(c),
			created: c.Created,
			dim:     c.Resolved,
		})
	}
	return rows, nil
}

func gatherMarkers(o ListOptions) ([]listRow, error) {
	g := git.New()
	root, err := g.TopLevel()
	if err != nil {
		return nil, fmt.Errorf("cannot determine repo root: %w", err)
	}

	markers := scanner.DefaultMarkers()
	if len(o.Markers) > 0 {
		markers = nil
		for _, kw := range o.Markers {
			markers = append(markers, markerForKeyword(kw))
		}
	}
	if len(o.ExcludeMarkers) > 0 {
		exclude := make(map[string]bool, len(o.ExcludeMarkers))
		for _, kw := range o.ExcludeMarkers {
			exclude[strings.ToUpper(kw)] = true
		}
		filtered := markers[:0]
		for _, m := range markers {
			if !exclude[m.Keyword] {
				filtered = append(filtered, m)
			}
		}
		markers = filtered
	}

	ms, err := scanner.ScanDirMarkers(root, markers, scanner.WalkOptions{KeepDir: keepCodeDir})
	if err != nil {
		return nil, fmt.Errorf("scanning code markers: %w", err)
	}

	var rows []listRow
	for _, m := range ms {
		rel := relPath(root, m.File)
		if !fileMatches(o.File, rel) {
			continue
		}
		if !grepMatches(o.Grep, m.Message, "") {
			continue
		}
		if o.Type != "" && !strings.EqualFold(m.Type, o.Type) {
			continue
		}
		kind := m.Keyword
		if m.Type != "" {
			if m.Scope != "" {
				kind = m.Keyword + " " + m.Type + "(" + m.Scope + ")"
			} else {
				kind = m.Keyword + " " + m.Type
			}
		}
		rows = append(rows, listRow{
			// Code markers keep their uppercase keyword (TODO, NOTE, …) so they
			// stay visually distinct from lowercase ticket kinds (comment, note)
			// even in uncolored output.
			kind: kind,
			file: rel,
			line: m.Line,
			text: m.Message,
			code: true,
		})
	}
	return rows, nil
}

// markerForKeyword builds a scanner.Marker for a user-supplied keyword using
// the loose (non-strict) form so all occurrences are visible in the list.
func markerForKeyword(kw string) scanner.Marker {
	return scanner.Marker{Keyword: strings.ToUpper(kw)}
}

// keepCodeDir prunes version-control directories during code-marker scans.
func keepCodeDir(p string) bool {
	switch filepath.Base(p) {
	case ".git", ".hg", ".svn":
		return false
	}
	return true
}

// relPath returns path relative to root using forward slashes, falling back to
// the original path if a relative path cannot be computed.
func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// ticketText returns the row text for a ticket: its title if set, else the first
// line of its body.
func ticketText(c *ticketdb.Comment) string {
	if c.Title != "" {
		return c.Title
	}
	text := c.Text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = text[:idx]
	}
	return text
}

// fileMatches reports whether path passes the --file filter. A trailing slash
// makes it a prefix match; otherwise it is an exact match. Empty filter matches.
func fileMatches(filter, path string) bool {
	if filter == "" {
		return true
	}
	if strings.HasSuffix(filter, "/") {
		return strings.HasPrefix(path, filter)
	}
	return path == filter
}

// grepMatches reports whether any of the given fields contains the needle
// (case-insensitive). Empty needle matches.
func grepMatches(needle string, fields ...string) bool {
	if needle == "" {
		return true
	}
	n := strings.ToLower(needle)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), n) {
			return true
		}
	}
	return false
}

// renderRows prints the unified list as aligned KIND / LOCATION / TEXT columns.
func renderRows(rows []listRow, cs CommentListStyles) {
	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}
	for _, line := range formatListRows(rows, cs, termWidth) {
		fmt.Println(line)
	}
}

// formatListRows formats the unified list into aligned KIND / LOCATION / TEXT
// lines. Column widths are derived from the rows' plain content.
func formatListRows(rows []listRow, cs CommentListStyles, termWidth int) []string {
	kindW, locW := 0, 0
	for _, r := range rows {
		if w := len(r.kind); w > kindW {
			kindW = w
		}
		if w := len(rowLocationPlain(r)); w > locW {
			locW = w
		}
	}
	if locW > 40 {
		locW = 40
	}

	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		kindStyled := cs.Branch.Render(r.kind)
		if !r.code {
			kindStyled = cs.Header.Render(r.kind)
		}
		if r.dim {
			kindStyled = cs.Label.Render(r.kind)
		}

		locPlain := rowLocationPlain(r)
		locStyled := rowLocationStyled(r, cs)
		locVisW := lipgloss.Width(locStyled)
		if len(locPlain) > locW {
			// Truncate overlong locations on the plain string and restyle dim.
			locPlain = locPlain[:locW-1] + "…"
			locStyled = cs.DirPart.Render(locPlain)
			locVisW = lipgloss.Width(locStyled)
		}

		used := kindW + 1 + locW + 1
		text := r.text
		textMax := termWidth - used
		if textMax < 10 {
			textMax = 10
		}
		if len(text) > textMax {
			if textMax > 3 {
				text = text[:textMax-3] + "..."
			} else {
				text = text[:textMax]
			}
		}
		if r.dim {
			text = cs.Label.Render(text)
		}

		var b strings.Builder
		b.WriteString(kindStyled)
		b.WriteString(strings.Repeat(" ", kindW-len(r.kind)))
		b.WriteByte(' ')
		b.WriteString(locStyled)
		if locVisW < locW {
			b.WriteString(strings.Repeat(" ", locW-locVisW))
		}
		b.WriteByte(' ')
		b.WriteString(text)
		lines = append(lines, b.String())
	}
	return lines
}

func rowLocationPlain(r listRow) string {
	if r.file != "" {
		return fmt.Sprintf("%s:%d", r.file, r.line)
	}
	return r.id
}

func rowLocationStyled(r listRow, cs CommentListStyles) string {
	if r.file != "" {
		if r.dim {
			return cs.Label.Render(rowLocationPlain(r))
		}
		return styleCommentPath(r.file, r.line, cs)
	}
	if r.dim {
		return cs.Label.Render(r.id)
	}
	return cs.Header.Render(r.id)
}
