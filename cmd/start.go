package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"github.com/fatih/color"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
	"github.com/nityam123-pixle/testify-cli/internal/server"
	"github.com/nityam123-pixle/testify-cli/internal/tui"
	"github.com/spf13/cobra"
	tea "github.com/charmbracelet/bubbletea"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Testify in the current project",
	Run:   runStart,
}

func runStart(cmd *cobra.Command, args []string) {
	var stopOnce sync.Once
	printStopMessage := func() {
		stopOnce.Do(func() {
			fmt.Println()
			red := color.New(color.FgRed, color.Bold).SprintFunc()
			tui.TypewriterPrintChar(red("  Testify Stopped. Thanks for Using Testify!"), 75*time.Millisecond)
		})
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		printStopMessage()
		os.Exit(0)
	}()
	defer printStopMessage()

	dir, _ := os.Getwd()

	tui.ClearScreen()
	tui.PrintBanner(true)

	roots := detector.FindProjectRoots(dir)
	if len(roots) > 1 {
		dir = tui.SelectProjectRoot(roots)
	} else if len(roots) == 1 {
		dir = roots[0]
	}

	info, routes, duration, err := tui.RunStartupScan(dir)
	if err != nil {
		if err.Error() == "scan aborted" {
			return
		}
		fmt.Println("  Error during scan:", err)
		return
	}

	if info.Framework == "" {
		fmt.Println("  Could not detect a known framework.")
		fmt.Println("  Supported: Express, Fastify, Hono, NestJS, FastAPI, Flask, Gin")
		return
	}
	tui.PrintScanSummary(info, len(routes), "7842", duration)

	go server.Start("7842", routes, info)
	tui.PrintReady("7842")

	for {
		tui.ClearScreen()
		tui.PrintBanner(false)
		selectedRoute := tui.RunInteractive(routes)
		if selectedRoute.Method == "" {
			break
		}

		tui.ClearScreen()
		tui.PrintBanner(false)
		req := tui.BuildRequest(selectedRoute, info)
		if req.Method == "" {
			continue // user cancelled the editor
		}
		
		workspace := tui.NewWorkspaceModel(routes, selectedRoute, req, info)
		p := tea.NewProgram(workspace, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Println("Error running workspace:", err)
			break
		}
	}
}
