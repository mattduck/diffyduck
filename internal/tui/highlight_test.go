package tui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
	"github.com/user/diffyduck/pkg/structure"
)

func TestSyntaxHighlighting_SpansGenerated(t *testing.T) {
	// Create a model with a Go file
	files := []sidebyside.FilePair{
		{
			OldPath: "a/test.go",
			NewPath: "b/test.go",
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			FoldLevel:  sidebyside.FoldHunks,
			OldContent: []string{"package main", "", "func hello() {", "\tfmt.Println(\"Hello\")", "}"},
			NewContent: []string{"package main", "", "func hello() {", "\tfmt.Println(\"Hello\")", "}"},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Manually trigger highlighting (normally done via message)
	cmd := m.RequestHighlight(0)
	msg := cmd()

	// Should get a HighlightReadyMsg
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	// Should have spans for both old and new content
	if len(hlMsg.OldSpans) == 0 {
		t.Error("Expected OldSpans to have entries")
	}
	if len(hlMsg.NewSpans) == 0 {
		t.Error("Expected NewSpans to have entries")
	}

	t.Logf("Got %d old spans, %d new spans", len(hlMsg.OldSpans), len(hlMsg.NewSpans))

	// Check that we have keyword spans (for "package", "func")
	hasKeyword := false
	for _, s := range hlMsg.OldSpans {
		// CategoryKeyword = 1
		if s.Category == 1 {
			hasKeyword = true
			break
		}
	}
	if !hasKeyword {
		t.Error("Expected to find keyword category in spans")
	}
}

func TestSyntaxHighlighting_GetLineSpans(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldHunks,
			OldContent: []string{"package main", "", "func hello() {"},
			NewContent: []string{"package main", "", "func hello() {"},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Trigger and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Get spans for line 1 ("package main")
	spans := m.getLineSpans(0, 1, false) // fileIndex=0, lineNum=1, isOld=false
	if len(spans) == 0 {
		t.Fatal("Expected spans for line 1")
	}

	t.Logf("Line 1 spans: %+v", spans)

	// "package" should be highlighted as keyword (bytes 0-7)
	foundPackage := false
	for _, s := range spans {
		if s.Start == 0 && s.End == 7 {
			foundPackage = true
			t.Logf("Found 'package' span: category=%d", s.Category)
		}
	}
	if !foundPackage {
		t.Error("Expected to find span for 'package' keyword at bytes 0-7")
	}

	// Get spans for line 3 ("func hello() {")
	spans3 := m.getLineSpans(0, 3, false)
	if len(spans3) == 0 {
		t.Fatal("Expected spans for line 3")
	}
	t.Logf("Line 3 spans: %+v", spans3)
}

func TestSyntaxHighlighting_RenderedOutput(t *testing.T) {
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			OldContent: []string{"package main"},
			NewContent: []string{"package main"},
		},
	}

	m := New(files)
	m.width = 120
	m.height = 24
	defer m.highlighter.Close()

	// Trigger and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Debug: check if spans are stored
	t.Logf("highlightSpans map has %d entries", len(m.highlightSpans))
	if fh, ok := m.highlightSpans[0]; ok {
		t.Logf("File 0: %d old spans, %d new spans", len(fh.OldSpans), len(fh.NewSpans))
	} else {
		t.Error("No highlight spans stored for file 0")
	}

	// Debug: check what getLineSpans returns during a simulated render
	spans := m.getLineSpans(0, 1, false)
	t.Logf("getLineSpans(0, 1, false) returned %d spans", len(spans))

	// Debug: check that HasContent returns true
	t.Logf("HasContent: %v", m.files[0].HasContent())

	// Debug: check what buildRows returns
	rows := m.buildRows()
	t.Logf("buildRows returned %d rows", len(rows))
	for i, row := range rows {
		if i > 3 {
			break
		}
		t.Logf("Row %d: isHeader=%v isSeparator=%v isBlank=%v fileIndex=%d old.Num=%d new.Num=%d",
			i, row.isHeader, row.isSeparator, row.isBlank, row.fileIndex, row.pair.Old.Num, row.pair.New.Num)
	}

	// Test applySyntaxHighlight directly to verify it works
	// (lipgloss strips ANSI codes in test environment without TTY)
	theme := m.highlighter.Theme()

	// Get style for keyword and render test text
	keywordStyle := theme.Style(spans[0].Category)
	styledKeyword := keywordStyle.Render("package")
	t.Logf("Keyword style render result: %q", styledKeyword)

	// Even without TTY, we can verify the code path works
	// by checking that applySyntaxHighlight produces output
	result := m.applySyntaxHighlight("package main", "package main", "package main", spans, false, false, 0, 0, 0)
	t.Logf("applySyntaxHighlight result: %q", result)

	// The result should contain "package main" (even if no ANSI codes due to no TTY)
	if !strings.Contains(result, "package") || !strings.Contains(result, "main") {
		t.Errorf("applySyntaxHighlight returned unexpected result: %q", result)
	}

	// Verify spans have correct categories (s.Category is already highlight.Category)
	for _, s := range spans {
		t.Logf("Span: start=%d end=%d category=%s", s.Start, s.End, s.Category.String())
	}

	// Verify we got the expected categories
	if len(spans) >= 1 && spans[0].Category != highlight.CategoryKeyword {
		t.Errorf("Expected first span to be Keyword, got %s", spans[0].Category.String())
	}
}

