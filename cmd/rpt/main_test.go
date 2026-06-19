package main

import (
	"testing"

	"github.com/mattduck/diffyduck/pkg/scanner"
	"github.com/mattduck/diffyduck/pkg/ticketdb"
)

func TestFormatViolation(t *testing.T) {
	// File-attached (positive line) uses the standard file:line form.
	fileV := scanner.Violation{File: "pkg/foo.go", Line: 12, Code: "no-bare-dict", Message: "use a struct"}
	if got, want := formatViolation(fileV), "pkg/foo.go:12: REVP(no-bare-dict) use a struct"; got != want {
		t.Errorf("file-attached: got %q, want %q", got, want)
	}

	// Standalone (line 0) omits the line number.
	standaloneV := scanner.Violation{File: "(ticket 7a9)", Line: 0, Code: "design", Message: "revisit the API"}
	if got, want := formatViolation(standaloneV), "(ticket 7a9): REVP(design) revisit the API"; got != want {
		t.Errorf("standalone: got %q, want %q", got, want)
	}
}

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
