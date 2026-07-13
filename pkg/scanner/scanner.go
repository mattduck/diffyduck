// Package scanner finds marker annotations (TODO, FIXME, RPT, …) in source
// files. Markers are configurable: each binary registers the keyword families it
// cares about. rpt scans for RPT (with NORPT suppression); tdb scans for the
// conventional code-comment markers (TODO/FIXME/RPT).
package scanner

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxScanLine caps the per-line buffer used when scanning source files for
// markers. Lines longer than this are skipped (see scanFileMarkers): a comment
// annotation never lives on a multi-megabyte line.
const maxScanLine = 4 * 1024 * 1024

// Marker defines a keyword family to scan for in source comments.
type Marker struct {
	// Keyword is the leading token of the annotation, e.g. "TODO" or "RPT".
	Keyword string
	// Strict requires the conventional-commit form "type(scope):" or "type:"
	// immediately after the keyword. When false the marker also matches a bare
	// keyword with no type, no scope, and an optional freeform message.
	Strict bool
	// Suppress is the keyword that suppresses this marker (e.g. "NORPT"). An
	// empty value disables suppression for the family.
	Suppress string
}

// RPTMarker returns the marker spec for rpt's RPT/NORPT annotations.
func RPTMarker() Marker {
	return Marker{Keyword: "RPT", Strict: true, Suppress: "NORPT"}
}

// DefaultMarkers returns the conventional code-comment markers tdb scans for.
// RPT is included in loose (non-strict) mode so malformed annotations are
// visible; rpt uses RPTMarker() directly for its strict enforcement.
func DefaultMarkers() []Marker {
	return []Marker{
		{Keyword: "TODO"},
		{Keyword: "FIXME"},
		{Keyword: "RPT"},
	}
}

// Match is a marker occurrence found in a source file.
type Match struct {
	File    string
	Line    int    // 1-based
	Keyword string // the marker keyword, e.g. "TODO" or "RPT"
	Type    string // optional conventional-commit type ("feat", "refactor", …)
	Scope   string // optional scope/code identifier ("auth", "use-pathlib", …)
	Message string
}

func (m Match) String() string {
	var kw string
	switch {
	case m.Type != "" && m.Scope != "":
		kw = fmt.Sprintf("%s %s(%s)", m.Keyword, m.Type, m.Scope)
	case m.Type != "":
		kw = fmt.Sprintf("%s %s", m.Keyword, m.Type)
	case m.Scope != "":
		kw = fmt.Sprintf("%s(%s)", m.Keyword, m.Scope)
	default:
		kw = m.Keyword
	}
	if m.Message == "" {
		return fmt.Sprintf("%s:%d: %s", m.File, m.Line, kw)
	}
	return fmt.Sprintf("%s:%d: %s %s", m.File, m.Line, kw, m.Message)
}

// Violation is an RPT annotation found in a source file. It is the RPT-specific
// view of a Match, preserving rpt's exact output format.
type Violation struct {
	File    string
	Line    int // 1-based
	Type    string
	Code    string
	Message string
}

func (v Violation) String() string {
	if v.Type != "" {
		return fmt.Sprintf("%s:%d: RPT %s(%s) %s", v.File, v.Line, v.Type, v.Code, v.Message)
	}
	return fmt.Sprintf("%s:%d: RPT(%s) %s", v.File, v.Line, v.Code, v.Message)
}

// ScanFile scans a single file for RPT annotations, applying NORPT
// suppressions. Returns violations that are not suppressed. Files with unknown
// extensions are skipped (returns nil, nil).
func ScanFile(path string) ([]Violation, error) {
	ms, err := ScanFileMarkers(path, []Marker{RPTMarker()})
	if err != nil {
		return nil, err
	}
	return toViolations(ms), nil
}

// ScanDir recursively scans files under dir for RPT violations.
func ScanDir(dir string, opts WalkOptions) ([]Violation, error) {
	ms, err := ScanDirMarkers(dir, []Marker{RPTMarker()}, opts)
	if err != nil {
		return nil, err
	}
	return toViolations(ms), nil
}

