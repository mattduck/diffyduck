package branches

import (
	"sort"
	"strings"
	"time"

	"github.com/user/diffyduck/pkg/git"
)

// GitQuerier provides the git operations needed for branch tree building.
type GitQuerier interface {
	MergeBase(a, b string) (string, error)
	AheadBehind(a, b string) (ahead, behind int, err error)
}

// UpstreamInfo describes the relationship between a local branch and its remote.
type UpstreamInfo struct {
	Name   string // remote tracking branch (e.g. "origin/main")
	Ahead  int    // commits local is ahead of upstream
	Behind int    // commits local is behind upstream
	Gone   bool   // upstream branch was deleted
}

// BranchNode represents a branch in the dependency tree.
type BranchNode struct {
	Name      string
	SHA       string // short SHA (7 chars)
	Subject   string
	Author    string
	Date      time.Time
	IsHead    bool
	HeadRef   string // specific branch name that is HEAD (empty if none)
	Virtual   bool   // true for fork point nodes (not a branch)
	Ahead     int    // commits ahead of parent (0 for roots)
	Behind    int    // commits behind parent (0 when up-to-date)
	Upstreams []UpstreamInfo
	Children  []*BranchNode

	// Internal fields for git operations during tree building.
	fullSHA string // full commit SHA
	ref     string // git ref name (branch name, or full SHA for virtual nodes)
}

// BuildTree infers parent-child relationships between branches and returns a forest.
// A branch B is "on top of" branch A if merge-base(A, B) == tip(A).
// Branches pointing to the same commit are merged into one node with comma-separated names.
// Siblings with a shared fork point get a virtual ancestor node inserted.
func BuildTree(branches []git.BranchInfo, q GitQuerier) ([]*BranchNode, error) {
	if len(branches) == 0 {
		return nil, nil
	}

	// Group branches that point to the same commit.
	type group struct {
		names     []string
		info      git.BranchInfo
		isHead    bool
		headRef   string
		upstreams []UpstreamInfo
	}
	shaGroups := make(map[string]*group)
	for _, b := range branches {
		g, ok := shaGroups[b.SHA]
		if !ok {
			g = &group{info: b}
			shaGroups[b.SHA] = g
		}
		g.names = append(g.names, b.Name)
		if b.IsHead {
			g.isHead = true
			g.headRef = b.Name
		}
		if b.Upstream != "" {
			g.upstreams = append(g.upstreams, UpstreamInfo{
				Name:   b.Upstream,
				Ahead:  b.UpstreamAhead,
				Behind: b.UpstreamBehind,
				Gone:   b.UpstreamGone,
			})
		}
	}

	// Build merged nodes — one per unique SHA.
	nodes := make(map[string]*BranchNode, len(shaGroups))
	for sha, g := range shaGroups {
		sort.Strings(g.names)
		displayKey := strings.Join(g.names, ", ")
		short := sha
		if len(short) > 7 {
			short = short[:7]
		}
		t, _ := time.Parse(time.RFC3339, g.info.Date)
		nodes[displayKey] = &BranchNode{
			Name:      displayKey,
			SHA:       short,
			Subject:   g.info.Subject,
			Author:    g.info.Author,
			Date:      t,
			IsHead:    g.isHead,
			HeadRef:   g.headRef,
			Upstreams: g.upstreams,
			fullSHA:   sha,
			ref:       g.names[0],
		}
	}

	if len(nodes) == 1 {
		for _, node := range nodes {
			return []*BranchNode{node}, nil
		}
	}

	// For each pair of merged nodes, compute merge-base
	type pairKey struct{ a, b string }
	mergeBases := make(map[pairKey]string)
	keys := make([]string, 0, len(nodes))
	for k := range nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			mb, err := q.MergeBase(nodes[keys[i]].ref, nodes[keys[j]].ref)
			if err != nil {
				return nil, err
			}
			mergeBases[pairKey{keys[i], keys[j]}] = mb
			mergeBases[pairKey{keys[j], keys[i]}] = mb
		}
	}

	// Find best parent for each node.
	parentOf := make(map[string]string)

	for _, child := range keys {
		bestParent := ""
		bestAhead := int(^uint(0) >> 1)

		for _, candidate := range keys {
			if candidate == child {
				continue
			}
			mb := mergeBases[pairKey{candidate, child}]
			if mb == "" {
				continue
			}
			if mb != nodes[candidate].fullSHA {
				continue
			}
			if nodes[candidate].fullSHA == nodes[child].fullSHA {
				continue
			}
			ahead, _, err := q.AheadBehind(nodes[child].ref, nodes[candidate].ref)
			if err != nil {
				return nil, err
			}
			if ahead < bestAhead {
				bestAhead = ahead
				bestParent = candidate
			}
		}

		if bestParent != "" {
			parentOf[child] = bestParent
		}
	}

	// Build tree: attach children, compute ahead/behind relative to parent
	for child, parent := range parentOf {
		ahead, behind, err := q.AheadBehind(nodes[child].ref, nodes[parent].ref)
		if err != nil {
			return nil, err
		}
		nodes[child].Ahead = ahead
		nodes[child].Behind = behind
		nodes[parent].Children = append(nodes[parent].Children, nodes[child])
	}

	// Sort children by date (most recent first)
	for _, node := range nodes {
		sort.Slice(node.Children, func(i, j int) bool {
			return effectiveDate(node.Children[i]).After(effectiveDate(node.Children[j]))
		})
	}

	// Collect roots, sorted by date (most recent first)
	var roots []*BranchNode
	for _, key := range keys {
		if _, hasParent := parentOf[key]; !hasParent {
			roots = append(roots, nodes[key])
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		return effectiveDate(roots[i]).After(effectiveDate(roots[j]))
	})

	// Insert virtual fork points where siblings diverged recently.
	// Collect all branch SHAs so we don't insert forks at existing branches.
	allBranchSHAs := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		allBranchSHAs[node.fullSHA] = true
	}
	var err error
	roots, err = insertForkPoints(roots, allBranchSHAs, q)
	if err != nil {
		return nil, err
	}

	// Recompute ahead/behind for virtual children relative to their parents.
	if err := recomputeVirtualAhead(roots, q); err != nil {
		return nil, err
	}

	// Remove any duplicate branches (same SHA appearing in multiple subtrees).
	roots = deduplicateTree(roots)

	return roots, nil
}

