package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattduck/diffyduck/pkg/rpconfig"
)

// completionContext holds the parsed state of a partial rpt command line.
type completionContext struct {
	cmd             string   // "check", "rules", "diff", etc.
	flags           []string // flags already committed
	refs            []string // positional refs (for diff) or paths (for check)
	current         string   // word being completed
	expectFlagValue string   // if last committed word was a flag that takes a value
}

var rptSubcommands = []string{"check", "rules", "diff", "show", "version", "help", "completion"}

type ruleCodesFunc func() []string

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

	for ; i < len(committed); i++ {
		w := committed[i]
		if isFlag(w) {
			ctx.flags = append(ctx.flags, w)
			if flagTakesValue(w) {
				i++
			}
			continue
		}
		ctx.refs = append(ctx.refs, w)
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
	case "check", "rules", "diff", "show", "version", "help", "completion":
		return true
	}
	return false
}

func isFlag(w string) bool {
	return len(w) > 0 && w[0] == '-'
}

func flagTakesValue(flag string) bool {
	switch flag {
	case "-rule", "-config":
		return true
	}
	return false
}

func generateCompletions(ctx completionContext, ruleCodes ruleCodesFunc) []string {
	if ctx.expectFlagValue != "" {
		return completeFlagValue(ctx.expectFlagValue, ctx.current, ruleCodes)
	}

	if strings.HasPrefix(ctx.current, "-") {
		return completeFlags(ctx)
	}

	if ctx.cmd == "" {
		return filterPrefix(rptSubcommands, ctx.current)
	}

	switch ctx.cmd {
	case "completion":
		return filterPrefix([]string{"bash", "zsh", "fish"}, ctx.current)
	case "help":
		return filterPrefix(rptSubcommands, ctx.current)
	case "diff":
		// Up to 2 refs; offer refs + flags.
		if len(ctx.refs) >= 2 {
			return completeFlags(ctx)
		}
		refs := listRefs()
		return append(filterPrefix(refs, ctx.current), completeFlags(ctx)...)
	case "show":
		// Up to 1 ref; offer refs + flags.
		if len(ctx.refs) >= 1 {
			return completeFlags(ctx)
		}
		refs := listRefs()
		return append(filterPrefix(refs, ctx.current), completeFlags(ctx)...)
	case "check":
		// Positional args are file paths; just offer flags.
		return completeFlags(ctx)
	case "rules":
		return completeFlags(ctx)
	}
	return nil
}

func completeFlagValue(flag, prefix string, ruleCodes ruleCodesFunc) []string {
	if flag == "-rule" && ruleCodes != nil {
		codes := ruleCodes()
		return filterPrefix(codes, prefix)
	}
	return nil
}

func flagsForCmd(cmd string) []string {
	switch cmd {
	case "check":
		return []string{"--oneline", "--statistics", "--unknown", "-rule", "-config"}
	case "rules":
		return []string{"-config"}
	case "diff":
		return []string{"-rule", "-config", "-a", "--cached", "--staged"}
	case "show":
		return []string{"-rule", "-config"}
	}
	return nil
}

func completeFlags(ctx completionContext) []string {
	available := flagsForCmd(ctx.cmd)
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

// listRefs returns local branch and tag names from the current git repo.
func listRefs() []string {
	var refs []string
	if out, err := exec.Command("git", "branch", "--format=%(refname:short)").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				refs = append(refs, line)
			}
		}
	}
	if out, err := exec.Command("git", "tag").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				refs = append(refs, line)
			}
		}
	}
	return refs
}

// ruleCodesFromConfig loads rule codes from the nearest revparrot.toml.
// Returns nil if no config is found (not an error for completion purposes).
func ruleCodesFromConfig() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	cfg, _, err := rpconfig.Load(cwd)
	if err != nil {
		return nil
	}
	codes := make([]string, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		codes = append(codes, r.Code)
	}
	return codes
}

func runComplete(args []string) {
	ctx := parseCompletionContext(args)
	for _, c := range generateCompletions(ctx, ruleCodesFromConfig) {
		fmt.Println(c)
	}
}

func runCompletion(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Print(usageCompletion)
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: rpt completion bash|zsh|fish")
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

const usageCompletion = `rpt completion - print shell completion script

Usage:
  rpt completion bash|zsh|fish

The script is printed to stdout. Source it in your shell config
or save it to the appropriate completions directory.

Install:
  # Bash
  eval "$(rpt completion bash)"
  # or save permanently:
  rpt completion bash > /etc/bash_completion.d/rpt

  # Zsh
  eval "$(rpt completion zsh)"
  # or save permanently:
  rpt completion zsh > "${fpath[1]}/_rpt" && compinit

  # Fish
  rpt completion fish | source
  # or save permanently:
  rpt completion fish > ~/.config/fish/completions/rpt.fish
`

const bashCompletionScript = `# rpt bash completion
# Install: eval "$(rpt completion bash)"
# Or:      rpt completion bash > /etc/bash_completion.d/rpt

_rpt_completions() {
    COMP_WORDBREAKS="${COMP_WORDBREAKS//=/}"

    local words=()
    local i
    for ((i = 1; i <= COMP_CWORD; i++)); do
        words+=("${COMP_WORDS[$i]}")
    done

    local IFS=$'\n'
    local candidates
    candidates=($(rpt __complete "${words[@]}" 2>/dev/null))

    if [[ ${#candidates[@]} -eq 0 ]]; then
        COMPREPLY=()
        return
    fi

    COMPREPLY=($(compgen -W "${candidates[*]}" -- "${COMP_WORDS[$COMP_CWORD]}"))
}

complete -o default -F _rpt_completions rpt
`

const zshCompletionScript = `#compdef rpt
# rpt zsh completion
# Install: eval "$(rpt completion zsh)"
# Or:      rpt completion zsh > "${fpath[1]}/_rpt" && compinit

_rpt() {
    local -a candidates
    local IFS=$'\n'
    candidates=($(rpt __complete "${words[@]:1:$CURRENT}" 2>/dev/null))
    if (( ${#candidates[@]} == 0 )); then
        _files
        return
    fi
    compadd -a candidates
}

compdef _rpt rpt
`

const fishCompletionScript = `# rpt fish completion
# Install: rpt completion fish | source
# Or:      rpt completion fish > ~/.config/fish/completions/rpt.fish

complete -c rpt -f -a '(
    set -l tokens (commandline -opc)
    set -l current (commandline -ct)
    rpt __complete $tokens[2..] $current 2>/dev/null
)'
`
