# diffyduck

git side-by-side terminal diff + log + review tool.

<video src="demo.mp4" autoplay loop muted playsinline width="100%"></video>

## Purpose and features

Diffyduck provides a git terminal diff/log view and some other miscellaneous
tools, with the goal of making it easier for me to read and review code
changes. I'm aiming for something faster to use than Github's PR interface, but
more advanced than a basic git pager like [delta](https://github.com/dandavison/delta) (I want it to feel like a pager,
but it's a TUI that triggers its own git commands).

Beyond a usual diff view, features include:

- Comments on diffs or commits, export comments. Comments stored in git state similar to git-notes
- Stats on changed classes/functions/symbols, parsed via tree-sitter
- Fold/expand inspired by org-mode
- Split windows to look at two different locations at once

## Install

No binaries yet. It's Go but we use tree-sitter, so you'll need both Go and a C compiler.

```sh
make bootstrap  # Pull some required tree-sitter code
make install  # Compile and install
```

## Usage

For the most part I'm trying to stick to git-replacement commands like `dfd
status`, `dfd diff`, `dfd log`, etc.

Run `dfd --help` to see usage and help output, which once looked like this:

```
$ dfd --help
dfd - terminal side-by-side diff viewer

Usage:
  dfd [flags] [refs] [-- paths]
  dfd <command> [flags] [args]

Commands:
  diff, d    Compare changes
  show       Show a commit
  log, l     Browse commit history
  clean      Delete persisted snapshots
  branch, b  Show branch dependency tree
  status, s  Show rich working tree status (default)
  comment, c List and edit comments
  config     Manage configuration
  completion Print shell completion script

Global flags:
  -h, --help       Show help
      --version    Show version

Use "dfd help <command>" for more about a command.
Press C-h inside dfd for keybinding help.
```

If you're in an interactive command, use `ctrl+h` to show the help view and see
keybindings.

## Status

Liable to change based on whatever I find useful, and has only been tested on my
own machine and preferred tools.

It does include a config feature where you can customise settings, theme and
keybindings, so somebody might find it useful as a starting point to customise
or fork.

## Code

99.9% generated, which was the only reason I could get time to do it.
