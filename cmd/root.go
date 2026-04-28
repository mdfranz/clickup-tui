package cmd

import (
	"fmt"
	"os"

	"clickup-tui/pkg/cache"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

var clearCache bool
var noCache bool

var rootCmd = &cobra.Command{
	Use:   "clickup-tui",
	Short: "A TUI for ClickUp",
	Long: `A TUI application built with Go, Cobra, and Bubble Tea to interact with ClickUp.`,
	Version: Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if clearCache {
			path := cache.CachePath()
			_ = os.Remove(path) // Ignore error if it doesn't exist
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("version", "v", false, "Print version and exit")
	rootCmd.PersistentFlags().BoolVarP(&noCache, "refresh", "r", false, "Bypass cache and fetch fresh data from the API")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "Clear the local cache before running")
}
