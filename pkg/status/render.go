// Package status renders a rich git-status overview to the terminal.
package status

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/diffyduck/pkg/diff"
	"github.com/user/diffyduck/pkg/highlight"
	"github.com/user/diffyduck/pkg/structure"
)

// ANSI color codes matching the TUI's lipgloss theme.
const (
	reset = "\033[0m"
	bold  = "\033[1m"

	// Status indicators (match fileStatusIndicator in view_header.go)
	fgGreen = "\033[32m"      // fg=2 - added indicator
	fgRed   = "\033[31m"      // fg=1 - deleted indicator
	fgBlue  = "\033[38;5;12m" // fg=12 - modified indicator
	fgCyan  = "\033[36m"      // fg=6 - renamed indicator
	fgGray  = "\033[38;5;8m"  // fg=8 - keywords, dim text
	fgWhite = "\033[38;5;7m"  // fg=7 - punctuation, params, names

	// File-level stats (match addedStyle/removedStyle in view.go: fg=10/fg=9)
	fgBrightGreen = "\033[38;5;10m" // fg=10 - file +N
	fgBrightRed   = "\033[38;5;9m"  // fg=9  - file -N

	// Symbol-level stats (match darkAddedStyle/darkRemovedStyle: fg=2/fg=1)
	fgDarkGreen = "\033[32m" // fg=2 - symbol +N
	fgDarkRed   = "\033[31m" // fg=1 - symbol -N

	// Signature syntax colors (match styleSig in view_commit.go)
	fgFuncName = "\033[38;5;4m" // fg=4 - function names (blue)
	fgTypeName = "\033[38;5;5m" // fg=5 - type names (magenta)

	// File path styling (match styleFileHeaderText in view_header.go)
	fgDir        = "\033[38;5;7m"      // fg=7  - directory part
	fgBasename   = "\033[38;5;15m"     // fg=15 - basename (bright white, modified)
	ulOn         = "\033[4m"           // underline on
	ulOff        = "\033[24m"          // underline off
	fgAddedUL    = "\033[1;4;38;5;10m" // bold + underline + fg=10 (added)
	fgRemovedUL  = "\033[1;4;38;5;9m"  // bold + underline + fg=9  (deleted)
	fgRenamedUL  = "\033[1;4;36m"      // bold + underline + fg=6  (renamed)
	fgModifiedUL = "\033[4;38;5;15m"   // underline + fg=15 (modified)
)

// FileChange represents a single file's status in a diff section.
type FileChange struct {
	Path     string
	OldPath  string // non-empty for renames
	Status   string // "+", "-", "~", "→"
	Added    int
	Removed  int
	IsBinary bool
	Symbols  []SymbolLine
}

// SymbolLine represents a single structural element change within a file.
type SymbolLine struct {
	Kind       string               // "func", "type", etc.
	Signature  string               // formatted signature
	ChangeKind structure.ChangeKind // added, deleted, modified
	Added      int
	Removed    int
	IsChild    bool // indented under a parent type
}

// TruncationLine represents the "...(N more)" overflow indicator.
type TruncationLine struct {
	Count int
}

// BuildFileChanges computes file changes with structural diffs from a parsed diff.
// workDir is the working directory for reading unstaged file content from disk.
// contentFetcher fetches git object content (old: "HEAD:path" or ":path", new: ":path" or disk).
// maxSymbols controls how many structural symbols to show per file (0 = skip symbols).
// If hl is nil, structural diffs are skipped regardless of maxSymbols.
func BuildFileChanges(
	parsed *diff.Diff,
	hl *highlight.Highlighter,
	fetchContent func(ref, path string) (string, error),
	readWorkingFile func(path string) (string, error),
	isStaged bool,
	maxSymbols int,
) []FileChange {
	if parsed == nil {
		return nil
	}

	var changes []FileChange
	for _, f := range parsed.Files {
		fc := fileChangeFromDiff(f)

		// Compute structural diff if highlighter is available and symbols requested
		if hl != nil && maxSymbols > 0 && !f.IsBinary {
			fc.Symbols = computeSymbols(f, hl, fetchContent, readWorkingFile, isStaged, maxSymbols)
		}

		changes = append(changes, fc)
	}
	return changes
}

