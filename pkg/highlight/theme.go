package highlight

import "github.com/charmbracelet/lipgloss"

// Theme maps semantic categories to terminal styles.
type Theme struct {
	Colors map[Category]lipgloss.Style
}

// DefaultTheme returns the default color scheme.
// Colors are chosen to work on both light and dark terminals,
// using basic ANSI colors (0-15) for maximum compatibility.
func DefaultTheme() Theme {
	return Theme{
		Colors: map[Category]lipgloss.Style{
			// Keywords in magenta
			CategoryKeyword: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),

			// Strings in green
			CategoryString: lipgloss.NewStyle().Foreground(lipgloss.Color("2")),

			// Numbers in cyan
			CategoryNumber: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),

			// Booleans in cyan (same as numbers - they're literals)
			CategoryBoolean: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),

			// Nil in cyan (same as other literals)
			CategoryNil: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),

			// Comments in gray
			CategoryComment:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
			CategoryDocComment: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

			// Functions in blue
			CategoryFunction:     lipgloss.NewStyle().Foreground(lipgloss.Color("4")),
			CategoryFunctionCall: lipgloss.NewStyle().Foreground(lipgloss.Color("4")),

			// Types in yellow
			CategoryType: lipgloss.NewStyle().Foreground(lipgloss.Color("3")),

			// Constants in cyan
			CategoryConstant: lipgloss.NewStyle().Foreground(lipgloss.Color("6")),

			// Variables, parameters, fields in default color
			CategoryVariable:  lipgloss.NewStyle(),
			CategoryParameter: lipgloss.NewStyle(),
			CategoryField:     lipgloss.NewStyle(),

			// Operators and punctuation in default color
			CategoryOperator:    lipgloss.NewStyle(),
			CategoryPunctuation: lipgloss.NewStyle(),

			// Tags (struct tags) in gray
			CategoryTag: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

			// Attributes in magenta (like keywords)
			CategoryAttribute: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),

			// Namespace/package in default
			CategoryNamespace: lipgloss.NewStyle(),

			// None = default
			CategoryNone: lipgloss.NewStyle(),
		},
	}
}

// Style returns the lipgloss style for a category.
// Returns an empty style if the category is not in the theme.
func (t Theme) Style(cat Category) lipgloss.Style {
	if s, ok := t.Colors[cat]; ok {
		return s
	}
	return lipgloss.NewStyle()
}
