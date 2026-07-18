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

// Run parses a write-side invocation (argv[0] is "add"/"edit"/"resolve"/
// "unresolve") and dispatches it. The unified `list` reader is handled
// separately. Styles and Highlighter from cfg are merged into the parsed
// options. A nil Highlighter yields plain (cgo-free) context rendering.
func Run(argv []string, cfg Options) error {
	// The unified `list` command merges db entries and file comments; it has its
	// own parser and is not a write-side invocation.
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
	switch opts.Sub {
	case "add":
		// add infers kind from the presence of a file:line target.
		return runCommentAdd(opts)
	case "edit":
		return runCommentEdit(opts.ID, opts.Resolved)
	case "resolve":
		resolved := true
		return runCommentEdit(opts.ID, &resolved)
	case "unresolve":
		resolved := false
		return runCommentEdit(opts.ID, &resolved)
	default:
		usage()
		return nil
	}
}

// ParseArgs parses write-side command-line arguments into Options. argv[0] must
// be a write command: "add", "edit", "resolve", or "unresolve".
func ParseArgs(argv []string) (Options, error) {
	var opts Options
	if len(argv) == 0 {
		return opts, fmt.Errorf("missing command")
	}

	switch argv[0] {
	case "add", "edit", "resolve", "unresolve":
		opts.Sub = argv[0]
	default:
		return opts, fmt.Errorf("unknown command %q (want add, edit, resolve, unresolve, or list)", argv[0])
	}
	rest := argv[1:]

	// Consume a positional argument: file:line for add, entry ID otherwise.
	if opts.Sub == "add" {
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			opts.AddTarget = rest[0]
			rest = rest[1:]
		}
	} else if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
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

// subTakesID reports whether a write subcommand is addressed by an entry ID.
func subTakesID(sub string) bool {
	switch sub {
	case "edit", "resolve", "unresolve":
		return true
	default:
		return false
	}
}

// parseFlag handles a single write-side flag. Returns extra args consumed.
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

	case arg == "--commit":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--commit requires a commit/branch/tag argument")
		}
		o.AddCommit = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--commit="):
		o.AddCommit = strings.TrimPrefix(arg, "--commit=")
		if o.AddCommit == "" {
			return 0, fmt.Errorf("--commit requires a commit/branch/tag argument")
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

	case arg == "--prefix":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--prefix requires a value")
		}
		o.Prefix = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--prefix="):
		o.Prefix = strings.TrimPrefix(arg, "--prefix=")
		if o.Prefix == "" {
			return 0, fmt.Errorf("--prefix requires a value")
		}

	case arg == "--ticket":
		if i+1 >= len(args) {
			return 0, fmt.Errorf("--ticket requires a reference (e.g. ABC-123, #123)")
		}
		o.Ticket = args[i+1]
		return 1, nil
	case strings.HasPrefix(arg, "--ticket="):
		o.Ticket = strings.TrimPrefix(arg, "--ticket=")
		if o.Ticket == "" {
			return 0, fmt.Errorf("--ticket requires a reference (e.g. ABC-123, #123)")
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

// validate checks for invalid flag/subcommand combinations on the write path
// (add/edit/resolve/unresolve). Reader-only flags are pointed back to `tdb list`.
func (o *Options) validate() error {
	if subTakesID(o.Sub) && o.ID == "" {
		return fmt.Errorf("%s requires an entry ID", o.Sub)
	}
	if o.Resolved != nil && o.Sub != "edit" {
		if o.Sub == "resolve" || o.Sub == "unresolve" {
			return fmt.Errorf("--resolved cannot be combined with %s (it already sets resolved state)", o.Sub)
		}
		return fmt.Errorf("--resolved is only valid for edit")
	}

	// Reader-domain flags don't apply to the write commands.
	for _, c := range []struct {
		flag string
		set  bool
	}{
		{"--file", o.File != ""},
		{"--grep", o.Grep != ""},
		{"--status", o.Status != ""},
		{"--since", o.Since != ""},
		{"-v/--verbose", o.Verbose},
		{"--raw", o.Raw},
		{"-n", o.NSet},
		{"-b/--branch", o.Branch != ""},
		{"--all-branches", o.AllBranches},
	} {
		if c.set {
			return fmt.Errorf("%s is only valid for `tdb list`", c.flag)
		}
	}

	// Content/tag flags apply only to add.
	for _, c := range []struct {
		flag string
		set  bool
	}{
		{"-m", o.AddMessage != ""},
		{"--commit", o.AddCommit != ""},
		{"--author", o.AuthorSet},
		{"--prefix", o.Prefix != ""},
		{"--type", o.Type != ""},
		{"--scope", o.Scope != ""},
		{"--ticket", o.Ticket != ""},
	} {
		if c.set && o.Sub != "add" {
			return fmt.Errorf("%s is only valid for add", c.flag)
		}
	}
	if o.Sub == "add" && o.AuthorSet && o.Author == "" {
		return fmt.Errorf("--author requires an author argument for add")
	}
	return nil
}

// usage prints the tdb CLI help to stdout.
func usage() {
	PrintUsage(os.Stdout)
}

// PrintUsage writes the tdb usage text to w.
func PrintUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: tdb list [options]
       tdb add [file:line] [options]
       tdb edit|resolve|unresolve <ID> [options]

list merges db entries and in-file comments (TODO/FIXME/RPT/…) into one view:
  --store VALUE          all (default), db, file
  --kind VALUE           all (default), comment, issue (db/all store only)
  --prefix LIST          filter by prefix keyword(s) (TODO, RPT, …); any store
  --exclude-prefix LIST  exclude these prefix keyword(s); any store
  --type VALUE           filter by type (feat, bug, epic, …); any store
  --scope CODE           filter by scope/code; any store
  --ticket REF           filter by external ticket ref (ABC-123, #123); any store
  --status VALUE         unresolved (default), resolved, all; db only
  --since DURATION       e.g. 30m, 6h, 30d, 2w, 3M, 1y, all; db only
  --author [NAME]        filter by author (bare = no author); db only
  --file PATH            filter by file (trailing / = prefix match)
  --grep TEXT            filter by text (case-insensitive)
  -v                     verbose block output (db only)
  --raw                  raw serialized blob output (db only)
  -n[N]                  limit combined rows (bare = all)
  --random               shuffle rows; returns one (or -n N) at random
  --stats                counts breakdown instead of the list (multi-dimension)
  --stats-group FIELD    collapse --stats to one field (store/kind/prefix/
                         type/scope/ticket/author/file/branch)
  --json                 machine-readable JSON array (any store)
  --exit-code            exit 1 if any rows match, 0 if none (CI gate)
  -b, --branch [NAME]    scope db entries to a branch (no value = all branches)
  --all-branches         db entries from all branches

write commands (db store only):
  add [file:line]        Add a db comment (with file:line) or db issue (standalone)
  edit <ID>              Edit a db entry in $EDITOR
  resolve <ID>           Mark a db entry resolved
  unresolve <ID>         Mark a db entry unresolved

Add options:
  -m MESSAGE             Entry text (else read from stdin)
  --commit REF           Commit/branch/tag to attach to
  --author NAME          Set author
  --prefix KW            Tag with a prefix keyword (RPT, TODO, …)
  --type VALUE           Tag with a type (feat, bug, epic, …)
  --scope CODE           Tag with a scope/code identifier
  --ticket REF           Tag with an external ticket ref (ABC-123, #123)

Edit options:
  --resolved true|false  Set resolved state without opening the editor
`)
}
