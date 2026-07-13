package main

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mattduck/diffyduck/pkg/scanner"
)

func TestColorFlagAliases(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want colorMode
	}{
		{name: "color", args: []string{"--color"}, want: colorAlways},
		{name: "colour", args: []string{"--colour"}, want: colorAlways},
		{name: "no color", args: []string{"--no-color"}, want: colorNever},
		{name: "no colour", args: []string{"--no-colour"}, want: colorNever},
		{name: "last flag wins enabled", args: []string{"--no-color", "--colour"}, want: colorAlways},
		{name: "last flag wins disabled", args: []string{"--color", "--no-colour"}, want: colorNever},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			got := colorAuto
			registerColorFlags(fs, &got)
			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got != tt.want {
				t.Fatalf("mode = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestViolationStylesRespectExplicitColorMode(t *testing.T) {
	v := scanner.Violation{
		File:    "src/example.go",
		Line:    12,
		Code:    "use-pathlib",
		Message: "use the preferred API",
	}

	colored := formatViolationOneline(v, "", defaultViolationStyles(colorAlways))
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("forced color output contains no ANSI escape: %q", colored)
	}

	plain := formatViolationOneline(v, "", defaultViolationStyles(colorNever))
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("disabled color output contains ANSI escape: %q", plain)
	}
}

func TestCheckCompletionsIncludeColorAliases(t *testing.T) {
	flags := flagsForCmd("check")
	for _, want := range []string{"--color", "--colour", "--no-color", "--no-colour"} {
		if !slices.Contains(flags, want) {
			t.Errorf("check completions missing %s", want)
		}
	}
}

func TestCheckCompletionsIncludeN(t *testing.T) {
	flags := flagsForCmd("check")
	if !slices.Contains(flags, "-n") {
		t.Error("check completions missing -n")
	}
}

func TestViolationSummary(t *testing.T) {
	tests := []struct {
		rendered int
		total    int
		want     string
	}{
		{rendered: 5, total: 5, want: "Found 5 problems."},
		{rendered: 1, total: 1, want: "Found 1 problem."},
		{rendered: 0, total: 0, want: "Found 0 problems."},
		{rendered: 5, total: 23, want: "Showing 5 of 23 problems."},
		{rendered: 1, total: 10, want: "Showing 1 of 10 problems."},
		{rendered: 0, total: 5, want: "Showing 0 of 5 problems."},
	}
	for _, tt := range tests {
		got := violationSummary(tt.rendered, tt.total)
		if got != tt.want {
			t.Errorf("violationSummary(%d, %d) = %q, want %q", tt.rendered, tt.total, got, tt.want)
		}
	}
}

func TestCheckSelectRejectsNonBuiltin(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "revparrot.toml")
	if err := os.WriteFile(cfgPath, []byte("[[rules]]\ncode = \"use-x\"\ntype = \"refactor\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// check validates only built-in check codes; a config rule code is rejected.
	if got := cmdCheck([]string{"-config", cfgPath, "-select", "use-x", dir}); got != 2 {
		t.Errorf("cmdCheck(-select use-x) = %d, want 2", got)
	}
	// A real built-in check code is accepted; an empty tree yields no problems.
	if got := cmdCheck([]string{"-config", cfgPath, "-select", "rpt-syntax", dir}); got != 0 {
		t.Errorf("cmdCheck(-select rpt-syntax) = %d, want 0", got)
	}
}

func TestNFlagTakesValue(t *testing.T) {
	if !flagTakesValue("-n") {
		t.Error("-n should be recognised as a value-taking flag for completion purposes")
	}
}

func TestCheckRejectsNegativeN(t *testing.T) {
	if got := cmdCheck([]string{"-n", "-1"}); got != 2 {
		t.Errorf("cmdCheck(-n -1) = %d, want 2", got)
	}
}
