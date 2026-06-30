package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
)

// PrintScanSummary displays a premium card summarizing the project stack and scan results.
func PrintScanSummary(info detector.StackInfo, routesCount int, wsPort string, durationMs int64) {
	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(BaseLayout.Render(CardTitle("Project Summary")))
	s.WriteString("\n")

	// Render rows
	rows := []string{
		AlignedRow("Framework", FrameworkBadge(info.Framework)),
		AlignedRow("Language", LanguageBadge(info.Language)),
	}

	envStr := ".env"
	if !info.HasDotEnv {
		envStr = "none"
	}
	rows = append(rows, AlignedRow("Environment", TextValue.Render(envStr)))

	portStr := info.Port
	if portStr == "" {
		portStr = "none"
	}
	rows = append(rows, AlignedRow("Port", PortBadge(portStr)))

	// Routes Found
	rows = append(rows, AlignedRow("Routes Found", lipgloss.NewStyle().Foreground(ColorYellow).Render(fmt.Sprintf("%d", routesCount))))

	if wsPort != "" {
		rows = append(rows, AlignedRow("WebSocket", lipgloss.NewStyle().Foreground(ColorCyan).Render("ws://localhost:"+wsPort)))
	}

	// Join rows
	cardContent := strings.Join(rows, "\n\n")

	// Wrap in card
	s.WriteString(BaseLayout.Render(CardStyle.Render(cardContent)))
	s.WriteString("\n\n")

	// Footer (Scan duration)
	footer := lipgloss.NewStyle().Foreground(ColorGreen).Render(fmt.Sprintf("%s Scan completed in %dms", IconSuccess, durationMs))
	s.WriteString(BaseLayout.Render(footer))

	// Center the entire block
	content := s.String()

	fmt.Println()
	TypewriterPrint(content)
	fmt.Println()
}
