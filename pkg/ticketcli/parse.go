package ticketcli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// errHelp is returned by ParseArgs when -h/--help is requested.
var errHelp = errors.New("help requested")

// Run parses a comment/note invocation (argv[0] is the "comment"/"c"/"note"/"n"
// token) and dispatches it. Styles and Highlighter from cfg are merged into the
// parsed options. A nil Highlighter yields plain (cgo-free) context rendering.
func Run(argv []string, cfg Options) error {
	// The unified `list` command merges tickets and code markers; it has its own
	// parser and is not a comment/note sub-invocation.
	if len(argv) > 0 && argv[0] == "list" {
		err := RunList(argv, cfg)
		if errors.Is(err, errHelp) {
			usage()
			return nil
		}
		return err
	}

	opts, err := ParseArgs(argv)
	if errors.Is(err, errHelp) {
		usage()
		return nil
	}
	if err != nil {
		return err
	}
	opts.Styles = cfg.Styles
	opts.Highlighter = cfg.Highlighter
	if opts.Note {
		return runNote(opts)
	}
	return runComment(opts)
}

// ParseArgs parses comment/note command-line arguments into Options. argv[0] must
// be the command token ("comment", "c", "note", or "n").
func ParseArgs(argv []string) (Options, error) {
	var opts Options
	if len(argv) == 0 {
		return opts, fmt.Errorf("missing command")
	}

	switch argv[0] {
	case "comment", "c":
		opts.Note = false
	case "note", "n":
		opts.Note = true
	default:
		return opts, fmt.Errorf("not a comment/note command: %q", argv[0])
	}
	rest := argv[1:]

	// Consume sub-subcommand (list/edit/add/resolve/unresolve), default "list".
	if len(rest) > 0 {
		switch rest[0] {
		case "list", "edit", "add", "resolve", "unresolve":
			opts.Sub = rest[0]
			rest = rest[1:]
		}
	}
	if opts.Sub == "" {
		opts.Sub = "list"
	}

	// Consume a positional argument: file:line for add, comment ID otherwise.
	if opts.Sub == "add" {
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			opts.AddTarget = rest[0]
			rest = rest[1:]
		}
	} else if subTakesID(opts.Sub, false) && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		opts.ID = rest[0]
		rest = rest[1:]
	}

	// Parse flags.
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		if len(arg) == 0 || arg[0] != '-' {
			return opts, fmt.Errorf("unexpected argument: %q", arg)
		}
		consumed, err := opts.parseFlag(arg, rest, i)
		if err != nil {
			return opts, err
		}
		i += consumed
	}

	if err := opts.validate(); err != nil {
		return opts, err
	}
	return opts, nil
}

// subTakesID reports whether a subcommand accepts a comment ID argument. When
// require is true, only subcommands that require an ID match (excludes "list").
func subTakesID(sub string, require bool) bool {
	switch sub {
	case "edit", "resolve", "unresolve":
		return true
	case "list":
		return !require
	default:
		return false
	}
}

