// Package inlinediff provides word-level diff highlighting for changed lines.
package inlinediff

// SpanType indicates whether a span of text is unchanged, added, or removed.
type SpanType int

const (
	Unchanged SpanType = iota
	Added
	Removed
)

// Span represents a contiguous range of characters with the same diff status.
type Span struct {
	Start int      // Start byte offset (inclusive)
	End   int      // End byte offset (exclusive)
	Type  SpanType // Whether this span is unchanged, added, or removed
}

// MinUnchangedGap is the minimum size of an unchanged span to keep it separate.
// Smaller unchanged gaps between changed spans get merged into the changed region.
const MinUnchangedGap = 3

// Diff compares two strings and returns spans indicating which parts changed.
// oldSpans describes the old string, newSpans describes the new string.
// Unchanged spans have the same content in both strings.
// Uses word-level diffing for more intuitive results.
func Diff(old, new string) (oldSpans, newSpans []Span) {
	if old == "" && new == "" {
		return nil, nil
	}
	if old == "" {
		return nil, []Span{{Start: 0, End: len(new), Type: Added}}
	}
	if new == "" {
		return []Span{{Start: 0, End: len(old), Type: Removed}}, nil
	}
	if old == new {
		return []Span{{Start: 0, End: len(old), Type: Unchanged}},
			[]Span{{Start: 0, End: len(new), Type: Unchanged}}
	}

	// Tokenize into words (preserving positions)
	oldTokens := tokenize(old)
	newTokens := tokenize(new)

	// Compute LCS on tokens
	lcs := computeTokenLCS(oldTokens, newTokens)

	// Build spans from token matching
	oldSpans = buildTokenSpans(old, oldTokens, lcs, Removed)
	newSpans = buildTokenSpans(new, newTokens, lcs, Added)

	// Merge small unchanged gaps between changed spans
	oldSpans = mergeSmallGaps(oldSpans, Removed)
	newSpans = mergeSmallGaps(newSpans, Added)

	return oldSpans, newSpans
}

// token represents a word or whitespace segment with its position
type token struct {
	text  string
	start int // byte offset in original string
	end   int // byte offset (exclusive)
}

// tokenize splits a string into tokens (words and whitespace runs)
func tokenize(s string) []token {
	var tokens []token
	start := 0
	inWord := false

	for i, r := range s {
		isWordChar := isWord(r)

		if i == 0 {
			inWord = isWordChar
			continue
		}

		// Transition detected
		if isWordChar != inWord {
			tokens = append(tokens, token{
				text:  s[start:i],
				start: start,
				end:   i,
			})
			start = i
			inWord = isWordChar
		}
	}

	// Don't forget the last token
	if start < len(s) {
		tokens = append(tokens, token{
			text:  s[start:],
			start: start,
			end:   len(s),
		})
	}

	return tokens
}

// isWord returns true if the rune is part of a word (not whitespace/punctuation)
func isWord(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '_'
}

// computeTokenLCS finds the longest common subsequence of token slices
func computeTokenLCS(a, b []token) []string {
	m, n := len(a), len(b)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1].text == b[j-1].text {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Backtrack
	lcs := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1].text == b[j-1].text {
			lcs = append(lcs, a[i-1].text)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// Reverse
	for i, j := 0, len(lcs)-1; i < j; i, j = i+1, j-1 {
		lcs[i], lcs[j] = lcs[j], lcs[i]
	}

	return lcs
}

// buildTokenSpans creates spans based on which tokens are in the LCS
func buildTokenSpans(_ string, tokens []token, lcs []string, changeType SpanType) []Span {
	var spans []Span
	var currentSpan *Span

	lcsIdx := 0

	for _, tok := range tokens {
		inLCS := lcsIdx < len(lcs) && tok.text == lcs[lcsIdx]

		var spanType SpanType
		if inLCS {
			spanType = Unchanged
			lcsIdx++
		} else {
			spanType = changeType
		}

		// Extend current span or start new one
		if currentSpan != nil && currentSpan.Type == spanType {
			currentSpan.End = tok.end
		} else {
			if currentSpan != nil {
				spans = append(spans, *currentSpan)
			}
			currentSpan = &Span{Start: tok.start, End: tok.end, Type: spanType}
		}
	}

	if currentSpan != nil {
		spans = append(spans, *currentSpan)
	}

	return spans
}

// mergeSmallGaps merges changed spans that are separated by small unchanged gaps.
// This reduces visual noise when LCS finds scattered common characters.
func mergeSmallGaps(spans []Span, changeType SpanType) []Span {
	if len(spans) <= 1 {
		return spans
	}

	var result []Span
	i := 0

	for i < len(spans) {
		current := spans[i]

		// If this is a changed span, look ahead for small gaps to merge
		if current.Type == changeType {
			// Extend this span by absorbing small unchanged gaps and subsequent changed spans
			for i+2 < len(spans) {
				gap := spans[i+1]
				next := spans[i+2]

				// Check if: gap is small unchanged AND next is same change type
				gapSize := gap.End - gap.Start
				if gap.Type == Unchanged && gapSize < MinUnchangedGap && next.Type == changeType {
					// Merge: extend current span to end of next
					current.End = next.End
					i += 2 // Skip gap and next, continue looking
				} else {
					break
				}
			}
		}

		result = append(result, current)
		i++
	}

	return result
}

// ShouldSkipInlineDiff returns true if the lines are too different for
// inline diff to be useful (would just highlight everything).
// Uses word-level similarity ratio.
func ShouldSkipInlineDiff(old, new string) bool {
	if old == "" || new == "" {
		return false // Show additions/deletions
	}

	oldTokens := tokenize(old)
	newTokens := tokenize(new)

	// Only count actual words, not whitespace
	oldWords := filterWords(oldTokens)
	newWords := filterWords(newTokens)

	if len(oldWords) == 0 || len(newWords) == 0 {
		return false
	}

	lcs := computeTokenLCS(oldWords, newWords)

	// Calculate similarity as ratio of common words to average word count
	avgLen := float64(len(oldWords)+len(newWords)) / 2
	similarity := float64(len(lcs)) / avgLen

	// Skip if less than 30% of words are common
	return similarity < 0.3
}

// filterWords returns only the word tokens (not whitespace/punctuation)
func filterWords(tokens []token) []token {
	var words []token
	for _, t := range tokens {
		// Check if token contains any word characters
		for _, r := range t.text {
			if isWord(r) {
				words = append(words, t)
				break
			}
		}
	}
	return words
}

// ShouldSkipBasedOnSpans returns true if the inline diff would highlight
// too much of the line to be useful.
func ShouldSkipBasedOnSpans(spans []Span, totalLen int) bool {
	if totalLen == 0 {
		return true
	}

	// Calculate how much is changed (not Unchanged)
	changedLen := 0
	for _, s := range spans {
		if s.Type != Unchanged {
			changedLen += s.End - s.Start
		}
	}

	// If more than 50% would be highlighted, skip inline diff
	return float64(changedLen)/float64(totalLen) > 0.5
}
