package main

import (
	"flag"
	"slices"
	"strings"
	"testing"

	"github.com/mattduck/diffyduck/pkg/scanner"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
)

func TestStateMessage(t *testing.T) {
	// Title wins when present.
	if got := stateMessage(&ticketdb.Comment{Title: "the title", Text: "the body"}); got != "the title" {
		t.Errorf("title: got %q", got)
	}
	// Otherwise the first line of the body.
	if got := stateMessage(&ticketdb.Comment{Text: "first line\nsecond line"}); got != "first line" {
		t.Errorf("body first line: got %q", got)
	}
}

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