// fileChangeFromDiff creates a FileChange from a diff.File.
func fileChangeFromDiff(f diff.File) FileChange {
	oldPath := strings.TrimPrefix(f.OldPath, "a/")
	newPath := strings.TrimPrefix(f.NewPath, "b/")

	fc := FileChange{
		Path:     newPath,
		Added:    f.TotalAdded,
		Removed:  f.TotalRemoved,
		IsBinary: f.IsBinary,
	}

	// Determine status
	switch {
	case f.OldPath == "/dev/null":
		fc.Status = "+"
	case f.NewPath == "/dev/null":
		fc.Status = "-"
		fc.Path = oldPath
	case f.IsRename || f.IsCopy || oldPath != newPath:
		fc.Status = "→"
		fc.OldPath = oldPath
	default:
		fc.Status = "~"
	}

	return fc
}

// computeSymbols extracts structural diff symbols for a file.
func computeSymbols(
	f diff.File,
	hl *highlight.Highlighter,
	fetchContent func(ref, path string) (string, error),
	readWorkingFile func(path string) (string, error),
	isStaged bool,
	maxSymbols int,
) []SymbolLine {
	newPath := strings.TrimPrefix(f.NewPath, "b/")
	oldPath := strings.TrimPrefix(f.OldPath, "a/")

	// Determine which path to use for language detection
	langPath := newPath
	if f.NewPath == "/dev/null" {
		langPath = oldPath
	}

	// Extract added/removed line numbers from hunks
	addedLines := make(map[int]bool)
	removedLines := make(map[int]bool)
	for _, hunk := range f.Hunks {
		oldLine := hunk.OldStart
		newLine := hunk.NewStart
		for _, line := range hunk.Lines {
			switch line.Type {
			case diff.Added:
				addedLines[newLine] = true
				newLine++
			case diff.Removed:
				removedLines[oldLine] = true
				oldLine++
			case diff.Context:
				oldLine++
				newLine++
			}
		}
	}

	// Fetch old content and extract structure
	var oldStruct *structure.Map
	if f.OldPath != "/dev/null" {
		var oldRef string
		if isStaged {
			oldRef = "HEAD"
		}
		// For unstaged: oldRef="" means index (staged version)
		oldContent, err := fetchContent(oldRef, oldPath)
		if err == nil && len(oldContent) > 0 {
			oldStruct = hl.ExtractStructure(langPath, []byte(oldContent))
		}
	}

	// Fetch new content and extract structure
	var newStruct *structure.Map
	if f.NewPath != "/dev/null" {
		var newContent string
		var err error
		if isStaged {
			// Staged: new content is from the index
			newContent, err = fetchContent("", newPath)
		} else if readWorkingFile != nil {
			// Unstaged: new content is from the working tree
			newContent, err = readWorkingFile(newPath)
		}
		if err == nil && len(newContent) > 0 {
			newStruct = hl.ExtractStructure(langPath, []byte(newContent))
		}
	}

	// Compute structural diff
	structDiff := structure.ComputeDiff(oldStruct, newStruct, addedLines, removedLines)
	if structDiff == nil || !structDiff.HasChanges() {
		return nil
	}

	// Get top changes using the shared helper
	nodes, truncated := structure.TopChanges(structDiff, maxSymbols)

	var symbols []SymbolLine
	for _, node := range nodes {
		entry := node.Change.Entry()
		if entry == nil {
			continue
		}
		symbols = append(symbols, SymbolLine{
			Kind:       entry.Kind,
			Signature:  formatEntrySignature(entry),
			ChangeKind: node.Change.Kind,
			Added:      node.Change.LinesAdded,
			Removed:    node.Change.LinesRemoved,
		})

		for _, child := range node.Children {
			childEntry := child.Entry()
			if childEntry == nil {
				continue
			}
			symbols = append(symbols, SymbolLine{
				Kind:       childEntry.Kind,
				Signature:  formatEntrySignature(childEntry),
				ChangeKind: child.Kind,
				Added:      child.LinesAdded,
				Removed:    child.LinesRemoved,
				IsChild:    true,
			})
		}
	}

	if truncated > 0 {
		symbols = append(symbols, SymbolLine{
			Kind:      fmt.Sprintf("...(%d more)", truncated),
			Signature: "",
		})
	}

	return symbols
}

