package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "v1.0.0"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Testify",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Testify %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
