package tui

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Copy copies text to the system clipboard using native commands.
func Copy(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		cmd = exec.Command("powershell", "Set-Clipboard")
	default:
		return fmt.Errorf("unsupported platform for clipboard operations")
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// Paste returns the current text from the system clipboard.
func Paste() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	case "windows":
		cmd = exec.Command("powershell", "Get-Clipboard")
	default:
		return "", fmt.Errorf("unsupported platform for clipboard operations")
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
