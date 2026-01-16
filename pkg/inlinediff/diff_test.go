package inlinediff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// assertSpansCoverString verifies that spans cover the entire string without gaps
func assertSpansCoverString(t *testing.T, s string, spans []Span) {
	t.Helper()
	if len(s) == 0 {
		assert.Empty(t, spans)
		return
	}

	assert.NotEmpty(t, spans, "spans should not be empty for non-empty string")

	// Check first span starts at 0
	assert.Equal(t, 0, spans[0].Start, "first span should start at 0")

	// Check last span ends at len(s)
	assert.Equal(t, len(s), spans[len(spans)-1].End, "last span should end at string length")

	// Check no gaps between spans
	for i := 1; i < len(spans); i++ {
		assert.Equal(t, spans[i-1].End, spans[i].Start, "span %d should start where span %d ends", i, i-1)
	}
}

func TestDiff_SimpleWordChange(t *testing.T) {
	old := "hello world"
	new := "hello there"

	oldSpans, newSpans := Diff(old, new)

	// Verify spans cover entire strings
	assertSpansCoverString(t, old, oldSpans)
	assertSpansCoverString(t, new, newSpans)

	// "hello " should be unchanged in both
	assert.Equal(t, Unchanged, oldSpans[0].Type)
	assert.Equal(t, "hello ", old[oldSpans[0].Start:oldSpans[0].End])

	assert.Equal(t, Unchanged, newSpans[0].Type)
	assert.Equal(t, "hello ", new[newSpans[0].Start:newSpans[0].End])

	// Should have some removed content in old
	hasRemoved := false
	for _, s := range oldSpans {
		if s.Type == Removed {
			hasRemoved = true
		}
	}
	assert.True(t, hasRemoved)

	// Should have some added content in new
	hasAdded := false
	for _, s := range newSpans {
		if s.Type == Added {
			hasAdded = true
		}
	}
	assert.True(t, hasAdded)
}

func TestDiff_Addition(t *testing.T) {
	old := "foo"
	new := "foo bar"

	oldSpans, newSpans := Diff(old, new)

	// Old is all unchanged, new has addition
	assert.Equal(t, []Span{
		{Start: 0, End: 3, Type: Unchanged},
	}, oldSpans)

	assert.Equal(t, []Span{
		{Start: 0, End: 3, Type: Unchanged},
		{Start: 3, End: 7, Type: Added},
	}, newSpans)
}

func TestDiff_Deletion(t *testing.T) {
	old := "foo bar"
	new := "foo"

	oldSpans, newSpans := Diff(old, new)

	// Old has deletion, new is all unchanged
	assert.Equal(t, []Span{
		{Start: 0, End: 3, Type: Unchanged},
		{Start: 3, End: 7, Type: Removed},
	}, oldSpans)

	assert.Equal(t, []Span{
		{Start: 0, End: 3, Type: Unchanged},
	}, newSpans)
}

func TestDiff_MultipleChanges(t *testing.T) {
	old := "the quick brown fox"
	new := "a slow brown dog"

	oldSpans, newSpans := Diff(old, new)

	// "the quick" -> "a slow", " brown " unchanged, "fox" -> "dog"
	// The exact spans depend on algorithm, but we should have:
	// - Some removed spans in old
	// - Some added spans in new
	// - " brown " should be unchanged in both

	// Verify brown is marked unchanged in both
	hasUnchangedBrown := false
	for _, s := range oldSpans {
		if s.Type == Unchanged && old[s.Start:s.End] == " brown " {
			hasUnchangedBrown = true
		}
	}
	assert.True(t, hasUnchangedBrown, "old should have unchanged ' brown '")

	hasUnchangedBrown = false
	for _, s := range newSpans {
		if s.Type == Unchanged && new[s.Start:s.End] == " brown " {
			hasUnchangedBrown = true
		}
	}
	assert.True(t, hasUnchangedBrown, "new should have unchanged ' brown '")

	// Verify there are some changes
	hasRemoved := false
	for _, s := range oldSpans {
		if s.Type == Removed {
			hasRemoved = true
		}
	}
	assert.True(t, hasRemoved, "old should have removed spans")

	hasAdded := false
	for _, s := range newSpans {
		if s.Type == Added {
			hasAdded = true
		}
	}
	assert.True(t, hasAdded, "new should have added spans")
}

