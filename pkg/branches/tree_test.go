package branches

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/diffyduck/pkg/git"
)

type mockQuerier struct {
	mergeBases   map[string]string // "a\x00b" -> sha
	aheadBehinds map[string][2]int // "a\x00b" -> [ahead, behind]
}

func (m *mockQuerier) MergeBase(a, b string) (string, error) {
	if sha, ok := m.mergeBases[a+"\x00"+b]; ok {
		return sha, nil
	}
	if sha, ok := m.mergeBases[b+"\x00"+a]; ok {
		return sha, nil
	}
	return "", nil
}

func (m *mockQuerier) AheadBehind(a, b string) (int, int, error) {
	if counts, ok := m.aheadBehinds[a+"\x00"+b]; ok {
		return counts[0], counts[1], nil
	}
	return 0, 0, nil
}

func TestBuildTree_Empty(t *testing.T) {
	roots, err := BuildTree(nil, &mockQuerier{})
	require.NoError(t, err)
	assert.Nil(t, roots)
}

func TestBuildTree_SingleBranch(t *testing.T) {
	branches := []git.BranchInfo{
		{Name: "main", SHA: "aaaaaaa0000000000000000000000000000000aa", Subject: "init", Date: "2025-01-01T00:00:00Z", IsHead: true},
	}
	roots, err := BuildTree(branches, &mockQuerier{})
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, "main", roots[0].Name)
	assert.True(t, roots[0].IsHead)
	assert.Empty(t, roots[0].Children)
}

func TestBuildTree_LinearChain(t *testing.T) {
	// main -> second -> third
	// merge-base(main, second) == main tip => second is on top of main
	// merge-base(second, third) == second tip => third is on top of second
	// merge-base(main, third) == main tip => third is also on top of main,
	//   but second is the closer parent
	mainSHA := "aaaaaaa0000000000000000000000000000000aa"
	secondSHA := "bbbbbbb0000000000000000000000000000000bb"
	thirdSHA := "ccccccc0000000000000000000000000000000cc"

	branches := []git.BranchInfo{
		{Name: "main", SHA: mainSHA, Subject: "init", Date: "2025-01-01T00:00:00Z"},
		{Name: "second", SHA: secondSHA, Subject: "feat: second", Date: "2025-01-02T00:00:00Z"},
		{Name: "third", SHA: thirdSHA, Subject: "feat: third", Date: "2025-01-03T00:00:00Z", IsHead: true},
	}

	q := &mockQuerier{
		mergeBases: map[string]string{
			"main\x00second":  mainSHA,
			"main\x00third":   mainSHA,
			"second\x00third": secondSHA,
		},
		aheadBehinds: map[string][2]int{
			"second\x00main":  {3, 0},
			"third\x00main":   {5, 0},
			"third\x00second": {2, 0},
		},
	}

	roots, err := BuildTree(branches, q)
	require.NoError(t, err)
	require.Len(t, roots, 1)

	// Root is main
	assert.Equal(t, "main", roots[0].Name)
	require.Len(t, roots[0].Children, 1)

	// main -> second
	second := roots[0].Children[0]
	assert.Equal(t, "second", second.Name)
	assert.Equal(t, 3, second.Ahead)
	assert.Equal(t, 0, second.Behind)
	require.Len(t, second.Children, 1)

	// second -> third
	third := second.Children[0]
	assert.Equal(t, "third", third.Name)
	assert.Equal(t, 2, third.Ahead)
	assert.True(t, third.IsHead)
}

