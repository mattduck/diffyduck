package main

import (
	"fmt"
	"strings"

	"github.com/mattduck/diffyduck/pkg/ticketdb"
)

// completionContext holds the parsed state of a partial tdb command line.
type completionContext struct {
	cmd             string   // top-level command: "list", "comment", "note", etc.
	sub             string   // sub-subcommand for comment/note: "list", "add", "edit", …
	flags           []string // flags already committed
	args            []string // non-flag positional args after cmd/sub
	current         string   // word being completed (may be empty)
	expectFlagValue string   // if last committed word was a flag that takes a value
}

var subcommands = []string{"list", "comment", "note", "help", "completion"}

var commentSubcmds = []string{"list", "add", "edit", "resolve", "unresolve"}
var noteSubcmds = []string{"list", "add", "edit"}

type commentIDsFunc func() []string

func parseCompletionContext(words []string) completionContext {
	ctx := completionContext{}
	if len(words) == 0 {
		return ctx
	}
	ctx.current = words[len(words)-1]
	committed := words[:len(words)-1]

	i := 0
	if i < len(committed) && isSubcommand(committed[i]) {
		ctx.cmd = committed[i]
		i++
	}

	// For comment/note, consume the sub-subcommand.
	if (ctx.cmd == "comment" || ctx.cmd == "c" || ctx.cmd == "note" || ctx.cmd == "n") &&
		i < len(committed) && isCommentSub(committed[i], ctx.cmd) {
		ctx.sub = committed[i]
		i++
	}

	for ; i < len(committed); i++ {
		w := committed[i]
		if isFlag(w) {
			ctx.flags = append(ctx.flags, w)
			if flagTakesValue(w) {
				i++ // skip value
			}
			continue
		}
		ctx.args = append(ctx.args, w)
	}

	if len(committed) > 0 {
		last := committed[len(committed)-1]
		if isFlag(last) && flagTakesValue(last) {
			ctx.expectFlagValue = last
		}
	}

	return ctx
}

func isSubcommand(w string) bool {
	switch w {
	case "list", "comment", "c", "note", "n", "help", "completion":
		return true
	}
	return false
}

func isCommentSub(w, cmd string) bool {
	if cmd == "note" || cmd == "n" {
		switch w {
		case "list", "add", "edit":
			return true
		}
		return false
	}
	switch w {
	case "list", "add", "edit", "resolve", "unresolve":
		return true
	}
	return false
}

func isFlag(w string) bool {
	return len(w) > 0 && w[0] == '-'
}

func flagTakesValue(flag string) bool {
	switch flag {
	case "--since", "--status", "--kind", "-n", "-b", "--branch", "-m", "--ref", "--author", "--file", "--grep", "--source", "--marker", "--exclude-marker", "--type", "--scope":
		return true
	}
	return false
}

func generateCompletions(ctx completionContext, commentIDs commentIDsFunc) []string {
	if ctx.expectFlagValue != "" {
		return completeFlagValue(ctx.expectFlagValue, ctx.current)
	}

	if strings.HasPrefix(ctx.current, "-") {
		return completeFlags(ctx)
	}

	if ctx.cmd == "" {
		return filterPrefix(subcommands, ctx.current)
	}

	switch ctx.cmd {
	case "completion":
		return filterPrefix([]string{"bash", "zsh", "fish"}, ctx.current)
	case "help":
		return filterPrefix(subcommands, ctx.current)
	case "list":
		return completeFlags(ctx)
	case "comment", "c":
		if ctx.sub == "" {
			return filterPrefix(commentSubcmds, ctx.current)
		}
		if len(ctx.args) == 0 && commentSubTakesID(ctx.sub) && commentIDs != nil {
			if ids := commentIDs(); len(ids) > 0 {
				return filterPrefix(ids, ctx.current)
			}
		}
		return completeFlags(ctx)
	case "note", "n":
		if ctx.sub == "" {
			return filterPrefix(noteSubcmds, ctx.current)
		}
		if len(ctx.args) == 0 && commentSubTakesID(ctx.sub) && commentIDs != nil {
			if ids := commentIDs(); len(ids) > 0 {
				return filterPrefix(ids, ctx.current)
			}
		}
		return completeFlags(ctx)
	}
	return nil
}

func commentSubTakesID(sub string) bool {
	switch sub {
	case "edit", "resolve", "unresolve":
		return true
	}
	return false
}

func completeFlagValue(flag, prefix string) []string {
	var values []string
	switch flag {
	case "--source":
		values = []string{"all", "state", "code"}
	case "--status":
		values = []string{"unresolved", "resolved", "all"}
	case "--kind":
		values = []string{"comment", "note", "all"}
	default:
		return nil
	}
	return filterPrefix(values, prefix)
}