func TestDiff_IdenticalLines(t *testing.T) {
	line := "identical content"

	oldSpans, newSpans := Diff(line, line)

	// Both should be entirely unchanged
	assert.Equal(t, []Span{
		{Start: 0, End: 17, Type: Unchanged},
	}, oldSpans)

	assert.Equal(t, []Span{
		{Start: 0, End: 17, Type: Unchanged},
	}, newSpans)
}

func TestDiff_CompletelyDifferent(t *testing.T) {
	old := "abcdefgh"
	new := "xyz12345"

	oldSpans, newSpans := Diff(old, new)

	// When lines are completely different, entire old is removed, entire new is added
	assert.Equal(t, []Span{
		{Start: 0, End: 8, Type: Removed},
	}, oldSpans)

	assert.Equal(t, []Span{
		{Start: 0, End: 8, Type: Added},
	}, newSpans)
}

func TestDiff_EmptyOld(t *testing.T) {
	old := ""
	new := "added"

	oldSpans, newSpans := Diff(old, new)

	assert.Empty(t, oldSpans)
	assert.Equal(t, []Span{
		{Start: 0, End: 5, Type: Added},
	}, newSpans)
}

func TestDiff_EmptyNew(t *testing.T) {
	old := "removed"
	new := ""

	oldSpans, newSpans := Diff(old, new)

	assert.Equal(t, []Span{
		{Start: 0, End: 7, Type: Removed},
	}, oldSpans)
	assert.Empty(t, newSpans)
}

func TestDiff_BothEmpty(t *testing.T) {
	oldSpans, newSpans := Diff("", "")

	assert.Empty(t, oldSpans)
	assert.Empty(t, newSpans)
}

func TestDiff_ChangeInMiddle(t *testing.T) {
	old := "prefix CHANGED suffix"
	new := "prefix MODIFIED suffix"

	oldSpans, newSpans := Diff(old, new)

	// Verify spans cover entire strings
	assertSpansCoverString(t, old, oldSpans)
	assertSpansCoverString(t, new, newSpans)

	// "prefix " should be unchanged at start
	assert.Equal(t, Unchanged, oldSpans[0].Type)
	assert.Equal(t, "prefix ", old[oldSpans[0].Start:oldSpans[0].End])

	// The last span should be unchanged and contain "suffix"
	lastOld := oldSpans[len(oldSpans)-1]
	assert.Equal(t, Unchanged, lastOld.Type)
	assert.Contains(t, old[lastOld.Start:lastOld.End], "suffix")

	// Should have removed spans in middle of old
	hasRemoved := false
	for _, s := range oldSpans {
		if s.Type == Removed {
			hasRemoved = true
		}
	}
	assert.True(t, hasRemoved, "should have removed spans in old")

	// Should have added spans in middle of new
	hasAdded := false
	for _, s := range newSpans {
		if s.Type == Added {
			hasAdded = true
		}
	}
	assert.True(t, hasAdded, "should have added spans in new")
}

// ShouldSkipInlineDiff returns true when lines are too different for useful inline diff
func TestShouldSkipInlineDiff_VeryDifferent(t *testing.T) {
	// When similarity is below threshold, skip inline diff
	old := "completely different content here"
	new := "xyz nothing alike at all 123"

	assert.True(t, ShouldSkipInlineDiff(old, new))
}

func TestShouldSkipInlineDiff_Similar(t *testing.T) {
	// When lines are similar enough, do inline diff
	old := "fmt.Println(x)"
	new := "fmt.Println(y)"

	assert.False(t, ShouldSkipInlineDiff(old, new))
}

