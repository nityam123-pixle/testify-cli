package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	baseBadge = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	methodBadgeBase = lipgloss.NewStyle().
			Width(8).
			Align(lipgloss.Center).
			Padding(0, 1).
			Bold(true)

	// Methods
	badgeGet = methodBadgeBase.Copy().Foreground(ColorGrayDarker).Background(ColorGreen)
	badgePost = methodBadgeBase.Copy().Foreground(ColorGrayDarker).Background(ColorBlue)
	badgePut = methodBadgeBase.Copy().Foreground(ColorGrayDarker).Background(ColorYellow)
	badgePatch = methodBadgeBase.Copy().Foreground(ColorGrayDarker).Background(ColorCyan)
	badgeDelete = methodBadgeBase.Copy().Foreground(ColorGrayDarker).Background(ColorRed)
	badgeFallback = methodBadgeBase.Copy().Foreground(ColorGrayLight).Background(ColorGrayDark)

	// Framework / Language
	badgeFramework = baseBadge.Copy().Foreground(ColorCyan).Background(ColorGrayDark)
	badgeLanguage = baseBadge.Copy().Foreground(ColorGreen).Background(ColorGrayDark)
	badgePort = baseBadge.Copy().Foreground(ColorYellow).Background(ColorGrayDark)

	// Statuses
	badgeStatusSuccess = baseBadge.Copy().Foreground(ColorGrayDarker).Background(ColorGreen)
	badgeStatusWarning = baseBadge.Copy().Foreground(ColorGrayDarker).Background(ColorYellow)
	badgeStatusError = baseBadge.Copy().Foreground(ColorGrayDarker).Background(ColorRed)
)

// MethodBadge returns a solid colored pill for HTTP methods.
func MethodBadge(method string) string {
	method = strings.ToUpper(method)
	switch method {
	case "GET":
		return badgeGet.Render(method)
	case "POST":
		return badgePost.Render(method)
	case "PUT":
		return badgePut.Render(method)
	case "PATCH":
		return badgePatch.Render(method)
	case "DELETE":
		return badgeDelete.Render(method)
	default:
		return badgeFallback.Render(method)
	}
}

// StatusBadge returns a solid colored pill for HTTP status codes.
func StatusBadge(status int) string {
	statusStr := fmt.Sprintf("%d", status)
	// Add common text if needed, e.g., "200 OK"
	if status == 200 {
		statusStr = "200 OK"
	} else if status == 201 {
		statusStr = "201 Created"
	} else if status == 404 {
		statusStr = "404 Not Found"
	} else if status == 500 {
		statusStr = "500 Server Error"
	}

	if status >= 200 && status < 300 {
		return badgeStatusSuccess.Render(IconSuccess + " " + statusStr)
	} else if status >= 400 && status < 500 {
		return badgeStatusWarning.Render(statusStr)
	} else if status >= 500 {
		return badgeStatusError.Render(statusStr)
	}
	return badgeFallback.Render(statusStr)
}

// FrameworkBadge returns a badge for the web framework.
func FrameworkBadge(framework string) string {
	return badgeFramework.Render(framework)
}

// LanguageBadge returns a badge for the programming language.
func LanguageBadge(lang string) string {
	return badgeLanguage.Render(lang)
}

// PortBadge returns a badge for the server port.
func PortBadge(port string) string {
	return badgePort.Render(port)
}

// Divider renders a subtle horizontal rule.
func Divider() string {
	return lipgloss.NewStyle().
		Foreground(ColorGrayDark).
		Render(strings.Repeat("─", 40))
}

// SectionHeader renders a bold cyan title with an underline/margin if needed.
func SectionHeader(title string) string {
	return TextTitle.Render(title)
}

// CardTitle is an alias to the CardTitleStyle in layout.go.
func CardTitle(title string) string {
	return CardTitleStyle.Render(title)
}

// MutedText renders subtle, less important text.
func MutedText(text string) string {
	return TextHint.Render(text)
}

// AlignedRow combines layout alignment with TextLabel and TextValue typography.
func AlignedRow(label, value string) string {
	lbl := TextLabel.Render(label)
	val := TextValue.Render(value)
	return AlignColumns(lbl, val, 18) // Default 18 width for standard columns
}

type Key struct {
	Name string
	Desc string
}

// KeyHint renders a single keyboard shortcut instruction for the status bar.
func KeyHint(key string, desc string) string {
	k := lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render(key)
	d := lipgloss.NewStyle().Foreground(ColorGrayLight).Render(desc)
	return fmt.Sprintf(" %s %s ", k, d)
}

// KeyHintBar renders a full-width horizontal status bar for shortcuts.
func KeyHintBar(keys []Key) string {
	var parts []string
	
	prefix := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render(" TESTIFY v1.0 ")
	parts = append(parts, prefix)

	for _, k := range keys {
		parts = append(parts, KeyHint(k.Name, k.Desc))
	}
	
	separator := lipgloss.NewStyle().Foreground(ColorGrayBase).Render("│")
	content := strings.Join(parts, separator)
	
	width := GetTerminalWidth()
	statusBar := lipgloss.NewStyle().
		Background(ColorGrayDarker).
		Width(width).
		Render(content)
		
	return statusBar
}
