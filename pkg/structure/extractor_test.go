package structure

import (
	"reflect"
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
		t.Logf("  Kind=%q Name=%q Params=%v ReturnType=%q Lines=%d-%d",
			e.Kind, e.Name, e.Params, e.ReturnType, e.StartLine, e.EndLine)
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

	// Check that params don't contain comments
	for _, p := range method.Params {
		if strings.Contains(p, "#") || strings.Contains(p, "type: ignore") {
			t.Errorf("Params should not contain comment, got %v", method.Params)
		}
	}

	// Check expected params (no comments, no trailing commas)
	expectedParams := []string{"self", "request: HttpRequest", "*args", "**kwargs"}
	if !reflect.DeepEqual(method.Params, expectedParams) {
		t.Errorf("Expected params %v, got %v", expectedParams, method.Params)
	}

	// Check compact signature format (no params, just ...)
	sig := method.FormatSignature(0)
	expected := "get(...)"
	if sig != expected {
		t.Errorf("Expected signature %q, got %q", expected, sig)
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
		t.Logf("  Kind=%q Name=%q Params=%v ReturnType=%q Lines=%d-%d",
			e.Kind, e.Name, e.Params, e.ReturnType, e.StartLine, e.EndLine)
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
	if e.ReturnType != "None" {
		t.Errorf("Expected return type 'None', got %q", e.ReturnType)
	}

	// Check that params don't contain newlines
	for _, p := range e.Params {
		if strings.Contains(p, "\n") {
			t.Errorf("Params should not contain newlines, got %v", e.Params)
		}
	}

	// Check compact signature format (no params, prioritize return type)
	sig := e.FormatSignature(0)
	expected := "save_form_POST_to_session(...) -> None"
	if sig != expected {
		t.Errorf("Expected signature %q, got %q", expected, sig)
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
		t.Logf("  Kind=%q Name=%q Receiver=%q Params=%v ReturnType=%q Lines=%d-%d",
			e.Kind, e.Name, e.Receiver, e.Params, e.ReturnType, e.StartLine, e.EndLine)
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
	if e.ReturnType != "error" {
		t.Errorf("Expected return type 'error', got %q", e.ReturnType)
	}

	// Check that params don't contain newlines
	for _, p := range e.Params {
		if strings.Contains(p, "\n") {
			t.Errorf("Params should not contain newlines, got %v", e.Params)
		}
	}

	// Check compact signature (no params, prioritize return type)
	sig := e.FormatSignature(0)
	expected := "ProcessRequest(...) -> error"
	if sig != expected {
		t.Errorf("Expected signature %q, got %q", expected, sig)
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
		t.Logf("  Kind=%q Name=%q Receiver=%q Params=%v ReturnType=%q Lines=%d-%d",
			e.Kind, e.Name, e.Receiver, e.Params, e.ReturnType, e.StartLine, e.EndLine)
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

	// Check receiver doesn't have trailing comma in generics
	if strings.Contains(e.Receiver, ",]") {
		t.Errorf("Receiver should not have trailing comma, got %q", e.Receiver)
	}

	// Check compact signature (receiver + name + empty params + return type)
	sig := e.FormatSignature(0)
	expected := "(m *Model[K, V]) Process() -> error"
	if sig != expected {
		t.Errorf("Expected signature %q, got %q", expected, sig)
	}
}

func TestPythonExtractor_SingleParam(t *testing.T) {
	// Test single-param function signatures
	pythonContent := `def get_user(user_id: int) -> User:
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

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	// Compact (width=0): shows (...) with return type
	compact := entries[0].FormatSignature(0)
	if compact != "get_user(...) -> User" {
		t.Errorf("Compact signature wrong: %q", compact)
	}

	// With enough width: shows full param
	wide := entries[0].FormatSignature(100)
	if wide != "get_user(user_id: int) -> User" {
		t.Errorf("Wide signature wrong: %q", wide)
	}
}

func TestGoExtractor_SingleParam(t *testing.T) {
	// Test single-param function signatures
	goContent := `package main

func GetUser(userID int) *User {
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

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	// Compact (width=0): shows (...) with return type
	compact := entries[0].FormatSignature(0)
	if compact != "GetUser(...) -> *User" {
		t.Errorf("Compact signature wrong: %q", compact)
	}

	// With enough width: shows full param
	wide := entries[0].FormatSignature(100)
	if wide != "GetUser(userID int) -> *User" {
		t.Errorf("Wide signature wrong: %q", wide)
	}
}

func TestFormatSignature_WidthTruncation(t *testing.T) {
	// Test that signatures truncate properly based on width
	e := Entry{
		Name:       "ProcessRequest",
		Kind:       "func",
		Params:     []string{"ctx context.Context", "request *Request", "options ...Option"},
		ReturnType: "error",
	}

	// Compact (width=0): no params, just (...)
	compact := e.FormatSignature(0)
	if compact != "ProcessRequest(...) -> error" {
		t.Errorf("Compact signature wrong: %q", compact)
	}

	// Wide enough for all params
	wide := e.FormatSignature(100)
	if wide != "ProcessRequest(ctx context.Context, request *Request, options ...Option) -> error" {
		t.Errorf("Wide signature wrong: %q", wide)
	}

	// Medium width: progressively add params until it doesn't fit
	// "ProcessRequest(...) -> error" = 28 chars
	// "ProcessRequest(ctx context.Context, ...) -> error" = 50 chars
	// "ProcessRequest(ctx context.Context, request *Request, ...) -> error" = 68 chars
	medium := e.FormatSignature(55)
	expected := "ProcessRequest(ctx context.Context, ...) -> error"
	if medium != expected {
		t.Errorf("Medium signature wrong: got %q, want %q", medium, expected)
	}

	// Slightly wider: can fit two params
	wider := e.FormatSignature(70)
	expected2 := "ProcessRequest(ctx context.Context, request *Request, ...) -> error"
	if wider != expected2 {
		t.Errorf("Wider signature wrong: got %q, want %q", wider, expected2)
	}
}

func TestFormatSignature_ReturnTypePriority(t *testing.T) {
	// Test that return type is always shown before params are added
	e := Entry{
		Name:       "fetchData",
		Kind:       "func",
		Params:     []string{"url string", "timeout time.Duration"},
		ReturnType: "(*Response, error)",
	}

	// At any width, return type should be complete before params are shown
	// "fetchData(...) -> (*Response, error)" = 37 chars
	// "fetchData(url string, ...) -> (*Response, error)" = 49 chars

	// Width just enough for compact + return type
	sig40 := e.FormatSignature(40)
	if !strings.HasSuffix(sig40, "-> (*Response, error)") {
		t.Errorf("Return type should be complete at width 40: %q", sig40)
	}
	if !strings.Contains(sig40, "(...)") {
		t.Errorf("Should show (...) at width 40: %q", sig40)
	}

	// Width enough for one param
	sig50 := e.FormatSignature(50)
	if !strings.HasSuffix(sig50, "-> (*Response, error)") {
		t.Errorf("Return type should be complete at width 50: %q", sig50)
	}
}

func TestFormatSignature_NoParams(t *testing.T) {
	// Test function with no params
	e := Entry{
		Name:       "Close",
		Kind:       "func",
		Params:     []string{},
		ReturnType: "error",
	}

	sig := e.FormatSignature(0)
	if sig != "Close() -> error" {
		t.Errorf("No-param signature wrong: %q", sig)
	}

	// Width shouldn't matter for no-param functions
	sigWide := e.FormatSignature(100)
	if sigWide != "Close() -> error" {
		t.Errorf("No-param wide signature wrong: %q", sigWide)
	}
}

func TestFormatSignature_NoReturnType(t *testing.T) {
	// Test function with no return type (like Python __init__)
	e := Entry{
		Name:   "__init__",
		Kind:   "def",
		Params: []string{"self", "name: str", "age: int"},
	}

	compact := e.FormatSignature(0)
	if compact != "__init__(...)" {
		t.Errorf("No-return compact signature wrong: %q", compact)
	}

	wide := e.FormatSignature(100)
	if wide != "__init__(self, name: str, age: int)" {
		t.Errorf("No-return wide signature wrong: %q", wide)
	}
}

func TestFormatSignature_WithReceiver(t *testing.T) {
	// Test Go method with receiver
	e := Entry{
		Name:       "Update",
		Kind:       "func",
		Receiver:   "(m *Model)",
		Params:     []string{"msg tea.Msg"},
		ReturnType: "(tea.Model, tea.Cmd)",
	}

	compact := e.FormatSignature(0)
	if compact != "(m *Model) Update(...) -> (tea.Model, tea.Cmd)" {
		t.Errorf("Receiver compact signature wrong: %q", compact)
	}

	wide := e.FormatSignature(100)
	if wide != "(m *Model) Update(msg tea.Msg) -> (tea.Model, tea.Cmd)" {
		t.Errorf("Receiver wide signature wrong: %q", wide)
	}
}

func TestFormatSignature_ClassNoSignature(t *testing.T) {
	// Test that types/classes return empty signature
	e := Entry{
		Name: "MyClass",
		Kind: "class",
	}

	sig := e.FormatSignature(0)
	if sig != "" {
		t.Errorf("Class should have empty signature, got: %q", sig)
	}

	sigWide := e.FormatSignature(100)
	if sigWide != "" {
		t.Errorf("Class should have empty signature at any width, got: %q", sigWide)
	}
}
