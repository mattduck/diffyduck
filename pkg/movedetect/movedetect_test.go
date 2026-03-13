package movedetect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func line(num int, content string, typ sidebyside.LineType) sidebyside.Line {
	return sidebyside.Line{Num: num, Content: content, Type: typ}
}

func empty() sidebyside.Line {
	return sidebyside.Line{Type: sidebyside.Empty}
}

func TestDetect_BasicMove(t *testing.T) {
	// File 0: three lines removed
	// File 1: same three lines added
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "func hello() {", sidebyside.Removed), New: empty()},
			{Old: line(2, "    fmt.Println(\"hi\")", sidebyside.Removed), New: empty()},
			{Old: line(3, "}", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(10, "func hello() {", sidebyside.Added)},
			{Old: empty(), New: line(11, "    fmt.Println(\"hi\")", sidebyside.Added)},
			{Old: empty(), New: line(12, "}", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.MaxGroup)

	// All six lines should be in group 1
	for pi := 0; pi < 3; pi++ {
		assert.Equal(t, 1, result.Groups[Key{0, pi, 1}], "remove file=0 pair=%d", pi)
		assert.Equal(t, 1, result.Groups[Key{1, pi, 0}], "add file=1 pair=%d", pi)
	}
}

func TestDetect_IndentationChange(t *testing.T) {
	// Same content but indented differently should still match.
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "line one", sidebyside.Removed), New: empty()},
			{Old: line(2, "line two", sidebyside.Removed), New: empty()},
			{Old: line(3, "line three", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(1, "    line one", sidebyside.Added)},
			{Old: empty(), New: line(2, "    line two", sidebyside.Added)},
			{Old: empty(), New: line(3, "    line three", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.MaxGroup)
}

func TestDetect_BelowMinBlock(t *testing.T) {
	// Only 2 matching lines — below the threshold of 3.
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "alpha", sidebyside.Removed), New: empty()},
			{Old: line(2, "beta", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(1, "alpha", sidebyside.Added)},
			{Old: empty(), New: line(2, "beta", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	assert.Empty(t, result.Groups)
}

func TestDetect_NoMatches(t *testing.T) {
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "aaa", sidebyside.Removed), New: empty()},
			{Old: line(2, "bbb", sidebyside.Removed), New: empty()},
			{Old: line(3, "ccc", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(1, "xxx", sidebyside.Added)},
			{Old: empty(), New: line(2, "yyy", sidebyside.Added)},
			{Old: empty(), New: line(3, "zzz", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	assert.Empty(t, result.Groups)
}

func TestDetect_MultipleGroups(t *testing.T) {
	// Two separate moved blocks.
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "alpha one", sidebyside.Removed), New: empty()},
			{Old: line(2, "alpha two", sidebyside.Removed), New: empty()},
			{Old: line(3, "alpha three", sidebyside.Removed), New: empty()},
			// context break
			{Old: line(4, "context", sidebyside.Context), New: line(4, "context", sidebyside.Context)},
			{Old: line(5, "beta one", sidebyside.Removed), New: empty()},
			{Old: line(6, "beta two", sidebyside.Removed), New: empty()},
			{Old: line(7, "beta three", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(1, "beta one", sidebyside.Added)},
			{Old: empty(), New: line(2, "beta two", sidebyside.Added)},
			{Old: empty(), New: line(3, "beta three", sidebyside.Added)},
			{Old: line(4, "context", sidebyside.Context), New: line(4, "context", sidebyside.Context)},
			{Old: empty(), New: line(5, "alpha one", sidebyside.Added)},
			{Old: empty(), New: line(6, "alpha two", sidebyside.Added)},
			{Old: empty(), New: line(7, "alpha three", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.MaxGroup)

	// Alpha group: removes at file0 pairs 0-2, adds at file1 pairs 4-6
	alphaGroup := result.Groups[Key{0, 0, 1}]
	assert.NotZero(t, alphaGroup)
	assert.Equal(t, alphaGroup, result.Groups[Key{1, 4, 0}])

	// Beta group: removes at file0 pairs 4-6, adds at file1 pairs 0-2
	betaGroup := result.Groups[Key{0, 4, 1}]
	assert.NotZero(t, betaGroup)
	assert.Equal(t, betaGroup, result.Groups[Key{1, 0, 0}])

	assert.NotEqual(t, alphaGroup, betaGroup)
}

func TestDetect_IncludesChangePairs(t *testing.T) {
	// Modified pairs (both sides have content) should be included in move
	// detection. A moved block can span across the boundary between pure
	// and paired regions when the diff transform coincidentally pairs some
	// moved lines with unrelated content.
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "same content", sidebyside.Removed), New: line(1, "different", sidebyside.Added)},
			{Old: line(2, "same content", sidebyside.Removed), New: line(2, "different", sidebyside.Added)},
			{Old: line(3, "same content", sidebyside.Removed), New: line(3, "different", sidebyside.Added)},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(10, "same content", sidebyside.Added)},
			{Old: empty(), New: line(11, "same content", sidebyside.Added)},
			{Old: empty(), New: line(12, "same content", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	// The remove side of change pairs should match the pure adds
	assert.Equal(t, 1, result.MaxGroup)
	for pi := 0; pi < 3; pi++ {
		assert.Equal(t, 1, result.Groups[Key{0, pi, 1}], "remove file=0 pair=%d", pi)
		assert.Equal(t, 1, result.Groups[Key{1, pi, 0}], "add file=1 pair=%d", pi)
	}
}

func TestDetect_WithinSameFile(t *testing.T) {
	// Move within the same file (e.g., reordering functions).
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(1, "func goodbye() {", sidebyside.Added)},
			{Old: empty(), New: line(2, "    fmt.Println(\"bye\")", sidebyside.Added)},
			{Old: empty(), New: line(3, "}", sidebyside.Added)},
			{Old: line(4, "context", sidebyside.Context), New: line(4, "context", sidebyside.Context)},
			{Old: line(5, "func goodbye() {", sidebyside.Removed), New: empty()},
			{Old: line(6, "    fmt.Println(\"bye\")", sidebyside.Removed), New: empty()},
			{Old: line(7, "}", sidebyside.Removed), New: empty()},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.MaxGroup)
}