func TestSyntaxHighlighting_FullFlow(t *testing.T) {
	// Simulate the complete flow: content loads -> highlighting triggered -> render
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
				},
			},
			// Content will be "loaded" via message
		},
	}

	m := New(files)
	m.width = 120
	m.height = 24
	defer m.highlighter.Close()

	// Simulate FileContentLoadedMsg (normally sent when content fetched)
	contentMsg := FileContentLoadedMsg{
		FileIndex:  0,
		OldContent: []string{"func hello() {", "\treturn", "}"},
		NewContent: []string{"func hello() {", "\treturn", "}"},
	}

	// Process the content loaded message
	newModel, cmd := m.Update(contentMsg)
	m = newModel.(Model)

	// The cmd should be a RequestHighlight command
	if cmd == nil {
		t.Fatal("Expected RequestHighlight command after content loaded")
	}

	// Execute the highlight command
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	// Process the highlight ready message
	newModel, _ = m.Update(hlMsg)
	m = newModel.(Model)

	// Verify spans are stored
	if len(m.highlightSpans) == 0 {
		t.Fatal("Expected highlight spans to be stored")
	}

	// Verify we can get line spans
	spans := m.getLineSpans(0, 1, false)
	if len(spans) == 0 {
		t.Fatal("Expected spans for line 1")
	}

	t.Logf("Full flow test: got %d spans for line 1", len(spans))
	for _, s := range spans {
		t.Logf("  Span: start=%d end=%d category=%s", s.Start, s.End, s.Category.String())
	}

	// Verify "func" is highlighted as keyword
	foundFunc := false
	for _, s := range spans {
		if s.Start == 0 && s.End == 4 && s.Category == highlight.CategoryKeyword {
			foundFunc = true
			break
		}
	}
	if !foundFunc {
		t.Error("Expected 'func' keyword span at bytes 0-4")
	}
}

func TestSyntaxHighlighting_NoSpansWithoutFullContent(t *testing.T) {
	// Without full content loaded, getLineSpans returns nil (no pairs-based fallback)
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			// OldContent and NewContent are nil (not loaded)
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// No spans until full content is loaded
	spans := m.getLineSpans(0, 1, false)
	if spans != nil {
		t.Error("Expected nil spans when full content is not loaded")
	}
}

func TestSyntaxHighlighting_FullContentRequestNoSpans(t *testing.T) {
	// RequestHighlight (for full content) should return nil when content is not loaded
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			// OldContent and NewContent are nil (not loaded)
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// RequestHighlight (for full content) should produce no spans when content not loaded
	cmd := m.RequestHighlight(0)
	msg := cmd()

	if msg != nil {
		hlMsg, ok := msg.(HighlightReadyMsg)
		if ok && (len(hlMsg.OldSpans) > 0 || len(hlMsg.NewSpans) > 0) {
			t.Error("Expected no spans from full-content highlighting when content is not loaded")
		}
	}
}

