package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
	"github.com/nityam123-pixle/testify-cli/internal/executor"
	"github.com/nityam123-pixle/testify-cli/internal/history"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

type activePane int

const (
	paneRequest activePane = 0
	paneResponse activePane = 1
)

type routeSelectedMsg struct {
	route scanner.Route
	req   executor.Request
}

type buildRequestRunner struct {
	route scanner.Route
	info  detector.StackInfo
	req   executor.Request
}

func (r *buildRequestRunner) Run() error {
	fmt.Print("\033[H\033[2J") // Clear screen natively
	PrintBanner(false)
	r.req = BuildRequest(r.route, r.info)
	return nil
}

type WorkspaceModel struct {
	routes        []scanner.Route
	showPalette   bool
	palette       routePaletteModel
	selectedRoute scanner.Route
	req           executor.Request
	info          detector.StackInfo
	dir           string

	editor        requestEditorModel
	activePane    activePane

	sending            bool
	targetStatus       string
	currentStatus      string
	statusMessage      string
	activeTab          int
	progressChan       chan string
	lastResponse       *executor.Response
	lastError          error
	responseOffset     int
	responseAnimActive bool
	responseAnimChars  int
	totalResponseChars int

	width  int
	height int

	spin spinner.Model
}



type typewriterTickMsg struct{}

func tickTypewriter() tea.Cmd {
	return tea.Tick(52*time.Millisecond, func(t time.Time) tea.Msg {
		return typewriterTickMsg{}
	})
}

func NewWorkspaceModel(routes []scanner.Route, route scanner.Route, req executor.Request, info detector.StackInfo) WorkspaceModel {
	// We need to initialize the editor model to match the request
	// But since BuildRequest might have been used, we can just use initialRequestEditorModel
	// and update it with the req values if needed.
	template := ""
	if req.Body != "" {
		template = req.Body // Use the body that was built
	} else if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		template = detectSchema(route, info)
	}

	dir, _ := os.Getwd()

	editor := initialRequestEditorModel(route, info, template)
	if auth, ok := req.Headers["Authorization"]; ok {
		editor.authInput.SetValue(auth)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorCyan)

	return WorkspaceModel{
		routes:        routes,
		selectedRoute: route,
		req:           req,
		info:          info,
		dir:           dir,
		editor:        editor,
		activePane:    paneRequest,
		spin:          s,
		palette:       initialRoutePaletteModel(routes),
		showPalette:   route.Path == "", // Show palette if no route initially selected
	}
}

func (m WorkspaceModel) Init() tea.Cmd {
	return m.spin.Tick
}

