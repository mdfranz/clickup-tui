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
	Long: `A TUI application built with Go, Cobra, and Bubble Tea to interact with ClickUp.`,
	Version: Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var noCache bool

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version and exit")
	rootCmd.PersistentFlags().BoolVarP(&noCache, "refresh", "r", false, "Bypass cache and fetch fresh data from the API")
}