func TestHighlightFileSync(t *testing.T) {
	// Test that highlightFileSync highlights from full content
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			OldContent: []string{"package main", "// comment"},
			NewContent: []string{"package main", "// comment"},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// No spans initially (pairs-based highlighting removed)
	if m.highlightSpans[0] != nil {
		t.Error("Expected no highlight spans initially")
	}

	// Highlight synchronously from full content
	m.highlightFileSync(0)

	if m.highlightSpans[0] == nil {
		t.Fatal("Expected full content highlight spans after highlightFileSync")
	}

	// Line 1: "package" should be keyword
	spans1 := m.getLineSpans(0, 1, false)
	if len(spans1) == 0 {
		t.Fatal("Expected spans for line 1")
	}
	foundPackage := false
	for _, s := range spans1 {
		if s.Start == 0 && s.End == 7 && s.Category == highlight.CategoryKeyword {
			foundPackage = true
			break
		}
	}
	if !foundPackage {
		t.Error("Expected 'package' keyword span for line 1")
	}

	// Line 2: "// comment" should be a comment
	spans2 := m.getLineSpans(0, 2, false)
	if len(spans2) == 0 {
		t.Fatal("Expected spans for line 2")
	}
	foundComment := false
	for _, s := range spans2 {
		if s.Category == highlight.CategoryComment {
			foundComment = true
			break
		}
	}
	if !foundComment {
		t.Error("Expected comment category for line 2")
	}
}

func TestGetLineSpans_NoSpansWithoutContent(t *testing.T) {
	// Without full content loaded, getLineSpans returns nil
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldHunks,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	spans := m.getLineSpans(0, 1, false)
	if spans != nil {
		t.Error("Expected nil spans when full content is not loaded")
	}
}

