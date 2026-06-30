package cmd

import (
	"fmt"
	"os"

	"github.com/nityam123-pixle/testify-cli/internal/history"
	"github.com/nityam123-pixle/testify-cli/internal/tui"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "View the last 20 test executions",
	Run:   runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) {
	dir, _ := os.Getwd()
	entries, err := history.Load(dir)
	if err != nil {
		fmt.Printf("Error loading history: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Println("No history found for this project.")
		return
	}

	fmt.Println()
	fmt.Println("  Recent tests:")
	fmt.Println()

	// Show last 20 entries newest-first
	start := len(entries) - 1
	end := len(entries) - 20
	if end < 0 {
		end = 0
	}

	for i := start; i >= end; i-- {
		e := entries[i]
		timeStr := e.Timestamp.Format("15:04:05")
		statusStr := tui.StatusBadge(e.Status)
		fmt.Printf("  [%s]  %-6s  %-30s  %s  (%s)\n", timeStr, e.Method, e.Path, statusStr, e.Duration)
	}
	fmt.Println()
}
