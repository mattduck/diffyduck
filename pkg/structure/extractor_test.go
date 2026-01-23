package structure

import (
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

func TestPythonExtractor_CommentsAndTrailingCommas(t *testing.T) {
	// Reproduce the issue with comments and trailing commas in parameters
	pythonContent := `class StorybookView:
    def get(
        # type: ignore[override]
        self,
        request: HttpRequest,
        *args,
        **kwargs,
    ):
        pass
`
	content := []byte(pythonContent)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_python.Language()))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	extractor := &pythonExtractor{}
	entries := extractor.Extract(tree, content)

	t.Logf("Extracted %d entries:", len(entries))
	for _, e := range entries {
		t.Logf("  Kind=%q Name=%q Signature=%q Lines=%d-%d", e.Kind, e.Name, e.Signature, e.StartLine, e.EndLine)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries (class + method), got %d", len(entries))
	}

	// Check the method entry
	method := entries[1]
	if method.Kind != "def" {
		t.Errorf("Expected kind 'def', got %q", method.Kind)
	}
	if method.Name != "get" {
		t.Errorf("Expected name 'get', got %q", method.Name)
	}

	// Check that comment is NOT in the signature
	if strings.Contains(method.Signature, "type: ignore") {
		t.Errorf("Signature should not contain comment, got %q", method.Signature)
	}
	if strings.Contains(method.Signature, "#") {
		t.Errorf("Signature should not contain '#', got %q", method.Signature)
	}

	// Check expected clean signature
	expected := "get(self, request: HttpRequest, *args, **kwargs)"
	if method.Signature != expected {
		t.Errorf("Expected signature %q, got %q", expected, method.Signature)
	}
}

func TestPythonExtractor_MultilineParams(t *testing.T) {
	// Reproduce the issue with multiline function parameters
	pythonContent := `def save_form_POST_to_session(
    request: HttpRequest, key: str, *, pk: int | None = None
) -> None:
    pass
`
	content := []byte(pythonContent)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_python.Language()))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	extractor := &pythonExtractor{}
	entries := extractor.Extract(tree, content)

	t.Logf("Extracted %d entries:", len(entries))
	for _, e := range entries {
		t.Logf("  Kind=%q Name=%q Signature=%q Lines=%d-%d", e.Kind, e.Name, e.Signature, e.StartLine, e.EndLine)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Kind != "def" {
		t.Errorf("Expected kind 'def', got %q", e.Kind)
	}
	if e.Name != "save_form_POST_to_session" {
		t.Errorf("Expected name 'save_form_POST_to_session', got %q", e.Name)
	}

	// Check if the signature contains newlines (the problematic case)
	if strings.Contains(e.Signature, "\n") {
		t.Errorf("Signature should not contain newlines, got %q", e.Signature)
	}

	// Check expected clean signature
	expected := "save_form_POST_to_session(request: HttpRequest, key: str, *, pk: int | None = None)"
	if e.Signature != expected {
		t.Errorf("Expected signature %q, got %q", expected, e.Signature)
	}
}

func TestGoExtractor_MultilineParams(t *testing.T) {
	// Test that Go multiline function parameters are normalized
	goContent := `package main

func ProcessRequest(
	ctx context.Context,
	request *Request,
	options ...Option,
) error {
	return nil
}
`
	content := []byte(goContent)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language()))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	extractor := &goExtractor{}
	entries := extractor.Extract(tree, content)

	t.Logf("Extracted %d entries:", len(entries))
	for _, e := range entries {
		t.Logf("  Kind=%q Name=%q Signature=%q Lines=%d-%d", e.Kind, e.Name, e.Signature, e.StartLine, e.EndLine)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Kind != "func" {
		t.Errorf("Expected kind 'func', got %q", e.Kind)
	}
	if e.Name != "ProcessRequest" {
		t.Errorf("Expected name 'ProcessRequest', got %q", e.Name)
	}

	// Check if the signature contains newlines (the problematic case)
	if strings.Contains(e.Signature, "\n") {
		t.Errorf("Signature should not contain newlines, got %q", e.Signature)
	}

	// Check expected clean signature (no trailing commas)
	expected := "ProcessRequest(ctx context.Context, request *Request, options ...Option)"
	if e.Signature != expected {
		t.Errorf("Expected signature %q, got %q", expected, e.Signature)
	}
}

func TestGoExtractor_MultilineReceiver(t *testing.T) {
	// Test that Go multiline method receivers are normalized
	goContent := `package main

func (m *Model[
	K,
	V,
]) Process() error {
	return nil
}
`
	content := []byte(goContent)

	parser := tree_sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language()))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	extractor := &goExtractor{}
	entries := extractor.Extract(tree, content)

	t.Logf("Extracted %d entries:", len(entries))
	for _, e := range entries {
		t.Logf("  Kind=%q Name=%q Signature=%q Lines=%d-%d", e.Kind, e.Name, e.Signature, e.StartLine, e.EndLine)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Kind != "func" {
		t.Errorf("Expected kind 'func', got %q", e.Kind)
	}
	if e.Name != "Process" {
		t.Errorf("Expected name 'Process', got %q", e.Name)
	}

	// Check if the signature contains newlines (the problematic case)
	if strings.Contains(e.Signature, "\n") {
		t.Errorf("Signature should not contain newlines, got %q", e.Signature)
	}

	// Check expected clean signature (no trailing commas in generics)
	expected := "(m *Model[K, V]) Process()"
	if e.Signature != expected {
		t.Errorf("Expected signature %q, got %q", expected, e.Signature)
	}
}
