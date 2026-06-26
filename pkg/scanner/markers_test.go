package scanner_test

import (
	"testing"

	"github.com/mattduck/diffyduck/pkg/scanner"
)

func TestScanFileMarkers_TODO(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.go", `package foo

// TODO: wire this up
func foo() {}
// FIXME handle the error
`)
	ms, err := scanner.ScanFileMarkers(path, scanner.DefaultMarkers())
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("expected 2 markers, got %d: %v", len(ms), ms)
	}
	if ms[0].Keyword != "TODO" || ms[0].Line != 3 || ms[0].Message != "wire this up" {
		t.Errorf("unexpected first marker: %+v", ms[0])
	}
	// "FIXME handle the error" — no colon, message is everything after the word.
	if ms[1].Keyword != "FIXME" || ms[1].Line != 5 || ms[1].Message != "handle the error" {
		t.Errorf("unexpected second marker: %+v", ms[1])
	}
}

func TestScanFileMarkers_ConventionalCommitForm(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.py", `# TODO feat(auth): assign this one
# FIXME
`)
	ms, err := scanner.ScanFileMarkers(path, scanner.DefaultMarkers())
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("expected 2 markers, got %d: %v", len(ms), ms)
	}
	if ms[0].Keyword != "TODO" || ms[0].Type != "feat" || ms[0].Scope != "auth" || ms[0].Message != "assign this one" {
		t.Errorf("unexpected TODO with type+scope: %+v", ms[0])
	}
	// Bare marker: no type, no scope, no message.
	if ms[1].Keyword != "FIXME" || ms[1].Scope != "" || ms[1].Message != "" {
		t.Errorf("unexpected bare FIXME: %+v", ms[1])
	}
}

func TestScanFileMarkers_WordBoundary(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.go", `package foo
// TODOLIST is not a marker
// FIXMES neither
// a trailing TODO is not at the start
`)
	ms, err := scanner.ScanFileMarkers(path, scanner.DefaultMarkers())
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 0 {
		t.Fatalf("expected 0 markers, got %d: %v", len(ms), ms)
	}
}

func TestScanFileMarkers_FilterByKeyword(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.go", `package foo
// TODO: one
// FIXME: two
`)
	ms, err := scanner.ScanFileMarkers(path, []scanner.Marker{{Keyword: "FIXME"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Keyword != "FIXME" {
		t.Fatalf("expected only FIXME, got %v", ms)
	}
}

func TestScanFileMarkers_RPTViaMarker(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.go", `package foo
// RPT fix(rule-a): fix this
// RPT malformed without type or parens
`)
	ms, err := scanner.ScanFileMarkers(path, []scanner.Marker{scanner.RPTMarker()})
	if err != nil {
		t.Fatal(err)
	}
	// Only the well-formed RPT type(scope): annotation matches.
	if len(ms) != 1 || ms[0].Scope != "rule-a" || ms[0].Type != "fix" || ms[0].Message != "fix this" {
		t.Fatalf("expected 1 RPT match, got %v", ms)
	}
}

func TestScanFileMarkers_CombinedFamiliesSuppression(t *testing.T) {
	dir := t.TempDir()
	// RPT keeps its NORPT suppression even when scanned alongside TODO; TODO
	// has no suppression keyword so it is always reported.
	path := writeFile(t, dir, "foo.py", `# NORPT(rule-a)
# RPT fix(rule-a): suppressed on the following line
y = {"a": 1}  # TODO: but this TODO stays
`)
	markers := append(scanner.DefaultMarkers(), scanner.RPTMarker())
	ms, err := scanner.ScanFileMarkers(path, markers)
	if err != nil {
		t.Fatal(err)
	}
	// The RPT on line 2 is not suppressed (NORPT targets line 3, the code
	// line), so we expect both the RPT and the TODO.
	if len(ms) != 2 {
		t.Fatalf("expected 2 markers (RPT + TODO), got %d: %v", len(ms), ms)
	}
}

func TestScanDirMarkers_Basic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.go", "package a\n// TODO: alpha\n")
	writeFile(t, dir, "b.py", "# FIXME: beta\n")
	writeFile(t, dir, "c.unknownext", "// TODO: ignored\n")

	ms, err := scanner.ScanDirMarkers(dir, scanner.DefaultMarkers(), scanner.WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 {
		t.Fatalf("expected 2 markers (unknown ext skipped), got %d: %v", len(ms), ms)
	}
}
