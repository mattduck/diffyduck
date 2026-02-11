package branches

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/git"
)

func TestFilterBranches(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	since := now.Add(-30 * 24 * time.Hour) // 1 month ago

	recent := now.Add(-7 * 24 * time.Hour).Format(time.RFC3339)    // 1 week ago
	old := now.Add(-60 * 24 * time.Hour).Format(time.RFC3339)      // 2 months ago
	veryOld := now.Add(-365 * 24 * time.Hour).Format(time.RFC3339) // 1 year ago

	tests := []struct {
		name             string
		branches         []git.BranchInfo
		defaultBranch    string
		worktreeBranches []string
		wantNames        []string
	}{
		{
			name: "filters old branches",
			branches: []git.BranchInfo{
				{Name: "recent-feat", Date: recent},
				{Name: "old-feat", Date: old},
				{Name: "very-old", Date: veryOld},
			},
			wantNames: []string{"recent-feat"},
		},
		{
			name: "keeps HEAD even if old",
			branches: []git.BranchInfo{
				{Name: "current", Date: veryOld, IsHead: true},
				{Name: "other-old", Date: veryOld},
			},
			wantNames: []string{"current"},
		},
		{
			name:          "keeps default branch even if old",
			defaultBranch: "main",
			branches: []git.BranchInfo{
				{Name: "main", Date: veryOld},
				{Name: "other-old", Date: veryOld},
			},
			wantNames: []string{"main"},
		},
		{
			name:             "keeps worktree branches even if old",
			worktreeBranches: []string{"wt-branch"},
			branches: []git.BranchInfo{
				{Name: "wt-branch", Date: veryOld},
				{Name: "other-old", Date: veryOld},
			},
			wantNames: []string{"wt-branch"},
		},
		{
			name: "keeps branches with unparseable dates",
			branches: []git.BranchInfo{
				{Name: "bad-date", Date: "not-a-date"},
				{Name: "old-feat", Date: veryOld},
			},
			wantNames: []string{"bad-date"},
		},
		{
			name:      "empty input",
			branches:  nil,
			wantNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterBranches(tt.branches, since, tt.defaultBranch, tt.worktreeBranches)
			var gotNames []string
			for _, b := range got {
				gotNames = append(gotNames, b.Name)
			}
			assert.Equal(t, tt.wantNames, gotNames)
		})
	}
}
