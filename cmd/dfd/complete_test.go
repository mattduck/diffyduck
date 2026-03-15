package main

import (
	"slices"
	"testing"

	"github.com/user/diffyduck/pkg/git"
)

func TestParseCompletionContext(t *testing.T) {
	tests := []struct {
		name          string
		words         []string
		wantCmd       string
		wantFlags     []string
		wantRefs      []string
		wantAfterSep  bool
		wantCurrent   string
		wantFlagValue string
	}{
		{
			name:        "empty input",
			words:       nil,
			wantCurrent: "",
		},
		{
			name:        "cursor only (no committed words)",
			words:       []string{""},
			wantCurrent: "",
		},
		{
			name:        "partial subcommand",
			words:       []string{"di"},
			wantCurrent: "di",
		},
		{
			name:        "subcommand committed, cursor empty",
			words:       []string{"diff", ""},
			wantCmd:     "diff",
			wantCurrent: "",
		},
		{
			name:        "alias expansion",
			words:       []string{"d", ""},
			wantCmd:     "diff",
			wantCurrent: "",
		},
		{
			name:        "show with ref",
			words:       []string{"show", "main", ""},
			wantCmd:     "show",
			wantRefs:    []string{"main"},
			wantCurrent: "",
		},
		{
			name:        "diff with flag",
			words:       []string{"diff", "--cached", ""},
			wantCmd:     "diff",
			wantFlags:   []string{"--cached"},
			wantCurrent: "",
		},
		{
			name:         "after separator",
			words:        []string{"diff", "main", "--", ""},
			wantCmd:      "diff",
			wantRefs:     []string{"main"},
			wantAfterSep: true,
			wantCurrent:  "",
		},
		{
			name:          "flag expecting value (--since)",
			words:         []string{"branch", "--since", ""},
			wantCmd:       "branch",
			wantFlags:     []string{"--since"},
			wantCurrent:   "",
			wantFlagValue: "--since",
		},
		{
			name:          "flag expecting value (-e)",
			words:         []string{"diff", "-e", ""},
			wantCmd:       "diff",
			wantFlags:     []string{"-e"},
			wantCurrent:   "",
			wantFlagValue: "-e",
		},
		{
			name:          "flag expecting value (-n)",
			words:         []string{"log", "-n", "2"},
			wantCmd:       "log",
			wantFlags:     []string{"-n"},
			wantCurrent:   "2",
			wantFlagValue: "-n",
		},
		{
			name:        "flag value already consumed",
			words:       []string{"branch", "--since", "30d", ""},
			wantCmd:     "branch",
			wantFlags:   []string{"--since"},
			wantCurrent: "",
		},
		{
			name:        "two refs for diff",
			words:       []string{"diff", "main", "feature", ""},
			wantCmd:     "diff",
			wantRefs:    []string{"main", "feature"},
			wantCurrent: "",
		},
		{
			name:        "partial flag",
			words:       []string{"diff", "--cac"},
			wantCmd:     "diff",
			wantCurrent: "--cac",
		},
		{
			name:        "inline equals",
			words:       []string{"branch", "--since="},
			wantCmd:     "branch",
			wantCurrent: "--since=",
		},
		{
			name:        "no subcommand with ref",
			words:       []string{"main", ""},
			wantRefs:    []string{"main"},
			wantCurrent: "",
		},
		{
			name:        "help subcommand",
			words:       []string{"help", ""},
			wantCmd:     "help",
			wantCurrent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := parseCompletionContext(tt.words)
			if ctx.cmd != tt.wantCmd {
				t.Errorf("cmd = %q, want %q", ctx.cmd, tt.wantCmd)
			}
			if !slices.Equal(ctx.flags, tt.wantFlags) {
				t.Errorf("flags = %v, want %v", ctx.flags, tt.wantFlags)
			}
			if !slices.Equal(ctx.refs, tt.wantRefs) {
				t.Errorf("refs = %v, want %v", ctx.refs, tt.wantRefs)
			}
			if ctx.afterSep != tt.wantAfterSep {
				t.Errorf("afterSep = %v, want %v", ctx.afterSep, tt.wantAfterSep)
			}
			if ctx.current != tt.wantCurrent {
				t.Errorf("current = %q, want %q", ctx.current, tt.wantCurrent)
			}
			if ctx.expectFlagValue != tt.wantFlagValue {
				t.Errorf("expectFlagValue = %q, want %q", ctx.expectFlagValue, tt.wantFlagValue)
			}
		})
	}
}