func (m WorkspaceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		if m.width < 60 {
			m.width = 60
		}
		if m.height < 20 {
			m.height = 20
		}

		leftWidth := int(float64(m.width) * 0.45) - 4

		m.editor.authInput.Width = leftWidth - 4
		m.editor.bodyInput.SetWidth(leftWidth - 4)
		m.editor.bodyInput.SetHeight(m.height - 15)

	case routeSelectedMsg:
		m.selectedRoute = msg.route
		m.req = msg.req
		
		template := ""
		if m.req.Method == "POST" || m.req.Method == "PUT" || m.req.Method == "PATCH" {
			template = detectSchema(m.selectedRoute, m.info)
		}
		if template != "" {
			m.req.Body = template
		}

		m.editor = initialRequestEditorModel(m.selectedRoute, m.info, template)
		if auth, ok := m.req.Headers["Authorization"]; ok {
			m.editor.authInput.SetValue(auth)
		}
		
		m.lastResponse = nil
		m.lastError = nil
		m.statusMessage = ""
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "/" && !m.showPalette {
			m.showPalette = true
			m.palette = initialRoutePaletteModel(m.routes) 
			return m, m.palette.searchInput.Focus()
		}

		if m.showPalette {
			switch msg.String() {
			case "esc":
				m.showPalette = false
				return m, nil
			case "enter":
				if len(m.palette.filtered) > 0 {
					newRoute := m.palette.filtered[m.palette.cursor]
					m.showPalette = false
					
					// Use tea.Exec to suspend Bubble Tea and run BuildRequest interactively
					cmdRunner := &buildRequestRunner{
						route: newRoute,
						info:  m.info,
					}
					
					return m, tea.Exec(cmdRunner, func(err error) tea.Msg {
						if cmdRunner.req.Method != "" {
							return routeSelectedMsg{
								route: newRoute,
								req:   cmdRunner.req,
							}
						}
						return nil
					})
				}
			default:
				var cmd tea.Cmd
				m.palette, cmd = m.palette.Update(msg)
				return m, cmd
			}
		}

		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyTab:
			if m.activePane == paneRequest {
				if m.editor.focusIndex == 0 && m.editor.hasBody {
					m.editor.focusIndex = 1
					m.editor.isEditingAuth = false
					m.editor.authInput.Blur()
					m.editor.bodyInput.Focus()
				} else {
					m.activePane = paneResponse
				}
			} else {
				m.activePane = paneRequest
				m.editor.focusIndex = 0
				m.editor.bodyInput.Blur()
			}
			return m, nil
		case tea.KeyShiftTab:
			if m.activePane == paneResponse {
				m.activePane = paneRequest
				if m.editor.hasBody {
					m.editor.focusIndex = 1
					m.editor.isEditingAuth = false
					m.editor.bodyInput.Focus()
				} else {
					m.editor.focusIndex = 0
				}
			} else if m.activePane == paneRequest {
				if m.editor.focusIndex == 1 {
					m.editor.focusIndex = 0
					m.editor.bodyInput.Blur()
				} else {
					m.activePane = paneResponse
				}
			}
			return m, nil
		case tea.KeyCtrlS:
			m.req.Headers["Authorization"] = m.editor.authInput.Value()
			if m.editor.hasBody {
				m.req.Body = m.editor.bodyInput.Value()
			}
			
			return m, func() tea.Msg { return requestStartedMsg{} }
		}

		if m.activePane == paneRequest {
			var newEditor tea.Model
			newEditor, cmd = m.editor.Update(msg)
			m.editor = newEditor.(requestEditorModel)
			cmds = append(cmds, cmd)
		} else if m.activePane == paneResponse {
			switch msg.String() {
			case "1":
				m.activeTab = 0
				m.responseOffset = 0
			case "2":
				m.activeTab = 1
				m.responseOffset = 0
			case "3":
				m.activeTab = 2
				m.responseOffset = 0
			case "up", "k":
				if m.responseOffset > 0 {
					m.responseOffset--
				}
			case "down", "j":
				m.responseOffset++
			case "pgup":
				m.responseOffset -= 10
				if m.responseOffset < 0 { m.responseOffset = 0 }
			case "pgdown":
				m.responseOffset += 10
			case "home":
				m.responseOffset = 0
			case "c":
				if m.lastResponse != nil {
					var content string
					if m.activeTab == 0 {
						content = m.lastResponse.Body
					} else if m.activeTab == 1 {
						var sb strings.Builder
						for k, v := range m.lastResponse.Headers {
							sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
						}
						content = sb.String()
					} else {
						content = m.lastResponse.Raw
					}
					
					err := Copy(content)
					if err != nil {
						m.statusMessage = "✗ Clipboard Unavailable"
					} else {
						m.statusMessage = "✓ Copied"
					}
					cmds = append(cmds, tickClearStatus())
				}
			case "r":
				m.lastResponse = nil
				m.lastError = nil
				m.statusMessage = "✓ Cleared"
				cmds = append(cmds, tickClearStatus())
			}
		}

	case requestStartedMsg:
		m.sending = true
		m.targetStatus = "Preparing request..."
		m.currentStatus = ""
		m.statusMessage = ""
		m.progressChan = make(chan string)
		
		cmds = append(cmds, m.spin.Tick)
		cmds = append(cmds, doRequest(m.req, m.progressChan))
		cmds = append(cmds, listenForProgress(m.progressChan))
		cmds = append(cmds, tickTypewriter())

	case progressMsg:
		m.targetStatus = string(msg)
		cmds = append(cmds, listenForProgress(m.progressChan))

	case responseMsg:
		m.sending = false
		m.lastResponse = &msg.resp
		m.lastError = nil
		m.targetStatus = fmt.Sprintf("✓ %d %s", msg.resp.Status, msg.resp.StatusText)
		
		entry := history.Entry{
			Timestamp:    time.Now(),
			Method:       m.req.Method,
			Path:         m.req.Path,
			Status:       msg.resp.Status,
			Duration:     msg.resp.Duration,
			RequestBody:  m.req.Body,
			ResponseBody: msg.resp.Body,
		}
		_ = history.Save(m.dir, entry)
		cmds = append(cmds, tickClearStatus())
		
		m.responseAnimActive = true
		m.responseAnimChars = 0
		
		// Need to calculate total visible characters
		var contentLines []string
		if m.activeTab == 0 {
			contentLines = strings.Split(highlightJSON(msg.resp.Body), "\n")
		} else if m.activeTab == 1 {
			for k, v := range msg.resp.Headers {
				contentLines = append(contentLines, AlignedRow("  "+k, TextValue.Render(v)))
			}
		} else if m.activeTab == 2 {
			contentLines = strings.Split(msg.resp.Raw, "\n")
		}
		
		m.totalResponseChars = 0
		for _, line := range contentLines {
			m.totalResponseChars += ansi.StringWidth(line) + 1 // +1 for newline
		}
		m.totalResponseChars += 500 // Padding for right pane headers/dividers
		cmds = append(cmds, tickTypewriter())

	case requestFailedMsg:
		m.sending = false
		m.lastError = msg.err
		m.targetStatus = fmt.Sprintf("✗ Error: %v", msg.err)
		cmds = append(cmds, tickClearStatus())
		
		m.responseAnimActive = true
		m.responseAnimChars = 0
		m.totalResponseChars = ansi.StringWidth(m.lastError.Error()) + 500
		cmds = append(cmds, tickTypewriter())

	case spinner.TickMsg:
		if m.sending {
			m.spin, cmd = m.spin.Update(msg)
			cmds = append(cmds, cmd)
		}
		
	case typewriterTickMsg:
		needsNextTick := false
		
		if m.currentStatus != m.targetStatus {
			if !strings.HasPrefix(m.targetStatus, m.currentStatus) {
				if len(m.currentStatus) > 0 {
					m.currentStatus = m.currentStatus[:len(m.currentStatus)-1]
				}
			} else {
				m.currentStatus = m.targetStatus[:len(m.currentStatus)+1]
			}
			needsNextTick = true
		}
		
		if m.responseAnimActive {
			if m.responseAnimChars < m.totalResponseChars {
				chunkSize := m.totalResponseChars / 45
				if chunkSize < 5 {
					chunkSize = 5
				}
				m.responseAnimChars += chunkSize
				
				if m.responseAnimChars > m.totalResponseChars {
					m.responseAnimChars = m.totalResponseChars
					m.responseAnimActive = false
				}
				needsNextTick = true
			} else {
				m.responseAnimActive = false
			}
		}
		
		if needsNextTick {
			cmds = append(cmds, tickTypewriter())
		}
		
	case clearStatusMsg:
		m.statusMessage = ""
		m.targetStatus = ""
		cmds = append(cmds, tickTypewriter())
	}

	return m, tea.Batch(cmds...)
}

