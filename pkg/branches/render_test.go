package branches

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"hours", now.Add(-2 * time.Hour), "2h ago"},
		{"days", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"weeks", now.Add(-14 * 24 * time.Hour), "2w ago"},
		{"months", now.Add(-60 * 24 * time.Hour), "2mo ago"},
		{"years", now.Add(-400 * 24 * time.Hour), "1y ago"},
		{"zero", time.Time{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, relativeTime(tt.t, now))
		})
	}
}

func TestRender_LinearChain(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	roots := []*BranchNode{
		{
			Name:    "main",
			SHA:     "a1b2c3d",
			Subject: "refactor: extract parser",
			Date:    now.Add(-2 * time.Hour),
			Children: []*BranchNode{
				{
					Name:    "second",
					SHA:     "e4f5g6h",
					Subject: "feat: add threading",
					Date:    now.Add(-30 * time.Minute),
					Ahead:   3,
					Behind:  2,
					Children: []*BranchNode{
						{
							Name:    "fourth",
							SHA:     "m0n1o2p",
							Subject: "fix: cursor jump",
							Date:    now.Add(-5 * time.Minute),
							Ahead:   1,
							IsHead:  true,
						},
					},
				},
				{
					Name:    "third",
					SHA:     "i7j8k9l",
					Subject: "feat: lazy loading",
					Date:    now.Add(-24 * time.Hour),
					Ahead:   2,
				},
			},
		},
	}

	got := RenderAt(roots, false, now)

	// Non-verbose: should have tree structure, counts, SHA, date — but not subject
	assert.Contains(t, got, "main")
	assert.Contains(t, got, "├─")
	assert.Contains(t, got, "└─")
	assert.Contains(t, got, "* fourth")
	assert.Contains(t, got, "+3 -2")
	assert.Contains(t, got, "+1")
	assert.Contains(t, got, "+2")
	assert.Contains(t, got, "a1b2c3d")
	assert.Contains(t, got, "2h ago")
	assert.Contains(t, got, "5m ago")
	assert.NotContains(t, got, "refactor: extract parser")

	// Verbose: should include subject and author
	verbose := RenderAt(roots, true, now)
	assert.Contains(t, verbose, "refactor: extract parser")
}

func TestRender_Empty(t *testing.T) {
	assert.Equal(t, "", RenderAt(nil, false, time.Now()))
}

func TestRender_SingleBranch(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	roots := []*BranchNode{
		{
			Name:    "main",
			SHA:     "a1b2c3d",
			Subject: "init",
			Date:    now.Add(-1 * time.Hour),
			IsHead:  true,
		},
	}

	got := RenderAt(roots, false, now)
	assert.Contains(t, got, "*")
	assert.Contains(t, got, "main")
	assert.Contains(t, got, "a1b2c3d")
	assert.Contains(t, got, "1h ago")
	// No tree connectors for single root
	assert.NotContains(t, got, "├─")
	assert.NotContains(t, got, "└─")
}

func TestRender_MultipleTrees(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	roots := []*BranchNode{
		{
			Name:    "main",
			SHA:     "a1b2c3d",
			Subject: "init",
			Date:    now.Add(-1 * time.Hour),
		},
		{
			Name:    "orphan",
			SHA:     "x9y8z7w",
			Subject: "orphan init",
			Date:    now.Add(-2 * time.Hour),
		},
	}

	got := RenderAt(roots, false, now)
	// Should have a blank line between trees
	assert.Contains(t, got, "\n\n")
	assert.Contains(t, got, "main")
	assert.Contains(t, got, "orphan")
}
