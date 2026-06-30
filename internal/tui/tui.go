package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/fatih/color"
	"golang.org/x/term"
)

var (
	cyan = color.New(color.FgCyan, color.Bold).SprintFunc()
	gray = color.New(color.FgHiBlack).SprintFunc()
)

// ClearScreen clears the terminal and moves the cursor to the top left.
func ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// GetTerminalWidth returns the current width of the terminal.
// Falls back to 80 if it cannot be determined.
func GetTerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// TypewriterPrint prints a multi-line string line-by-line with a slight delay.
func TypewriterPrint(text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		fmt.Println(line)
		time.Sleep(15 * time.Millisecond)
	}
}

// TypewriterPrintChar prints text character by character with ANSI support.
func TypewriterPrintChar(text string, speed time.Duration) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		total := ansi.StringWidth(line)
		if total == 0 {
			fmt.Println(line)
			continue
		}
		for i := 1; i <= total; i++ {
			fmt.Printf("\r%s\033[K", ansi.Truncate(line, i, ""))
			time.Sleep(speed)
		}
		fmt.Println()
	}
}

func PrintBanner(animate bool) {
	lines := []string{
		`    / ████████╗ \   ███████╗███████╗████████╗██╗███████╗██╗   ██╗`,
		`   |  ╚══██╔══╝  |  ██╔════╝██╔════╝╚══██╔══╝██║██╔════╝╚██╗ ██╔╝`,
		`  <      ██║      > █████╗  ███████╗   ██║   ██║█████╗   ╚████╔╝ `,
		`   |     ██║     |  ██╔══╝  ╚════██║   ██║   ██║██╔══╝    ╚██╔╝  `,
		`    \    ╚═╝    /   ███████╗███████║   ██║   ██║██║        ██║   `,
		`                    ╚══════╝╚══════╝   ╚═╝   ╚═╝╚═╝        ╚═╝   `,
	}

	colors := []string{
		"#FF8A7A", "#FF9382", "#FF9C8A", "#FFA592", "#FFAE9A", "#FFB7A2", "#FFC0AA",
		"#FFC4B8", "#FFC8C6", "#FFCCD4", "#FFD0E2", "#F5C4E6", "#EBB8EA", "#E1ACEE",
		"#D7A0F2", "#CD94F6", "#C388FA", "#B97CFE", "#AF70FF", "#A564FF", "#9B5EFF",
		"#9168FF", "#8772FF", "#7D7CFF", "#7386FF", "#6990FF", "#5F9AFF", "#55A4FF",
		"#4BAEFF", "#41B8FF", "#4EC2FF", "#5BCCFF", "#68D6FF", "#75E0FF", "#82EAFF",
		"#8AEEDE", "#92F2BC", "#9AF69A", "#8EF5A0", "#82F4A6", "#76F3AC", "#6AF2B2",
	}

	var styledLines []string
	for _, line := range lines {
		runes := []rune(line)
		var b strings.Builder
		for i, r := range runes {
			colorIndex := int(float64(i) / float64(len(runes)-1) * float64(len(colors)-1))
			if colorIndex >= len(colors) {
				colorIndex = len(colors) - 1
			}
			s := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIndex])).Render(string(r))
			b.WriteString(s)
		}
		styledLines = append(styledLines, b.String())
	}
	
	styledLines = append(styledLines, "")
	styledLines = append(styledLines, gray("                    API testing, reimagined"))
	styledLines = append(styledLines, gray("             Fast • Beautiful • Developer First"))
	styledLines = append(styledLines, "")
	styledLines = append(styledLines, gray("──────────────────────────────────────────────────────────────────────────────"))
	
	content := strings.Join(styledLines, "\n")
	
	fmt.Println()
	if animate {
		TypewriterPrint(content)
	} else {
		fmt.Println(content)
	}
	fmt.Println()
}