// parseFlag handles a single comment/note flag. Returns extra args consumed.
func (o *Options) parseFlag(arg string, args []string, i int) (int, error) {
	switch {
	case arg == "--help" || arg == "-h":
		return 0, errHelp

	case arg == "-v" || arg == "--verbose":
		o.Verbose = true

	case strings.HasPrefix(arg, "--since="):
		o.Since = strings.TrimPrefix(arg, "--since=")
	case arg == "--since":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--since requires a duration (e.g. 30m, 6h, 30d, 2w, 3M, 1y, all)")
		}
		o.Since = args[i+1]
		return 1, nil

	case arg == "-n":
		// Bare -n (no following integer) is treated as -n0 (uncapped).
		if i+1 >= len(args) {
			o.N = 0
			o.NSet = true
			return 0, nil
		}
		n, err := strconv.Atoi(args[i+1])
		if err != nil {
			o.N = 0
			o.NSet = true
			return 0, nil
		}
		o.N = n
		o.NSet = true
		return 1, nil
	case strings.HasPrefix(arg, "-n"):
		nStr := strings.TrimPrefix(arg, "-n")
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return 0, fmt.Errorf("-n requires an integer, got %q", nStr)
		}
		o.N = n
		o.NSet = true

	case arg == "-b":
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			o.Branch = args[i+1]
			return 1, nil
		}
		o.AllBranches = true
	case arg == "--branch":
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			o.Branch = args[i+1]
			return 1, nil
		}
		o.AllBranches = true
	case strings.HasPrefix(arg, "--branch="):
		val := strings.TrimPrefix(arg, "--branch=")
		if val == "" {
			return 0, fmt.Errorf("--branch requires a branch name")
		}
		o.Branch = val

	case arg == "-m":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("-m requires a message argument")
		}
		o.AddMessage = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "-m"):
		o.AddMessage = strings.TrimPrefix(arg, "-m")

	case arg == "--ref":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--ref requires a ref argument (branch, tag, or commit)")
		}
		o.AddRef = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--ref="):
		o.AddRef = strings.TrimPrefix(arg, "--ref=")
		if o.AddRef == "" {
			return 0, fmt.Errorf("--ref requires a ref argument")
		}

	case arg == "--author":
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			o.Author = args[i+1]
			o.AuthorSet = true
			return 1, nil
		}
		o.AuthorSet = true
	case strings.HasPrefix(arg, "--author="):
		o.Author = strings.TrimPrefix(arg, "--author=")
		if o.Author == "" {
			return 0, fmt.Errorf("--author requires a value when using = syntax")
		}
		o.AuthorSet = true

	case arg == "--file":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--file requires a file path argument")
		}
		o.File = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--file="):
		o.File = strings.TrimPrefix(arg, "--file=")
		if o.File == "" {
			return 0, fmt.Errorf("--file requires a file path argument")
		}

	case arg == "--grep":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--grep requires a search pattern")
		}
		o.Grep = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--grep="):
		o.Grep = strings.TrimPrefix(arg, "--grep=")
		if o.Grep == "" {
			return 0, fmt.Errorf("--grep requires a search pattern")
		}

	case arg == "--marker":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--marker requires a value")
		}
		o.Marker = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--marker="):
		o.Marker = strings.TrimPrefix(arg, "--marker=")
		if o.Marker == "" {
			return 0, fmt.Errorf("--marker requires a value")
		}

	case arg == "--type":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--type requires a value")
		}
		o.Type = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--type="):
		o.Type = strings.TrimPrefix(arg, "--type=")
		if o.Type == "" {
			return 0, fmt.Errorf("--type requires a value")
		}

	case arg == "--scope":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--scope requires a value")
		}
		o.Scope = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--scope="):
		o.Scope = strings.TrimPrefix(arg, "--scope=")
		if o.Scope == "" {
			return 0, fmt.Errorf("--scope requires a value")
		}

	case arg == "--raw":
		o.Raw = true
	case arg == "--all-branches":
		o.AllBranches = true

	case arg == "--kind":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--kind requires a value (comment, note, all)")
		}
		if err := setKind(o, args[i+1]); err != nil {
			return 0, err
		}
		return 1, nil
	case strings.HasPrefix(arg, "--kind="):
		if err := setKind(o, strings.TrimPrefix(arg, "--kind=")); err != nil {
			return 0, err
		}

	case arg == "--status":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--status requires a value (unresolved, resolved, all)")
		}
		if err := setStatus(o, args[i+1]); err != nil {
			return 0, err
		}
		return 1, nil
	case strings.HasPrefix(arg, "--status="):
		if err := setStatus(o, strings.TrimPrefix(arg, "--status=")); err != nil {
			return 0, err
		}

	case arg == "--resolved":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--resolved requires a value (true or false)")
		}
		if err := setResolved(o, args[i+1]); err != nil {
			return 0, err
		}
		return 1, nil
	case strings.HasPrefix(arg, "--resolved="):
		if err := setResolved(o, strings.TrimPrefix(arg, "--resolved=")); err != nil {
			return 0, err
		}

	default:
		return 0, fmt.Errorf("unknown flag: %s", arg)
	}
	return 0, nil
}

func setKind(o *Options, val string) error {
	switch val {
	case "comment", "note", "all":
		o.Kind = val
		return nil
	default:
		return fmt.Errorf("--kind must be comment, note, or all; got %q", val)
	}
}

func setStatus(o *Options, val string) error {
	switch val {
	case "unresolved", "resolved", "all":
		o.Status = val
		return nil
	default:
		return fmt.Errorf("--status must be unresolved, resolved, or all; got %q", val)
	}
}

func setResolved(o *Options, val string) error {
	switch val {
	case "true":
		v := true
		o.Resolved = &v
	case "false":
		v := false
		o.Resolved = &v
	default:
		return fmt.Errorf("--resolved must be true or false; got %q", val)
	}
	return nil
}