// insertForkPoints finds siblings whose merge-base is more recent than their
// parent and inserts a virtual fork-point node between them.
// allBranchSHAs contains the full SHAs of all real branches in the tree.
func insertForkPoints(nodes []*BranchNode, allBranchSHAs map[string]bool, q GitQuerier) ([]*BranchNode, error) {
	// Recurse into each node's children first (bottom-up)
	for _, node := range nodes {
		var err error
		node.Children, err = insertForkPoints(node.Children, allBranchSHAs, q)
		if err != nil {
			return nil, err
		}
	}

	if len(nodes) < 2 {
		return nodes, nil
	}

	// Group siblings by their pairwise merge-base.
	type forkGroup struct {
		forkSHA string
		members []*BranchNode
	}

	grouped := make(map[int]bool)
	var groups []forkGroup

	for i := 0; i < len(nodes); i++ {
		if grouped[i] {
			continue
		}
		for j := i + 1; j < len(nodes); j++ {
			if grouped[j] {
				continue
			}
			if nodes[i].ref == "" || nodes[j].ref == "" {
				continue
			}
			mb, err := q.MergeBase(nodes[i].ref, nodes[j].ref)
			if err != nil || mb == "" {
				continue
			}

			// Skip if a branch already exists at this commit
			if allBranchSHAs[mb] {
				continue
			}

			// Only insert if the fork is meaningfully closer than the parent.
			aheadI, _, err := q.AheadBehind(nodes[i].ref, mb)
			if err != nil {
				return nil, err
			}
			if nodes[i].Ahead > 0 && aheadI >= nodes[i].Ahead {
				continue
			}

			// Find or create a group for this fork point
			found := false
			for k := range groups {
				if groups[k].forkSHA == mb {
					if !grouped[j] {
						groups[k].members = append(groups[k].members, nodes[j])
						grouped[j] = true
					}
					if !grouped[i] {
						groups[k].members = append(groups[k].members, nodes[i])
						grouped[i] = true
					}
					found = true
					break
				}
			}
			if !found {
				groups = append(groups, forkGroup{
					forkSHA: mb,
					members: []*BranchNode{nodes[i], nodes[j]},
				})
				grouped[i] = true
				grouped[j] = true
			}
		}
	}

	if len(groups) == 0 {
		return nodes, nil
	}

	// Build the new node list: ungrouped nodes + virtual fork nodes
	var result []*BranchNode
	for i, node := range nodes {
		if !grouped[i] {
			result = append(result, node)
		}
	}

	for _, g := range groups {
		short := g.forkSHA
		if len(short) > 7 {
			short = short[:7]
		}

		fork := &BranchNode{
			SHA:     short,
			Virtual: true,
			fullSHA: g.forkSHA,
			ref:     g.forkSHA,
		}

		// Recompute ahead/behind for each member relative to the fork point
		for _, member := range g.members {
			ahead, behind, err := q.AheadBehind(member.ref, g.forkSHA)
			if err != nil {
				return nil, err
			}
			member.Ahead = ahead
			member.Behind = behind
		}

		sort.Slice(g.members, func(i, j int) bool {
			return effectiveDate(g.members[i]).After(effectiveDate(g.members[j]))
		})
		fork.Children = g.members
		result = append(result, fork)
	}

	sort.Slice(result, func(i, j int) bool {
		return effectiveDate(result[i]).After(effectiveDate(result[j]))
	})

	return result, nil
}

