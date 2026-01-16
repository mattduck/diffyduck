package git

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockGit_Show(t *testing.T) {
	mock := &MockGit{
		ShowOutput: "diff --git a/foo.go b/foo.go\n",
	}

	out, err := mock.Show("HEAD")
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/foo.go b/foo.go\n", out)
}

func TestMockGit_ShowError(t *testing.T) {
	mock := &MockGit{
		ShowError: errors.New("not a git repo"),
	}

	_, err := mock.Show("HEAD")
	require.Error(t, err)
	assert.Equal(t, "not a git repo", err.Error())
}

func TestMockGit_Diff(t *testing.T) {
	mock := &MockGit{
		DiffOutput: "diff --git a/bar.go b/bar.go\n",
	}

	out, err := mock.Diff()
	require.NoError(t, err)
	assert.Equal(t, "diff --git a/bar.go b/bar.go\n", out)
}

func TestMockGit_DiffWithArgs(t *testing.T) {
	mock := &MockGit{
		DiffOutput: "cached diff output",
	}

	out, err := mock.Diff("--cached")
	require.NoError(t, err)
	assert.Equal(t, "cached diff output", out)
}

func TestMockGit_DiffError(t *testing.T) {
	mock := &MockGit{
		DiffError: errors.New("not a git repo"),
	}

	_, err := mock.Diff()
	require.Error(t, err)
	assert.Equal(t, "not a git repo", err.Error())
}

func TestGitInterface(t *testing.T) {
	// Verify both implementations satisfy the interface
	var _ Git = &RealGit{}
	var _ Git = &MockGit{}
}
