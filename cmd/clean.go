package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"clickup-tui/pkg/cache"
	"clickup-tui/pkg/config"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove configuration and cache files",
	Run: func(cmd *cobra.Command, args []string) {
		reader := bufio.NewReader(os.Stdin)

		// Check primary config
		configPath, err := config.ConfigPath()
		if err == nil {
			cleanFile(reader, configPath, "config")
		}

		// Check legacy config
		legacyPath, err := config.GetLegacyConfigPath()
		if err == nil {
			cleanFile(reader, legacyPath, "legacy config")
		}

		// Check cache
		cachePath := cache.CachePath()
		cleanFile(reader, cachePath, "cache")
	},
}

func cleanFile(reader *bufio.Reader, path string, label string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}

	fmt.Printf("Remove %s file at %s? [y/N]: ", label, path)
	response, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		if err := os.Remove(path); err != nil {
			fmt.Printf("Error removing %s: %v\n", label, err)
		} else {
			fmt.Printf("%s removed.\n", strings.Title(label))
		}
	}
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
