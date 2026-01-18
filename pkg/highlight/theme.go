package highlight

import "github.com/charmbracelet/lipgloss"

// Theme maps semantic categories to terminal styles.
type Theme struct {
	Colors map[Category]lipgloss.Style
}

// DefaultTheme returns the default color scheme.
func DefaultTheme() Theme {
	return Theme{
		Colors: map[Category]lipgloss.Style{
			// Keywords in blue
			CategoryKeyword: lipgloss.NewStyle().Foreground(lipgloss.Color("4")),

			// Strings in gray
			CategoryString: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

			// Numbers in bold yellow
			CategoryNumber: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),

			// Booleans in bold yellow (same as constants)
			CategoryBoolean: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),

			// Nil in bold yellow (same as constants)
			CategoryNil: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),

			// Constants in bold yellow
			CategoryConstant: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true),

			// Comments in gray
			CategoryComment:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
			CategoryDocComment: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

			// Functions in bright blue
			CategoryFunction:     lipgloss.NewStyle().Foreground(lipgloss.Color("12")),
			CategoryFunctionCall: lipgloss.NewStyle().Foreground(lipgloss.Color("12")),

			// Types in bright magenta
			CategoryType: lipgloss.NewStyle().Foreground(lipgloss.Color("13")),

			// Variables and parameters in default color
			CategoryVariable:  lipgloss.NewStyle(),
			CategoryParameter: lipgloss.NewStyle(),

			// Fields/properties in bright cyan
			CategoryField: lipgloss.NewStyle().Foreground(lipgloss.Color("14")),

			// Operators in white
			CategoryOperator: lipgloss.NewStyle().Foreground(lipgloss.Color("7")),

			// Punctuation in white
			CategoryPunctuation: lipgloss.NewStyle().Foreground(lipgloss.Color("7")),

			// Tags (struct tags) in gray
			CategoryTag: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),

			// Attributes/decorators in blue (like keywords)
			CategoryAttribute: lipgloss.NewStyle().Foreground(lipgloss.Color("4")),

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
