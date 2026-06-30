package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
)

type projectSelectorModel struct {
	roots    []string
	infos    []detector.StackInfo
	cursor   int
	selected string
}

func (m projectSelectorModel) Init() tea.Cmd {
	return nil
}

func (m projectSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.roots) - 1
			}
		case "down", "j":
			if m.cursor < len(m.roots)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}
		case "enter":
			m.selected = m.roots[m.cursor]
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m projectSelectorModel) View() string {
	var s strings.Builder

	// Add top padding
	s.WriteString("\n")

	// Header
	s.WriteString(BaseLayout.Render(CardTitle(fmt.Sprintf("Found %d project roots:", len(m.roots)))))
	s.WriteString("\n\n")

	// Cards
	for i, root := range m.roots {
		info := m.infos[i]

		// Card Style logic
		cardStyle := CardStyle.Copy()
		if m.cursor == i {
			cardStyle = cardStyle.BorderForeground(ColorCyan)
		}

		// Number circle/badge
		numStr := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render(fmt.Sprintf("%d", i+1))
		
		// If selected, we can highlight the number badge
		numBadgeStyle := lipgloss.NewStyle().
			Width(3).
			Align(lipgloss.Center)
			
		if m.cursor == i {
			numBadgeStyle = numBadgeStyle.
				Background(ColorGrayDark).
				Foreground(ColorCyan)
		}
		numBadge := numBadgeStyle.Render(numStr)

		// Directory and type
		dirStr := TextValue.Render(filepath.Base(root) + "/")
		typeStr := ""
		if strings.Contains(strings.ToLower(info.Framework), "fastapi") || strings.Contains(strings.ToLower(info.Framework), "backend") {
			typeStr = MutedText("API server · Backend")
		} else {
			typeStr = MutedText("Web app · Frontend")
		}

		leftCol := lipgloss.JoinVertical(lipgloss.Left, dirStr, typeStr)

		// Badges on the right
		badges := lipgloss.JoinHorizontal(lipgloss.Top,
			FrameworkBadge(info.Framework),
			"  ",
			LanguageBadge(info.Language),
		)

		// A flexible space between the left column and badges to align them nicely.
		// For a terminal UI without exact flexbox, we can calculate padding or just use a fixed spacing.
		// Since we want aligned columns as per the UI spec, let's just pad leftCol to a fixed width.
		paddedLeftCol := lipgloss.NewStyle().Width(30).Render(leftCol)

		row := lipgloss.JoinHorizontal(lipgloss.Top,
			numBadge,
			"   ",
			paddedLeftCol,
			" ",
			badges,
		)

		s.WriteString(BaseLayout.Render(cardStyle.Render(row)))
		s.WriteString("\n\n")
	}

	// Instruction Input simulation
	s.WriteString(BaseLayout.Render(fmt.Sprintf("Select root to scan (%d/%d): ", m.cursor+1, len(m.roots))))
	// The fake blinking cursor block
	s.WriteString(lipgloss.NewStyle().Background(ColorWhite).Foreground(ColorGrayDarker).Render(" "))
	s.WriteString("\n\n")

	// Footer
	s.WriteString(BaseLayout.Render(KeyHintBar([]Key{
		{Name: "↑/↓", Desc: "Navigate"},
		{Name: "Enter", Desc: "Select"},
		{Name: "q", Desc: "Quit"},
	})))
	s.WriteString("\n")

	return s.String()
}

// SelectProjectRoot takes a slice of project root paths and returns the user's selection
// via a Bubble Tea TUI prompt.
func SelectProjectRoot(roots []string) string {
	m := projectSelectorModel{
		roots: roots,
		infos: make([]detector.StackInfo, len(roots)),
	}

	for i, root := range roots {
		m.infos[i] = detector.Detect(root)
	}

	// Enforce requirement: "backend highlighted first"
	// Let's sort the backend to the top if it exists.
	for i := range m.roots {
		if strings.Contains(strings.ToLower(m.infos[i].Framework), "fastapi") || strings.Contains(strings.ToLower(m.infos[i].Framework), "backend") {
			if i > 0 {
				m.roots[0], m.roots[i] = m.roots[i], m.roots[0]
				m.infos[0], m.infos[i] = m.infos[i], m.infos[0]
			}
			break
		}
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return roots[0]
	}

	if fm, ok := finalModel.(projectSelectorModel); ok && fm.selected != "" {
		return fm.selected
	}

	return roots[0] // fallback
}