// ScanFiles scans an explicit list of files for RPT violations, applying NORPT
// suppressions. Unlike ScanDir it performs no directory traversal: callers that
// already hold a file list (e.g. from git) avoid walking ignored trees. Files
// with unknown extensions are skipped.
func ScanFiles(paths []string) ([]Violation, error) {
	ms, err := ScanFilesMarkers(paths, []Marker{RPTMarker()})
	if err != nil {
		return nil, err
	}
	return toViolations(ms), nil
}

func toViolations(ms []Match) []Violation {
	if len(ms) == 0 {
		return nil
	}
	out := make([]Violation, len(ms))
	for i, m := range ms {
		out[i] = Violation{File: m.File, Line: m.Line, Type: m.Type, Code: m.Scope, Message: m.Message}
	}
	return out
}

// ScanFileMarkers scans a single file for the given markers, applying each
// marker's suppression keyword. Files with unknown extensions are skipped
// (returns nil, nil).
func ScanFileMarkers(path string, markers []Marker) ([]Match, error) {
	lang, ok := langForFile(filepath.Base(path))
	if !ok {
		return nil, nil
	}
	return scanFileMarkers(path, lang, markers)
}

func scanFileMarkers(path string, lang language, markers []Marker) ([]Match, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	type rawMatch struct {
		line     int
		keyword  string
		typeName string
		scope    string
		msg      string
	}
	type suppress struct {
		line    int    // line number the suppression targets
		keyword string // marker keyword this suppression applies to
		code    string // empty = all codes
	}

	var matches []rawMatch
	var suppressions []suppress
	var pending []suppress // whole-line suppressions awaiting the next code line

	sc := bufio.NewScanner(f)
	// Enlarge the line buffer well past bufio's 64KB default so ordinary long
	// lines (minified JS, bundled assets, big JSON blobs) don't trip the scanner.
	sc.Buffer(make([]byte, 64*1024), maxScanLine)
	lineNum := 0
	prefix := lang.linePrefix

	for sc.Scan() {
		lineNum++
		text := sc.Text()
		trimmed := strings.TrimSpace(text)

		segments := commentSegments(text, prefix)
		if len(segments) == 0 {
			// No comment on this line. Flush any pending preceding-line
			// suppressions onto the first non-blank, non-comment line.
			if len(pending) > 0 && trimmed != "" {
				for _, p := range pending {
					p.line = lineNum
					suppressions = append(suppressions, p)
				}
				pending = nil
			}
			continue
		}

		for _, seg := range segments {
			// A segment matches at most one marker keyword (the families share no
			// common prefix), so stop at the first hit.
			for _, m := range markers {
				if typeName, scope, msg, ok := parseMarker(m, seg.body); ok {
					matches = append(matches, rawMatch{lineNum, m.Keyword, typeName, scope, msg})
					break
				}
			}
			// A suppression can co-occur with a match on the same line (e.g.
			// "RPT(x): msg // NORPT(x)"), so scan suppressions independently.
			for _, m := range markers {
				if m.Suppress == "" {
					continue
				}
				if code, ok := parseSuppress(m.Suppress, seg.body); ok {
					if seg.isWholeLine {
						pending = append(pending, suppress{keyword: m.Keyword, code: code})
					} else {
						suppressions = append(suppressions, suppress{line: lineNum, keyword: m.Keyword, code: code})
					}
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		// A line longer than maxScanLine is never a hand-written comment (it is
		// almost always minified or generated content). Rather than aborting the
		// whole scan, treat it as end-of-file for this file and keep the matches
		// found before it. Other read errors are still fatal.
		if !errors.Is(err, bufio.ErrTooLong) {
			return nil, err
		}
	}

	// Build suppression lookup: (line, keyword) -> set of suppressed codes
	// ("" = all).
	type supKey struct {
		line    int
		keyword string
	}
	suppressed := make(map[supKey]map[string]bool)
	for _, s := range suppressions {
		k := supKey{s.line, s.keyword}
		if suppressed[k] == nil {
			suppressed[k] = make(map[string]bool)
		}
		suppressed[k][s.code] = true
	}

	var out []Match
	for _, m := range matches {
		if isSuppressed(suppressed[supKey{m.line, m.keyword}], m.scope) {
			continue
		}
		out = append(out, Match{File: path, Line: m.line, Keyword: m.keyword, Type: m.typeName, Scope: m.scope, Message: m.msg})
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

// parseMarker attempts to parse a marker occurrence from a comment body.
// body is the text after the line-comment prefix, trimmed. Returns the optional
// type and scope captures, the message, and whether the body matched.
//
// Both strict and loose markers support the conventional-commit form:
//
//	KEYWORD type(scope): message   — type and scope
//	KEYWORD type: message          — type only
//
// Loose markers (Strict=false) additionally accept a bare keyword with no type,
// no scope, and an optional freeform message:
//
//	KEYWORD: message
//	KEYWORD message
//	KEYWORD
func parseMarker(m Marker, body string) (typeName, scope, msg string, ok bool) {
	if !strings.HasPrefix(body, m.Keyword) {
		return "", "", "", false
	}
	rest := body[len(m.Keyword):]

	// Try structured form: " type(scope):" or " type:".
	if strings.HasPrefix(rest, " ") {
		word, rest2, hasWord := parseWord(rest[1:]) // skip leading space
		if hasWord {
			if strings.HasPrefix(rest2, "(") {
				if closeIdx := strings.Index(rest2, "):"); closeIdx >= 0 {
					scopeVal := strings.TrimSpace(rest2[1:closeIdx])
					msgVal := strings.TrimSpace(rest2[closeIdx+2:])
					return word, scopeVal, msgVal, true
				}
			}
			if strings.HasPrefix(rest2, ":") {
				msgVal := strings.TrimSpace(rest2[1:])
				return word, "", msgVal, true
			}
		}
	}

	if m.Strict {
		return "", "", "", false
	}

	// Loose: the next character after the keyword must not be alphanumeric
	// (prevents "TODOLIST" from matching).
	if rest != "" && isWordChar(rest[0]) {
		return "", "", "", false
	}
	// Strip optional leading space and optional ":" separator.
	rest = strings.TrimPrefix(rest, " ")
	rest = strings.TrimPrefix(rest, ":")
	return "", "", strings.TrimSpace(rest), true
}

// parseWord scans a word token (letters, digits, hyphens, underscores) from the
// start of s. Returns the word, the remainder, and whether a word was found.
func parseWord(s string) (word, rest string, ok bool) {
	i := 0
	for i < len(s) && (isWordChar(s[i]) || s[i] == '-' || s[i] == '_') {
		i++
	}
	if i == 0 {
		return "", s, false
	}
	return s[:i], s[i:], true
}

// parseSuppress attempts to parse a suppression from a comment body for the
// given suppression keyword. Returns the rule code (empty = suppress all) and
// true on success. Examples (keyword "NORPT"):
// "NORPT" -> ("", true), "NORPT(bare-dict)" -> ("bare-dict", true).
func parseSuppress(keyword, body string) (code string, ok bool) {
	if !strings.HasPrefix(body, keyword) {
		return "", false
	}
	rest := body[len(keyword):]
	if rest == "" || strings.TrimSpace(rest) == "" {
		return "", true
	}
	if strings.HasPrefix(rest, "(") {
		closeIdx := strings.Index(rest, ")")
		if closeIdx < 0 {
			return "", false
		}
		return strings.TrimSpace(rest[1:closeIdx]), true
	}
	return "", false
}

// isWordChar reports whether c is an ASCII letter or digit.
func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
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

// ScanFilesMarkers scans an explicit list of files for the given markers,
// returning all unsuppressed matches. It performs no directory traversal. Files
// with unknown extensions are skipped.
func ScanFilesMarkers(paths []string, markers []Marker) ([]Match, error) {
	var out []Match
	for _, path := range paths {
		ms, err := ScanFileMarkers(path, markers)
		if err != nil {
			return nil, fmt.Errorf("scanning %s: %w", path, err)
		}
		out = append(out, ms...)
	}
	return out, nil
}

// ScanDirMarkers recursively scans files under dir for the given markers,
// returning all unsuppressed matches. Files with unknown extensions are skipped.
// The opts predicates further restrict which files are scanned and which
// directories are walked.
func ScanDirMarkers(dir string, markers []Marker, opts WalkOptions) ([]Match, error) {
	var out []Match
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
		ms, err := ScanFileMarkers(path, markers)
		if err != nil {
			return fmt.Errorf("scanning %s: %w", path, err)
		}
		out = append(out, ms...)
		return nil
	})
	return out, err
}
