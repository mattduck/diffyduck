package main

import (
	"fmt"
	"strings"

	"github.com/user/diffyduck/pkg/git"
)

// completionContext represents the parsed state of a partial command line
// for generating tab completion candidates.
type completionContext struct {
	cmd      string   // subcommand (may be empty if not yet typed)
	flags    []string // flags already committed
	refs     []string // positional refs already committed
	afterSep bool     // true if "--" has been seen
	current  string   // the word being completed (may be empty)

	// If the last committed word was a flag that takes a separate value,
	// this is set to that flag name (e.g. "--since", "-e", "-n").
	expectFlagValue string
}

// subcommands is the list of completable subcommand names.
// Aliases (d, l, b, s) are excluded — users who know them don't need completion.
var subcommands = []string{
	"diff", "show", "log", "clean", "branch", "status", "config", "comment", "help", "completion",
}

// parseCompletionContext parses a word list from __complete into a context.
// words contains everything after "dfd __complete". The last element is the
// word being completed (may be ""); preceding elements are committed words.
func parseCompletionContext(words []string) completionContext {
	ctx := completionContext{}
	if len(words) == 0 {
		return ctx
	}

	ctx.current = words[len(words)-1]
	committed := words[:len(words)-1]

	// Consume subcommand from first committed word if it matches.
	i := 0
	if i < len(committed) && isSubcommand(committed[i]) {
		ctx.cmd = expandAlias(committed[i])
		i++
	}

	// Walk remaining committed words.
	for ; i < len(committed); i++ {
		w := committed[i]
		if w == "--" {
			ctx.afterSep = true
			continue
		}
		if ctx.afterSep {
			continue // paths after -- are not interesting for context
		}
		if isFlag(w) {
			ctx.flags = append(ctx.flags, w)
			if flagTakesValue(w) {
				i++ // skip the consumed value
			}
			continue
		}
		ctx.refs = append(ctx.refs, w)
	}

	// Check if the cursor word is a value for the previous flag.
	if len(committed) > 0 && !ctx.afterSep {
		last := committed[len(committed)-1]
		if isFlag(last) && flagTakesValue(last) {
			ctx.expectFlagValue = last
		}
	}

	return ctx
}

func isSubcommand(w string) bool {
	switch w {
	case "diff", "d", "show", "log", "l", "clean", "branch", "b",
		"config", "status", "s", "comment", "c", "help", "completion":
		return true
	}
	return false
}

func isFlag(w string) bool {
	return len(w) > 0 && w[0] == '-'
}

// flagTakesValue returns true if the flag consumes a separate next word as its value.
func flagTakesValue(flag string) bool {
	switch flag {
	case "--exclude", "-e", "-n", "--since", "--status", "--cpuprofile":
		return true
	}
	return false
}

// generateCompletions returns candidate completions for the given context.
// The gitFunc parameter allows injecting a git instance for testing.
func generateCompletions(ctx completionContext, g git.Git) []string {
	// Flag value completion.
	if ctx.expectFlagValue != "" {
		return completeFlagValue(ctx.expectFlagValue, ctx.current)
	}

	// Inline --flag=value completion.
	if eqIdx := strings.Index(ctx.current, "="); eqIdx > 0 {
		flagPart := ctx.current[:eqIdx]
		valuePart := ctx.current[eqIdx+1:]
		values := completeFlagValue(flagPart, valuePart)
		var result []string
		for _, v := range values {
			result = append(result, flagPart+"="+v)
		}
		return result
	}

	// After -- separator: return nothing, let shell do file completion.
	if ctx.afterSep {
		return nil
	}

	// Flag completion: current word starts with "-".
	if strings.HasPrefix(ctx.current, "-") {
		return completeFlags(ctx)
	}

	// No subcommand yet: complete subcommands.
	if ctx.cmd == "" {
		return filterPrefix(subcommands, ctx.current)
	}

	// Subcommand-specific positional completion.
	switch ctx.cmd {
	case "help":
		return filterPrefix(subcommands, ctx.current)
	case "completion":
		return filterPrefix([]string{"bash", "zsh", "fish"}, ctx.current)
	case "diff", "show", "log":
		return completeRefs(ctx, g)
	case "comment":
		// Complete sub-subcommand (list, edit) if not yet given.
		if len(ctx.refs) == 0 {
			return filterPrefix([]string{"list", "edit"}, ctx.current)
		}
		return completeFlags(ctx)
	default:
		// clean, config, branch, status: no positional args, offer flags.
		return completeFlags(ctx)
	}
}

func completeFlagValue(flag, prefix string) []string {
	var values []string
	switch flag {
	case "--since":
		values = []string{"7d", "2w", "1m", "3m", "1y", "all"}
	case "--untracked-files", "-u":
		values = []string{"no", "normal", "all"}
	case "--status":
		values = []string{"unresolved", "resolved", "all"}
	default:
		return nil
	}
	return filterPrefix(values, prefix)
}

// flagsForCmd returns the flags valid for a subcommand.
func flagsForCmd(cmd string) []string {
	global := []string{"--help", "--debug"}

	switch cmd {
	case "diff", "":
		return append(global,
			"--cached", "--staged", "--unstaged",
			"--all", "--snapshots", "--no-snapshots",
			"--exclude")
	case "show":
		return global
	case "log":
		return append(global, "--exclude")
	case "clean":
		return global
	case "branch":
		return append(global, "--verbose", "--since")
	case "status":
		return append(global, "--symbols", "--untracked-files", "--branches")
	case "comment":
		return append(global, "-n", "--since", "--status", "--oneline", "--all-branches")
	case "config":
		return append(global, "--init", "--force", "--print", "--path", "--edit")
	default:
		return global
	}
}

