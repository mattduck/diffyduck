package ticketcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
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

// randShuffle shuffles n elements via swap. It's a package seam so tests can
// substitute a deterministic ordering; production uses the auto-seeded global
// rand (Go 1.20+).
var randShuffle = rand.Shuffle

// Store values for the unified list: where an entry physically lives.
const (
	StoreAll  = "all"
	StoreDB   = "db"
	StoreFile = "file"
)

// ErrExitCode is a sentinel returned by list when --exit-code is set and at
// least one row matched. It carries no user-facing message: the caller maps it
// to exit status 1 (matches found) without printing an error.
var ErrExitCode = errors.New("exit-code: matches found")

// exitCodeResult returns ErrExitCode when the --exit-code gate is enabled and at
// least one row matched, else nil.
func exitCodeResult(enabled, matched bool) error {
	if enabled && matched {
		return ErrExitCode
	}
	return nil
}

// ListOptions holds the parsed inputs for the unified `tdb list` command, which
// merges db entries and in-file comments into one view.
type ListOptions struct {
	Store           string   // all (default), db, file
	Prefixes        []string // restrict to these prefix keywords (empty = defaults)
	ExcludePrefixes []string // exclude these prefix keywords from results
	Type            string   // --type filter; any store
	Ticket          string   // --ticket filter: external tracker ref; any store
	File            string   // --file filter (trailing / = prefix match)
	Grep            string   // --grep filter (case-insensitive)
	Status          string   // db filter: unresolved (default), resolved, all
	Scope           string   // --scope filter: scope tag or file-comment scope
	Kind            string   // kind filter: comment, issue, all (db/all store only)
	Since           string   // --since duration filter (db/all store only)
	Author          string   // --author value (db/all store only)
	AuthorSet       bool     // true if --author was explicitly passed
	Verbose         bool     // -v: block output with code context (any store)
	Raw             bool     // --raw: serialized blob output (db store only)
	JSON            bool     // --json: machine-readable output (any source)
	ID              string   // positional ID lookup (state source only)
	N               int      // -n cap on combined rows (0 = uncapped)
	NSet            bool
	ExitCode        bool   // --exit-code: exit 1 if any rows match, 0 if none
	Random          bool   // --random: shuffle rows and (unless -n given) return one
	Stats           bool   // --stats: show counts instead of the row list
	StatsGroup      string // --stats-group FIELD: collapse --stats to one dimension

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
	o.Store = StoreAll
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

		case arg == "--store":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--store requires a value (all, db, file)")
			}
			if err := setStore(&o, v); err != nil {
				return o, err
			}
		case strings.HasPrefix(arg, "--store="):
			if err := setStore(&o, strings.TrimPrefix(arg, "--store=")); err != nil {
				return o, err
			}

		case arg == "--prefix":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--prefix requires a keyword (e.g. TODO,FIXME)")
			}
			o.Prefixes = append(o.Prefixes, splitList(v)...)
		case strings.HasPrefix(arg, "--prefix="):
			o.Prefixes = append(o.Prefixes, splitList(strings.TrimPrefix(arg, "--prefix="))...)

		case arg == "--exclude-prefix":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--exclude-prefix requires a keyword (e.g. RPT)")
			}
			o.ExcludePrefixes = append(o.ExcludePrefixes, splitList(v)...)
		case strings.HasPrefix(arg, "--exclude-prefix="):
			o.ExcludePrefixes = append(o.ExcludePrefixes, splitList(strings.TrimPrefix(arg, "--exclude-prefix="))...)

		case arg == "--type":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--type requires a value")
			}
			o.Type = v
		case strings.HasPrefix(arg, "--type="):
			o.Type = strings.TrimPrefix(arg, "--type=")

		case arg == "--ticket":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--ticket requires a reference (e.g. ABC-123, #123)")
			}
			o.Ticket = v
		case strings.HasPrefix(arg, "--ticket="):
			o.Ticket = strings.TrimPrefix(arg, "--ticket=")

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

		case arg == "--scope":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--scope requires a value")
			}
			o.Scope = v
		case strings.HasPrefix(arg, "--scope="):
			o.Scope = strings.TrimPrefix(arg, "--scope=")

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
		case arg == "--exit-code":
			o.ExitCode = true
		case arg == "--random":
			o.Random = true
		case arg == "--stats":
			o.Stats = true
		case arg == "--stats-group":
			v, ok := next()
			if !ok {
				return o, fmt.Errorf("--stats-group requires a field (store, kind, prefix, type, scope, ticket, author, file, branch)")
			}
			o.Stats = true
			o.StatsGroup = v
		case strings.HasPrefix(arg, "--stats-group="):
			o.Stats = true
			o.StatsGroup = strings.TrimPrefix(arg, "--stats-group=")
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

		case arg == "--json":
			o.JSON = true

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
	if o.JSON && (o.Verbose || o.Raw) {
		return o, fmt.Errorf("--json cannot be combined with -v/--raw")
	}
	if o.JSON && o.ID != "" {
		return o, fmt.Errorf("--json cannot be combined with an ID lookup")
	}
	if o.Random {
		if o.ID != "" {
			return o, fmt.Errorf("--random cannot be combined with an ID lookup")
		}
		if o.Verbose || o.Raw {
			return o, fmt.Errorf("--random cannot be combined with -v/--raw")
		}
	}
	if o.Stats {
		if o.ID != "" {
			return o, fmt.Errorf("--stats cannot be combined with an ID lookup")
		}
		if o.Verbose || o.Raw {
			return o, fmt.Errorf("--stats cannot be combined with -v/--raw")
		}
		if o.Random {
			return o, fmt.Errorf("--stats cannot be combined with --random")
		}
		if o.StatsGroup != "" && !validStatField(o.StatsGroup) {
			return o, fmt.Errorf("invalid --stats-group %q (want store, kind, prefix, type, scope, ticket, author, file, branch)", o.StatsGroup)
		}
	}
	if o.Store == StoreFile {
		// A file comment always concerns a code range, so there are no file issues.
		if o.Kind == "issue" {
			return o, fmt.Errorf("no file issues exist: a file comment always concerns a code range (drop --kind issue, or use --store db)")
		}
		if o.Since != "" {
			return o, fmt.Errorf("--since is only valid for db entries")
		}
		if o.AuthorSet {
			return o, fmt.Errorf("--author is only valid for db entries")
		}
		if o.Raw {
			// --raw dumps the git-ref serialized blob, which only db entries have.
			return o, fmt.Errorf("--raw requires --store db")
		}
		if o.ID != "" {
			return o, fmt.Errorf("ID lookup is only valid for db entries")
		}
	}
	if o.Store == StoreAll {
		if o.ID != "" {
			return o, fmt.Errorf("ID lookup requires --store db")
		}
		if o.Raw {
			return o, fmt.Errorf("--raw requires --store db")
		}
	}
	return o, nil
}