func TestStructureExtraction_ViewGoFile(t *testing.T) {
	// Test structure extraction on actual view.go content
	// This tests that both types and methods are extracted correctly
	viewGoContent := `package tui

import "strings"

var headerStyle = lipgloss.NewStyle()

// View implements tea.Model.
func (m Model) View() string {
	return ""
}

// displayRow represents one row in the view
type displayRow struct {
	fileIndex int
	isHeader  bool
}

// buildRows creates all displayable rows from the model data.
func (m Model) buildRows() []displayRow {
	var rows []displayRow
	return rows
}

func helperFunction() {
	// standalone function
}
`
	lines := strings.Split(viewGoContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/view.go",
			NewPath:    "b/view.go",
			FoldLevel:  sidebyside.FoldHunks,
			NewContent: lines,
			OldContent: lines,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package tui", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package tui", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	cmd := m.RequestHighlight(0)
	if cmd == nil {
		t.Fatal("RequestHighlight returned nil")
	}

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	if len(hlMsg.NewStructure) == 0 {
		t.Fatal("No structure entries extracted")
	}

	m.storeHighlightSpans(hlMsg)

	// Log all extracted entries
	t.Log("Extracted structure entries:")
	for _, e := range m.structureMaps[0].NewStructure.Entries {
		t.Logf("  %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
	}

	// Verify we found the type
	entries8 := m.getStructureAtLine(0, 8)
	t.Logf("Structure at line 8 (inside View method): %+v", entries8)

	entries14 := m.getStructureAtLine(0, 14)
	t.Logf("Structure at line 14 (inside displayRow): %+v", entries14)

	entries20 := m.getStructureAtLine(0, 20)
	t.Logf("Structure at line 20 (inside buildRows): %+v", entries20)

	entries25 := m.getStructureAtLine(0, 25)
	t.Logf("Structure at line 25 (inside helperFunction): %+v", entries25)

	// Verify specific entries exist
	if len(entries8) == 0 {
		t.Error("Expected to find structure at line 8 (View method)")
	}
	if len(entries14) == 0 {
		t.Error("Expected to find structure at line 14 (displayRow type)")
	}
	if len(entries20) == 0 {
		t.Error("Expected to find structure at line 20 (buildRows method)")
	}
	if len(entries25) == 0 {
		t.Error("Expected to find structure at line 25 (helperFunction)")
	}
}

func TestStructureExtraction_LineTruncationBreaksSyntax(t *testing.T) {
	// Test that line truncation (adding [...truncated]) breaks tree-sitter parsing.
	// When a line is truncated mid-syntax, tree-sitter can't determine function boundaries.
	//
	// This simulates view.go where line 187 is a long rows = append(...) that gets
	// truncated, breaking the syntax inside buildRows function.
	contentWithTruncatedLine := `package main

type MyType struct {
	field int
}

func CompleteBeforeTruncation() {
	x := 1
}

func FunctionWithTruncatedLine() {
	// This line simulates a long line that gets truncated mid-syntax
	rows = append(rows, SomeStruct{field1: val1, field2: val2, field3[...truncated]
	y := 2
}

func FunctionAfterTruncation() {
	z := 3
}
`
	lines := strings.Split(contentWithTruncatedLine, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldHunks,
			NewContent: lines,
			OldContent: lines,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Log what was extracted
	t.Log("Extracted entries (may be incomplete due to syntax errors):")
	if m.structureMaps[0] != nil && m.structureMaps[0].NewStructure != nil {
		for _, e := range m.structureMaps[0].NewStructure.Entries {
			t.Logf("  %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
		}
	}

	// Check what was found
	entriesType := m.getStructureAtLine(0, 4)
	entriesComplete := m.getStructureAtLine(0, 8)
	entriesTruncated := m.getStructureAtLine(0, 13)
	entriesAfter := m.getStructureAtLine(0, 19)

	t.Logf("Line 4 (MyType): %+v", entriesType)
	t.Logf("Line 8 (CompleteBeforeTruncation): %+v", entriesComplete)
	t.Logf("Line 13 (FunctionWithTruncatedLine): %+v", entriesTruncated)
	t.Logf("Line 19 (FunctionAfterTruncation): %+v", entriesAfter)

	// MyType should be found (before any truncation)
	if len(entriesType) == 0 {
		t.Error("Expected to find MyType at line 4")
	}

	// CompleteBeforeTruncation should be found
	if len(entriesComplete) == 0 {
		t.Error("Expected to find CompleteBeforeTruncation at line 8")
	}

	// These may or may not be found depending on tree-sitter error recovery
	// The point is to see how truncation affects parsing
	if len(entriesTruncated) == 0 {
		t.Log("FunctionWithTruncatedLine NOT found - truncation broke parsing")
	}
	if len(entriesAfter) == 0 {
		t.Log("FunctionAfterTruncation NOT found - truncation broke parsing of subsequent functions")
	}
}

func TestStructureExtraction_TruncatedFile(t *testing.T) {
	// Simulate a file that was truncated due to size limits.
	// Tree-sitter should still parse the available content and extract
	// structure for complete functions, even if the file ends mid-syntax.
	//
	// This simulates a file truncated at line 12 - the second function
	// is incomplete (no closing brace), but the first function should
	// still have valid structure extracted.
	truncatedContent := []string{
		"package main",
		"",
		"func CompleteFunction() {",
		"	x := 1",
		"	y := 2",
		"}",
		"",
		"func IncompleteFunction() {",
		"	a := 1",
		"	b := 2",
		"	// File truncated here - no closing brace",
	}

	files := []sidebyside.FilePair{
		{
			OldPath:             "a/large.go",
			NewPath:             "b/large.go",
			FoldLevel:           sidebyside.FoldHunks,
			NewContent:          truncatedContent,
			OldContent:          truncatedContent,
			NewContentTruncated: true, // Marked as truncated
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 4, Content: "	x := 1", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Trigger full content highlighting (which extracts structure)
	cmd := m.RequestHighlight(0)
	if cmd == nil {
		t.Fatal("RequestHighlight returned nil command")
	}

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	// Structure should be extracted despite truncation
	if len(hlMsg.NewStructure) == 0 {
		t.Fatal("Expected structure entries to be extracted from truncated content")
	}

	// Store the structure
	m.storeHighlightSpans(hlMsg)

	// Verify structureMaps was populated
	if m.structureMaps[0] == nil || m.structureMaps[0].NewStructure == nil {
		t.Fatal("structureMaps should be populated for truncated file")
	}

	// Look up structure at line 4 (inside CompleteFunction)
	entries := m.getStructureAtLine(0, 4)
	if len(entries) == 0 {
		t.Fatal("Should find structure entries at line 4 (inside CompleteFunction)")
	}

	// Verify we found CompleteFunction
	found := false
	for _, e := range entries {
		if e.Name == "CompleteFunction" && e.Kind == "func" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected to find CompleteFunction at line 4, got: %+v", entries)
	}

	// Line 9 should be inside IncompleteFunction (even though it's truncated)
	entries9 := m.getStructureAtLine(0, 9)
	if len(entries9) == 0 {
		t.Log("Note: IncompleteFunction not found at line 9 - tree-sitter may not have parsed incomplete function")
	} else {
		for _, e := range entries9 {
			t.Logf("Found at line 9: %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
		}
	}
}

func TestStructureExtraction_PythonFile(t *testing.T) {
	// Test structure extraction for Python files
	pythonContent := `import os

class MyClass:
    def __init__(self):
        self.value = 0

    def method(self):
        return self.value

def standalone_function():
    return 42

@decorator
def decorated_function():
    pass
`
	lines := strings.Split(pythonContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.py",
			NewPath:    "b/test.py",
			FoldLevel:  sidebyside.FoldHunks,
			NewContent: lines,
			OldContent: lines,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "import os", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "import os", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	cmd := m.RequestHighlight(0)
	if cmd == nil {
		t.Fatal("RequestHighlight returned nil")
	}

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	if len(hlMsg.NewStructure) == 0 {
		t.Fatal("No structure entries extracted from Python file")
	}

	m.storeHighlightSpans(hlMsg)

	// Log all extracted entries
	t.Log("Extracted Python structure entries:")
	for _, e := range m.structureMaps[0].NewStructure.Entries {
		t.Logf("  %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
	}

	// Verify we found the class
	entries4 := m.getStructureAtLine(0, 4)
	t.Logf("Structure at line 4 (inside MyClass.__init__): %+v", entries4)
	if len(entries4) == 0 {
		t.Error("Expected to find structure at line 4 (inside MyClass)")
	}

	// Verify we found standalone_function
	entries11 := m.getStructureAtLine(0, 11)
	t.Logf("Structure at line 11 (inside standalone_function): %+v", entries11)
	if len(entries11) == 0 {
		t.Error("Expected to find structure at line 11 (standalone_function)")
	}

	// Verify we found decorated_function
	entries15 := m.getStructureAtLine(0, 15)
	t.Logf("Structure at line 15 (inside decorated_function): %+v", entries15)
	if len(entries15) == 0 {
		t.Error("Expected to find structure at line 15 (decorated_function)")
	}

	// Verify the kinds are correct
	foundClass := false
	foundDef := false
	for _, e := range m.structureMaps[0].NewStructure.Entries {
		if e.Kind == "class" && e.Name == "MyClass" {
			foundClass = true
		}
		if e.Kind == "def" && e.Name == "standalone_function" {
			foundDef = true
		}
	}
	if !foundClass {
		t.Error("Expected to find 'class MyClass'")
	}
	if !foundDef {
		t.Error("Expected to find 'def standalone_function'")
	}
}

func TestStructureExtraction_PythonDecoratedClassWithMethods(t *testing.T) {
	// Test that methods inside decorated classes are detected
	pythonContent := `@dataclass
class MyDataClass:
    value: int = 0

    def get_value(self):
        return self.value

    @property
    def doubled(self):
        return self.value * 2
`
	lines := strings.Split(pythonContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.py",
			NewPath:    "b/test.py",
			FoldLevel:  sidebyside.FoldHunks,
			NewContent: lines,
			OldContent: lines,
			Pairs: []sidebyside.LinePair{
				{
					Old: sidebyside.Line{Num: 1, Content: "@dataclass", Type: sidebyside.Context},
					New: sidebyside.Line{Num: 1, Content: "@dataclass", Type: sidebyside.Context},
				},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	cmd := m.RequestHighlight(0)
	if cmd == nil {
		t.Fatal("RequestHighlight returned nil")
	}

	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	if len(hlMsg.NewStructure) == 0 {
		t.Fatal("No structure entries extracted from Python file")
	}

	m.storeHighlightSpans(hlMsg)

	// Log all extracted entries
	t.Log("Extracted Python structure entries:")
	for _, e := range m.structureMaps[0].NewStructure.Entries {
		t.Logf("  %s %s (lines %d-%d)", e.Kind, e.Name, e.StartLine, e.EndLine)
	}

	// Verify we found the decorated class
	foundClass := false
	foundGetValue := false
	foundDoubled := false
	for _, e := range m.structureMaps[0].NewStructure.Entries {
		if e.Kind == "class" && e.Name == "MyDataClass" {
			foundClass = true
		}
		if e.Kind == "def" && e.Name == "get_value" {
			foundGetValue = true
		}
		if e.Kind == "def" && e.Name == "doubled" {
			foundDoubled = true
		}
	}

	if !foundClass {
		t.Error("Expected to find 'class MyDataClass' (decorated class)")
	}
	if !foundGetValue {
		t.Error("Expected to find 'def get_value' (method inside decorated class)")
	}
	if !foundDoubled {
		t.Error("Expected to find 'def doubled' (decorated method inside decorated class)")
	}
}

func TestStructuralDiff_ComputedOnHighlightReady(t *testing.T) {
	// Test that structural diff is computed when storing highlight spans.
	// Old file has: FuncA, FuncB (will be deleted), TypeX
	// New file has: FuncA (modified), TypeX, FuncC (added)
	oldContent := `package main

func FuncA() {
	x := 1
}

func FuncB() {
	y := 2
}

type TypeX struct {
	field int
}
`
	newContent := `package main

func FuncA() {
	x := 1
	z := 3
}

type TypeX struct {
	field int
}

func FuncC() {
	w := 4
}
`
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Build Pairs that show the diff:
	// - FuncA: line 5 added (z := 3)
	// - FuncB: lines 7-9 removed
	// - FuncC: lines 12-14 added
	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldHunks,
			OldContent: oldLines,
			NewContent: newLines,
			Pairs: []sidebyside.LinePair{
				// Context lines (package, func FuncA, etc.)
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 2, Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				// Added line in FuncA
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 5, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 6, Type: sidebyside.Context}, New: sidebyside.Line{Num: 7, Type: sidebyside.Context}},
				// Removed FuncB
				{Old: sidebyside.Line{Num: 7, Type: sidebyside.Removed}, New: sidebyside.Line{Num: 0, Type: sidebyside.Empty}},
				{Old: sidebyside.Line{Num: 8, Type: sidebyside.Removed}, New: sidebyside.Line{Num: 0, Type: sidebyside.Empty}},
				{Old: sidebyside.Line{Num: 9, Type: sidebyside.Removed}, New: sidebyside.Line{Num: 0, Type: sidebyside.Empty}},
				{Old: sidebyside.Line{Num: 10, Type: sidebyside.Context}, New: sidebyside.Line{Num: 8, Type: sidebyside.Context}},
				// TypeX context
				{Old: sidebyside.Line{Num: 11, Type: sidebyside.Context}, New: sidebyside.Line{Num: 9, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 12, Type: sidebyside.Context}, New: sidebyside.Line{Num: 10, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 13, Type: sidebyside.Context}, New: sidebyside.Line{Num: 11, Type: sidebyside.Context}},
				// Added FuncC
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 12, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 13, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 14, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 15, Type: sidebyside.Added}},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Request and store highlighting (which computes structural diff)
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	if !ok {
		t.Fatalf("Expected HighlightReadyMsg, got %T", msg)
	}

	m.storeHighlightSpans(hlMsg)

	// Verify structural diff was computed
	fs := m.structureMaps[0]
	if fs == nil {
		t.Fatal("FileStructure not stored")
	}
	if fs.StructuralDiff == nil {
		t.Fatal("StructuralDiff not computed")
	}

	diff := fs.StructuralDiff
	t.Logf("Structural diff has %d changes", len(diff.Changes))
	for _, c := range diff.Changes {
		t.Logf("  %s %s", c.Kind.Symbol(), c.Name())
	}

	// Find changes by name
	byName := make(map[string]*structure.ElementChange)
	for i := range diff.Changes {
		c := &diff.Changes[i]
		byName[c.Name()] = c
	}

	// Verify FuncA is modified (has added line 5)
	if c, ok := byName["FuncA"]; !ok {
		t.Error("Expected FuncA in diff")
	} else if c.Kind != structure.ChangeModified {
		t.Errorf("Expected FuncA to be modified, got %s", c.Kind)
	}

	// Verify FuncB is deleted
	if c, ok := byName["FuncB"]; !ok {
		t.Error("Expected FuncB in diff")
	} else if c.Kind != structure.ChangeDeleted {
		t.Errorf("Expected FuncB to be deleted, got %s", c.Kind)
	}

	// Verify TypeX is unchanged
	if c, ok := byName["TypeX"]; !ok {
		t.Error("Expected TypeX in diff")
	} else if c.Kind != structure.ChangeUnchanged {
		t.Errorf("Expected TypeX to be unchanged, got %s", c.Kind)
	}

	// Verify FuncC is added
	if c, ok := byName["FuncC"]; !ok {
		t.Error("Expected FuncC in diff")
	} else if c.Kind != structure.ChangeAdded {
		t.Errorf("Expected FuncC to be added, got %s", c.Kind)
	}

	// Verify HasChanges and ChangedOnly
	if !diff.HasChanges() {
		t.Error("Expected HasChanges() to be true")
	}
	changed := diff.ChangedOnly()
	if len(changed) != 3 { // FuncA modified, FuncB deleted, FuncC added
		t.Errorf("Expected 3 changed elements, got %d", len(changed))
	}
}

func TestStructuralDiff_RenderedInView(t *testing.T) {
	// Test that structural diff rows appear in buildRows when file is folded
	// (structural diff rows are a preview shown only in folded state)
	oldContent := `package main

func FuncA() {
	x := 1
}
`
	newContent := `package main

func FuncA() {
	x := 1
	z := 3
}

func FuncB() {
	y := 2
}
`
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldStructure, // Structural diff rows appear in Normal (structural diff) view
			OldContent: oldLines,
			NewContent: newLines,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 2, Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				// Added line in FuncA
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 5, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Type: sidebyside.Context}},
				// Added FuncB
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 7, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 8, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 9, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 10, Type: sidebyside.Added}},
			},
		},
	}

	m := New(files)
	defer m.highlighter.Close()
	m.width = 120
	m.height = 40

	// Request and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg := msg.(HighlightReadyMsg)
	m.storeHighlightSpans(hlMsg)

	// Build rows and check for structural diff rows
	rows := m.buildRows()

	var structDiffRows []displayRow
	for _, row := range rows {
		if row.isStructuralDiff {
			structDiffRows = append(structDiffRows, row)
		}
	}

	t.Logf("Found %d structural diff rows", len(structDiffRows))
	for _, row := range structDiffRows {
		t.Logf("  %q (blank=%v)", row.structuralDiffLine, row.structuralDiffIsBlank)
	}

	// Should have structural diff rows (FuncA modified, FuncB added, plus blank)
	if len(structDiffRows) == 0 {
		t.Error("Expected structural diff rows to be present")
	}

	// Verify the rows have correct fileIndex
	for _, row := range structDiffRows {
		if row.fileIndex != 0 {
			t.Errorf("Expected fileIndex 0, got %d", row.fileIndex)
		}
	}

	// Verify we can find expected change kinds and names in the lines
	foundModified := false
	foundAdded := false
	for _, row := range structDiffRows {
		if row.structuralDiffChangeKind == structure.ChangeModified && strings.Contains(row.structuralDiffLine, "FuncA") {
			foundModified = true
		}
		if row.structuralDiffChangeKind == structure.ChangeAdded && strings.Contains(row.structuralDiffLine, "FuncB") {
			foundAdded = true
		}
	}

	if !foundModified {
		t.Error("Expected to find modified FuncA in structural diff rows")
	}
	if !foundAdded {
		t.Error("Expected to find added FuncB in structural diff rows")
	}
}

func TestStoreHighlightSpans_UpdatesTotalLinesWhenCommitUnfolded(t *testing.T) {
	// Test that totalLines is updated when structural diff is stored
	// and the commit is not folded. Structural diff rows appear as a preview
	// under folded files, so the file must be FoldHeader for rows to appear.
	oldContent := `package main

func FuncA() {
	x := 1
}
`
	newContent := `package main

func FuncA() {
	x := 1
	y := 2
}
`
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldStructure, // Structural diff rows appear in Normal (structural diff) view
			OldContent: oldLines,
			NewContent: newLines,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 5, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Type: sidebyside.Context}},
			},
		},
	}

	// Create model with a commit that is NOT folded (files visible but folded)
	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123"},
		Files:       files,
		FoldLevel:   sidebyside.CommitFileHeaders, // Not folded - files are visible
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	defer m.highlighter.Close()

	// Build rows to populate cache
	m.rebuildRowsCache()
	totalLinesBefore := m.w().totalLines

	// Request and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "expected HighlightReadyMsg")

	m.storeHighlightSpans(hlMsg)

	// totalLines should increase to include structural diff rows
	// because file is folded and structural diff has changes
	assert.Greater(t, m.w().totalLines, totalLinesBefore,
		"totalLines should increase when structural diff is stored for folded file")
}