// formatEntrySignature formats an entry's name or signature for display.
func formatEntrySignature(entry *structure.Entry) string {
	sig := entry.FormatSignature(80) // reasonable width for terminal output
	if sig == "" {
		return entry.Name
	}
	return sig
}

// Render produces the full status output string.
// untrackedExpanded, when non-nil, replaces the plain untracked listing with
// full file change details (used by --all mode).
func Render(branchTree string, staged, unstaged []FileChange, untracked []string, untrackedExpanded []FileChange) string {
	var b strings.Builder

	// Branch tree
	if branchTree != "" {
		b.WriteString(branchTree)
		if !strings.HasSuffix(branchTree, "\n") {
			b.WriteByte('\n')
		}
	}

	needsGap := branchTree != ""

	// Section bar prefixes: thick vertical line in section color
	const bar = "┃"
	stagedBar := "\033[32m" + bar + reset + " "         // fg=2
	unstagedBar := "\033[33m" + bar + reset + " "       // fg=3
	untrackedBar := "\033[38;5;10m" + bar + reset + " " // fg=10

	// Staged section
	if len(staged) > 0 {
		if needsGap {
			b.WriteByte('\n')
		}
		b.WriteString(stagedBar + bold + "Staged:" + reset + "\n")
		renderFileChanges(&b, staged, stagedBar)
		needsGap = true
	}

	// Unstaged section
	if len(unstaged) > 0 {
		if needsGap {
			b.WriteByte('\n')
		}
		b.WriteString(unstagedBar + bold + "Unstaged:" + reset + "\n")
		renderFileChanges(&b, unstaged, unstagedBar)
		needsGap = true
	}

	// Untracked section — expanded (--all) or plain listing
	if len(untrackedExpanded) > 0 {
		if needsGap {
			b.WriteByte('\n')
		}
		b.WriteString(untrackedBar + bold + "Untracked:" + reset + "\n")
		renderFileChanges(&b, untrackedExpanded, untrackedBar)
		needsGap = true
	} else if len(untracked) > 0 {
		if needsGap {
			b.WriteByte('\n')
		}
		b.WriteString(untrackedBar + bold + "Untracked:" + reset + "\n")
		for _, path := range untracked {
			b.WriteString(untrackedBar + "  " + fgGray + path + reset + "\n")
		}
		needsGap = true
	}

	// Summary line
	allChanges := concat(staged, unstaged, untrackedExpanded)
	if len(allChanges) > 0 {
		var totalAdded, totalRemoved int
		for _, fc := range allChanges {
			totalAdded += fc.Added
			totalRemoved += fc.Removed
		}
		if needsGap {
			b.WriteByte('\n')
		}
		b.WriteString(fgWhite + fmt.Sprintf("%d files", len(allChanges)) + reset)
		if totalAdded > 0 {
			b.WriteString(" " + fgBrightGreen + fmt.Sprintf("+%d", totalAdded) + reset)
		}
		if totalRemoved > 0 {
			b.WriteString(" " + fgBrightRed + fmt.Sprintf("-%d", totalRemoved) + reset)
		}
		b.WriteByte('\n')
	} else if len(staged) == 0 && len(unstaged) == 0 && len(untracked) == 0 && len(untrackedExpanded) == 0 {
		if needsGap {
			b.WriteByte('\n')
		}
		b.WriteString("Nothing to commit, working tree clean\n")
	}

	return b.String()
}

// renderFileChanges renders a list of file changes to the builder.
func renderFileChanges(b *strings.Builder, changes []FileChange, prefix string) {
	for i, fc := range changes {
		renderFileLine(b, fc, prefix)
		for _, sym := range fc.Symbols {
			renderSymbolLine(b, sym, prefix)
		}
		// Blank line after files with symbols, except after the last file
		if len(fc.Symbols) > 0 && i < len(changes)-1 {
			b.WriteString(prefix + "\n")
		}
	}
}

