package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "testify",
	Short: "Testify — instant API testing for your terminal",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(addCmd)
	
	customHelp := func(cmd *cobra.Command, args []string) {
		fmt.Println("\033[36m\033[1mTESTIFY v1.0.0\033[0m")
		fmt.Println("\033[90mInstant API testing for your terminal.\033[0m\n")
		
		fmt.Println("\033[36mUSAGE\033[0m")
		fmt.Printf("  testify [command]\n\n")
		
		fmt.Println("\033[36mCORE COMMANDS\033[0m")
		fmt.Println("  \033[32mstart\033[0m      Start the interactive workspace (TUI)")
		fmt.Println("  \033[32mscan\033[0m       Scan project and list detected routes")
		
		fmt.Println("\n\033[36mADDITIONAL COMMANDS\033[0m")
		fmt.Println("  \033[32madd\033[0m        Interactively add a custom route to testify.json")
		fmt.Println("  \033[32mhistory\033[0m    View the last 20 test executions")
		fmt.Println("  \033[32mversion\033[0m    Print the version number of Testify")
		fmt.Println("  \033[32mhelp\033[0m       Help about any command")
		
		fmt.Println("\n\033[36mFLAGS\033[0m")
		fmt.Println("  -h, --help   Show help for testify")
		
		fmt.Println("\nUse \"testify [command] --help\" for more information about a command.")
	}
	
	rootCmd.SetHelpFunc(customHelp)
	rootCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		customHelp(cmd, nil)
		return nil
	})
}
