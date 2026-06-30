package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

type routeSelectorModel struct {
	routes        []scanner.Route
	cursor        int
	selected      int
	viewportStart int
}

func (m routeSelectorModel) Init() tea.Cmd {
	return nil
}

func (m routeSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.selected = -1
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.viewportStart {
				m.viewportStart = m.cursor
			}
		case "down", "j":
			if m.cursor < len(m.routes)-1 {
				m.cursor++
			}
			if m.cursor >= m.viewportStart+15 {
				m.viewportStart = m.cursor - 14
			}
		case "enter":
			m.selected = m.cursor
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m routeSelectorModel) View() string {
	var s strings.Builder

	// Top Padding
	s.WriteString("\n")

	// Header
	headerStr := fmt.Sprintf("Select a route to test (%d/%d)", m.cursor+1, len(m.routes))
	s.WriteString(BaseLayout.Render(CardTitle(headerStr)))
	s.WriteString("\n\n")

	// Viewport logic
	end := m.viewportStart + 15
	if end > len(m.routes) {
		end = len(m.routes)
	}

	// Calculate max path width for neat alignment
	maxPathWidth := 30
	for i := m.viewportStart; i < end; i++ {
		if len(m.routes[i].Path) > maxPathWidth {
			maxPathWidth = len(m.routes[i].Path)
		}
	}
	if maxPathWidth > 50 {
		maxPathWidth = 50 // cap it
	}

	// Render routes in viewport
	var cardRows []string
	
	if m.viewportStart > 0 {
		cardRows = append(cardRows, MutedText(fmt.Sprintf("  ... %d more routes above", m.viewportStart)))
	} else {
		cardRows = append(cardRows, " ") // spacing
	}

	for i := m.viewportStart; i < end; i++ {
		r := m.routes[i]
		
		// If it's the first item in viewport OR the file changed, print the file header
		if i == m.viewportStart || r.File != m.routes[i-1].File {
			if i > m.viewportStart {
				cardRows = append(cardRows, "") // Blank line between groups
			}
			fileColor := ColorCyan
			if r.File == "testify.json (custom)" || strings.Contains(r.File, "(custom)") || strings.Contains(r.File, "(auto-detected)") {
				fileColor = ColorYellow
			}
			fileHeader := lipgloss.NewStyle().Foreground(fileColor).Bold(true).Render("  📁 " + r.File)
			cardRows = append(cardRows, fileHeader)
		}

		methodStr := r.Method
		renderedMethod := MethodBadge(r.Method)
		padLen := 7 - len(methodStr)
		if padLen < 0 {
			padLen = 0
		}
		renderedMethod += strings.Repeat(" ", padLen)
		
		pathStr := r.Path
		if len(pathStr) > maxPathWidth {
			pathStr = pathStr[:maxPathWidth-3] + "..."
		}
		pathPad := maxPathWidth - len(pathStr)
		if pathPad < 0 {
			pathPad = 0
		}
		
		renderedPath := lipgloss.NewStyle().Foreground(ColorWhite).Render(pathStr + strings.Repeat(" ", pathPad))
		
		if i == m.cursor {
			// To add a selection arrow without breaking ANSI codes, we render the raw string without the leading space, and prepend the arrow
			rawRow := fmt.Sprintf("%s %s", renderedMethod, renderedPath)
			styledRow := lipgloss.NewStyle().Background(ColorGrayDark).Render(rawRow)
			cardRows = append(cardRows, lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("> ") + styledRow)
		} else {
			cardRows = append(cardRows, "  " + fmt.Sprintf("%s %s", renderedMethod, renderedPath))
		}
	}

	if end < len(m.routes) {
		cardRows = append(cardRows, MutedText(fmt.Sprintf("  ... %d more routes below", len(m.routes)-end)))
	} else {
		cardRows = append(cardRows, " ") // spacing
	}

	s.WriteString(BaseLayout.Render(strings.Join(cardRows, "\n")))
	s.WriteString("\n\n")
	
	// Hints
	var hints []Key
	hints = append(hints, Key{Name: "↑/↓", Desc: "Navigate"})
	hints = append(hints, Key{Name: "Enter", Desc: "Select"})
	hints = append(hints, Key{Name: "Esc", Desc: "Cancel"})
	
	s.WriteString(BaseLayout.Render(KeyHintBar(hints)))
	s.WriteString("\n")

	return s.String()
}

func RunInteractive(routes []scanner.Route) scanner.Route {
	p := tea.NewProgram(routeSelectorModel{routes: routes, cursor: 0, selected: -1, viewportStart: 0})
	m, err := p.Run()
	if err != nil {
		fmt.Printf("Error running TUI: %v\n", err)
		return scanner.Route{}
	}
	model := m.(routeSelectorModel)
	if model.selected >= 0 && model.selected < len(routes) {
		return routes[model.selected]
	}
	return scanner.Route{}
}
