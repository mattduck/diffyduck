package branches

import (
	"time"

	"github.com/user/diffyduck/pkg/git"
)

// FilterBranches removes branches whose latest commit is older than since,
// except for branches that are always kept: the current HEAD, the default
// branch, and branches with active worktrees.
func FilterBranches(all []git.BranchInfo, since time.Time, defaultBranch string, worktreeBranches []string) []git.BranchInfo {
	wtSet := make(map[string]bool, len(worktreeBranches))
	for _, name := range worktreeBranches {
		wtSet[name] = true
	}

	var result []git.BranchInfo
	for _, b := range all {
		if b.IsHead || b.Name == defaultBranch || wtSet[b.Name] {
			result = append(result, b)
			continue
		}
		t, err := time.Parse(time.RFC3339, b.Date)
		if err != nil {
			result = append(result, b) // keep if date unparseable
			continue
		}
		if !t.Before(since) {
			result = append(result, b)
		}
	}
	return result
}
