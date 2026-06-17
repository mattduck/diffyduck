package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mattduck/diffyduck/pkg/scanner"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanFile_Basic(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.py", `
x = 1
# REVP(bare-dict): use a dataclass here
y = {"a": 1}
z = 2
`)
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(vs), vs)
	}
	if vs[0].Code != "bare-dict" {
		t.Errorf("expected bare-dict, got %q", vs[0].Code)
	}
	if vs[0].Line != 3 {
		t.Errorf("expected line 3, got %d", vs[0].Line)
	}
}

func TestScanFile_NorevpInline(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.py", `
x = {"a": 1}  # NOREVP(bare-dict)
# REVP(bare-dict): should not appear because suppressed inline
`)
	// The REVP is on line 3, which has no inline suppression —
	// it should still appear. The inline NOREVP only suppresses line 2.
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation (line 3 not suppressed), got %d: %v", len(vs), vs)
	}
}

func TestScanFile_NorevpInlineSuppressesRevpOnSameLine(t *testing.T) {
	dir := t.TempDir()
	// REVP and NOREVP on the same line (unusual but valid).
	path := writeFile(t, dir, "foo.go", `package foo
// REVP(some-rule): message  // NOREVP(some-rule)
`)
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// The REVP and NOREVP are both on line 2; suppression should apply.
	if len(vs) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(vs), vs)
	}
}

func TestScanFile_NorevpBareInline(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.py", `
# REVP(rule-a): violation a  # NOREVP
`)
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 0 {
		t.Errorf("expected 0 violations (bare NOREVP suppresses all), got %d", len(vs))
	}
}

func TestScanFile_NorevpPrecedingLine(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.py", `
# NOREVP(bare-dict)
# REVP(bare-dict): this should be suppressed
y = {"a": 1}
`)
	// NOREVP on line 2 applies to next non-blank code line.
	// The REVP comment on line 3 is itself a comment line — the NOREVP stays
	// pending until the code line on line 4.
	// The REVP on line 3 is NOT a code line, so the suppression applies to line 4.
	// But the REVP annotation itself is on line 3 — it is not suppressed.
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// REVP on line 3: not suppressed (preceding NOREVP targets line 4, not line 3).
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(vs), vs)
	}
}

func TestScanFile_UnknownExtSkipped(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "file.unknownext", `# REVP(rule): message`)
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 0 {
		t.Errorf("expected 0 violations for unknown ext, got %d", len(vs))
	}
}

func TestScanFile_GoSlashSlash(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.go", `package foo

// REVP(some-rule): fix this
func foo() {}
`)
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Code != "some-rule" {
		t.Errorf("expected 1 violation with code some-rule, got %v", vs)
	}
}

func TestScanFile_MultipleViolations(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "foo.py", `
# REVP(rule-a): first
x = 1
# REVP(rule-b): second
y = 2
`)
	vs, err := scanner.ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(vs), vs)
	}
}

func TestScanDir_Basic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.py", "# REVP(r1): one\nx = 1\n")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "", filepath.Join(dir, "sub", "b.py"), "# REVP(r2): two\ny = 2\n")

	vs, err := scanner.ScanDir(dir, scanner.WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(vs), vs)
	}
}

func TestScanDir_KeepFileFilters(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep.py", "# REVP(r1): one\nx = 1\n")
	writeFile(t, dir, "skip.py", "# REVP(r2): two\ny = 2\n")

	vs, err := scanner.ScanDir(dir, scanner.WalkOptions{
		KeepFile: func(p string) bool { return filepath.Base(p) == "keep.py" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Code != "r1" {
		t.Fatalf("expected only r1 from keep.py, got %v", vs)
	}
}

func TestScanDir_KeepDirPrunes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "top.py", "# REVP(r1): one\nx = 1\n")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, "", filepath.Join(dir, ".git", "hook.py"), "# REVP(r2): two\ny = 2\n")

	vs, err := scanner.ScanDir(dir, scanner.WalkOptions{
		KeepDir: func(p string) bool { return filepath.Base(p) != ".git" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) != 1 || vs[0].Code != "r1" {
		t.Fatalf("expected only r1 (pruned .git), got %v", vs)
	}
}