// effectiveDate returns the date to use for sorting a node.
// For virtual nodes (which have no date), it returns the most recent child's date.
func effectiveDate(node *BranchNode) time.Time {
	if !node.Virtual || len(node.Children) == 0 {
		return node.Date
	}
	best := time.Time{}
	for _, child := range node.Children {
		d := effectiveDate(child)
		if d.After(best) {
			best = d
		}
	}
	return best
}

// recomputeVirtualAhead walks the tree and sets Ahead/Behind on virtual nodes
// relative to their parent.
func recomputeVirtualAhead(nodes []*BranchNode, q GitQuerier) error {
	for _, node := range nodes {
		for _, child := range node.Children {
			if child.Virtual && node.ref != "" && child.ref != "" {
				ahead, behind, err := q.AheadBehind(child.ref, node.ref)
				if err != nil {
					return err
				}
				child.Ahead = ahead
				child.Behind = behind
			}
		}
		if err := recomputeVirtualAhead(node.Children, q); err != nil {
			return err
		}
	}
	return nil
}

// deduplicateTree removes branches that appear in multiple places in the tree,
// keeping only the first occurrence (depth-first order). After removal, virtual
// nodes with 0-1 children are unwrapped (their children promoted to the parent).
func deduplicateTree(roots []*BranchNode) []*BranchNode {
	seen := make(map[string]bool)
	return dedup(roots, seen)
}

func dedup(nodes []*BranchNode, seen map[string]bool) []*BranchNode {
	var result []*BranchNode
	for _, node := range nodes {
		if !node.Virtual && seen[node.fullSHA] {
			continue
		}
		if !node.Virtual {
			seen[node.fullSHA] = true
		}
		node.Children = dedup(node.Children, seen)
		if node.Virtual && len(node.Children) <= 1 {
			result = append(result, node.Children...)
			continue
		}
		result = append(result, node)
	}
	return result
}