func flagsForCmd(cmd, sub string) []string {
	switch cmd {
	case "list":
		return []string{"--source", "--marker", "--exclude-marker", "--type", "--scope", "--file", "--grep", "--status", "-n", "--random", "--json", "--exit-code", "-b", "--branch", "--all-branches", "--help"}
	case "comment", "c":
		switch sub {
		case "add":
			return []string{"-m", "--ref", "--author", "--marker", "--type", "--scope", "--help"}
		case "edit":
			return []string{"-m", "--help"}
		case "resolve", "unresolve":
			return []string{"--help"}
		default: // list or no sub
			return []string{"-n", "-v", "--verbose", "-b", "--branch", "--since", "--status", "--kind", "--raw", "--all-branches", "--resolved", "--ref", "--author", "--file", "--grep", "--marker", "--type", "--scope", "--help"}
		}
	case "note", "n":
		switch sub {
		case "add":
			return []string{"-m", "--ref", "--author", "--marker", "--type", "--scope", "--help"}
		case "edit":
			return []string{"-m", "--help"}
		default:
			return []string{"-n", "-v", "--verbose", "-b", "--branch", "--since", "--status", "--raw", "--all-branches", "--resolved", "--ref", "--author", "--file", "--grep", "--marker", "--type", "--scope", "--help"}
		}
	}
	return []string{"--help"}
}

func completeFlags(ctx completionContext) []string {
	available := flagsForCmd(ctx.cmd, ctx.sub)
	used := make(map[string]bool, len(ctx.flags))
	for _, f := range ctx.flags {
		used[f] = true
	}
	var result []string
	for _, f := range available {
		if !used[f] && strings.HasPrefix(f, ctx.current) {
			result = append(result, f)
		}
	}
	return result
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

func commentIDsFromStore() []string {
	store := ticketdb.NewStore("")
	idx, err := store.ReadIndex()
	if err != nil {
		return nil
	}
	return idx.All()
}

func runComplete(args []string) {
	ctx := parseCompletionContext(args)
	for _, c := range generateCompletions(ctx, commentIDsFromStore) {
		fmt.Println(c)
	}
}

func runCompletion(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usageCompletion)
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: tdb completion bash|zsh|fish")
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

const usageCompletion = `tdb completion - print shell completion script

Usage:
  tdb completion bash|zsh|fish

The script is printed to stdout. Source it in your shell config
or save it to the appropriate completions directory.

Install:
  # Bash
  eval "$(tdb completion bash)"
  # or save permanently:
  tdb completion bash > /etc/bash_completion.d/tdb

  # Zsh
  eval "$(tdb completion zsh)"
  # or save permanently:
  tdb completion zsh > "${fpath[1]}/_tdb" && compinit

  # Fish
  tdb completion fish | source
  # or save permanently:
  tdb completion fish > ~/.config/fish/completions/tdb.fish
`

const bashCompletionScript = `# tdb bash completion
# Install: eval "$(tdb completion bash)"
# Or:      tdb completion bash > /etc/bash_completion.d/tdb

_tdb_completions() {
    COMP_WORDBREAKS="${COMP_WORDBREAKS//=/}"

    local words=()
    local i
    for ((i = 1; i <= COMP_CWORD; i++)); do
        words+=("${COMP_WORDS[$i]}")
    done

    local IFS=$'\n'
    local candidates
    candidates=($(tdb __complete "${words[@]}" 2>/dev/null))

    if [[ ${#candidates[@]} -eq 0 ]]; then
        COMPREPLY=()
        return
    fi

    COMPREPLY=($(compgen -W "${candidates[*]}" -- "${COMP_WORDS[$COMP_CWORD]}"))
}

complete -o default -F _tdb_completions tdb
`

const zshCompletionScript = `#compdef tdb
# tdb zsh completion
# Install: eval "$(tdb completion zsh)"
# Or:      tdb completion zsh > "${fpath[1]}/_tdb" && compinit

_tdb() {
    local -a candidates
    local IFS=$'\n'
    candidates=($(tdb __complete "${words[@]:1:$CURRENT}" 2>/dev/null))
    if (( ${#candidates[@]} == 0 )); then
        _files
        return
    fi
    compadd -a candidates
}

compdef _tdb tdb
`

const fishCompletionScript = `# tdb fish completion
# Install: tdb completion fish | source
# Or:      tdb completion fish > ~/.config/fish/completions/tdb.fish

complete -c tdb -f -a '(
    set -l tokens (commandline -opc)
    set -l current (commandline -ct)
    tdb __complete $tokens[2..] $current 2>/dev/null
)'
`