func TestShouldSkipInlineDiff_AssertEqualVsContains(t *testing.T) {
	// Real case: assert.Equal vs assert.Contains
	old := `assert.Equal(t, "", output)`
	new := `assert.Contains(t, output, "0%")`

	// These share words like "assert", "t", "output" so word similarity is high
	// ShouldSkipInlineDiff returns false (lines are "similar" at word level)
	assert.False(t, ShouldSkipInlineDiff(old, new))

	// But when we compute the actual spans, too much would be highlighted
	// so ShouldSkipBasedOnSpans catches it (52% changed > 50% threshold)
	oldSpans, _ := Diff(old, new)
	assert.True(t, ShouldSkipBasedOnSpans(oldSpans, len(old)),
		"should skip because >50%% of line would be highlighted")
}

func TestDiff_WordLevel_NoCommonWords(t *testing.T) {
	// "Syntax Highlighting" vs "Inline/Word Diff" - no common words
	old := "Syntax Highlighting"
	new := "Inline/Word Diff"

	oldSpans, newSpans := Diff(old, new)

	// Entire old line should be removed (no common words)
	assert.Len(t, oldSpans, 1)
	assert.Equal(t, Removed, oldSpans[0].Type)
	assert.Equal(t, old, old[oldSpans[0].Start:oldSpans[0].End])

	// Entire new line should be added
	assert.Len(t, newSpans, 1)
	assert.Equal(t, Added, newSpans[0].Type)
	assert.Equal(t, new, new[newSpans[0].Start:newSpans[0].End])
}

func TestDiff_GapMerging(t *testing.T) {
	// When changed spans are separated by tiny unchanged gaps, merge them
	// Example: "a_b" vs "x_y" - the "_" is common but too small to keep separate
	old := "a_b"
	new := "x_y"

	oldSpans, newSpans := Diff(old, new)

	// With gap merging (MinUnchangedGap=3), the single "_" gap should be merged
	// Result should be one span covering the whole string
	assert.Len(t, oldSpans, 1, "should merge into single span")
	assert.Equal(t, Removed, oldSpans[0].Type)

	assert.Len(t, newSpans, 1, "should merge into single span")
	assert.Equal(t, Added, newSpans[0].Type)
}

func TestDiff_GapPreserved_WhenLargeEnough(t *testing.T) {
	// When unchanged gap is large enough, preserve it
	old := "changed    unchanged"
	new := "modified   unchanged"

	oldSpans, newSpans := Diff(old, new)

	// "unchanged" should be preserved as unchanged in both
	hasUnchangedOld := false
	for _, s := range oldSpans {
		if s.Type == Unchanged && old[s.Start:s.End] == "unchanged" {
			hasUnchangedOld = true
		}
	}
	assert.True(t, hasUnchangedOld, "should preserve 'unchanged' as unchanged in old")

	hasUnchangedNew := false
	for _, s := range newSpans {
		if s.Type == Unchanged && new[s.Start:s.End] == "unchanged" {
			hasUnchangedNew = true
		}
	}
	assert.True(t, hasUnchangedNew, "should preserve 'unchanged' as unchanged in new")
}

func TestShouldSkipBasedOnSpans_MostlyChanged(t *testing.T) {
	// If >50% would be highlighted, should skip
	spans := []Span{
		{Start: 0, End: 4, Type: Unchanged}, // 4 bytes unchanged
		{Start: 4, End: 10, Type: Removed},  // 6 bytes changed
	}
	totalLen := 10

	assert.True(t, ShouldSkipBasedOnSpans(spans, totalLen), "60% changed should skip")
}

func TestShouldSkipBasedOnSpans_MostlyUnchanged(t *testing.T) {
	// If <50% would be highlighted, should not skip
	spans := []Span{
		{Start: 0, End: 6, Type: Unchanged}, // 6 bytes unchanged
		{Start: 6, End: 10, Type: Removed},  // 4 bytes changed
	}
	totalLen := 10

	assert.False(t, ShouldSkipBasedOnSpans(spans, totalLen), "40% changed should not skip")
}
