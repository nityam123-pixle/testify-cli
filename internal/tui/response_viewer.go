package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/executor"
)

var (
	jsonKeyRegex = regexp.MustCompile(`"([^"]+)":`)
)

func highlightJSON(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		// keys
		line = regexp.MustCompile(`^(\s*)"([^"]+)":`).ReplaceAllStringFunc(line, func(m string) string {
			idx := strings.Index(m, "\"")
			spaces := m[:idx]
			keyStr := m[idx : len(m)-1]
			return spaces + lipgloss.NewStyle().Foreground(ColorCyan).Render(keyStr) + ":"
		})

		// strings
		line = regexp.MustCompile(`^(\s*)"([^"]*)"(,?)`).ReplaceAllStringFunc(line, func(m string) string {
			if strings.Contains(m, ":") {
				return m
			}
			parts := regexp.MustCompile(`"([^"]*)"`).FindStringIndex(m)
			if len(parts) == 2 {
				spaces := m[:parts[0]]
				valStr := m[parts[0]:parts[1]]
				suffix := m[parts[1]:]
				return spaces + lipgloss.NewStyle().Foreground(ColorGreen).Render(valStr) + suffix
			}
			return m
		})

		// numbers
		line = regexp.MustCompile(`^(\s*)([0-9.-]+)(,?)`).ReplaceAllStringFunc(line, func(m string) string {
			parts := regexp.MustCompile(`([0-9.-]+)`).FindStringIndex(m)
			if len(parts) == 2 {
				spaces := m[:parts[0]]
				valStr := m[parts[0]:parts[1]]
				suffix := m[parts[1]:]
				return spaces + lipgloss.NewStyle().Foreground(ColorYellow).Render(valStr) + suffix
			}
			return m
		})

		// booleans
		line = regexp.MustCompile(`^(\s*)(true|false)(,?)`).ReplaceAllStringFunc(line, func(m string) string {
			parts := regexp.MustCompile(`(true|false)`).FindStringIndex(m)
			if len(parts) == 2 {
				spaces := m[:parts[0]]
				valStr := m[parts[0]:parts[1]]
				suffix := m[parts[1]:]
				return spaces + lipgloss.NewStyle().Foreground(ColorPurple).Render(valStr) + suffix
			}
			return m
		})

		// null
		line = regexp.MustCompile(`^(\s*)(null)(,?)`).ReplaceAllStringFunc(line, func(m string) string {
			parts := regexp.MustCompile(`(null)`).FindStringIndex(m)
			if len(parts) == 2 {
				spaces := m[:parts[0]]
				valStr := m[parts[0]:parts[1]]
				suffix := m[parts[1]:]
				return spaces + lipgloss.NewStyle().Foreground(ColorRed).Render(valStr) + suffix
			}
			return m
		})

		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// PrintResponseInteractive displays the HTTP response metadata and body in a premium card.
func PrintResponseInteractive(resp executor.Response) string {
	fmt.Println()

	var s strings.Builder

	// Header / Metadata Rows
	var cardRows []string

	sizeStr := fmt.Sprintf("%.1f KB", float64(resp.Size)/1024.0)
	if resp.Size < 1024 {
		sizeStr = fmt.Sprintf("%d B", resp.Size)
	}

	timeColor := ColorGreen
	if resp.Duration >= 1000*time.Millisecond {
		timeColor = ColorRed
	} else if resp.Duration >= 200*time.Millisecond {
		timeColor = ColorYellow
	}
	timeStr := lipgloss.NewStyle().Foreground(timeColor).Render(resp.Duration.String())

	headerTxt := lipgloss.NewStyle().Foreground(ColorCyan).Render(fmt.Sprintf("%d headers (press h to expand)", len(resp.Headers)))

	cardRows = append(cardRows, AlignedRow("Status", StatusBadge(resp.Status)))
	cardRows = append(cardRows, AlignedRow("Time", timeStr))
	cardRows = append(cardRows, AlignedRow("Size", TextValue.Render(sizeStr)))
	cardRows = append(cardRows, AlignedRow("Headers", headerTxt))

	cardRows = append(cardRows, Divider())

	// Format Body
	bodyToPrint := resp.Body

	var js map[string]interface{}
	var jsArr []interface{}
	isJson := false
	if json.Unmarshal([]byte(resp.Body), &js) == nil {
		if pretty, err := json.MarshalIndent(js, "", "  "); err == nil {
			bodyToPrint = string(pretty)
			isJson = true
		}
	} else if json.Unmarshal([]byte(resp.Body), &jsArr) == nil {
		if pretty, err := json.MarshalIndent(jsArr, "", "  "); err == nil {
			bodyToPrint = string(pretty)
			isJson = true
		}
	}

	if isJson {
		bodyToPrint = highlightJSON(bodyToPrint)
	}

	lines := strings.Split(bodyToPrint, "\n")
	if len(lines) > 80 {
		linesToPrint := lines[:80]
		remaining := len(lines) - 80
		bodyToPrint = strings.Join(linesToPrint, "\n")
		bodyToPrint += fmt.Sprintf("\n  %s", MutedText(fmt.Sprintf("... (%d more lines — press c to copy full response to clipboard)", remaining)))
	}

	// Indent body lines slightly inside the card
	for _, line := range strings.Split(bodyToPrint, "\n") {
		cardRows = append(cardRows, "  "+line)
	}

	cardContent := strings.Join(cardRows, "\n")
	s.WriteString(BaseLayout.Render(CardStyle.Render(cardContent)))
	s.WriteString("\n\n")

	fmt.Print(s.String())

	// Interactive Loop
	reader := bufio.NewReader(os.Stdin)
	for {
		footer := KeyHintBar([]Key{
			{Name: "h", Desc: "Headers"},
			{Name: "c", Desc: "Copy"},
			{Name: "r", Desc: "Retry"},
			{Name: "Enter", Desc: "Continue"},
		})
		fmt.Println()
		fmt.Print(footer)

		b, err := reader.ReadByte()
		if err != nil {
			fmt.Println()
			return "continue"
		}

		if b == 'r' {
			reader.ReadString('\n')
			fmt.Println()
			return "retry"
		} else if b == 'c' {
			reader.ReadString('\n')
			var cmd *exec.Cmd
			if runtime.GOOS == "darwin" {
				cmd = exec.Command("pbcopy")
			} else {
				cmd = exec.Command("xclip", "-selection", "clipboard")
			}
			cmd.Stdin = strings.NewReader(resp.Body)
			err := cmd.Run()
			if err != nil {
				fmt.Println(lipgloss.NewStyle().Foreground(ColorRed).Render("\n  Failed to copy to clipboard"))
			} else {
				fmt.Println(lipgloss.NewStyle().Foreground(ColorGreen).Render("\n  " + IconSuccess + " Copied to clipboard"))
			}
		} else if b == 'h' {
			reader.ReadString('\n')
			
			var hRows []string
			for k, v := range resp.Headers {
				kStr := k
				if len(kStr) > 15 {
					kStr = kStr[:14] + "…"
				}
				vStr := v
				if len(vStr) > 40 {
					vStr = vStr[:39] + "…"
				}
				hRows = append(hRows, AlignedRow(kStr, TextValue.Render(vStr)))
			}
			fmt.Println("\n" + BaseLayout.Render(CardStyle.Render(strings.Join(hRows, "\n"))))
			fmt.Println()
		} else if b == '\n' {
			return "continue"
		} else {
			reader.ReadString('\n')
		}
	}
}
