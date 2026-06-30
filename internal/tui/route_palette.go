package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

type routePaletteModel struct {
	searchInput   textinput.Model
	routes        []scanner.Route
	filtered      []scanner.Route
	cursor        int
	viewportStart int
	width         int
	height        int
}

func initialRoutePaletteModel(routes []scanner.Route) routePaletteModel {
	ti := textinput.New()
	ti.Placeholder = "Search routes..."
	ti.Prompt = "🔍 "
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	ti.PromptStyle = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ColorWhite)

	sortedRoutes := make([]scanner.Route, len(routes))
	copy(sortedRoutes, routes)
	sort.SliceStable(sortedRoutes, func(i, j int) bool {
		return sortedRoutes[i].Method < sortedRoutes[j].Method
	})

	return routePaletteModel{
		searchInput: ti,
		routes:      sortedRoutes,
		filtered:    sortedRoutes,
	}
}

func (m *routePaletteModel) filter() {
	query := strings.ToLower(m.searchInput.Value())
	if query == "" {
		m.filtered = m.routes
	} else {
		var filtered []scanner.Route
		for _, r := range m.routes {
			if strings.Contains(strings.ToLower(r.Path), query) || strings.Contains(strings.ToLower(r.Method), query) {
				filtered = append(filtered, r)
			}
		}
		m.filtered = filtered
	}
	m.cursor = 0
	m.viewportStart = 0
}

func (m routePaletteModel) Update(msg tea.Msg) (routePaletteModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor < m.viewportStart {
				m.viewportStart = m.cursor
			}
		case "down", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			if m.cursor >= m.viewportStart+8 {
				m.viewportStart = m.cursor - 7
			}
		default:
			var tiCmd tea.Cmd
			m.searchInput, tiCmd = m.searchInput.Update(msg)
			cmds = append(cmds, tiCmd)
			m.filter()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m routePaletteModel) View() string {
	var s strings.Builder

	title := lipgloss.NewStyle().Foreground(ColorWhite).Bold(true).Render("  Switch Route")
	subtitle := MutedText("  Search and jump to any endpoint")
	divider := lipgloss.NewStyle().Foreground(ColorGrayDark).Render("  " + strings.Repeat("─", m.width-8))
	
	s.WriteString(title + "\n" + divider + "\n" + subtitle + "\n\n")

	countStr := fmt.Sprintf("%d matching routes", len(m.filtered))
	if len(m.filtered) == len(m.routes) {
		countStr = fmt.Sprintf("%d routes", len(m.routes))
	}
	s.WriteString(MutedText("  "+countStr) + "\n")

	searchWidth := m.width - 8
	if searchWidth < 20 {
		searchWidth = 20
	}
	m.searchInput.Width = searchWidth - 4
	
	// Unbordered search box
	searchBox := lipgloss.NewStyle().
		Padding(0, 1).
		Width(searchWidth).
		Render(m.searchInput.View())

	s.WriteString("  " + searchBox + "\n\n")

	if len(m.filtered) == 0 {
		s.WriteString(MutedText("  No matching routes\n  Try another search.\n\n"))
	} else {
		end := m.viewportStart + 8
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		maxPathWidth := 30
		for i := m.viewportStart; i < end; i++ {
			if len(m.filtered[i].Path) > maxPathWidth {
				maxPathWidth = len(m.filtered[i].Path)
			}
		}
		if maxPathWidth > 50 {
			maxPathWidth = 50
		}

		for i := m.viewportStart; i < end; i++ {
			r := m.filtered[i]

			// Force method badge width to 6 (so DELETE fits, and GET is padded)
			methodText := r.Method
			padLen := 6 - len(methodText)
			if padLen < 0 {
				padLen = 0
			}
			
			// We can use a fixed width lipgloss style for the badge
			var badgeColor lipgloss.Color
			switch r.Method {
			case "GET":
				badgeColor = ColorGreen
			case "POST":
				badgeColor = ColorBlue
			case "PUT":
				badgeColor = ColorYellow
			case "PATCH":
				badgeColor = ColorCyan
			case "DELETE":
				badgeColor = ColorRed
			default:
				badgeColor = ColorWhite
			}
			
			badgeStyle := lipgloss.NewStyle().
				Foreground(badgeColor).
				Width(6).
				Align(lipgloss.Right)
				
			renderedMethod := badgeStyle.Render(r.Method)

			pathStr := r.Path
			if len(pathStr) > maxPathWidth {
				pathStr = pathStr[:maxPathWidth-3] + "..."
			}
			pathPad := maxPathWidth - len(pathStr)
			if pathPad < 0 {
				pathPad = 0
			}
			rawPathPadding := strings.Repeat(" ", pathPad)

			if i == m.cursor {
				// Cyan accent + brighter background
				selectedBg := lipgloss.Color("#2A2B3D") // Slightly brighter than base
				
				rowStyle := lipgloss.NewStyle().Background(selectedBg)
				
				styledMethod := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Width(6).Align(lipgloss.Right).Background(selectedBg).Render(r.Method)
				styledPath := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Background(selectedBg).Render(pathStr + rawPathPadding)
				
				rawRow := rowStyle.Render("❯ ") + " " + styledMethod + "   " + styledPath
				s.WriteString("  " + rawRow + "\n")
			} else {
				renderedPath := lipgloss.NewStyle().Foreground(ColorWhite).Render(pathStr + rawPathPadding)
				s.WriteString("      " + renderedMethod + "   " + renderedPath + "\n")
			}
		}
	}
	s.WriteString("\n")

	footerWidth := m.width - 4
	if footerWidth < 20 {
		footerWidth = 20
	}
	
	// Pills
	pillStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorGrayBase)
	
	footerStr := pillStyle.Render("[↑↓]") + " " + descStyle.Render("Navigate") + "    " +
		pillStyle.Render("[Enter]") + " " + descStyle.Render("Select") + "    " +
		pillStyle.Render("[Esc]") + " " + descStyle.Render("Cancel")

	footer := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(ColorGrayDark).
		Width(footerWidth).
		PaddingTop(1).
		Render(footerStr)

	s.WriteString("  " + footer)

	return s.String()
}
