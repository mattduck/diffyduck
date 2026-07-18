// Command tdb (ticketdb) is the CLI over the git-state store: listing db
// entries and file comments (tdb list) and adding/editing db entries
// (add/edit/resolve/unresolve). It is a sibling frontend to dfd, built on the
// shared pkg/ticketcli and pkg/ticketdb packages.
//
// tdb is deliberately free of any tree-sitter dependency and builds with
// CGO_ENABLED=0; the verbose comment view renders code context as plain text.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/mattduck/diffyduck/pkg/config"
	"github.com/mattduck/diffyduck/pkg/ticketcli"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		ticketcli.PrintUsage(os.Stdout)
		return
	}

	switch args[0] {
	case "__complete":
		runComplete(args[1:])
		return
	case "completion":
		if err := runCompletion(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		return
	}

	// Missing config is fine — StylesFromConfig handles the zero value.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		os.Exit(2)
	}

	// Exit codes follow the grep/git convention: 0 = ok, 1 = the --exit-code
	// gate matched rows, 2 = an actual error.
	if err := ticketcli.Run(args, ticketcli.Options{
		Styles: ticketcli.StylesFromConfig(cfg.Theme),
	}); err != nil {
		if errors.Is(err, ticketcli.ErrExitCode) {
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
}