func TestBuildTree_MultipleChildren(t *testing.T) {
	// main -> alpha, main -> beta
	mainSHA := "aaaaaaa0000000000000000000000000000000aa"
	alphaSHA := "bbbbbbb0000000000000000000000000000000bb"
	betaSHA := "ccccccc0000000000000000000000000000000cc"

	branches := []git.BranchInfo{
		{Name: "main", SHA: mainSHA, Subject: "init", Date: "2025-01-01T00:00:00Z"},
		{Name: "alpha", SHA: alphaSHA, Subject: "feat: alpha", Date: "2025-01-02T00:00:00Z"},
		{Name: "beta", SHA: betaSHA, Subject: "feat: beta", Date: "2025-01-03T00:00:00Z"},
	}

	q := &mockQuerier{
		mergeBases: map[string]string{
			"alpha\x00main": mainSHA,
			"beta\x00main":  mainSHA,
			"alpha\x00beta": mainSHA, // diverged from same point on main
		},
		aheadBehinds: map[string][2]int{
			"alpha\x00main": {2, 0},
			"beta\x00main":  {3, 0},
		},
	}

	roots, err := BuildTree(branches, q)
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, "main", roots[0].Name)
	require.Len(t, roots[0].Children, 2)
	assert.Equal(t, "beta", roots[0].Children[0].Name)  // most recent first (2025-01-03)
	assert.Equal(t, "alpha", roots[0].Children[1].Name) // older (2025-01-02)
}

func TestBuildTree_IndependentTrees(t *testing.T) {
	// Two branches with no common ancestor
	branches := []git.BranchInfo{
		{Name: "main", SHA: "aaaaaaa0000000000000000000000000000000aa", Subject: "init", Date: "2025-01-01T00:00:00Z"},
		{Name: "orphan", SHA: "bbbbbbb0000000000000000000000000000000bb", Subject: "orphan init", Date: "2025-01-02T00:00:00Z"},
	}

	q := &mockQuerier{
		mergeBases: map[string]string{}, // no common ancestor
	}

	roots, err := BuildTree(branches, q)
	require.NoError(t, err)
	require.Len(t, roots, 2)
	assert.Equal(t, "orphan", roots[0].Name) // most recent first (2025-01-02)
	assert.Equal(t, "main", roots[1].Name)   // older (2025-01-01)
}

func TestBuildTree_SameCommit(t *testing.T) {
	// Two branches pointing at the same commit — merged into one node
	sha := "aaaaaaa0000000000000000000000000000000aa"
	branches := []git.BranchInfo{
		{Name: "main", SHA: sha, Subject: "init", Date: "2025-01-01T00:00:00Z"},
		{Name: "copy", SHA: sha, Subject: "init", Date: "2025-01-01T00:00:00Z"},
	}

	q := &mockQuerier{}

	roots, err := BuildTree(branches, q)
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, "copy, main", roots[0].Name) // alphabetical
}

