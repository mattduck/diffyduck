package status

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stripANSI removes ANSI escape codes for test assertions.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestRender_EmptyStatus(t *testing.T) {
	out := Render("", nil, nil, nil, nil)
	stripped := stripANSI(out)
	assert.Contains(t, stripped, "Nothing to commit, working tree clean")
}

func TestRender_BranchTreeOnly(t *testing.T) {
	out := Render("  main\n  └─ feature (HEAD)\n", nil, nil, nil, nil)
	stripped := stripANSI(out)
	assert.Contains(t, stripped, "main")
	assert.Contains(t, stripped, "feature (HEAD)")
	assert.Contains(t, stripped, "Nothing to commit")
}

func TestRender_StagedSection(t *testing.T) {
	staged := []FileChange{
		{Path: "pkg/foo.go", Status: "+", Added: 42},
		{Path: "pkg/bar.go", Status: "~", Added: 5, Removed: 3},
	}

	out := Render("", staged, nil, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "Staged:")
	assert.Contains(t, stripped, "+ pkg/foo.go +42")
	assert.Contains(t, stripped, "~ pkg/bar.go +5 -3")
	assert.NotContains(t, stripped, "Unstaged:")
	assert.NotContains(t, stripped, "Untracked:")
}

func TestRender_UnstagedSection(t *testing.T) {
	unstaged := []FileChange{
		{Path: "main.go", Status: "~", Added: 10, Removed: 2},
	}

	out := Render("", nil, unstaged, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "Unstaged:")
	assert.Contains(t, stripped, "~ main.go +10 -2")
	assert.NotContains(t, stripped, "Staged:")
}

func TestRender_UntrackedSection(t *testing.T) {
	untracked := []string{"new_file.go", "another.go"}

	out := Render("", nil, nil, untracked, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "Untracked:")
	assert.Contains(t, stripped, "new_file.go")
	assert.Contains(t, stripped, "another.go")
}

func TestRender_AllSections(t *testing.T) {
	staged := []FileChange{{Path: "staged.go", Status: "+", Added: 10}}
	unstaged := []FileChange{{Path: "unstaged.go", Status: "~", Added: 5, Removed: 2}}
	untracked := []string{"new.go"}

	out := Render("  main\n", staged, unstaged, untracked, nil)
	stripped := stripANSI(out)

	// Verify section order
	stagedIdx := strings.Index(stripped, "Staged:")
	unstagedIdx := strings.Index(stripped, "Unstaged:")
	untrackedIdx := strings.Index(stripped, "Untracked:")

	assert.Greater(t, stagedIdx, 0)
	assert.Greater(t, unstagedIdx, stagedIdx)
	assert.Greater(t, untrackedIdx, unstagedIdx)
}

func TestRender_FileStatusIndicators(t *testing.T) {
	changes := []FileChange{
		{Path: "added.go", Status: "+", Added: 10},
		{Path: "deleted.go", Status: "-", Removed: 5},
		{Path: "modified.go", Status: "~", Added: 3, Removed: 1},
		{Path: "new.go", OldPath: "old.go", Status: "→"},
	}

	out := Render("", changes, nil, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "+ added.go")
	assert.Contains(t, stripped, "- deleted.go")
	assert.Contains(t, stripped, "~ modified.go")
	assert.Contains(t, stripped, "→ old.go → new.go")
}

func TestRender_WithSymbols(t *testing.T) {
	changes := []FileChange{
		{
			Path: "main.go", Status: "~", Added: 20, Removed: 5,
			Symbols: []SymbolLine{
				{Kind: "func", Signature: "Foo(...)", Added: 15, Removed: 3},
				{Kind: "func", Signature: "Bar()", Added: 5, Removed: 2, IsChild: true},
			},
		},
	}

	out := Render("", changes, nil, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "func Foo(...) +15 -3")
	assert.Contains(t, stripped, "func Bar() +5 -2")
}

func TestRender_TruncationMarker(t *testing.T) {
	changes := []FileChange{
		{
			Path: "big.go", Status: "~", Added: 100,
			Symbols: []SymbolLine{
				{Kind: "func", Signature: "A()", Added: 50},
				{Kind: "...(3 more)", Signature: ""},
			},
		},
	}

	out := Render("", changes, nil, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "...(3 more)")
}

func TestRender_DeletedFileNoStats(t *testing.T) {
	changes := []FileChange{
		{Path: "gone.go", Status: "-", Removed: 100},
	}

	out := Render("", changes, nil, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "- gone.go -100")
}

func TestRender_UntrackedExpanded(t *testing.T) {
	expanded := []FileChange{
		{Path: "new_file.go", Status: "+", Added: 30},
		{Path: "another.go", Status: "+", Added: 15},
	}

	out := Render("", nil, nil, nil, expanded)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "Untracked:")
	assert.Contains(t, stripped, "+ new_file.go +30")
	assert.Contains(t, stripped, "+ another.go +15")
}

func TestRender_UntrackedExpandedOverridesPlain(t *testing.T) {
	plain := []string{"should_not_appear.go"}
	expanded := []FileChange{
		{Path: "expanded.go", Status: "+", Added: 10},
	}

	// When both are provided, expanded wins
	out := Render("", nil, nil, plain, expanded)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "Untracked:")
	assert.Contains(t, stripped, "+ expanded.go +10")
	assert.NotContains(t, stripped, "should_not_appear.go")
}

func TestRender_SummaryLine(t *testing.T) {
	staged := []FileChange{
		{Path: "a.go", Status: "+", Added: 10},
		{Path: "b.go", Status: "~", Added: 5, Removed: 3},
	}
	unstaged := []FileChange{
		{Path: "c.go", Status: "~", Added: 2, Removed: 1},
	}

	out := Render("", staged, unstaged, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "3 files +17 -4")
}

func TestRender_SummaryLineNoRemoved(t *testing.T) {
	staged := []FileChange{
		{Path: "a.go", Status: "+", Added: 10},
	}

	out := Render("", staged, nil, nil, nil)
	stripped := stripANSI(out)

	assert.Contains(t, stripped, "1 files +10")
	assert.NotContains(t, stripped, "-")
}

func TestRender_NoSummaryForPlainUntracked(t *testing.T) {
	// Plain untracked listing (no FileChanges) should not show summary
	out := Render("", nil, nil, []string{"new.go"}, nil)
	stripped := stripANSI(out)

	assert.NotContains(t, stripped, "files")
	assert.NotContains(t, stripped, "Nothing to commit")
}

func TestFormatStats(t *testing.T) {
	tests := []struct {
		added, removed int
		want           string
	}{
		{0, 0, ""},
		{5, 0, "+5"},
		{0, 3, "-3"},
		{5, 3, "+5 -3"},
	}

	for _, tt := range tests {
		stripped := stripANSI(formatStats(tt.added, tt.removed))
		assert.Equal(t, tt.want, stripped)
	}
}