func (m WorkspaceModel) View() string {
	if m.width == 0 {
		return "Initializing workspace..."
	}



	leftWidth := int(float64(m.width) * 0.45)
	rightWidth := int(float64(m.width) * 0.55)

	leftBorderColor := ColorGrayDark
	rightBorderColor := ColorGrayDark

	if m.activePane == paneRequest {
		leftBorderColor = ColorCyan
	} else {
		rightBorderColor = ColorCyan
	}

	var leftRows []string
	authLabel := "Authorization"
	if m.editor.focusIndex == 0 && m.activePane == paneRequest {
		authLabel = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("› " + authLabel)
	} else {
		authLabel = "  " + authLabel
	}

	if m.editor.isEditingAuth && m.activePane == paneRequest {
		leftRows = append(leftRows, AlignedRow(authLabel, m.editor.authInput.View()))
	} else {
		authVal := m.editor.authInput.Value()
		masked := "None"
		if authVal != "" {
			if len(authVal) > 6 {
				masked = authVal[:6] + "••••••••"
			} else {
				masked = "••••••••"
			}
		}

		authStr := fmt.Sprintf("%s [change]", masked)
		if m.editor.focusIndex == 0 && m.activePane == paneRequest {
			authStr = lipgloss.NewStyle().Foreground(ColorCyan).Render(authStr)
		} else {
			authStr = MutedText(authStr)
		}
		leftRows = append(leftRows, AlignedRow(authLabel, authStr))
	}

	leftRows = append(leftRows, AlignedRow("  Content-Type", TextValue.Render("application/json")))
	
	leftPaneDivider := lipgloss.NewStyle().Foreground(ColorGrayDark).Render("  " + strings.Repeat("─", leftWidth-8))
	leftRows = append(leftRows, leftPaneDivider)

	bodyLabel := "Body (JSON)"
	if m.editor.focusIndex == 1 && m.activePane == paneRequest {
		bodyLabel = lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("› " + bodyLabel)
	} else {
		bodyLabel = "  " + bodyLabel
	}
	leftRows = append(leftRows, bodyLabel)
	leftRows = append(leftRows, "")
	
	bodyView := lipgloss.NewStyle().PaddingLeft(4).Render(m.editor.bodyInput.View())
	leftRows = append(leftRows, bodyView)
	
	leftContent := strings.Join(leftRows, "\n")
	leftPane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(leftBorderColor).
		Width(leftWidth - 2).
		Height(m.height - 6).
		Padding(1, 1).
		Render(leftContent)

	var rightRows []string
	
	if m.sending {
		rightRows = append(rightRows, lipgloss.NewStyle().Foreground(ColorCyan).Render(fmt.Sprintf("%s %s", m.spin.View(), m.currentStatus)))
		rightRows = append(rightRows, "")
	}

	if m.lastError != nil && !m.sending {
		rightRows = append(rightRows, lipgloss.NewStyle().Foreground(ColorRed).Bold(true).Render("✗ Request Failed"))
		rightRows = append(rightRows, "")
		rightRows = append(rightRows, m.lastError.Error())
	} else if m.lastResponse != nil {
		resp := m.lastResponse
		statusStr := StatusBadge(resp.Status)
		
		sizeStr := fmt.Sprintf("%.1f KB", float64(resp.Size)/1024.0)
		if resp.Size < 1024 {
			sizeStr = fmt.Sprintf("%d B", resp.Size)
		}
		sizeStr = TextValue.Render(sizeStr)

		durStr := fmt.Sprintf("%v", resp.Duration)
		if resp.Duration > 500*time.Millisecond {
			durStr = lipgloss.NewStyle().Foreground(ColorYellow).Render(durStr)
		} else if resp.Duration > 2*time.Second {
			durStr = lipgloss.NewStyle().Foreground(ColorRed).Render(durStr)
		} else {
			durStr = lipgloss.NewStyle().Foreground(ColorGreen).Render(durStr)
		}

		rightRows = append(rightRows, AlignedRow("  Status", statusStr))
		rightRows = append(rightRows, AlignedRow("  Time", durStr))
		rightRows = append(rightRows, AlignedRow("  Size", sizeStr))
		
		paneDivider := lipgloss.NewStyle().Foreground(ColorGrayDark).Render("  " + strings.Repeat("─", rightWidth-8))
		rightRows = append(rightRows, paneDivider)
		
		tabStyle := lipgloss.NewStyle().Foreground(ColorGrayLight)
		activeTabStyle := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Underline(true)
		
		tab1 := tabStyle.Render("1 Body")
		tab2 := tabStyle.Render("2 Headers")
		tab3 := tabStyle.Render("3 Raw")
		
		if m.activeTab == 0 {
			tab1 = activeTabStyle.Render("1 Body")
		} else if m.activeTab == 1 {
			tab2 = activeTabStyle.Render("2 Headers")
		} else if m.activeTab == 2 {
			tab3 = activeTabStyle.Render("3 Raw")
		}
		
		tabs := lipgloss.JoinHorizontal(lipgloss.Left, "  ", tab1, "   ", tab2, "   ", tab3)
		rightRows = append(rightRows, tabs)
		rightRows = append(rightRows, paneDivider)
		
		maxLines := m.height - 18
		if m.sending {
			maxLines -= 2 
		}
		if maxLines < 0 {
			maxLines = 0
		}
		
		var contentLines []string
		
		if m.activeTab == 0 { // Body
			bodyText := highlightJSON(resp.Body)
			contentLines = strings.Split(bodyText, "\n")
		} else if m.activeTab == 1 { // Headers
			for k, v := range resp.Headers {
				contentLines = append(contentLines, AlignedRow("  "+k, TextValue.Render(v)))
			}
		} else if m.activeTab == 2 { // Raw
			contentLines = strings.Split(resp.Raw, "\n")
		}
		
		if len(contentLines) > maxLines {
			if m.responseOffset > len(contentLines)-maxLines {
				m.responseOffset = len(contentLines) - maxLines
			}
			if m.responseOffset < 0 {
				m.responseOffset = 0
			}

			start := m.responseOffset
			end := start + maxLines

			visibleLines := contentLines[start:end]
			if end < len(contentLines) {
				visibleLines[len(visibleLines)-1] = MutedText(fmt.Sprintf("  ... (truncated %d more lines, scroll down or 'c' to copy)", len(contentLines)-end))
			}
			if start > 0 {
				visibleLines[0] = MutedText("  ... (scrolled)")
			}
			contentLines = visibleLines
		} else {
			m.responseOffset = 0
		}
		
		rightRows = append(rightRows, strings.Join(contentLines, "\n"))
	} else if !m.sending {
		rightRows = append(rightRows, MutedText("Press Ctrl+S to send request"))
	}

	rightContent := strings.Join(rightRows, "\n")
	
	if m.responseAnimActive {
		var animLines []string
		rendered := 0
		lines := strings.Split(rightContent, "\n")
		for _, line := range lines {
			lineChars := ansi.StringWidth(line) + 1
			if rendered+lineChars <= m.responseAnimChars {
				animLines = append(animLines, line)
				rendered += lineChars
			} else if rendered < m.responseAnimChars {
				charsLeft := m.responseAnimChars - rendered
				animLines = append(animLines, ansi.Truncate(line, charsLeft, ""))
				rendered += charsLeft
				break
			} else {
				break
			}
		}
		rightContent = strings.Join(animLines, "\n")
	}

	rightPane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rightBorderColor).
		Width(rightWidth - 2).
		Height(m.height - 6).
		Padding(1, 1).
		Render(rightContent)

	panes := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	header := lipgloss.JoinHorizontal(lipgloss.Center, MethodBadge(m.selectedRoute.Method), "  ", TextValue.Render(m.selectedRoute.Path))
	
	title := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("TESTIFY v1.0")
	
	// Subtract 4 for BaseLayout horizontal padding
	padding := m.width - lipgloss.Width(title) - lipgloss.Width(header) - 4
	if padding < 0 { padding = 1 }
	
	spacer := lipgloss.NewStyle().Width(padding).Render("")
	topBar := lipgloss.JoinHorizontal(lipgloss.Center, title, spacer, header)

	var s strings.Builder

	if !m.showPalette {
		// Shortcuts with vibrant colors
		fwStr := lipgloss.NewStyle().Foreground(ColorGreen).Render(m.info.Framework)
		hostStr := lipgloss.NewStyle().Foreground(ColorYellow).Render(m.req.BaseURL)
		methStr := lipgloss.NewStyle().Foreground(ColorCyan).Render(m.req.Method)

		footerLeft := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render("TESTIFY v1.0") + MutedText(" │ ") +
			fwStr + MutedText(" │ ") + hostStr + MutedText(" │ ") + methStr + MutedText(" │ ")
		
		keyStyleTab := lipgloss.NewStyle().Foreground(ColorPurple).Bold(true)
		keyStyleSend := lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
		keyStyleRoutes := lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)
		keyStyleTabs := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true)
		keyStyleEsc := lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
		
		descStyle := lipgloss.NewStyle().Foreground(ColorGrayLight)

		footerLeft += keyStyleTab.Render("Tab") + " " + descStyle.Render("Switch") + MutedText(" │ ")
		footerLeft += keyStyleSend.Render("Ctrl+S") + " " + descStyle.Render("Send") + MutedText(" │ ")
		footerLeft += keyStyleRoutes.Render("/") + " " + descStyle.Render("Routes") + MutedText(" │ ")
		footerLeft += keyStyleTabs.Render("1/2/3") + " " + descStyle.Render("Tabs") + MutedText(" │ ")
		footerLeft += keyStyleEsc.Render("Esc") + " " + descStyle.Render("Exit")
		
		var statusStr string
		if m.sending {
			statusStr = lipgloss.NewStyle().Foreground(ColorCyan).Render(fmt.Sprintf("%s %s", m.spin.View(), m.currentStatus))
		} else if m.currentStatus != "" {
			statusStr = m.currentStatus
			if strings.HasPrefix(m.currentStatus, "✓") {
				statusStr = lipgloss.NewStyle().Foreground(ColorGreen).Render(statusStr)
			} else if strings.HasPrefix(m.currentStatus, "✗") {
				statusStr = lipgloss.NewStyle().Foreground(ColorRed).Render(statusStr)
			}
		}

		footerPadding := m.width - lipgloss.Width(footerLeft) - lipgloss.Width(statusStr) - 4
		if footerPadding < 0 {
			footerPadding = 1
		}
		
		footerSpacer := lipgloss.NewStyle().Width(footerPadding).Render("")
		footerStr := lipgloss.JoinHorizontal(lipgloss.Left, "  ", footerLeft, footerSpacer, statusStr, "  ")
		
		s.WriteString(footerStr)
	}

	workspaceView := "\n" + BaseLayout.Render(topBar) + "\n" + BaseLayout.Render(panes) + "\n" + s.String() + "\n"
	
	if m.showPalette {
		// Dim the background
		workspaceView = "\x1b[2m" + strings.ReplaceAll(workspaceView, "\x1b[0m", "\x1b[0m\x1b[2m") + "\x1b[0m"

		paletteWidth := int(float64(m.width) * 0.60)
		m.palette.width = paletteWidth
		paletteView := m.palette.View()
		
		overlayStyle := CardStyle.Copy().
			Width(paletteWidth).
			Background(lipgloss.Color("#1E1E2E")) // opaque background
			
		renderedPalette := overlayStyle.Render(paletteView)
		
		// Calculate position
		bgLines := strings.Split(workspaceView, "\n")
		fgLines := strings.Split(renderedPalette, "\n")
		
		// Remove empty trailing lines from split
		if len(bgLines) > 0 && bgLines[len(bgLines)-1] == "" {
			bgLines = bgLines[:len(bgLines)-1]
		}
		
		bgHeight := len(bgLines)
		fgHeight := len(fgLines)
		
		y := (bgHeight - fgHeight) / 2
		if y < 0 {
			y = 0
		}
		
		x := (m.width - paletteWidth) / 2
		if x < 0 {
			x = 0
		}
		
		for i, fgLine := range fgLines {
			bgIdx := y + i
			if bgIdx < 0 || bgIdx >= len(bgLines) {
				continue
			}
			
			bgLine := bgLines[bgIdx]
			
			fgWidth := lipgloss.Width(fgLine)
			bgWidth := lipgloss.Width(bgLine)
			
			if x >= bgWidth {
				continue
			}
			
			left := ansi.Truncate(bgLine, x, "")
			
			right := ""
			if x+fgWidth < bgWidth {
				right = ansi.TruncateLeft(bgLine, x+fgWidth, "")
			}
			
			bgLines[bgIdx] = left + fgLine + right
		}
		
		return strings.Join(bgLines, "\n")
	}

	return workspaceView
}

func (r *buildRequestRunner) SetStdin(io.Reader)  {}
func (r *buildRequestRunner) SetStdout(io.Writer) {}
func (r *buildRequestRunner) SetStderr(io.Writer) {}