func TestGenerateCompletions(t *testing.T) {
	mockGit := &git.MockGit{
		Branches: []git.BranchInfo{
			{Name: "main"},
			{Name: "feature/login"},
			{Name: "fix/bug-123"},
		},
		TagNames: []string{"v1.0.0", "v2.0.0"},
	}

	mockCommentIDs := func() []string {
		return []string{"1739380234567", "1739380299000", "1739400000000"}
	}

	tests := []struct {
		name        string
		words       []string
		commentIDs  commentIDsFunc
		wantContain []string
		wantAbsent  []string
	}{
		{
			name:        "empty: subcommands",
			words:       []string{""},
			wantContain: []string{"diff", "show", "log", "branch", "status", "config", "comment"},
		},
		{
			name:        "partial subcommand",
			words:       []string{"di"},
			wantContain: []string{"diff"},
			wantAbsent:  []string{"show", "log"},
		},
		{
			name:        "diff flags",
			words:       []string{"diff", "--"},
			wantContain: []string{"--cached", "--staged", "--all", "--exclude"},
			wantAbsent:  []string{"--verbose", "--since", "--init"},
		},
		{
			name:        "branches flags",
			words:       []string{"branch", "--"},
			wantContain: []string{"--verbose", "--since"},
			wantAbsent:  []string{"--cached", "--all"},
		},
		{
			name:        "config flags",
			words:       []string{"config", "--"},
			wantContain: []string{"--init", "--force", "--print", "--path", "--edit"},
			wantAbsent:  []string{"--cached"},
		},
		{
			name:        "already used flag excluded",
			words:       []string{"diff", "--cached", "--"},
			wantContain: []string{"--all"},
			wantAbsent:  []string{"--cached", "--staged"}, // --staged is alias of --cached
		},
		{
			name:        "ref completion with prefix",
			words:       []string{"diff", "ma"},
			wantContain: []string{"main"},
			wantAbsent:  []string{"feature/login", "fix/bug-123"},
		},
		{
			name:        "ref completion includes tags",
			words:       []string{"show", "v"},
			wantContain: []string{"v1.0.0", "v2.0.0"},
		},
		{
			name:        "show max 1 ref then flags",
			words:       []string{"show", "main", ""},
			wantContain: []string{"--help", "--debug"},
			wantAbsent:  []string{"main", "feature/login"}, // no more refs
		},
		{
			name:        "diff max 2 refs then flags",
			words:       []string{"diff", "main", "feature/login", ""},
			wantContain: []string{"--help"},
			wantAbsent:  []string{"main", "v1.0.0"}, // no more refs
		},
		{
			name:        "after separator: empty (file completion)",
			words:       []string{"diff", "--", ""},
			wantContain: nil,
		},
		{
			name:        "flag value --since",
			words:       []string{"branch", "--since", ""},
			wantContain: []string{"30m", "7d", "2w", "3M", "1y", "all"},
		},
		{
			name:        "flag value --since with prefix",
			words:       []string{"branch", "--since", "3"},
			wantContain: []string{"30m", "3M"},
			wantAbsent:  []string{"7d", "2w", "1y", "all"},
		},
		{
			name:        "inline --since=value",
			words:       []string{"branch", "--since="},
			wantContain: []string{"--since=7d", "--since=2w", "--since=30m"},
		},
		{
			name:        "inline --since=partial",
			words:       []string{"branch", "--since=3"},
			wantContain: []string{"--since=30m", "--since=3M"},
			wantAbsent:  []string{"--since=7d"},
		},
		{
			name:        "help completes subcommands",
			words:       []string{"help", ""},
			wantContain: []string{"diff", "show", "log", "branch", "comment"},
		},
		{
			name:        "help partial",
			words:       []string{"help", "b"},
			wantContain: []string{"branch"},
			wantAbsent:  []string{"diff", "show"},
		},
		{
			name:        "completion completes shells",
			words:       []string{"completion", ""},
			wantContain: []string{"bash", "zsh", "fish"},
		},
		{
			name:        "status flags",
			words:       []string{"status", "--"},
			wantContain: []string{"--symbols", "--untracked-files", "--branches"},
			wantAbsent:  []string{"--cached", "--since"},
		},
		{
			name:        "comment sub-subcommands",
			words:       []string{"comment", ""},
			wantContain: []string{"list", "edit"},
			wantAbsent:  []string{"diff", "show"},
		},
		{
			name:        "comment sub-subcommand partial",
			words:       []string{"comment", "l"},
			wantContain: []string{"list"},
			wantAbsent:  []string{"edit"},
		},
		{
			name:        "comment flags after sub-subcommand",
			words:       []string{"comment", "list", "--"},
			wantContain: []string{"--since", "--status", "--verbose", "--all-branches"},
			wantAbsent:  []string{"--cached", "--oneline"},
		},
		{
			name:        "comment --status value",
			words:       []string{"comment", "list", "--status", ""},
			wantContain: []string{"unresolved", "resolved", "all"},
		},
		{
			name:        "comment inline --status=",
			words:       []string{"comment", "list", "--status="},
			wantContain: []string{"--status=unresolved", "--status=resolved", "--status=all"},
		},
		{
			name:        "comment edit completes IDs",
			words:       []string{"comment", "edit", ""},
			commentIDs:  mockCommentIDs,
			wantContain: []string{"1739380234567", "1739380299000", "1739400000000"},
		},
		{
			name:        "comment edit ID with prefix",
			words:       []string{"comment", "edit", "17394"},
			commentIDs:  mockCommentIDs,
			wantContain: []string{"1739400000000"},
			wantAbsent:  []string{"1739380234567", "1739380299000"},
		},
		{
			name:       "comment edit no IDs without store",
			words:      []string{"comment", "edit", ""},
			commentIDs: nil,
			wantAbsent: []string{"1739380234567"},
		},
		{
			name:        "comment resolve completes IDs",
			words:       []string{"comment", "resolve", ""},
			commentIDs:  mockCommentIDs,
			wantContain: []string{"1739380234567", "1739380299000", "1739400000000"},
		},
		{
			name:        "comment unresolve completes IDs",
			words:       []string{"comment", "unresolve", ""},
			commentIDs:  mockCommentIDs,
			wantContain: []string{"1739380234567", "1739380299000", "1739400000000"},
		},
		{
			name:        "no subcommand matches subcommands only",
			words:       []string{"co"},
			wantContain: []string{"comment", "config", "completion"},
			wantAbsent:  []string{"main", "diff"},
		},
		{
			name:        "log flags",
			words:       []string{"log", "--"},
			wantContain: []string{"--exclude", "--since"},
			wantAbsent:  []string{"--cached", "--verbose"},
		},
		{
			name:       "flag value --untracked-files",
			words:      []string{"status", "--untracked-files", ""},
			wantAbsent: []string{"no", "normal", "all"},
			// --untracked-files is not in flagTakesValue (it uses = syntax),
			// so this completes as a positional, which for status means flags.
		},
		{
			name:        "inline --untracked-files=",
			words:       []string{"status", "--untracked-files="},
			wantContain: []string{"--untracked-files=no", "--untracked-files=normal", "--untracked-files=all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := parseCompletionContext(tt.words)
			got := generateCompletions(ctx, mockGit, tt.commentIDs)

			for _, want := range tt.wantContain {
				if !slices.Contains(got, want) {
					t.Errorf("missing expected candidate %q in %v", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if slices.Contains(got, absent) {
					t.Errorf("unexpected candidate %q in %v", absent, got)
				}
			}
		})
	}
}

func TestFilterPrefix(t *testing.T) {
	items := []string{"alpha", "beta", "gamma", "alphabet"}
	got := filterPrefix(items, "alp")
	want := []string{"alpha", "alphabet"}
	if !slices.Equal(got, want) {
		t.Errorf("filterPrefix = %v, want %v", got, want)
	}
}

func TestFlagAliases(t *testing.T) {
	// Verify symmetry: if A aliases B, B aliases A.
	pairs := [][2]string{
		{"--cached", "--staged"},
		{"-a", "--all"},
		{"-e", "--exclude"},
		{"-v", "--verbose"},
		{"-S", "--symbols"},
		{"-u", "--untracked-files"},
		{"-b", "--branches"},
		{"-h", "--help"},
	}
	for _, pair := range pairs {
		a, b := pair[0], pair[1]
		if !slices.Contains(flagAliases(a), b) {
			t.Errorf("flagAliases(%q) should contain %q", a, b)
		}
		if !slices.Contains(flagAliases(b), a) {
			t.Errorf("flagAliases(%q) should contain %q", b, a)
		}
	}
}

func TestCompleteFlagValue(t *testing.T) {
	// --since returns duration suggestions
	got := completeFlagValue("--since", "")
	if len(got) == 0 {
		t.Error("--since should return suggestions")
	}

	// --exclude returns nothing (dynamic)
	got = completeFlagValue("--exclude", "")
	if len(got) != 0 {
		t.Errorf("--exclude should return nil, got %v", got)
	}

	// unknown flag returns nothing
	got = completeFlagValue("--unknown", "")
	if len(got) != 0 {
		t.Errorf("unknown flag should return nil, got %v", got)
	}
}

func TestListRefsNilGit(t *testing.T) {
	// Should not panic with nil git.
	refs := listRefs(nil)
	if len(refs) != 0 {
		t.Errorf("expected no refs with nil git, got %v", refs)
	}
}
