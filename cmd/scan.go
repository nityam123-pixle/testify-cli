package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/nityam123-pixle/testify-cli/internal/detector"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
	"github.com/nityam123-pixle/testify-cli/internal/tui"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan project and list detected routes",
	Run:   runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, args []string) {
	dir, _ := os.Getwd()

	tui.PrintBanner(true)

	roots := detector.FindProjectRoots(dir)
	if len(roots) > 1 {
		dir = tui.SelectProjectRoot(roots)
	} else if len(roots) == 1 {
		dir = roots[0]
	}

	fmt.Println("  Scanning project...")
	info := detector.Detect(dir)

	if info.Framework == "" {
		fmt.Println("  Could not detect a known framework.")
		return
	}

	startScan := time.Now()
	routes := scanner.ScanRoutes(dir, info)
	duration := time.Since(startScan).Milliseconds()

	tui.PrintScanSummary(info, len(routes), "", duration)
	
	tui.PrintRoutes(routes)
}