func TestBuildTree_SameCommitWithChildren(t *testing.T) {
	// main and third point to the same commit, second is on top of them
	sharedSHA := "aaaaaaa0000000000000000000000000000000aa"
	secondSHA := "bbbbbbb0000000000000000000000000000000bb"

	branches := []git.BranchInfo{
		{Name: "main", SHA: sharedSHA, Subject: "init", Date: "2025-01-01T00:00:00Z"},
		{Name: "third", SHA: sharedSHA, Subject: "init", Date: "2025-01-01T00:00:00Z"},
		{Name: "second", SHA: secondSHA, Subject: "feat: second", Date: "2025-01-02T00:00:00Z"},
	}

	q := &mockQuerier{
		mergeBases: map[string]string{
			// main is the representative ref for the merged node
			"main\x00second": sharedSHA,
		},
		aheadBehinds: map[string][2]int{
			"second\x00main": {1, 0},
		},
	}

	roots, err := BuildTree(branches, q)
	require.NoError(t, err)
	require.Len(t, roots, 1)
	assert.Equal(t, "main, third", roots[0].Name)
	require.Len(t, roots[0].Children, 1)
	assert.Equal(t, "second", roots[0].Children[0].Name)
	assert.Equal(t, 1, roots[0].Children[0].Ahead)
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBuildTree_Deduplication(t *testing.T) {
	// Verify that deduplication removes branches appearing in multiple subtrees.
	// Direct test of deduplicateTree since the fork-point algorithm can produce
	// duplicate nodes in complex histories.
	mainSHA := "aaaaaaa0000000000000000000000000000000aa"
	featureSHA := "bbbbbbb0000000000000000000000000000000bb"
	childSHA := "ccccccc0000000000000000000000000000000cc"

	feature := &BranchNode{
		Name:    "feature",
		SHA:     "bbbbbbb",
		Date:    mustParseTime("2025-01-02T00:00:00Z"),
		fullSHA: featureSHA,
		ref:     "feature",
	}
	child := &BranchNode{
		Name:    "child",
		SHA:     "ccccccc",
		Date:    mustParseTime("2025-01-03T00:00:00Z"),
		fullSHA: childSHA,
		ref:     "child",
		Ahead:   1,
	}

	// Simulate a tree where "feature" appears under two virtual forks
	fork1 := &BranchNode{
		SHA:     "fff1111",
		Virtual: true,
		fullSHA: "fff11110000000000000000000000000000000ff",
		ref:     "fff11110000000000000000000000000000000ff",
		Children: []*BranchNode{
			feature,
			child,
		},
	}
	// feature is duplicated (same pointer or same SHA)
	featureDup := &BranchNode{
		Name:    "feature",
		SHA:     "bbbbbbb",
		Date:    mustParseTime("2025-01-02T00:00:00Z"),
		fullSHA: featureSHA,
		ref:     "feature",
	}
	fork2 := &BranchNode{
		SHA:     "fff2222",
		Virtual: true,
		fullSHA: "fff22220000000000000000000000000000000ff",
		ref:     "fff22220000000000000000000000000000000ff",
		Children: []*BranchNode{
			featureDup,
		},
	}

	root := &BranchNode{
		Name:     "main",
		SHA:      "aaaaaaa",
		Date:     mustParseTime("2025-01-01T00:00:00Z"),
		fullSHA:  mainSHA,
		ref:      "main",
		Children: []*BranchNode{fork1, fork2},
	}

	result := deduplicateTree([]*BranchNode{root})
	require.Len(t, result, 1)

	// Flatten all non-virtual names to verify no duplicates
	var names []string
	var collect func([]*BranchNode)
	collect = func(nodes []*BranchNode) {
		for _, n := range nodes {
			if !n.Virtual {
				names = append(names, n.Name)
			}
			collect(n.Children)
		}
	}
	collect(result)

	// Each branch should appear exactly once
	seen := make(map[string]int)
	for _, name := range names {
		seen[name]++
	}
	for name, count := range seen {
		assert.Equal(t, 1, count, "branch %q appears %d times, want 1", name, count)
	}
	// fork2 had only 1 child (featureDup) which was a duplicate, so after dedup
	// fork2 has 0 children and gets unwrapped (removed)
	assert.Len(t, result[0].Children, 1, "fork2 should be unwrapped after dedup")
}

func TestBuildTree_StaleBranch(t *testing.T) {
	// second is based on main, but main has moved on (behind > 0)
	// Both are roots, and a virtual fork point is inserted at their merge-base
	mainSHA := "aaaaaaa0000000000000000000000000000000aa"
	secondSHA := "bbbbbbb0000000000000000000000000000000bb"
	oldMainSHA := "ddddddd0000000000000000000000000000000dd"

	branches := []git.BranchInfo{
		{Name: "main", SHA: mainSHA, Subject: "latest on main", Date: "2025-01-03T00:00:00Z"},
		{Name: "second", SHA: secondSHA, Subject: "feat: second", Date: "2025-01-02T00:00:00Z"},
	}

	q := &mockQuerier{
		mergeBases: map[string]string{
			"main\x00second": oldMainSHA,
		},
		aheadBehinds: map[string][2]int{
			"main\x00" + oldMainSHA:   {2, 0},
			"second\x00" + oldMainSHA: {3, 0},
		},
	}

	roots, err := BuildTree(branches, q)
	require.NoError(t, err)
	// Should have 1 root: the virtual fork point
	require.Len(t, roots, 1)
	assert.True(t, roots[0].Virtual)
	assert.Equal(t, "ddddddd", roots[0].SHA)
	require.Len(t, roots[0].Children, 2)
	assert.Equal(t, "main", roots[0].Children[0].Name)
	assert.Equal(t, 2, roots[0].Children[0].Ahead)
	assert.Equal(t, "second", roots[0].Children[1].Name)
	assert.Equal(t, 3, roots[0].Children[1].Ahead)
}
