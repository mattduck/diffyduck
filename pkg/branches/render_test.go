package branches

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
	assert.Contains(t, got, "┌─")
	assert.Contains(t, got, "├─")
	assert.Contains(t, got, "*fourth")
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
	assert.NotContains(t, got, "┌─")
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

func TestRender_UpstreamTracking(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	roots := []*BranchNode{
		{
			Name:      "main",
			SHA:       "a1b2c3d",
			Subject:   "init",
			Date:      now.Add(-2 * time.Hour),
			Upstreams: []UpstreamInfo{{Name: "origin/main"}},
			Children: []*BranchNode{
				{
					Name:      "feature",
					SHA:       "e4f5g6h",
					Subject:   "feat: wip",
					Date:      now.Add(-30 * time.Minute),
					Ahead:     3,
					Upstreams: []UpstreamInfo{{Name: "origin/feature", Ahead: 2}},
				},
				{
					Name:      "stale",
					SHA:       "i7j8k9l",
					Subject:   "fix: old",
					Date:      now.Add(-24 * time.Hour),
					Ahead:     1,
					Upstreams: []UpstreamInfo{{Name: "origin/stale", Behind: 5}},
				},
				{
					Name:      "pruned",
					SHA:       "m0n1o2p",
					Subject:   "chore: cleanup",
					Date:      now.Add(-3 * 24 * time.Hour),
					Ahead:     1,
					Upstreams: []UpstreamInfo{{Name: "origin/pruned", Gone: true}},
				},
				{
					Name:    "local-only",
					SHA:     "q3r4s5t",
					Subject: "feat: local",
					Date:    now.Add(-1 * time.Hour),
					Ahead:   2,
				},
			},
		},
	}

	got := RenderAt(roots, false, now)

	// Synced upstream shows "="
	assert.Contains(t, got, "origin/main =")
	// Ahead-only shows arrow up
	assert.Contains(t, got, "origin/feature ↑2")
	// Behind shows arrow down
	assert.Contains(t, got, "origin/stale ↓5")
	// Gone upstream
	assert.Contains(t, got, "origin/pruned gone")
	// No upstream for local-only
	assert.NotContains(t, got, "local-only =")
	assert.NotContains(t, got, "origin/local-only")
}

func TestRender_MultipleUpstreams(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	// Two branches at same SHA, each with its own upstream
	roots := []*BranchNode{
		{
			Name:    "main, release",
			SHA:     "a1b2c3d",
			Subject: "release v1",
			Date:    now.Add(-1 * time.Hour),
			Upstreams: []UpstreamInfo{
				{Name: "origin/main"},
				{Name: "origin/release", Ahead: 1},
			},
		},
	}

	got := RenderAt(roots, false, now)
	// Each upstream is individually colored, so check them separately
	assert.Contains(t, got, "origin/main =")
	assert.Contains(t, got, "origin/release ↑1")
	// Synced and ahead-only upstreams should be dim (fg=8)
	assert.Contains(t, got, colorBrightBlk+"origin/main =")
	assert.Contains(t, got, colorBrightBlk+"origin/release ↑1")
}

func TestRender_HeadRefUnderline(t *testing.T) {
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	roots := []*BranchNode{
		{
			Name:    "main, second, third",
			SHA:     "a1b2c3d",
			Subject: "init",
			Date:    now.Add(-1 * time.Hour),
			IsHead:  true,
			HeadRef: "second",
			Upstreams: []UpstreamInfo{
				{Name: "origin/main"},
				{Name: "origin/second"},
			},
		},
	}

	got := RenderAt(roots, false, now)
	// Asterisk should be before the HEAD branch, not the first name
	assert.Contains(t, got, "*\033[4msecond\033[24m")
	assert.NotContains(t, got, "*main")
	// HEAD branch name should be underlined, others should not
	assert.Contains(t, got, "\033[4msecond\033[24m")
	assert.NotContains(t, got, "\033[4mmain\033[24m")
	assert.NotContains(t, got, "\033[4mthird\033[24m")
	// Matching upstream should be underlined
	assert.Contains(t, got, "\033[4morigin/second\033[24m")
	assert.NotContains(t, got, "\033[4morigin/main\033[24m")
}

func TestRender_ColumnAlignment(t *testing.T) {
	// Verify that root branches (no tree-drawing chars) align with child
	// branches (which contain multi-byte box-drawing chars like ┌─).
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	roots := []*BranchNode{
		{
			Name: "main",
			SHA:  "a1b2c3d",
			Date: now.Add(-2 * time.Hour),
			Children: []*BranchNode{
				{
					Name:  "feature",
					SHA:   "e4f5g6h",
					Date:  now.Add(-30 * time.Minute),
					Ahead: 3,
				},
			},
		},
		{
			Name: "orphan",
			SHA:  "x9y8z7w",
			Date: now.Add(-3 * 24 * time.Hour),
		},
	}

	got := RenderAt(roots, false, now)

	// Strip ANSI escape codes for alignment checking
	stripped := stripANSI(got)
	lines := strings.Split(strings.TrimRight(stripped, "\n"), "\n")

	// Find SHA visual column position in each non-blank line.
	// Use rune count (not byte offset) so box-drawing chars count as 1 column.
	var shaPositions []int
	for _, line := range lines {
		if line == "" {
			continue
		}
		for _, sha := range []string{"a1b2c3d", "e4f5g6h", "x9y8z7w"} {
			idx := strings.Index(line, sha)
			if idx >= 0 {
				shaPositions = append(shaPositions, utf8.RuneCountInString(line[:idx]))
				break
			}
		}
	}

	// All SHA columns should start at the same position
	assert.NotEmpty(t, shaPositions, "should find SHA in output lines")
	for i := 1; i < len(shaPositions); i++ {
		assert.Equal(t, shaPositions[0], shaPositions[i],
			"SHA column misaligned between lines %d and %d", 0, i)
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip to 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
		} else {
			out.WriteByte(s[i])
			i++
		}
	}
	return out.String()
}
