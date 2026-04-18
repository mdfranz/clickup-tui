package cmd

import (
	"fmt"
	"os"

	"clickup-tui/pkg/config"
	"clickup-tui/pkg/util"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			if config.IsNotExist(err) {
				fmt.Println("No configuration found. Run 'clickup-tui setup' first.")
				return
			}
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Current Configuration:\n")
		fmt.Printf("  Workspace: %s (%s)\n", cfg.WorkspaceName, cfg.WorkspaceID)
		fmt.Printf("  Space:     %s (%s)\n", cfg.SpaceName, cfg.SpaceID)
		fmt.Printf("  Folders:\n")
		for _, f := range cfg.Folders {
			fmt.Printf("    - %s (%s)\n", f.Name, f.ID)
		}

		pat, err := util.GetClickUpPAT()
		if err == nil {
			client, cleanup := newCachedClient(pat)
			defer cleanup()
			user, err := client.GetUser()
			if err == nil {
				fmt.Printf("\nUser Details:\n")
				fmt.Printf("  Name:    %s\n", user.Username)
				fmt.Printf("  User ID: %d\n", user.ID)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
