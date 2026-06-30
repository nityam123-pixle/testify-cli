package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors based on premium developer themes (Catppuccin Macchiato / Tokyo Night vibes)
var (
	// Base Colors
	ColorCyan   = lipgloss.Color("#8bd5ca") // Macchiato Teal/Cyan
	ColorGreen  = lipgloss.Color("#a6da95") // Macchiato Green
	ColorBlue   = lipgloss.Color("#8aadf4") // Macchiato Blue
	ColorPurple = lipgloss.Color("#c6a0f6") // Macchiato Mauve
	ColorYellow = lipgloss.Color("#eed49f") // Macchiato Yellow
	ColorRed    = lipgloss.Color("#ed8796") // Macchiato Red
	ColorWhite  = lipgloss.Color("#cad3f5") // Macchiato Text

	// Gray Scale (Backgrounds, borders, muted text)
	ColorGrayLight = lipgloss.Color("#a5adcb") // Subtext 0
	ColorGrayBase  = lipgloss.Color("#6e738d") // Overlay 1 (Hint text)
	ColorGrayDark  = lipgloss.Color("#363a4f") // Surface 0 (Borders, selected rows)
	ColorGrayDarker = lipgloss.Color("#1e2030") // Mantle (Card backgrounds)
)

// Typography Styles
var (
	// TextTitle is for major section headers or modal titles
	TextTitle = lipgloss.NewStyle().
			Foreground(ColorCyan).
			Bold(true)

	// TextSubtitle is for grouping related information
	TextSubtitle = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)

	// TextLabel is for keys in key-value pairs (e.g., "Port:")
	TextLabel = lipgloss.NewStyle().
			Foreground(ColorGrayLight)

	// TextValue is for the values in key-value pairs
	TextValue = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Bold(true)

	// TextHint is for subtle descriptions, keybindings, or empty states
	TextHint = lipgloss.NewStyle().
			Foreground(ColorGrayBase)

	// TextError is for error messages
	TextError = lipgloss.NewStyle().
			Foreground(ColorRed)

	// TextWarning is for warnings
	TextWarning = lipgloss.NewStyle().
			Foreground(ColorYellow)

	// TextSuccess is for positive status
	TextSuccess = lipgloss.NewStyle().
			Foreground(ColorGreen)
)
