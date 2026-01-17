package highlight

// Span represents a highlighted range in source code.
type Span struct {
	Start    int      // byte offset from start of content
	End      int      // byte offset from start of content (exclusive)
	Category Category // semantic category for this span
}

// SpansForLine extracts spans that overlap with a specific line.
// lineStart and lineEnd are byte offsets for the line (including newline if present).
// The returned spans have offsets adjusted to be relative to lineStart.
func SpansForLine(spans []Span, lineStart, lineEnd int) []Span {
	var result []Span
	for _, s := range spans {
		// Skip spans entirely before this line
		if s.End <= lineStart {
			continue
		}
		// Stop once we're past this line
		if s.Start >= lineEnd {
			break
		}
		// Clip span to line boundaries and adjust to line-relative offsets
		start := max(0, s.Start-lineStart)
		end := min(lineEnd-lineStart, s.End-lineStart)
		result = append(result, Span{
			Start:    start,
			End:      end,
			Category: s.Category,
		})
	}
	return result
}

// MergeSpans combines overlapping spans, preferring the later span's category.
// Input spans should be sorted by Start position.
func MergeSpans(spans []Span) []Span {
	if len(spans) == 0 {
		return nil
	}

	result := make([]Span, 0, len(spans))
	current := spans[0]

	for i := 1; i < len(spans); i++ {
		next := spans[i]
		if next.Start >= current.End {
			// No overlap, emit current and move on
			result = append(result, current)
			current = next
		} else {
			// Overlap: later span wins for the overlapping region
			if next.Start > current.Start {
				// Emit the non-overlapping part of current
				result = append(result, Span{
					Start:    current.Start,
					End:      next.Start,
					Category: current.Category,
				})
			}
			// The overlapping region uses next's category
			if next.End > current.End {
				current = next
			} else {
				// next is contained within current
				// Emit next, then continue with remainder of current
				result = append(result, next)
				if next.End < current.End {
					current = Span{
						Start:    next.End,
						End:      current.End,
						Category: current.Category,
					}
				} else {
					// next.End == current.End, need to get next span
					if i+1 < len(spans) {
						current = spans[i+1]
						i++
					} else {
						current = Span{} // Will be skipped
					}
				}
			}
		}
	}

	if current.End > current.Start {
		result = append(result, current)
	}

	return result
}