// renderFileLine renders a single file status line.
func renderFileLine(b *strings.Builder, fc FileChange, prefix string) {
	// Status indicator with color
	var styledStatus string
	switch fc.Status {
	case "+":
		styledStatus = fgGreen + "+" + reset
	case "-":
		styledStatus = fgRed + "-" + reset
	case "→":
		styledStatus = fgCyan + "→" + reset
	case "~":
		styledStatus = fgBlue + "~" + reset
	}

	// Styled file path: directory in fg=7, basename underlined in change color
	var styledPath string
	if fc.OldPath != "" {
		styledPath = styleFilePath(fc.OldPath, fc.Status) + fgDir + " → " + reset + styleFilePath(fc.Path, fc.Status)
	} else {
		styledPath = styleFilePath(fc.Path, fc.Status)
	}

	// File-level stats use bright colors (fg=10/9)
	stats := formatFileStats(fc.Added, fc.Removed)

	b.WriteString(prefix + "  " + styledStatus + " " + styledPath)
	if stats != "" {
		b.WriteString(" " + stats)
	}
	b.WriteByte('\n')
}

// styleFilePath styles a file path: directory part in fg=7, basename underlined
// in the color matching the change type.
func styleFilePath(path, status string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Pick the basename style based on change type
	var basenameStyle string
	switch status {
	case "+":
		basenameStyle = fgAddedUL
	case "-":
		basenameStyle = fgRemovedUL
	case "→":
		basenameStyle = fgRenamedUL
	default: // "~"
		basenameStyle = fgModifiedUL
	}

	if dir == "." {
		return basenameStyle + base + reset
	}
	return fgDir + dir + "/" + reset + basenameStyle + base + reset
}

// renderSymbolLine renders a single structural element line.
func renderSymbolLine(b *strings.Builder, sym SymbolLine, prefix string) {
	indent := "      "
	if sym.IsChild {
		indent = "        "
	}

	// Check for truncation marker
	if sym.Signature == "" && strings.HasPrefix(sym.Kind, "...") {
		b.WriteString(prefix + indent + fgGray + sym.Kind + reset + "\n")
		return
	}

	// Kind in gray (fg=8), signature with syntax colors
	stats := formatSymbolStats(sym.Added, sym.Removed)
	styledSig := styleSigPlain(sym.Signature, sym.Kind, sym.ChangeKind)

	b.WriteString(prefix + indent + fgGray + sym.Kind + " " + reset + styledSig)
	if stats != "" {
		b.WriteString(" " + stats)
	}
	b.WriteByte('\n')
}

// styleSigPlain applies syntax-style ANSI coloring to a signature string.
// Matches the TUI's styleSig: func names in fg=4, type names in fg=5,
// punctuation in fg=7, params in fg=7.
// For added/deleted changes, the identifier name is underlined in green/red.
func styleSigPlain(sig string, kind string, changeKind structure.ChangeKind) string {
	if sig == "" {
		return ""
	}

	// Determine name color based on element kind and change kind.
	// Added: underline + fg=2 (green). Deleted: underline + fg=1 (red).
	// Modified: fg=7 (white) for func names, fg=5 for type names.
	nameColor := fgWhite
	if kind == "type" || kind == "class" || kind == "struct" || kind == "interface" {
		nameColor = fgTypeName
	}
	switch changeKind {
	case structure.ChangeAdded:
		nameColor = "\033[4;32m" // underline + fg=2
	case structure.ChangeDeleted:
		nameColor = "\033[4;31m" // underline + fg=1
	}

	// No parens — plain type/class name
	if !strings.Contains(sig, "(") {
		return nameColor + sig + reset
	}

	var result strings.Builder

	rest := sig

	// Handle optional receiver prefix: "(m *Model) "
	if strings.HasPrefix(rest, "(") {
		closeParen := strings.Index(rest, ") ")
		if closeParen > 0 {
			nextOpen := strings.Index(rest[1:], "(")
			if nextOpen >= 0 && closeParen < nextOpen+1 {
				// It's a receiver — highlight type within
				result.WriteString(styleReceiverPlain(rest[:closeParen+1]))
				result.WriteString(" ")
				rest = rest[closeParen+2:]
			}
		}
	}

	// "Name(params)" or "Name(params) -> ReturnType"
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		result.WriteString(nameColor + rest + reset)
		return result.String()
	}

	// Function/method name
	name := rest[:parenIdx]
	result.WriteString(nameColor + name + reset)
	rest = rest[parenIdx:]

	// Split on " -> " for return type
	arrowIdx := strings.Index(rest, " -> ")
	var paramsPart, returnType string
	if arrowIdx >= 0 {
		paramsPart = rest[:arrowIdx]
		returnType = rest[arrowIdx+4:] // skip " -> "
	} else {
		paramsPart = rest
	}

	// Params: split "name Type" pairs to highlight types in magenta
	result.WriteString(styleParamsPlain(paramsPart))

	// Return type in fg=5 (magenta)
	if returnType != "" {
		result.WriteString(" " + fgWhite + "->" + reset + " " + fgTypeName + returnType + reset)
	}

	return result.String()
}

