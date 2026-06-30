package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/executor"
)

type loadingModel struct {
	req      executor.Request
	spinner  spinner.Model
	resp     *executor.Response
	err      error
	quitting bool
}

type requestStartedMsg struct{}

type responseMsg struct {
	resp executor.Response
}

type requestFailedMsg struct {
	err error
}

func doRequest(req executor.Request, progress chan<- string) tea.Cmd {
	return func() tea.Msg {
		if progress != nil {
			defer close(progress)
		}
		resp, err := executor.Execute(req, progress)
		if err != nil {
			return requestFailedMsg{err: err}
		}
		return responseMsg{resp: resp}
	}
}

type progressMsg string

func listenForProgress(sub chan string) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-sub
		if !ok {
			return nil
		}
		return progressMsg(msg)
	}
}

func (m loadingModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, doRequest(m.req, nil))
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	case responseMsg:
		m.resp = &msg.resp
		m.err = nil
		m.quitting = true
		return m, tea.Quit
	case requestFailedMsg:
		m.err = msg.err
		m.quitting = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m loadingModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteString("\n")

	// e.g. "◐ Waiting for response..."
	spinStr := lipgloss.NewStyle().Foreground(ColorCyan).Render(m.spinner.View())
	row := lipgloss.JoinHorizontal(lipgloss.Center, "  ", spinStr, " Waiting for response... ")
	
	s.WriteString(BaseLayout.Render(row))
	s.WriteString("\n\n")
	
	return s.String()
}

// RunLoading displays a spinner while executing the request
func RunLoading(req executor.Request) (executor.Response, error) {
	s := spinner.New()
	s.Spinner = customSpinner
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	m := loadingModel{
		req:     req,
		spinner: s,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return executor.Response{}, err
	}

	fm, ok := finalModel.(loadingModel)
	if !ok || fm.err != nil {
		return executor.Response{}, fm.err
	}
	if fm.resp == nil {
		return executor.Response{}, fmt.Errorf("request cancelled")
	}

	return *fm.resp, nil
}
