package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "clickup-tui",
	Short: "A TUI for ClickUp",
	Long:  `A TUI application built with Go, Cobra, and Bubble Tea to interact with ClickUp.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Root flags can be added here
}