func TestStoreHighlightSpans_DoesNotInvalidateCacheWhenCommitFolded(t *testing.T) {
	// Test that row cache is NOT invalidated when structural diff is stored
	// but the commit is folded (structural diff rows wouldn't be visible anyway).
	oldContent := `package main

func FuncA() {
	x := 1
}
`
	newContent := `package main

func FuncA() {
	x := 1
	y := 2
}
`
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldHunks,
			OldContent: oldLines,
			NewContent: newLines,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 5, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Type: sidebyside.Context}},
			},
		},
	}

	// Create model with a commit that IS folded
	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123"},
		Files:       files,
		FoldLevel:   sidebyside.CommitFolded, // Folded - file headers not visible
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	defer m.highlighter.Close()

	// Build rows to populate cache
	m.rebuildRowsCache()
	assert.True(t, m.w().rowsCacheValid, "cache should be valid after rebuild")

	// Request and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "expected HighlightReadyMsg")

	m.storeHighlightSpans(hlMsg)

	// Cache should NOT be invalidated because commit is folded
	assert.True(t, m.w().rowsCacheValid, "cache should remain valid when commit is folded")
}

func TestStoreHighlightSpans_DoesNotInvalidateCacheWhenNoStructuralChanges(t *testing.T) {
	// Test that row cache is NOT invalidated when there are no structural changes.
	// Use identical old/new content with only context lines (no adds/removes).
	content := `package main

func FuncA() {
	x := 1
}
`
	lines := strings.Split(content, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldHunks,
			OldContent: lines,
			NewContent: lines,
			// All context lines - no actual changes
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 2, Type: sidebyside.Context}, New: sidebyside.Line{Num: 2, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 5, Type: sidebyside.Context}},
			},
		},
	}

	commit := sidebyside.CommitSet{
		Info:        sidebyside.CommitInfo{SHA: "abc123"},
		Files:       files,
		FoldLevel:   sidebyside.CommitFileHeaders,
		FilesLoaded: true,
	}
	m := NewWithCommits([]sidebyside.CommitSet{commit})
	defer m.highlighter.Close()

	// Build rows to populate cache
	m.rebuildRowsCache()
	assert.True(t, m.w().rowsCacheValid, "cache should be valid after rebuild")

	// Request and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "expected HighlightReadyMsg")

	m.storeHighlightSpans(hlMsg)

	// Structural diff should have no changes since old == new
	fs := m.structureMaps[0]
	require.NotNil(t, fs)
	if fs.StructuralDiff != nil {
		assert.False(t, fs.StructuralDiff.HasChanges(), "structural diff should have no changes")
	}

	// Cache should remain valid when no structural changes
	assert.True(t, m.w().rowsCacheValid, "cache should remain valid when no structural changes")
}

