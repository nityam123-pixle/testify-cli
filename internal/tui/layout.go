package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// BaseLayout adds the standard left padding so the CLI doesn't hug the left edge
	BaseLayout = lipgloss.NewStyle().
			PaddingLeft(2)

	// CardStyle represents a bordered, slightly padded box for grouping content
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorGrayDark).
			Padding(1, 4)
	
	// CardTitleStyle is used for headers inside a card, usually positioned at the top left
	CardTitleStyle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true).
			MarginBottom(1)
)

// AlignColumns takes a label and a value and pads the label to a fixed width
// so multiple rows align perfectly.
func AlignColumns(label string, value string, labelWidth int) string {
	lbl := lipgloss.NewStyle().Width(labelWidth).Render(label)
	return lbl + value
}