// styleReceiverPlain highlights a Go receiver like "(m *Model)".
func styleReceiverPlain(recv string) string {
	inner := recv[1 : len(recv)-1] // "m *Model"
	parts := strings.SplitN(inner, " ", 2)

	var result strings.Builder
	result.WriteString(fgWhite + "(" + reset)
	if len(parts) == 2 {
		result.WriteString(fgWhite + parts[0] + reset)
		result.WriteString(" ")
		result.WriteString(fgTypeName + parts[1] + reset)
	} else {
		result.WriteString(fgWhite + inner + reset)
	}
	result.WriteString(fgWhite + ")" + reset)
	return result.String()
}

// styleParamsPlain highlights a parameter list, splitting "name Type" pairs
// so types render in magenta (fg=5) and names/punctuation in white (fg=7).
func styleParamsPlain(params string) string {
	if params == "(...)" || params == "()" {
		return fgWhite + params + reset
	}
	if !strings.HasPrefix(params, "(") || !strings.HasSuffix(params, ")") {
		return params
	}
	inner := params[1 : len(params)-1]

	hasEllipsis := strings.HasSuffix(inner, ", ...")
	if hasEllipsis {
		inner = strings.TrimSuffix(inner, ", ...")
	}

	var result strings.Builder
	result.WriteString(fgWhite + "(" + reset)

	paramList := strings.Split(inner, ", ")
	for i, p := range paramList {
		if i > 0 {
			result.WriteString(fgWhite + "," + reset + " ")
		}
		spaceIdx := strings.Index(p, " ")
		if spaceIdx >= 0 {
			paramName := p[:spaceIdx]
			paramType := p[spaceIdx+1:]
			result.WriteString(fgWhite + paramName + reset)
			result.WriteString(" ")
			result.WriteString(fgTypeName + paramType + reset)
		} else {
			result.WriteString(fgWhite + p + reset)
		}
	}

	if hasEllipsis {
		result.WriteString(fgWhite + ", ..." + reset)
	}

	result.WriteString(fgWhite + ")" + reset)
	return result.String()
}

// formatFileStats returns colored "+N -M" for file-level display (bright colors).
func formatFileStats(added, removed int) string {
	if added == 0 && removed == 0 {
		return ""
	}
	var parts []string
	if added > 0 {
		parts = append(parts, fgBrightGreen+fmt.Sprintf("+%d", added)+reset)
	}
	if removed > 0 {
		parts = append(parts, fgBrightRed+fmt.Sprintf("-%d", removed)+reset)
	}
	return strings.Join(parts, " ")
}

// formatSymbolStats returns colored "+N -M" for symbol-level display (dark/standard colors).
func formatSymbolStats(added, removed int) string {
	if added == 0 && removed == 0 {
		return ""
	}
	var parts []string
	if added > 0 {
		parts = append(parts, fgDarkGreen+fmt.Sprintf("+%d", added)+reset)
	}
	if removed > 0 {
		parts = append(parts, fgDarkRed+fmt.Sprintf("-%d", removed)+reset)
	}
	return strings.Join(parts, " ")
}

// formatStats returns colored "+N -M" string (used by tests, uses symbol-level colors).
func formatStats(added, removed int) string {
	return formatSymbolStats(added, removed)
}

// concat joins multiple FileChange slices.
func concat(slices ...[]FileChange) []FileChange {
	var out []FileChange
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}

// ReadWorkingFile returns a function that reads files relative to a working directory.
func ReadWorkingFile(workDir string) func(path string) (string, error) {
	return func(path string) (string, error) {
		fullPath := filepath.Join(workDir, path)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
