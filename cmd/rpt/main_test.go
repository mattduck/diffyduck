package main

import (
	"testing"

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