// validate checks for invalid flag/subcommand combinations.
func (o *Options) validate() error {
	name := "comment"
	if o.Note {
		name = "note"
	}

	if subTakesID(o.Sub, true) && o.ID == "" {
		return fmt.Errorf("%s %s requires a comment ID", name, o.Sub)
	}
	if o.Resolved != nil && o.Sub != "edit" {
		if o.Sub == "resolve" || o.Sub == "unresolve" {
			return fmt.Errorf("--resolved cannot be combined with %s %s (it already sets resolved state)", name, o.Sub)
		}
		return fmt.Errorf("--resolved is only valid for %s edit", name)
	}
	if o.Kind != "" && o.Sub != "list" {
		return fmt.Errorf("--kind is only valid for %s list", name)
	}
	if o.Kind != "" && o.Note {
		return fmt.Errorf("--kind cannot be used with note (use comment list --kind instead)")
	}
	if o.AddMessage != "" && o.Sub != "add" {
		return fmt.Errorf("-m is only valid for %s add", name)
	}
	if o.AddRef != "" && o.Sub != "add" {
		return fmt.Errorf("--ref is only valid for %s add", name)
	}
	if o.AuthorSet && o.Sub != "add" && o.Sub != "list" {
		return fmt.Errorf("--author is only valid for %s add and %s list", name, name)
	}
	if o.File != "" && o.Sub != "list" {
		return fmt.Errorf("--file is only valid for %s list", name)
	}
	if o.Grep != "" && o.Sub != "list" {
		return fmt.Errorf("--grep is only valid for %s list", name)
	}
	if o.Marker != "" && o.Sub != "add" && o.Sub != "list" {
		return fmt.Errorf("--marker is only valid for %s add and %s list", name, name)
	}
	if o.Type != "" && o.Sub != "add" && o.Sub != "list" {
		return fmt.Errorf("--type is only valid for %s add and %s list", name, name)
	}
	if o.Scope != "" && o.Sub != "add" && o.Sub != "list" {
		return fmt.Errorf("--scope is only valid for %s add and %s list", name, name)
	}
	if o.Sub == "add" && o.AuthorSet && o.Author == "" {
		return fmt.Errorf("--author requires an author argument for %s add", name)
	}
	if o.Note && o.Sub == "add" && o.AddTarget != "" {
		return fmt.Errorf("note add does not accept a file:line argument (use comment add instead)")
	}
	if o.AllBranches && o.Branch != "" {
		return fmt.Errorf("--all-branches and --branch cannot be used together")
	}
	return nil
}

// usage prints comment/note CLI help to stdout.
func usage() {
	PrintUsage(os.Stdout)
}

// PrintUsage writes comment/note usage text to w.
func PrintUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: tdb list [options]
       tdb comment <subcommand> [options]
       tdb note <subcommand> [options]

list merges git-state tickets and in-code markers (TODO/FIXME/RPT/…) into one view:
  --source VALUE         all (default), state (tickets), code (markers)
  --marker LIST          filter by marker keyword(s) (TODO, RPT, …); both sources
  --exclude-marker LIST  exclude these marker keyword(s); both sources
  --type VALUE           filter by type (feat, fix, refactor, …); both sources
  --scope CODE           filter by scope/code (ticket tag or RPT annotation scope)
  --status VALUE         unresolved (default), resolved, all; tickets only
  --file PATH            filter by file (trailing / = prefix match)
  --grep TEXT            filter by text (case-insensitive)
  -n[N]                  limit combined rows (bare = all)
  --json                 machine-readable JSON array (any source)
  --exit-code            exit 1 if any rows match, 0 if none (CI gate)
  -b, --branch [NAME]    scope tickets to a branch (no value = all branches)
  --all-branches         tickets from all branches

comment / note subcommands:
  list [ID]              List comments (or show one by ID suffix)
  add [file:line]        Add a comment (file:line) or standalone note
  edit <ID>              Edit a comment in $EDITOR
  resolve <ID>           Mark a comment resolved
  unresolve <ID>         Mark a comment unresolved

List options:
  -v                     Verbose block output
  --raw                  Raw serialized blob output
  -n[N]                  Limit count (newest N; negative = oldest; bare = all)
  --status VALUE         unresolved (default), resolved, all
  --kind VALUE           comment, note, all (comment only)
  --since DURATION       e.g. 30m, 6h, 30d, 2w, 3M, 1y, all
  -b, --branch [NAME]    Filter by branch (no value = all branches)
  --all-branches         Show comments from all branches
  --author [NAME]        Filter by author (bare = no author)
  --file PATH            Filter by file (trailing / = prefix match)
  --grep TEXT            Filter by comment text (case-insensitive)
  --marker KW            Filter by marker keyword (RPT, TODO, …)
  --type VALUE           Filter by type (feat, fix, refactor, …)
  --scope CODE           Filter by scope/code

Add options:
  -m MESSAGE             Comment text (else read from stdin)
  --ref REF              Commit/branch/tag to attach to
  --author NAME          Set author
  --marker KW            Tag with a marker keyword (RPT, TODO, …)
  --type VALUE           Tag with a type (feat, fix, refactor, …)
  --scope CODE           Tag with a scope/code identifier

Edit options:
  --resolved true|false  Set resolved state without opening the editor
`)
}