func TestStoreHighlightSpans_InvalidatesCacheInDiffMode(t *testing.T) {
	// Test that row cache is invalidated in diff mode (no commit structure).
	// This uses New() instead of NewWithCommits() to simulate diff command.
	// Structural diff rows only appear when a file is folded (preview mode).
	oldContent := `package main

func FuncA() {
	x := 1
}
`
	newContent := `package main

func FuncA() {
	x := 1
	y := 2
}
`
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	files := []sidebyside.FilePair{
		{
			OldPath:    "a/test.go",
			NewPath:    "b/test.go",
			FoldLevel:  sidebyside.FoldStructure, // Structural diff rows appear in Normal (structural diff) view
			OldContent: oldLines,
			NewContent: newLines,
			Pairs: []sidebyside.LinePair{
				{Old: sidebyside.Line{Num: 1, Type: sidebyside.Context}, New: sidebyside.Line{Num: 1, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 3, Type: sidebyside.Context}, New: sidebyside.Line{Num: 3, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 4, Type: sidebyside.Context}, New: sidebyside.Line{Num: 4, Type: sidebyside.Context}},
				{Old: sidebyside.Line{Num: 0, Type: sidebyside.Empty}, New: sidebyside.Line{Num: 5, Type: sidebyside.Added}},
				{Old: sidebyside.Line{Num: 5, Type: sidebyside.Context}, New: sidebyside.Line{Num: 6, Type: sidebyside.Context}},
			},
		},
	}

	// Use New() to create model without explicit commit structure (diff mode)
	m := New(files)
	defer m.highlighter.Close()

	// Build rows to populate cache
	m.rebuildRowsCache()
	totalLinesBefore := m.w().totalLines

	// Request and store highlighting
	cmd := m.RequestHighlight(0)
	msg := cmd()
	hlMsg, ok := msg.(HighlightReadyMsg)
	require.True(t, ok, "expected HighlightReadyMsg")

	m.storeHighlightSpans(hlMsg)

	// totalLines should increase when structural diff has changes (file is folded, so preview rows appear)
	assert.Greater(t, m.w().totalLines, totalLinesBefore,
		"totalLines should increase in diff mode when structural diff has changes")
}
