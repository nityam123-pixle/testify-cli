package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

// PrintRoutes outputs the scanned routes to the terminal with pagination
func PrintRoutes(routes []scanner.Route) {
	if len(routes) == 0 {
		fmt.Println(BaseLayout.Render(TextWarning.Render(IconWarning + "  No routes found.")))
		return
	}

	groups := make(map[string][]scanner.Route)
	var order []string
	for _, r := range routes {
		if _, exists := groups[r.File]; !exists {
			order = append(order, r.File)
		}
		groups[r.File] = append(groups[r.File], r)
	}

	fmt.Print("\n")
	headerStr := fmt.Sprintf("Routes (%d found)", len(routes))
	fmt.Println(BaseLayout.Render(CardTitle(headerStr)))
	fmt.Print("\n")

	count := 0
	page := 1
	totalPages := (len(routes) + 19) / 20
	showAll := false

	for _, file := range order {
		fileColor := ColorCyan
		if file == "testify.json (custom)" || strings.Contains(file, "(custom)") || strings.Contains(file, "(auto-detected)") {
			fileColor = ColorYellow
		}
		fileHeader := lipgloss.NewStyle().Foreground(fileColor).Bold(true).Render("📁 " + file)
		fmt.Println(BaseLayout.Render(fileHeader))

		for _, r := range groups[file] {
			// Method
			methodStr := MethodBadge(r.Method)

			// Path
			pathStr := TextValue.Render(r.Path)

			// We need aligned columns. Let's pad the path so descriptions or just layout looks clean.
			paddedPath := lipgloss.NewStyle().Width(40).Render(pathStr)
			
			// If we wanted to parse handler name as description, we could. 
			// But for now, we'll just align method and path.
			
			row := lipgloss.JoinHorizontal(lipgloss.Top, methodStr, "  ", paddedPath)
			
			// Indent the row slightly under the file header
			indentedRow := lipgloss.NewStyle().PaddingLeft(2).Render(row)
			
			fmt.Println(BaseLayout.Render(indentedRow))
			count++

			if !showAll && count%20 == 0 && count < len(routes) {
				// Pagination footer
				pageInfo := lipgloss.NewStyle().Foreground(ColorCyan).Render(fmt.Sprintf("— Page %d of %d —", page, totalPages))
				keys := KeyHintBar([]Key{
					{Name: "Enter", Desc: "Next"},
					{Name: "a", Desc: "All"},
					{Name: "q", Desc: "Quit"},
				})
				
				fmt.Print("\n")
				fmt.Print(BaseLayout.Render(lipgloss.JoinHorizontal(lipgloss.Top, pageInfo, "    ", keys)))
				fmt.Print(" ")

				reader := bufio.NewReader(os.Stdin)
				b, _ := reader.ReadByte()
				if b == 'a' {
					showAll = true
					reader.ReadString('\n')
				} else if b == 'q' {
					fmt.Println()
					return
				} else if b != '\n' {
					reader.ReadString('\n')
				}
				// Clear the pagination line
				fmt.Print("\r                                                                      \r")
				page++
			}
		}
		fmt.Println()
	}
}