func completeFlags(ctx completionContext) []string {
	available := flagsForCmd(ctx.cmd)
	used := make(map[string]bool, len(ctx.flags))
	for _, f := range ctx.flags {
		used[f] = true
		// Mark aliases as used too.
		for _, alias := range flagAliases(f) {
			used[alias] = true
		}
	}

	var result []string
	for _, f := range available {
		if used[f] {
			continue
		}
		if strings.HasPrefix(f, ctx.current) {
			result = append(result, f)
		}
	}
	return result
}

// flagAliases returns alternate names for a flag.
func flagAliases(flag string) []string {
	switch flag {
	case "--cached":
		return []string{"--staged"}
	case "--staged":
		return []string{"--cached"}
	case "-a":
		return []string{"--all"}
	case "--all":
		return []string{"-a"}
	case "-e":
		return []string{"--exclude"}
	case "--exclude":
		return []string{"-e"}
	case "-v":
		return []string{"--verbose"}
	case "--verbose":
		return []string{"-v"}
	case "-S":
		return []string{"--symbols"}
	case "--symbols":
		return []string{"-S"}
	case "-u":
		return []string{"--untracked-files"}
	case "--untracked-files":
		return []string{"-u"}
	case "-b":
		return []string{"--branches"}
	case "--branches":
		return []string{"-b"}
	case "-h":
		return []string{"--help"}
	case "--help":
		return []string{"-h"}
	}
	return nil
}

func completeRefs(ctx completionContext, g git.Git) []string {
	maxRefs := 2 // diff
	switch ctx.cmd {
	case "show", "log":
		maxRefs = 1
	}
	if len(ctx.refs) >= maxRefs {
		return completeFlags(ctx)
	}

	refs := listRefs(g)
	candidates := filterPrefix(refs, ctx.current)
	// Also offer flags when completing a ref position.
	candidates = append(candidates, completeFlags(ctx)...)
	return candidates
}

func listRefs(g git.Git) []string {
	if g == nil {
		return nil
	}
	var refs []string
	if branches, err := g.LocalBranches(); err == nil {
		for _, b := range branches {
			refs = append(refs, b.Name)
		}
	}
	if tags, err := g.Tags(); err == nil {
		refs = append(refs, tags...)
	}
	return refs
}

func filterPrefix(items []string, prefix string) []string {
	var result []string
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return result
}

// runComplete handles the hidden __complete subcommand.
func runComplete(args []string) error {
	ctx := parseCompletionContext(args)
	g := git.New()
	candidates := generateCompletions(ctx, g)
	for _, c := range candidates {
		fmt.Println(c)
	}
	return nil
}

// runCompletion handles "dfd completion <shell>" and prints the shell script.
func runCompletion(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printUsage("completion")
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: dfd completion bash|zsh|fish")
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletionScript)
	case "zsh":
		fmt.Print(zshCompletionScript)
	case "fish":
		fmt.Print(fishCompletionScript)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", args[0])
	}
	return nil
}

const bashCompletionScript = `# dfd bash completion
# Install: eval "$(dfd completion bash)"
# Or:      dfd completion bash > /etc/bash_completion.d/dfd

_dfd_completions() {
    # Keep --flag=value as one word
    COMP_WORDBREAKS="${COMP_WORDBREAKS//=/}"

    local words=()
    local i
    for ((i = 1; i <= COMP_CWORD; i++)); do
        words+=("${COMP_WORDS[$i]}")
    done

    local IFS=$'\n'
    local candidates
    candidates=($(dfd __complete "${words[@]}" 2>/dev/null))

    if [[ ${#candidates[@]} -eq 0 ]]; then
        COMPREPLY=()
        return
    fi

    COMPREPLY=($(compgen -W "${candidates[*]}" -- "${COMP_WORDS[$COMP_CWORD]}"))
}

complete -o default -F _dfd_completions dfd
`

const zshCompletionScript = `#compdef dfd
# dfd zsh completion
# Install: eval "$(dfd completion zsh)"
# Or:      dfd completion zsh > "${fpath[1]}/_dfd" && compinit

_dfd() {
    local -a candidates
    local IFS=$'\n'

    # words[1] is "dfd"; pass words[2..CURRENT] to __complete
    candidates=($(dfd __complete "${words[@]:1:$CURRENT}" 2>/dev/null))

    if (( ${#candidates[@]} == 0 )); then
        _files
        return
    fi

    compadd -a candidates
}

compdef _dfd dfd
`

const fishCompletionScript = `# dfd fish completion
# Install: dfd completion fish | source
# Or:      dfd completion fish > ~/.config/fish/completions/dfd.fish

complete -c dfd -f -a '(
    set -l tokens (commandline -opc)
    set -l current (commandline -ct)
    dfd __complete $tokens[2..] $current 2>/dev/null
)'
`

const usageCompletion = `dfd completion - print shell completion script

Usage:
  dfd completion bash|zsh|fish

The script is printed to stdout. Source it in your shell config
or save it to the appropriate completions directory.

Install:
  # Bash
  eval "$(dfd completion bash)"
  # or save permanently:
  dfd completion bash > /etc/bash_completion.d/dfd

  # Zsh
  eval "$(dfd completion zsh)"
  # or save permanently:
  dfd completion zsh > "${fpath[1]}/_dfd" && compinit

  # Fish
  dfd completion fish | source
  # or save permanently:
  dfd completion fish > ~/.config/fish/completions/dfd.fish
`
