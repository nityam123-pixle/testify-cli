package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PrintReady displays the ready screen after the websocket server starts.
// This is a placeholder for Task UI-09.
func PrintReady(port string) {
	var s strings.Builder
	
	var cardRows []string
	wsStr := lipgloss.NewStyle().Foreground(ColorCyan).Render("ws://localhost:" + port)
	cardRows = append(cardRows, fmt.Sprintf("%s  WebSocket agent on %s", IconSuccess, wsStr))
	
	devStr := lipgloss.NewStyle().Foreground(ColorCyan).Render("testify.dev")
	cardRows = append(cardRows, fmt.Sprintf("%s  Open %s to start testing", IconInfo, devStr))
	
	cardContent := strings.Join(cardRows, "\n")
	s.WriteString(BaseLayout.Render(CardStyle.Render(cardContent)))

	content := s.String()

	fmt.Println()
	TypewriterPrint(content)
	fmt.Println()
}
