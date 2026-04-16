package cmd

import (
	"fmt"
	"os"
	"strings"

	"clickup-tui/pkg/ai"
	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"
	"clickup-tui/pkg/filter"
	"clickup-tui/pkg/ui"
	"clickup-tui/pkg/util"

	"github.com/charmbracelet/glamour"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	sumAll  bool
	sumMine bool
	sumTeam bool
)

var summarizeCmd = &cobra.Command{
	Use:   "summarize",
	Short: "Generate an AI summary for each configured folder",
	Long:  `Display a high-level AI summary of active tasks for each folder in your configured ClickUp workspace.`,
	Run: func(cmd *cobra.Command, args []string) {
		if sumTeam {
			sumMine = false
		}

		cfg, err := config.LoadConfig()
		if err != nil {
			if config.IsNotExist(err) {
				fmt.Println("No configuration found. Run 'clickup-tui setup' first.")
				return
			}
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		pat, err := util.GetClickUpPAT()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		client := clickup.NewClient(pat)

		summarizer, err := ai.NewSummarizer()
		if err != nil {
			fmt.Printf("Error initializing AI summarizer: %v\n", err)
			os.Exit(1)
		}

		currentUser, err := client.GetUser()
		if err != nil {
			fmt.Printf("Error getting current user: %v\n", err)
			os.Exit(1)
		}

		if len(cfg.Folders) == 0 {
			fmt.Println("No folders configured. Run 'clickup-tui setup' to select folders.")
			return
		}

		// Get terminal width
		width, _, _ := term.GetSize(int(os.Stdout.Fd()))
		if width <= 0 {
			width = 80
		}

		fmt.Println(ui.HeaderStyle.Render(fmt.Sprintf("Folder Summaries for Space: %s", cfg.SpaceName)))

		for _, folder := range cfg.Folders {
			loadSpinner := ui.NewConsoleSpinner(fmt.Sprintf("Summarizing folder: %s", folder.Name))
			loadSpinner.Start()

			lists, err := client.GetLists(folder.ID)
			if err != nil {
				loadSpinner.Stop()
				fmt.Printf("  %s\n", ui.NoTasksStyle.Render(fmt.Sprintf("Error getting lists: %v", err)))
				continue
			}

			var allFolderTasks []clickup.Task
			for _, list := range lists {
				tasks, err := client.GetTasks(list.ID)
				if err != nil {
					continue
				}

				for _, task := range tasks {
					if filter.ShouldIncludeTask(task, currentUser.ID.String(), sumAll, sumMine) {
						// Fetch full task details for better summary
						fullTask, err := client.GetTask(task.ID)
						if err == nil {
							allFolderTasks = append(allFolderTasks, fullTask)
						} else {
							allFolderTasks = append(allFolderTasks, task)
						}
					}
				}
			}

			if len(allFolderTasks) == 0 {
				loadSpinner.Stop()
				fmt.Println("  " + ui.FolderStyle.MarginTop(0).PaddingLeft(0).Render(fmt.Sprintf("Folder: %s", folder.Name)))
				fmt.Println(ui.NoTasksStyle.PaddingLeft(6).Render("No active tasks found."))
				continue
			}

			loadSpinner.Stop()
			fmt.Println("  " + ui.FolderStyle.MarginTop(0).PaddingLeft(0).Render(fmt.Sprintf("Folder: %s", folder.Name)))

			summary, err := summarizer.SummarizeTasks(folder.Name, allFolderTasks)
			if err != nil {
				fmt.Printf("      %s\n", ui.NoTasksStyle.Render(fmt.Sprintf("Error generating summary: %v", err)))
				continue
			}

			// Render Markdown using glamour
			r, _ := glamour.NewTermRenderer(
				glamour.WithStandardStyle("dark"),
				glamour.WithWordWrap(width-15),
			)
			out, _ := r.Render(summary)

			// Indent the rendered output
			lines := strings.Split(strings.TrimSpace(out), "\n")
			for _, line := range lines {
				fmt.Printf("      %s\n", line)
			}
			fmt.Println()
		}
	},
}

func init() {
	summarizeCmd.Flags().BoolVarP(&sumAll, "all", "a", false, "Include all open tasks (including backlog and scoping)")
	summarizeCmd.Flags().BoolVar(&sumTeam, "team", false, "Include work for the entire team")
	summarizeCmd.Flags().BoolVar(&sumMine, "mine", true, "Only include tasks assigned to you")
	rootCmd.AddCommand(summarizeCmd)
}
