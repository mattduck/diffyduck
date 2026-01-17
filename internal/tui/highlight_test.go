package tui

import (
	"strings"
	"testing"

	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestSyntaxHighlighting_SpansGenerated(t *testing.T) {
	// Create a model with a Go file
	files := []sidebyside.FilePair{
		{
			OldPath: "a/test.go",
			NewPath: "b/test.go",
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			FoldLevel:  sidebyside.FoldExpanded,
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
			FoldLevel:  sidebyside.FoldExpanded,
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
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
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
		t.Logf("Row %d: isHeader=%v isSeparator=%v isBlank=%v fileIndex=%d left.Num=%d right.Num=%d",
			i, row.isHeader, row.isSeparator, row.isBlank, row.fileIndex, row.pair.Left.Num, row.pair.Right.Num)
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
	result := m.applySyntaxHighlight("package main", "package main", "package main", spans, 0, 0)
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
			FoldLevel: sidebyside.FoldExpanded,
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "func hello() {", Type: sidebyside.Context},
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

func TestSyntaxHighlighting_NormalViewNoHighlight(t *testing.T) {
	// In normal view (FoldNormal), content is not loaded, so no highlighting
	files := []sidebyside.FilePair{
		{
			OldPath:   "a/test.go",
			NewPath:   "b/test.go",
			FoldLevel: sidebyside.FoldNormal, // Normal view
			Pairs: []sidebyside.LinePair{
				{
					Left:  sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
					Right: sidebyside.Line{Num: 1, Content: "package main", Type: sidebyside.Context},
				},
			},
			// OldContent and NewContent are nil (not loaded)
		},
	}

	m := New(files)
	defer m.highlighter.Close()

	// Try to get line spans - should return nil because content not loaded
	spans := m.getLineSpans(0, 1, false)
	if spans != nil {
		t.Errorf("Expected nil spans in normal view without content, got %d spans", len(spans))
	}

	// Even if we trigger highlighting, it should produce no spans (no content to parse)
	cmd := m.RequestHighlight(0)
	msg := cmd()

	if msg != nil {
		hlMsg, ok := msg.(HighlightReadyMsg)
		if ok && (len(hlMsg.OldSpans) > 0 || len(hlMsg.NewSpans) > 0) {
			t.Error("Expected no spans when content is not loaded")
		}
	}
}