func setListKind(o *ListOptions, v string) error {
	switch v {
	case "comment", "issue", "all":
		o.Kind = v
		return nil
	default:
		return fmt.Errorf("--kind must be comment, issue, or all; got %q", v)
	}
}

func setStore(o *ListOptions, v string) error {
	switch v {
	case "all":
		o.Store = StoreAll
	case "db", "state", "tickets":
		o.Store = StoreDB
	case "file", "code", "markers":
		o.Store = StoreFile
	default:
		return fmt.Errorf("--store must be all, db, or file; got %q", v)
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

// prefixMatches reports whether kw equals any keyword in list (case-insensitive).
// An empty list matches nothing (callers guard with len(list) > 0).
func prefixMatches(list []string, kw string) bool {
	for _, m := range list {
		if strings.EqualFold(m, kw) {
			return true
		}
	}
	return false
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
	// State-only mode uses the richer ticket renderer — except under --json
	// (one uniform schema), --random (compact one-per-row selection), or --stats
	// (counts), which route every source through the merged path.
	if o.Store == StoreDB && !o.JSON && !o.Random && !o.Stats {
		return runStateList(o)
	}

	var rows []listRow

	if o.Store != StoreFile {
		ticketRows, err := gatherTickets(o)
		if err != nil {
			return err
		}
		rows = append(rows, ticketRows...)
	}

	// File comments are always kind=comment, so --kind issue excludes them.
	if o.Store != StoreDB && o.Kind != "issue" {
		markerRows, err := gatherMarkers(o)
		if err != nil {
			return err
		}
		rows = append(rows, markerRows...)
	}

	if o.Stats {
		return renderStats(rows, o)
	}

	if len(rows) == 0 {
		if o.JSON {
			fmt.Println("[]")
		} else {
			fmt.Println("No db entries or file comments found")
		}
		return exitCodeResult(o.ExitCode, false)
	}

	rows, totalCount, truncated := selectRows(rows, o)

	if o.JSON {
		if err := renderRowsJSON(rows); err != nil {
			return err
		}
		return exitCodeResult(o.ExitCode, true)
	}

	if o.Verbose {
		renderRowsVerbose(rows, o)
		if truncated {
			fmt.Printf("\n%s\n", o.Styles.Label.Render(fmt.Sprintf("%d/%d", len(rows), totalCount)))
		}
		return exitCodeResult(o.ExitCode, true)
	}

	renderRows(rows, o.Styles)
	if truncated {
		fmt.Println(o.Styles.Label.Render(fmt.Sprintf("%d/%d", len(rows), totalCount)))
	}
	return exitCodeResult(o.ExitCode, true)
}

// renderRowsVerbose prints each row as a full block: db entries via
// formatCommentBlock, file comments via formatMarkerBlock (which reads their
// surrounding source). It backs the merged (--store all/file) verbose path;
// --store db verbose is handled by runStateList's richer renderer.
func renderRowsVerbose(rows []listRow, o ListOptions) {
	termWidth := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}
	// Resolve the repo root once, only if a file comment needs its context read.
	var root string
	for _, r := range rows {
		if r.code {
			if rt, err := git.New().TopLevel(); err == nil {
				root = rt
			}
			break
		}
	}
	now := time.Now()
	for i, r := range rows {
		if i > 0 {
			fmt.Print("\n\n")
		}
		if r.comment != nil {
			fmt.Print(formatCommentBlock(r.comment, o.Highlighter, termWidth, "", now, o.Styles))
		} else {
			fmt.Print(formatMarkerBlock(r, root, o.Highlighter, termWidth, o.Styles))
		}
	}
}

// selectRows applies ordering then the row limit. Default ordering is tickets
// first (newest first) then code markers (by file/line); --random shuffles via
// the randShuffle seam instead. The limit is -n if set; otherwise 1 under
// --random and 5 under -v (verbose blocks are large), and uncapped for the
// default one-line view. It returns the selected rows, the pre-limit total, and
// whether truncation occurred.
func selectRows(rows []listRow, o ListOptions) (selected []listRow, total int, truncated bool) {
	if o.Random {
		randShuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
	} else {
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
	}

	total = len(rows)
	limit := 0
	switch {
	case o.NSet && o.N > 0:
		limit = o.N
	case o.Random && !o.NSet:
		limit = 1
	case o.Verbose && !o.NSet:
		// Verbose blocks are large, so default to a small cap (matching the db
		// renderer); an explicit -n — including bare -n for all — overrides.
		limit = 5
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
		truncated = true
	}
	return rows, total, truncated
}

// defaultStatDimensions is the multi-dimension breakdown shown by bare --stats.
var defaultStatDimensions = []string{"store", "kind", "prefix", "type", "scope", "ticket"}

// validStatField reports whether f is a groupable --stats-group dimension.
func validStatField(f string) bool {
	switch f {
	case "store", "kind", "prefix", "type", "scope", "ticket", "author", "file", "branch":
		return true
	}
	return false
}

// statFieldValue extracts the value of a stats dimension from a row ("" = unset).
func statFieldValue(r listRow, field string) string {
	switch field {
	case "store":
		if r.code {
			return "file"
		}
		return "db"
	case "prefix":
		return r.marker
	case "kind":
		return r.tkind
	case "type":
		return r.mtype
	case "scope":
		return r.scope
	case "ticket":
		return r.ticket
	case "author":
		return r.author
	case "file":
		return r.file
	case "branch":
		return r.branch
	}
	return ""
}

// statCount is one value/count pair within a stats dimension.
type statCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// countStatDim tallies rows by a dimension, ordered by count desc then value asc.
func countStatDim(rows []listRow, field string) []statCount {
	counts := map[string]int{}
	for _, r := range rows {
		counts[statFieldValue(r, field)]++
	}
	out := make([]statCount, 0, len(counts))
	for v, c := range counts {
		out = append(out, statCount{Value: v, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
}

// renderStats prints counts for the (already filtered) rows: a multi-dimension
// breakdown by default, or a single dimension when --stats-group is set.
func renderStats(rows []listRow, o ListOptions) error {
	dims := defaultStatDimensions
	if o.StatsGroup != "" {
		dims = []string{o.StatsGroup}
	}

	type dimResult struct {
		field  string
		counts []statCount
	}
	var results []dimResult
	for _, field := range dims {
		counts := countStatDim(rows, field)
		// For the default breakdown, drop dimensions no row populates.
		if o.StatsGroup == "" {
			if len(counts) == 0 || (len(counts) == 1 && counts[0].Value == "") {
				continue
			}
		}
		results = append(results, dimResult{field: field, counts: counts})
	}

	if o.JSON {
		type dimJSON struct {
			Field  string      `json:"field"`
			Counts []statCount `json:"counts"`
		}
		payload := struct {
			Total      int       `json:"total"`
			Dimensions []dimJSON `json:"dimensions"`
		}{Total: len(rows), Dimensions: []dimJSON{}}
		for _, r := range results {
			payload.Dimensions = append(payload.Dimensions, dimJSON{Field: r.field, Counts: r.counts})
		}
		b, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return exitCodeResult(o.ExitCode, len(rows) > 0)
	}

	if len(rows) == 0 {
		fmt.Println("No db entries or file comments found")
		return exitCodeResult(o.ExitCode, false)
	}

	cs := o.Styles
	countW := len(strconv.Itoa(len(rows)))
	for i, r := range results {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(cs.Header.Render(r.field))
		for _, sc := range r.counts {
			val := sc.Value
			if val == "" {
				// The kind dimension (comment/issue) only applies to db entries;
				// an empty value there is a file comment, not an unset field.
				if r.field == "kind" {
					val = "(n/a — file comments)"
				} else {
					val = "(none)"
				}
			}
			fmt.Printf("  %s  %s\n", cs.Label.Render(fmt.Sprintf("%*d", countW, sc.Count)), val)
		}
	}
	fmt.Printf("\n%s %d\n", cs.Label.Render("total"), len(rows))
	return exitCodeResult(o.ExitCode, len(rows) > 0)
}

// listRow is a single line in the unified list, normalized across both sources.
// The display renderer uses only kind/file/line/id/text/dim/code; the remaining
// fields carry the richer per-source detail that --json emits.
type listRow struct {
	kind    string    // "comment", "issue", or a file-comment prefix (+ type/scope for display)
	file    string    // path for path-style location ("" when using id)
	line    int       // line number for path location
	id      string    // short id (db issues)
	text    string    // title or first line of text/message
	created time.Time // db creation time (zero for file comments)
	dim     bool      // resolved/closed → render dim
	code    bool      // true for file comments

	// JSON-only detail (ignored by the aligned-column renderer).
	fullID   string // db full id
	tkind    string // "comment" | "issue" (db kind without in-progress suffix)
	body     string // db full text (text is only the one-line summary)
	author   string // db author
	status   string // db effective status
	branch   string // db branch
	commit   string // db attached commit SHA
	resolved bool   // db resolved flag
	marker   string // prefix keyword (RPT, TODO, …) — both stores
	mtype    string // type — both stores
	scope    string // scope/code identifier — both stores
	ticket   string // external tracker ref — both stores

	// comment is the source db entry (nil for file comments); retained so the
	// merged verbose renderer can produce the full block without re-reading.
	comment *ticketdb.Comment
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
		case "issue":
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
		if o.Scope != "" && !strings.EqualFold(o.Scope, c.Scope) {
			continue
		}
		if o.Type != "" && !strings.EqualFold(o.Type, c.Type) {
			continue
		}
		if o.Ticket != "" && !strings.EqualFold(o.Ticket, c.Ticket) {
			continue
		}
		if len(o.Prefixes) > 0 && !prefixMatches(o.Prefixes, c.Prefix) {
			continue
		}
		if len(o.ExcludePrefixes) > 0 && prefixMatches(o.ExcludePrefixes, c.Prefix) {
			continue
		}

		kind := "comment"
		if c.IsStandalone() {
			kind = "issue"
		}
		if st := c.EffectiveStatus(); st == ticketdb.StatusInProgress {
			kind = kind + "*" // in-progress marker
		}

		tkind := "comment"
		if c.IsStandalone() {
			tkind = "issue"
		}
		rows = append(rows, listRow{
			kind:    kind,
			file:    c.File,
			line:    c.Line,
			id:      shortIDs[c.ID],
			text:    ticketText(c),
			created: c.Created,
			dim:     c.Resolved,

			fullID:   c.ID,
			tkind:    tkind,
			body:     c.Text,
			author:   c.Author,
			status:   c.EffectiveStatus(),
			marker:   c.Prefix,
			mtype:    c.Type,
			scope:    c.Scope,
			ticket:   c.Ticket,
			branch:   c.Branch,
			commit:   c.CommitSHA,
			resolved: c.Resolved,
			comment:  c,
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
	if len(o.Prefixes) > 0 {
		markers = nil
		for _, kw := range o.Prefixes {
			markers = append(markers, markerForKeyword(kw))
		}
	}
	if len(o.ExcludePrefixes) > 0 {
		exclude := make(map[string]bool, len(o.ExcludePrefixes))
		for _, kw := range o.ExcludePrefixes {
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

	// Scan only the files git tracks or would track. Driving the scan from git's
	// file list (rather than walking the tree and filtering) skips gitignored
	// trees entirely — a repo's virtualenvs and node_modules can hold hundreds of
	// thousands of files, and walking them dominates the runtime.
	files, err := g.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("listing repo files: %w", err)
	}
	paths := make([]string, len(files))
	for i, rel := range files {
		paths[i] = filepath.Join(root, rel)
	}

	ms, err := scanner.ScanFilesMarkers(paths, markers)
	if err != nil {
		return nil, fmt.Errorf("scanning code markers: %w", err)
	}

	var rows []listRow
	for _, m := range ms {
		rel := relPath(root, m.File)
		if !fileMatches(o.File, rel) {
			continue
		}
		if !grepMatches(o.Grep, m.Message, m.Ticket) {
			continue
		}
		if o.Type != "" && !strings.EqualFold(m.Type, o.Type) {
			continue
		}
		// --scope matches the annotation's scope (e.g. the "foo" in
		// "RPT refactor(foo):").
		if o.Scope != "" && !strings.EqualFold(o.Scope, m.Scope) {
			continue
		}
		if o.Ticket != "" && !strings.EqualFold(o.Ticket, m.Ticket) {
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
			kind:   kind,
			file:   rel,
			line:   m.Line,
			text:   m.Message,
			code:   true,
			marker: m.Keyword,
			mtype:  m.Type,
			scope:  m.Scope,
			ticket: m.Ticket,
		})
	}
	return rows, nil
}

// markerForKeyword builds a scanner.Marker for a user-supplied keyword using
// the loose (non-strict) form so all occurrences are visible in the list.
func markerForKeyword(kw string) scanner.Marker {
	return scanner.Marker{Keyword: strings.ToUpper(kw)}
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

// listRowJSON is the machine-readable form of a unified-list row emitted by
// --json. db-only fields are omitted for file comments, so consumers can branch
// on "store". db entries carry "id" (usable with `tdb comment resolve/edit`);
// file comments carry no id and are acted on by editing the annotated line in
// place.
type listRowJSON struct {
	Store string `json:"store"` // "db" | "file"
	File  string `json:"file,omitempty"`
	Line  int    `json:"line,omitempty"`
	Text  string `json:"text"`           // one-line summary (title or first body line)
	Body  string `json:"body,omitempty"` // db full text; empty for file comments

	// shared fields — carried by both stores
	Prefix string `json:"prefix,omitempty"` // RPT, TODO, …
	Type   string `json:"type,omitempty"`   // classification
	Scope  string `json:"scope,omitempty"`  // scope / code identifier
	Ticket string `json:"ticket,omitempty"` // external tracker ref (ABC-123, #123)

	// db-only fields
	ID       string `json:"id,omitempty"`
	ShortID  string `json:"short_id,omitempty"`
	Kind     string `json:"kind,omitempty"` // "comment" | "issue"
	Author   string `json:"author,omitempty"`
	Status   string `json:"status,omitempty"`
	Resolved *bool  `json:"resolved,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Created  string `json:"created,omitempty"` // RFC3339
}

// rowsToJSON converts unified-list rows to their machine-readable form, mapping
// each source to its field subset.
func rowsToJSON(rows []listRow) []listRowJSON {
	out := make([]listRowJSON, 0, len(rows))
	for _, r := range rows {
		if r.code {
			out = append(out, listRowJSON{
				Store:  "file",
				File:   r.file,
				Line:   r.line,
				Text:   r.text,
				Prefix: r.marker,
				Type:   r.mtype,
				Scope:  r.scope,
				Ticket: r.ticket,
			})
			continue
		}
		resolved := r.resolved
		jr := listRowJSON{
			Store:    "db",
			File:     r.file,
			Line:     r.line,
			Text:     r.text,
			Body:     r.body,
			Prefix:   r.marker,
			Type:     r.mtype,
			Scope:    r.scope,
			Ticket:   r.ticket,
			ID:       r.fullID,
			ShortID:  r.id,
			Kind:     r.tkind,
			Author:   r.author,
			Status:   r.status,
			Resolved: &resolved,
			Branch:   r.branch,
			Commit:   r.commit,
		}
		if !r.created.IsZero() {
			jr.Created = r.created.Format(time.RFC3339)
		}
		out = append(out, jr)
	}
	return out
}

// renderRowsJSON prints the unified list as a JSON array to stdout. Field order
// and the empty-list `[]` case are stable so agents can parse the output.
func renderRowsJSON(rows []listRow) error {
	b, err := json.MarshalIndent(rowsToJSON(rows), "", "  ")
	if err != nil {
		return fmt.Errorf("encoding json: %w", err)
	}
	fmt.Println(string(b))
	return nil
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
