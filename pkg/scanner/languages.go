package scanner

// language holds comment syntax for a source language.
type language struct {
	linePrefix string // single-line comment prefix, e.g. "//" or "#"
}

// langByExt maps file extensions (without leading dot) to their language.
var langByExt = map[string]language{
	// Go
	"go": {linePrefix: "//"},
	// Python
	"py": {linePrefix: "#"},
	// JavaScript / TypeScript
	"js":  {linePrefix: "//"},
	"ts":  {linePrefix: "//"},
	"jsx": {linePrefix: "//"},
	"tsx": {linePrefix: "//"},
	// Ruby
	"rb": {linePrefix: "#"},
	// Shell
	"sh":   {linePrefix: "#"},
	"bash": {linePrefix: "#"},
	// SQL
	"sql": {linePrefix: "--"},
	// Lua
	"lua": {linePrefix: "--"},
	// Config / data formats
	"toml": {linePrefix: "#"},
	"yaml": {linePrefix: "#"},
	"yml":  {linePrefix: "#"},
	// Rust
	"rs": {linePrefix: "//"},
	// C / C++
	"c":   {linePrefix: "//"},
	"cpp": {linePrefix: "//"},
	"h":   {linePrefix: "//"},
	// Java / Kotlin / Swift
	"java":  {linePrefix: "//"},
	"kt":    {linePrefix: "//"},
	"swift": {linePrefix: "//"},
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
