package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "clickup-tui",
	Short: "A TUI for ClickUp",
	Long: `A TUI application built with Go, Cobra, and Bubble Tea to interact with ClickUp.

Commands:
  setup    - Configure your workspace, space, and folders
  tasks    - Display tasks from your workspace
  browse   - Interactively browse tasks
  show     - Display current configuration

Examples:
  clickup-tui setup                    # Configure workspace
  clickup-tui tasks --all              # Show all open tasks
  clickup-tui tasks --detailed         # Show tasks with comments
  clickup-tui browse                   # Browse tasks interactively`,
	Version: Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version and exit")
}
