package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/user/diffyduck/pkg/sidebyside"
)

func TestIsConflictMarkerLine(t *testing.T) {
	tests := []struct {
		name string
		line sidebyside.Line
		want bool
	}{
		{
			name: "conflict start marker",
			line: sidebyside.Line{Type: sidebyside.Added, Content: "<<<<<<< HEAD"},
			want: true,
		},
		{
			name: "conflict separator",
			line: sidebyside.Line{Type: sidebyside.Added, Content: "======="},
			want: true,
		},
		{
			name: "conflict end marker",
			line: sidebyside.Line{Type: sidebyside.Added, Content: ">>>>>>> feature-branch"},
			want: true,
		},
		{
			name: "normal added line",
			line: sidebyside.Line{Type: sidebyside.Added, Content: "some code here"},
			want: false,
		},
		{
			name: "context line with marker content is not conflict",
			line: sidebyside.Line{Type: sidebyside.Context, Content: "<<<<<<< HEAD"},
			want: false,
		},
		{
			name: "removed line with marker content is not conflict",
			line: sidebyside.Line{Type: sidebyside.Removed, Content: "<<<<<<< HEAD"},
			want: false,
		},
		{
			name: "separator with trailing text is not conflict",
			line: sidebyside.Line{Type: sidebyside.Added, Content: "======= some text"},
			want: false,
		},
		{
			name: "empty line is not conflict",
			line: sidebyside.Line{Type: sidebyside.Added, Content: ""},
			want: false,
		},
		{
			name: "partial marker is not conflict",
			line: sidebyside.Line{Type: sidebyside.Added, Content: "<<<<<"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isConflictMarkerLine(tt.line))
		})
	}
}

func TestMarkConflictBlocks(t *testing.T) {
	mkRow := func(content string) displayRow {
		return displayRow{
			kind: RowKindContent,
			pair: sidebyside.LinePair{
				New: sidebyside.Line{Type: sidebyside.Added, Content: content},
			},
		}
	}

	t.Run("marks zones correctly", func(t *testing.T) {
		rows := []displayRow{
			mkRow("normal line before"),
			mkRow("<<<<<<< HEAD"),
			mkRow("our code"),
			mkRow("======="),
			mkRow("their code"),
			mkRow(">>>>>>> feature"),
			mkRow("normal line after"),
		}
		markConflictBlocks(rows, true)

		assert.Equal(t, conflictNone, rows[0].conflictZone)
		assert.Equal(t, conflictOurs, rows[1].conflictZone)      // <<<<<<< HEAD
		assert.Equal(t, conflictOurs, rows[2].conflictZone)      // our code
		assert.Equal(t, conflictSeparator, rows[3].conflictZone) // =======
		assert.Equal(t, conflictTheirs, rows[4].conflictZone)    // their code
		assert.Equal(t, conflictTheirs, rows[5].conflictZone)    // >>>>>>> (still theirs on this line)
		assert.Equal(t, conflictNone, rows[6].conflictZone)      // after block
	})

	t.Run("handles multiple conflict blocks", func(t *testing.T) {
		rows := []displayRow{
			mkRow("<<<<<<< HEAD"),
			mkRow("======="),
			mkRow(">>>>>>> branch"),
			mkRow("gap"),
			mkRow("<<<<<<< HEAD"),
			mkRow("second ours"),
			mkRow("======="),
			mkRow("second theirs"),
			mkRow(">>>>>>> branch"),
		}
		markConflictBlocks(rows, true)

		assert.Equal(t, conflictOurs, rows[0].conflictZone)
		assert.Equal(t, conflictSeparator, rows[1].conflictZone)
		assert.Equal(t, conflictTheirs, rows[2].conflictZone)
		assert.Equal(t, conflictNone, rows[3].conflictZone)
		assert.Equal(t, conflictOurs, rows[4].conflictZone)
		assert.Equal(t, conflictOurs, rows[5].conflictZone)
		assert.Equal(t, conflictSeparator, rows[6].conflictZone)
		assert.Equal(t, conflictTheirs, rows[7].conflictZone)
		assert.Equal(t, conflictTheirs, rows[8].conflictZone)
	})

	t.Run("skips non-content rows", func(t *testing.T) {
		rows := []displayRow{
			mkRow("<<<<<<< HEAD"),
			{kind: RowKindSeparator}, // hunk separator mid-block
			mkRow("our code"),
			mkRow("======="),
			mkRow(">>>>>>> branch"),
		}
		markConflictBlocks(rows, true)

		assert.Equal(t, conflictOurs, rows[0].conflictZone)
		assert.Equal(t, conflictNone, rows[1].conflictZone) // non-content row untouched
		assert.Equal(t, conflictOurs, rows[2].conflictZone)
		assert.Equal(t, conflictSeparator, rows[3].conflictZone)
		assert.Equal(t, conflictTheirs, rows[4].conflictZone)
	})

	t.Run("noop when hasConflicts is false", func(t *testing.T) {
		rows := []displayRow{
			mkRow("<<<<<<< HEAD"),
			mkRow("======="),
			mkRow(">>>>>>> branch"),
		}
		markConflictBlocks(rows, false)

		assert.Equal(t, conflictNone, rows[0].conflictZone)
		assert.Equal(t, conflictNone, rows[1].conflictZone)
		assert.Equal(t, conflictNone, rows[2].conflictZone)
	})
}
