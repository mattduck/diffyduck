package scanner

// language holds comment syntax for a source language.
type language struct {
	linePrefix string // single-line comment prefix, e.g. "//" or "#"
	blockStart string // block-comment opener, e.g. "/*" (empty if unsupported)
	blockEnd   string // block-comment closer, e.g. "*/" (empty if unsupported)
}

// cLike is the comment syntax shared by C-family languages (JS/TS/Go/Rust/…):
// "//" line comments plus "/* */" block comments. JSX/TSX inherit this, which
// is what lets `{/* RPT ... */}` annotations be scanned (a "//" between JSX
// tags would render as literal text, so block comments are the usable form).
var cLike = language{linePrefix: "//", blockStart: "/*", blockEnd: "*/"}

// langByExt maps file extensions (without leading dot) to their language.
var langByExt = map[string]language{
	// Go
	"go": cLike,
	// Python
	"py": {linePrefix: "#"},
	// JavaScript / TypeScript
	"js":  cLike,
	"ts":  cLike,
	"jsx": cLike,
	"tsx": cLike,
	// Ruby
	"rb": {linePrefix: "#"},
	// Shell
	"sh":   {linePrefix: "#"},
	"bash": {linePrefix: "#"},
	// SQL ("--" line comments, "/* */" block comments)
	"sql": {linePrefix: "--", blockStart: "/*", blockEnd: "*/"},
	// Lua (block comment is --[[ ]], not handled)
	"lua": {linePrefix: "--"},
	// Config / data formats
	"toml": {linePrefix: "#"},
	"yaml": {linePrefix: "#"},
	"yml":  {linePrefix: "#"},
	// Rust
	"rs": cLike,
	// C / C++
	"c":   cLike,
	"cpp": cLike,
	"h":   cLike,
	// Java / Kotlin / Swift
	"java":  cLike,
	"kt":    cLike,
	"swift": cLike,
}

// langForFile returns the language for the given filename, and false if unknown.
func langForFile(name string) (language, bool) {
	ext := fileExt(name)
	if ext == "" {
		return language{}, false
	}
	l, ok := langByExt[ext]
	return l, ok
}

// fileExt returns the lowercase extension of name without the leading dot.
func fileExt(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '.' {
			ext := name[i+1:]
			return toLower(ext)
		}
		if name[i] == '/' {
			break
		}
	}
	return ""
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
