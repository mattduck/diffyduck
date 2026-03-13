// Package movedetect identifies blocks of code that were moved (deleted from
// one location and added to another) across all files in a diff. It works by
// collecting pure-remove and pure-add lines, normalising away leading
// whitespace (so re-indented moves are still detected), and finding matching
// consecutive sequences of at least MinBlock lines.
package movedetect

import (
	"hash/fnv"
	"strings"

	"github.com/user/diffyduck/pkg/sidebyside"
)

// Key identifies one side of a specific line pair in the diff.
type Key struct {
	FileIndex int
	PairIndex int
	Side      int // 0 = new/left (adds), 1 = old/right (removes)
}

// Result holds the output of move detection.
// Groups maps each matched line to a 1-based group ID.
// Lines in the same group were moved together.
type Result struct {
	Groups   map[Key]int
	MaxGroup int // highest group ID assigned
}

// lineRef is a collected line from one side of the diff.
type lineRef struct {
	fileIndex int
	pairIndex int
	norm      string // content with leading whitespace stripped
	hash      uint64
}

// Detect scans all files for moved blocks of at least minBlock consecutive
// lines. It collects all removed and added lines — both pure (one side empty)
// and from change pairs (both sides have content). Change-pair lines are
// included because a moved block can span across the boundary between pure
// and paired regions when the diff transform coincidentally pairs some moved
// lines with unrelated content.
//
// fileIndexOffset is added to each file's slice index when building Keys,
// so callers can pass a sub-slice of a larger files array and still get
// globally-correct FileIndex values in the result.
func Detect(files []sidebyside.FilePair, minBlock, fileIndexOffset int) *Result {
	if minBlock < 1 {
		minBlock = 3
	}

	var removes, adds []lineRef

	for fi, file := range files {
		globalFI := fi + fileIndexOffset
		for pi, pair := range file.Pairs {
			if pair.Old.Type == sidebyside.Removed {
				norm := strings.TrimLeft(pair.Old.Content, " \t")
				removes = append(removes, lineRef{globalFI, pi, norm, hashLine(norm)})
			}
			if pair.New.Type == sidebyside.Added {
				norm := strings.TrimLeft(pair.New.Content, " \t")
				adds = append(adds, lineRef{globalFI, pi, norm, hashLine(norm)})
			}
		}
	}

	if len(removes) == 0 || len(adds) == 0 {
		return &Result{Groups: nil}
	}

	// Build index: hash -> positions in adds slice
	addIdx := make(map[uint64][]int, len(adds))
	for i, a := range adds {
		addIdx[a.hash] = append(addIdx[a.hash], i)
	}

	usedRem := make([]bool, len(removes))
	usedAdd := make([]bool, len(adds))
	groups := make(map[Key]int)
	groupID := 0
	// contentGroup maps a block's normalized content signature to a group ID
	// so that identical moved blocks share the same color.
	contentGroup := make(map[string]int)

	for ri := 0; ri < len(removes); ri++ {
		if usedRem[ri] {
			continue
		}
		// Skip blank/trivial lines as block starters — they produce
		// too many false-positive matches.
		if removes[ri].norm == "" {
			continue
		}

		candidates := addIdx[removes[ri].hash]
		bestAI := -1
		bestLen := 0

		// Find the longest matching block starting at this remove position.
		for _, ai := range candidates {
			if usedAdd[ai] {
				continue
			}
			if adds[ai].norm != removes[ri].norm {
				continue
			}
			matchLen := extendMatch(removes, adds, ri, ai, usedRem, usedAdd)
			if matchLen > bestLen {
				bestLen = matchLen
				bestAI = ai
			}
		}

		if bestLen >= minBlock {
			// Build content signature from normalized lines.
			sig := blockSignature(removes, ri, bestLen)
			gid, seen := contentGroup[sig]
			if !seen {
				groupID++
				gid = groupID
				contentGroup[sig] = gid
			}
			for k := 0; k < bestLen; k++ {
				usedRem[ri+k] = true
				usedAdd[bestAI+k] = true
				r := removes[ri+k]
				a := adds[bestAI+k]
				groups[Key{r.fileIndex, r.pairIndex, 1}] = gid
				groups[Key{a.fileIndex, a.pairIndex, 0}] = gid
			}
		}
	}

	return &Result{Groups: groups, MaxGroup: groupID}
}

// extendMatch returns how many consecutive lines match starting at ri in
// removes and ai in adds. Lines must be consecutive within the same file
// (adjacent pair indices).
func extendMatch(removes, adds []lineRef, ri, ai int, usedRem, usedAdd []bool) int {
	n := 1
	for {
		rn := ri + n
		an := ai + n
		if rn >= len(removes) || an >= len(adds) {
			break
		}
		if usedRem[rn] || usedAdd[an] {
			break
		}
		if !consecutive(removes[rn-1], removes[rn]) {
			break
		}
		if !consecutive(adds[an-1], adds[an]) {
			break
		}
		if removes[rn].norm != adds[an].norm {
			break
		}
		n++
	}
	return n
}

// consecutive returns true if b immediately follows a in the same file.
func consecutive(a, b lineRef) bool {
	return a.fileIndex == b.fileIndex && b.pairIndex == a.pairIndex+1
}

// blockSignature returns a string key for the normalized content of a block,
// used to give identical moved blocks the same group ID.
func blockSignature(refs []lineRef, start, length int) string {
	var b strings.Builder
	for i := 0; i < length; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(refs[start+i].norm)
	}
	return b.String()
}

func hashLine(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}