func TestDetect_MovedBlockSpansChangePairs(t *testing.T) {
	// A moved block where some lines end up in change pairs (both sides have
	// content) and some are pure removes/adds. The entire block should be
	// detected as a single move group.
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			// Pure adds (left only) — lines 1-3
			{Old: empty(), New: line(1, "    alpha", sidebyside.Added)},
			{Old: empty(), New: line(2, "    beta", sidebyside.Added)},
			{Old: empty(), New: line(3, "    gamma", sidebyside.Added)},
			// Change pairs where the remove side has moved content
			{Old: line(10, "alpha", sidebyside.Removed), New: line(4, "unrelated x", sidebyside.Added)},
			{Old: line(11, "beta", sidebyside.Removed), New: line(5, "unrelated y", sidebyside.Added)},
			{Old: line(12, "gamma", sidebyside.Removed), New: line(6, "unrelated z", sidebyside.Added)},
			// Pure removes (right only) — lines 13-15
			{Old: line(13, "delta", sidebyside.Removed), New: empty()},
			{Old: line(14, "epsilon", sidebyside.Removed), New: empty()},
			{Old: line(15, "zeta", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			// Pure adds matching the removes above
			{Old: empty(), New: line(20, "delta", sidebyside.Added)},
			{Old: empty(), New: line(21, "epsilon", sidebyside.Added)},
			{Old: empty(), New: line(22, "zeta", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)

	// The remove side (lines 10-15) should match with adds:
	// removes 10-12 (in change pairs) match adds 1-3 (pure)
	// removes 13-15 (pure) match adds 20-22 (pure)
	// Since 10-15 are consecutive removes and 1-3 + 20-22 are in different files,
	// we get two groups.

	// Group for removes 10-12 ↔ adds 1-3 (within same file, all consecutive)
	group1 := result.Groups[Key{0, 3, 1}] // remove at pair 3
	assert.NotZero(t, group1)
	assert.Equal(t, group1, result.Groups[Key{0, 0, 0}]) // add at pair 0

	// Group for removes 13-15 ↔ adds 20-22 (cross-file)
	group2 := result.Groups[Key{0, 6, 1}] // remove at pair 6
	assert.NotZero(t, group2)
	assert.Equal(t, group2, result.Groups[Key{1, 0, 0}]) // add at pair 0 in file 1
}

func TestDetect_IdenticalBlocksSameGroup(t *testing.T) {
	// Two identical blocks moved to different places should share the same
	// group ID (same color) since there's no way to tell which source
	// matched which destination.
	files := []sidebyside.FilePair{
		{Pairs: []sidebyside.LinePair{
			{Old: line(1, "func hello() {", sidebyside.Removed), New: empty()},
			{Old: line(2, "    fmt.Println(\"hi\")", sidebyside.Removed), New: empty()},
			{Old: line(3, "}", sidebyside.Removed), New: empty()},
			{Old: line(4, "context", sidebyside.Context), New: line(4, "context", sidebyside.Context)},
			{Old: line(5, "func hello() {", sidebyside.Removed), New: empty()},
			{Old: line(6, "    fmt.Println(\"hi\")", sidebyside.Removed), New: empty()},
			{Old: line(7, "}", sidebyside.Removed), New: empty()},
		}},
		{Pairs: []sidebyside.LinePair{
			{Old: empty(), New: line(1, "func hello() {", sidebyside.Added)},
			{Old: empty(), New: line(2, "    fmt.Println(\"hi\")", sidebyside.Added)},
			{Old: empty(), New: line(3, "}", sidebyside.Added)},
			{Old: line(4, "context", sidebyside.Context), New: line(4, "context", sidebyside.Context)},
			{Old: empty(), New: line(5, "func hello() {", sidebyside.Added)},
			{Old: empty(), New: line(6, "    fmt.Println(\"hi\")", sidebyside.Added)},
			{Old: empty(), New: line(7, "}", sidebyside.Added)},
		}},
	}

	result := Detect(files, 3, 0)
	require.NotNil(t, result)

	// All four blocks should share the same group ID
	group1 := result.Groups[Key{0, 0, 1}]
	group2 := result.Groups[Key{0, 4, 1}]
	group3 := result.Groups[Key{1, 0, 0}]
	group4 := result.Groups[Key{1, 4, 0}]
	assert.NotZero(t, group1)
	assert.Equal(t, group1, group2, "identical remove blocks should share group")
	assert.Equal(t, group1, group3, "removes and adds should share group")
	assert.Equal(t, group1, group4, "all identical blocks should share group")
	assert.Equal(t, 1, result.MaxGroup, "only one group for identical content")
}

func TestDetect_EmptyInput(t *testing.T) {
	result := Detect(nil, 3, 0)
	require.NotNil(t, result)
	assert.Empty(t, result.Groups)
}
