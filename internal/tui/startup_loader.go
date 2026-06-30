package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

var customSpinner = spinner.Spinner{
	Frames: []string{"◐", "◓", "◑", "◒"},
	FPS:    time.Second / 10,
}

type detectResultMsg struct {
	info detector.StackInfo
}

type routesResultMsg struct {
	routes []scanner.Route
}

type startupModel struct {
	dir      string
	spinner  spinner.Model
	message  string
	info     *detector.StackInfo
	routes   []scanner.Route
	quitting bool
	duration int64
	start    time.Time
}

func doDetect(dir string) tea.Cmd {
	return func() tea.Msg {
		info := detector.Detect(dir)
		return detectResultMsg{info: info}
	}
}

func doScan(dir string, stackInfo detector.StackInfo) tea.Cmd {
	return func() tea.Msg {
		routes := scanner.ScanRoutes(dir, stackInfo)
		return routesResultMsg{routes: routes}
	}
}

func (m startupModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doDetect(m.dir))
}

func (m startupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case detectResultMsg:
		m.info = &msg.info
		if msg.info.Framework == "" {
			m.quitting = true
			return m, tea.Quit
		}
		m.message = "Scanning routes..."
		return m, doScan(m.dir, msg.info)
	case routesResultMsg:
		m.routes = msg.routes
		m.duration = time.Since(m.start).Milliseconds()
		m.message = "Preparing workspace..."
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m startupModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")

	spinStr := lipgloss.NewStyle().Foreground(ColorCyan).Render(m.spinner.View())
	msgStr := TextValue.Render(m.message)
	
	row := lipgloss.JoinHorizontal(lipgloss.Center, "  ", spinStr, " ", msgStr)
	
	width := GetTerminalWidth()
	centered := lipgloss.PlaceHorizontal(width, lipgloss.Center, row)
	
	s.WriteString(centered)
	s.WriteString("\n\n")
	
	return s.String()
}

// RunStartupScan displays a multi-stage spinner while analyzing the project
func RunStartupScan(dir string) (detector.StackInfo, []scanner.Route, int64, error) {
	s := spinner.New()
	s.Spinner = customSpinner
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	m := startupModel{
		dir:     dir,
		spinner: s,
		message: "Detecting framework...",
		start:   time.Now(),
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return detector.StackInfo{}, nil, 0, err
	}

	fm, ok := finalModel.(startupModel)
	if !ok || fm.info == nil {
		return detector.StackInfo{}, nil, 0, fmt.Errorf("scan aborted")
	}

	return *fm.info, fm.routes, fm.duration, nil
}
