// Command tdb (ticketdb) is the CLI over the git-state ticket store: listing,
// adding, and editing comments and notes. It is a sibling frontend to dfd, built
// on the shared pkg/ticketcli and pkg/ticketdb packages.
//
// tdb is deliberately free of any tree-sitter dependency and builds with
// CGO_ENABLED=0; the verbose comment view renders code context as plain text.
package main

import (
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

	// Missing config is fine — StylesFromConfig handles the zero value.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: load config: %v\n", err)
		os.Exit(1)
	}

	if err := ticketcli.Run(args, ticketcli.Options{
		Styles: ticketcli.StylesFromConfig(cfg.Theme),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
