// Package scanner finds REVP annotations and NOREVP suppressions in source files.
package scanner

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	revpPrefix  = "REVP("
	norevpBare  = "NOREVP"
	norevpOpen  = "NOREVP("
	norevpClose = ")"
)

// Violation is a REVP annotation found in a source file.
type Violation struct {
	File    string
	Line    int // 1-based
	Code    string
	Message string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s:%d: REVP(%s) %s", v.File, v.Line, v.Code, v.Message)
}

// ScanFile scans a single file for REVP annotations, applying NOREVP suppressions.
// Returns violations that are not suppressed.
// Files with unknown extensions are skipped (returns nil, nil).
func ScanFile(path string) ([]Violation, error) {
	lang, ok := langForFile(filepath.Base(path))
	if !ok {
		return nil, nil
	}
	return scanFile(path, lang)
}

func scanFile(path string, lang language) ([]Violation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type rawViolation struct {
		line int
		code string
		msg  string
	}

	type suppress struct {
		line int    // line number the suppression targets
		code string // empty = all rules
	}

	var violations []rawViolation
	var suppressions []suppress

	scanner := bufio.NewScanner(f)
	lineNum := 0
	var prevLineNorevp *suppress // NOREVP on preceding line targeting next code line

	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)
		prefix := lang.linePrefix

		segments := commentSegments(text, prefix)
		if len(segments) == 0 {
			// No comment on this line. If we had a preceding-line NOREVP pending,
			// apply it to this line (the first non-blank, non-comment line).
			if prevLineNorevp != nil && trimmed != "" {
				prevLineNorevp.line = lineNum
				suppressions = append(suppressions, *prevLineNorevp)
				prevLineNorevp = nil
			}
			continue
		}

		for _, seg := range segments {
			// Parse REVP annotation from each comment segment.
			if v, ok := parseREVP(path, lineNum, seg.body); ok {
				violations = append(violations, rawViolation{lineNum, v.Code, v.Message})
			}

			// Parse NOREVP suppression from each comment segment.
			if sup, ok := parseNOREVP(seg.body); ok {
				if seg.isWholeLine {
					// Comment occupies the whole line (possibly with leading whitespace):
					// preceding-line suppression — apply to next non-blank, non-comment line.
					prevLineNorevp = &suppress{code: sup}
				} else {
					// Inline suppression — applies to this line.
					suppressions = append(suppressions, suppress{line: lineNum, code: sup})
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Build suppression lookup: line -> set of suppressed codes ("" = all).
	suppressed := make(map[int]map[string]bool)
	for _, s := range suppressions {
		if suppressed[s.line] == nil {
			suppressed[s.line] = make(map[string]bool)
		}
		suppressed[s.line][s.code] = true
	}

	var out []Violation
	for _, v := range violations {
		if isSuppressed(suppressed[v.line], v.code) {
			continue
		}
		out = append(out, Violation{File: path, Line: v.line, Code: v.code, Message: v.msg})
	}
	return out, nil
}

// isSuppressed returns true if the given code is suppressed by the set of
// suppression entries for a line. An empty-string key means "suppress all".
func isSuppressed(entries map[string]bool, code string) bool {
	if entries == nil {
		return false
	}
	return entries[""] || entries[code]
}

// commentSegment is a single comment occurrence found on a line.
type commentSegment struct {
	startIdx    int    // byte index of the prefix in the original line
	body        string // text after the prefix, trimmed
	isWholeLine bool   // true if nothing but whitespace precedes the first prefix
}

// commentSegments returns all comment segments on a line, in order.
// Each occurrence of prefix that is not inside a string is returned.
// We do not parse strings — this is intentionally simple; false positives
// from prefix-like content inside string literals are acceptable.
func commentSegments(line, prefix string) []commentSegment {
	var segs []commentSegment
	search := line
	offset := 0
	firstPrefix := true
	for {
		idx := strings.Index(search, prefix)
		if idx < 0 {
			break
		}
		absIdx := offset + idx
		body := strings.TrimSpace(search[idx+len(prefix):])
		isWholeLine := firstPrefix && strings.TrimSpace(line[:absIdx]) == ""
		segs = append(segs, commentSegment{
			startIdx:    absIdx,
			body:        body,
			isWholeLine: isWholeLine,
		})
		// Advance past this prefix to find subsequent ones.
		advance := idx + len(prefix)
		search = search[advance:]
		offset += advance
		firstPrefix = false
	}
	return segs
}

// parseREVP attempts to parse a REVP annotation from a comment body.
// comment is the text after the line-comment prefix, trimmed.
// Example input: "REVP(bare-dict): avoid using bare dict here"
func parseREVP(file string, line int, comment string) (Violation, bool) {
	if !strings.HasPrefix(comment, revpPrefix) {
		return Violation{}, false
	}
	rest := comment[len(revpPrefix):]
	closeIdx := strings.Index(rest, "):")
	if closeIdx < 0 {
		return Violation{}, false
	}
	code := strings.TrimSpace(rest[:closeIdx])
	msg := strings.TrimSpace(rest[closeIdx+2:])
	if code == "" {
		return Violation{}, false
	}
	return Violation{File: file, Line: line, Code: code, Message: msg}, true
}

// parseNOREVP attempts to parse a NOREVP suppression from a comment body.
// Returns the rule code (empty string = suppress all) and true on success.
// Examples: "NOREVP" -> ("", true), "NOREVP(bare-dict)" -> ("bare-dict", true)
func parseNOREVP(comment string) (string, bool) {
	if !strings.HasPrefix(comment, norevpBare) {
		return "", false
	}
	rest := comment[len(norevpBare):]
	// Bare NOREVP (possibly followed by whitespace or end of string).
	if rest == "" || strings.TrimSpace(rest) == "" {
		return "", true
	}
	// NOREVP(code)
	if strings.HasPrefix(rest, "(") {
		closeIdx := strings.Index(rest, ")")
		if closeIdx < 0 {
			return "", false
		}
		code := strings.TrimSpace(rest[1:closeIdx])
		return code, true
	}
	return "", false
}

// WalkOptions controls which paths ScanDir visits. Either predicate may be nil,
// in which case it accepts everything.
type WalkOptions struct {
	// KeepFile reports whether the file at path should be scanned. The path is
	// the same form WalkDir produces (rooted at the dir passed to ScanDir).
	KeepFile func(path string) bool
	// KeepDir reports whether ScanDir should descend into the directory at path.
	// Returning false prunes the directory and everything beneath it.
	KeepDir func(path string) bool
}

// ScanDir recursively scans files under dir, returning all unsuppressed
// violations. Files with unknown extensions are skipped. The opts predicates
// further restrict which files are scanned and which directories are walked.
func ScanDir(dir string, opts WalkOptions) ([]Violation, error) {
	var out []Violation
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if opts.KeepDir != nil && !opts.KeepDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if opts.KeepFile != nil && !opts.KeepFile(path) {
			return nil
		}
		vs, err := ScanFile(path)
		if err != nil {
			return fmt.Errorf("scanning %s: %w", path, err)
		}
		out = append(out, vs...)
		return nil
	})
	return out, err
}
