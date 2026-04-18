package cmd

import (
	"fmt"
	"os"

	"clickup-tui/pkg/cache"
	"clickup-tui/pkg/clickup"

	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local API response cache",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Delete the local cache file",
	Run: func(cmd *cobra.Command, args []string) {
		path := cache.CachePath()
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No cache file found.")
				return
			}
			fmt.Printf("Error removing cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cache cleared.")
	},
}

var cacheInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show cache statistics",
	Run: func(cmd *cobra.Command, args []string) {
		path := cache.CachePath()
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No cache file found.")
				return
			}
			fmt.Printf("Error reading cache: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Cache file: %s\n", path)
		fmt.Printf("File size: %d bytes\n", info.Size())
		fmt.Println()

		// Load and display detailed stats
		inner := &noopClient{}
		cached, err := cache.NewCachedClient(inner, true)
		if err != nil {
			fmt.Printf("Error loading cache: %v\n", err)
			return
		}
		fmt.Println(cached.Info())
	},
}

func init() {
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheInfoCmd)
	rootCmd.AddCommand(cacheCmd)
}

// noopClient is a placeholder that satisfies clickup.API for cache info loading.
type noopClient struct{}

func (n *noopClient) GetTeams() ([]clickup.Team, error)                  { return nil, nil }
func (n *noopClient) GetUser() (clickup.User, error)                     { return clickup.User{}, nil }
func (n *noopClient) GetSpaces(string) ([]clickup.Space, error)          { return nil, nil }
func (n *noopClient) GetFolders(string) ([]clickup.Folder, error)        { return nil, nil }
func (n *noopClient) GetLists(string) ([]clickup.List, error)            { return nil, nil }
func (n *noopClient) GetList(string) (clickup.List, error)               { return clickup.List{}, nil }
func (n *noopClient) GetTasks(string) ([]clickup.Task, error)            { return nil, nil }
func (n *noopClient) GetRecentTasks(string, int64) ([]clickup.Task, error) { return nil, nil }
func (n *noopClient) GetTask(string) (clickup.Task, error)               { return clickup.Task{}, nil }
func (n *noopClient) GetTaskComments(string) ([]clickup.Comment, error)  { return nil, nil }
func (n *noopClient) GetWorkspaceUsers(string) ([]clickup.User, error)   { return nil, nil }
func (n *noopClient) UpdateTaskStatus(string, string) error              { return nil }
func (n *noopClient) CreateTaskComment(string, string) error             { return nil }
func (n *noopClient) CreateTask(string, string, string, string, []int64) (clickup.Task, error) {
	return clickup.Task{}, nil
}
