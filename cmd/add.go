package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/charmbracelet/lipgloss"

	"github.com/nityam123-pixle/testify-cli/internal/scanner"
	"github.com/nityam123-pixle/testify-cli/internal/tui"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Interactively add a custom route to testify.json",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)
		
		errStyle := lipgloss.NewStyle().Foreground(tui.ColorRed)
		successStyle := lipgloss.NewStyle().Foreground(tui.ColorGreen)
		dimStyle := lipgloss.NewStyle().Foreground(tui.ColorGrayLight)
		whiteStyle := lipgloss.NewStyle().Foreground(tui.ColorWhite)

		// Step 1: Method
		var method string
		for attempts := 0; attempts < 3; attempts++ {
			fmt.Print("  Method (GET/POST/PUT/PATCH/DELETE): ")
			input, _ := reader.ReadString('\n')
			input = strings.ToUpper(strings.TrimSpace(input))

			if input == "GET" || input == "POST" || input == "PUT" || input == "PATCH" || input == "DELETE" {
				method = input
				break
			}
			fmt.Println(errStyle.Render("  Invalid method. Must be GET, POST, PUT, PATCH, or DELETE."))
		}
		if method == "" {
			fmt.Println(errStyle.Render("  Too many failed attempts. Exiting."))
			os.Exit(1)
		}

		// Step 2: Path
		var path string
		for {
			fmt.Print("  Path (e.g. /api/auth/sign-up/email): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if strings.HasPrefix(input, "/") {
				path = input
				break
			}
			fmt.Println(errStyle.Render("  Invalid path. Must start with a forward slash (/)."))
		}

		// Step 3: Label
		fmt.Print("  Label/source (e.g. better-auth, optional — press Enter to skip): ")
		label, _ := reader.ReadString('\n')
		label = strings.TrimSpace(label)
		if label == "" {
			label = "custom"
		}

		// Step 4: Confirm
		badge := tui.MethodBadge(method)
		pathStr := whiteStyle.Render(path)
		labelStr := dimStyle.Render(" (" + label + ")")
		fmt.Printf("\n%s %s%s\n", badge, pathStr, labelStr)

		fmt.Print("  Add this route? (y/n): ")
		confirm, _ := reader.ReadString('\n')
		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "y" && confirm != "yes" {
			fmt.Println("  Cancelled.")
			os.Exit(0)
		}

		// Step 5: Write
		dir, _ := os.Getwd()
		configPath := filepath.Join(dir, "testify.json")
		
		var config scanner.TestifyConfig
		data, err := os.ReadFile(configPath)
		if err == nil {
			_ = json.Unmarshal(data, &config)
		}

		newRoute := scanner.Route{
			Method: method,
			Path:   path,
			File:   label,
		}
		config.CustomRoutes = append(config.CustomRoutes, newRoute)

		newData, _ := json.MarshalIndent(config, "", "  ")
		if err := os.WriteFile(configPath, newData, 0644); err != nil {
			fmt.Println(errStyle.Render("  Error writing to testify.json: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println(successStyle.Render("  ✓ Route added to testify.json"))
		fmt.Println(dimStyle.Render("  Run testify start to test it."))
	},
}
