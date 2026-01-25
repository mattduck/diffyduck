package structure

// ChangeKind represents how a structural element changed between old and new versions.
type ChangeKind int

const (
	ChangeUnchanged ChangeKind = iota // Element exists in both, no diff lines overlap
	ChangeAdded                       // Element only in new
	ChangeDeleted                     // Element only in old
	ChangeModified                    // Element exists in both, diff lines overlap
)

// String returns a human-readable name for the change kind.
func (k ChangeKind) String() string {
	switch k {
	case ChangeUnchanged:
		return "unchanged"
	case ChangeAdded:
		return "added"
	case ChangeDeleted:
		return "deleted"
	case ChangeModified:
		return "modified"
	default:
		return "unknown"
	}
}

// Symbol returns a single-character symbol for the change kind.
func (k ChangeKind) Symbol() string {
	switch k {
	case ChangeUnchanged:
		return " "
	case ChangeAdded:
		return "+"
	case ChangeDeleted:
		return "-"
	case ChangeModified:
		return "~"
	default:
		return "?"
	}
}

// ElementChange represents a structural element and how it changed.
type ElementChange struct {
	Kind         ChangeKind
	OldEntry     *Entry // nil for Added
	NewEntry     *Entry // nil for Deleted
	LinesAdded   int    // Number of added lines within this element's boundaries
	LinesRemoved int    // Number of removed lines within this element's boundaries
}

// Name returns the element's name (from whichever entry is available).
func (c ElementChange) Name() string {
	if c.NewEntry != nil {
		return c.NewEntry.Name
	}
	if c.OldEntry != nil {
		return c.OldEntry.Name
	}
	return ""
}

// Entry returns the primary entry (NewEntry if available, else OldEntry).
func (c ElementChange) Entry() *Entry {
	if c.NewEntry != nil {
		return c.NewEntry
	}
	return c.OldEntry
}

// StructuralDiff contains the diff result for a file's structural elements.
type StructuralDiff struct {
	Changes []ElementChange
}

// HasChanges returns true if there are any added, deleted, or modified elements.
func (d *StructuralDiff) HasChanges() bool {
	for _, c := range d.Changes {
		if c.Kind != ChangeUnchanged {
			return true
		}
	}
	return false
}

// ChangedOnly returns only the elements that were added, deleted, or modified.
func (d *StructuralDiff) ChangedOnly() []ElementChange {
	var result []ElementChange
	for _, c := range d.Changes {
		if c.Kind != ChangeUnchanged {
			result = append(result, c)
		}
	}
	return result
}

// ComputeDiff compares old and new structure maps and returns a structural diff.
// addedLines contains 1-based line numbers that were added in the new file.
// removedLines contains 1-based line numbers that were removed from the old file.
func ComputeDiff(oldMap, newMap *Map, addedLines, removedLines map[int]bool) *StructuralDiff {
	diff := &StructuralDiff{}

	// Handle nil maps
	if oldMap == nil && newMap == nil {
		return diff
	}

	// Build name -> entry maps for matching
	oldByName := make(map[string]*Entry)
	newByName := make(map[string]*Entry)

	if oldMap != nil {
		for i := range oldMap.Entries {
			e := &oldMap.Entries[i]
			key := entryKey(e)
			oldByName[key] = e
		}
	}

	if newMap != nil {
		for i := range newMap.Entries {
			e := &newMap.Entries[i]
			key := entryKey(e)
			newByName[key] = e
		}
	}

	// Track which old entries have been matched
	matched := make(map[string]bool)

	// Process new entries: check if they exist in old
	if newMap != nil {
		for i := range newMap.Entries {
			newEntry := &newMap.Entries[i]
			key := entryKey(newEntry)

			if oldEntry, ok := oldByName[key]; ok {
				// Matched: count lines changed within this element
				matched[key] = true

				linesRemoved := countOverlap(oldEntry.StartLine, oldEntry.EndLine, removedLines)
				linesAdded := countOverlap(newEntry.StartLine, newEntry.EndLine, addedLines)

				kind := ChangeUnchanged
				if linesRemoved > 0 || linesAdded > 0 {
					kind = ChangeModified
				}

				diff.Changes = append(diff.Changes, ElementChange{
					Kind:         kind,
					OldEntry:     oldEntry,
					NewEntry:     newEntry,
					LinesAdded:   linesAdded,
					LinesRemoved: linesRemoved,
				})
			} else {
				// Only in new: added - count all added lines within its range
				linesAdded := countOverlap(newEntry.StartLine, newEntry.EndLine, addedLines)
				diff.Changes = append(diff.Changes, ElementChange{
					Kind:       ChangeAdded,
					NewEntry:   newEntry,
					LinesAdded: linesAdded,
				})
			}
		}
	}

	// Process old entries that weren't matched: deleted
	if oldMap != nil {
		for i := range oldMap.Entries {
			oldEntry := &oldMap.Entries[i]
			key := entryKey(oldEntry)

			if !matched[key] {
				// Deleted: count all removed lines within its range
				linesRemoved := countOverlap(oldEntry.StartLine, oldEntry.EndLine, removedLines)
				diff.Changes = append(diff.Changes, ElementChange{
					Kind:         ChangeDeleted,
					OldEntry:     oldEntry,
					LinesRemoved: linesRemoved,
				})
			}
		}
	}

	return diff
}

// entryKey returns a unique key for matching entries between old and new.
// For methods, includes the receiver type to distinguish methods on different types.
func entryKey(e *Entry) string {
	if e.Receiver != "" {
		return e.Kind + ":" + e.Receiver + "." + e.Name
	}
	return e.Kind + ":" + e.Name
}

// countOverlap returns the number of lines in the range [start, end] that are in the lines set.
func countOverlap(start, end int, lines map[int]bool) int {
	if lines == nil {
		return 0
	}
	count := 0
	for line := start; line <= end; line++ {
		if lines[line] {
			count++
		}
	}
	return count
}
