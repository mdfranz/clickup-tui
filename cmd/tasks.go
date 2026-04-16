package cmd

import (
	"fmt"
	"os"
	"strings"

	"clickup-tui/pkg/ai"
	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"
	"clickup-tui/pkg/filter"
	"clickup-tui/pkg/format"
	"clickup-tui/pkg/ui"
	"clickup-tui/pkg/util"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	showAll   bool
	detailed  bool
	summarize bool
	mine      bool
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Show tasks in configured folders",
	Long:  `Display tasks from your configured ClickUp workspace.\n\nFlags:\n  --all, -a: Show all open tasks (includes backlog and scoping)\n  --detailed, -d: Show the last 3 comments for each task\n  --summarize, -s: Generate an AI summary of each task\n  --mine: Only show tasks assigned to you (default true)`,
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

		pat, err := util.GetClickUpPAT()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		client := clickup.NewClient(pat)

		var summarizer *ai.Summarizer
		if summarize {
			summarizer, err = ai.NewSummarizer()
			if err != nil {
				fmt.Printf("Error initializing AI summarizer: %v\n", err)
				os.Exit(1)
			}
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

		title := "Active Tasks"
		if showAll {
			title = "All Open Tasks"
		}
		if mine {
			title += " (Mine)"
		}
		fmt.Println(ui.HeaderStyle.Render(fmt.Sprintf("%s for Space: %s", title, cfg.SpaceName)))

		for _, folder := range cfg.Folders {
			fmt.Println(ui.FolderStyle.Render(fmt.Sprintf("Folder: %s", folder.Name)))

			lists, err := client.GetLists(folder.ID)
			if err != nil {
				fmt.Println(ui.NoTasksStyle.Render(fmt.Sprintf("Error getting lists: %v", err)))
				continue
			}

			if len(lists) == 0 {
				fmt.Println(ui.NoTasksStyle.Render("No lists found in this folder."))
				continue
			}

			foundTasks := false
			for _, list := range lists {
				tasks, err := client.GetTasks(list.ID)
				if err != nil {
					fmt.Println(ui.NoTasksStyle.Render(fmt.Sprintf("Error getting tasks for list %s: %v", list.Name, err)))
					continue
				}

				if len(tasks) > 0 {
					var filteredTasks []clickup.Task
					for _, task := range tasks {
						if filter.ShouldIncludeTask(task, currentUser.ID.String(), showAll, mine) {
							filteredTasks = append(filteredTasks, task)
						}
					}

					if len(filteredTasks) > 0 {
						if !foundTasks {
							foundTasks = true
						}
						fmt.Println(ui.ListStyle.Render(fmt.Sprintf("List: %s", list.Name)))
						for _, task := range filteredTasks {
							status := task.Status.Status
							sColor := ui.StatusColors[strings.ToLower(status)]
							if sColor == "" {
								sColor = ui.ColorGray
							}

							formattedDate := format.FormatTaskDate(task.DateUpdated)

							// Format assignees (excluding current user)
							var otherAssignees []string
							for _, a := range task.Assignees {
								if a.ID.String() != currentUser.ID.String() {
									otherAssignees = append(otherAssignees, a.Username)
								}
							}
							assigneesStr := ""
							if len(otherAssignees) > 0 {
								assigneesStr = ui.AssigneeStyle.Render(" @" + strings.Join(otherAssignees, ", @"))
							}

							styledStatus := ui.StatusStyle.
								Foreground(lipgloss.Color(sColor)).
								Render("[" + status + "]")
							styledID := ui.IDStyle.Render("(" + task.ID + ")")
							styledDate := ui.DateStyle.Render(formattedDate)

							fmt.Println(ui.TaskStyle.Render(fmt.Sprintf("%s %s %s %s %s", styledStatus, task.Name, assigneesStr, styledID, styledDate)))

							if summarize {
								fullTask, err := client.GetTask(task.ID)
								if err == nil {
									comments, _ := client.GetTaskComments(task.ID)
									summary, err := summarizer.SummarizeTask(fullTask, comments)
									if err == nil {
										r, _ := glamour.NewTermRenderer(
											glamour.WithStandardStyle("dark"),
											glamour.WithWordWrap(width-35),
										)
										out, _ := r.Render(summary)
										lines := strings.Split(strings.TrimSpace(out), "\n")
										for _, line := range lines {
											fmt.Printf("%s%s\n", strings.Repeat(" ", 22), ui.SummaryStyle.Render(line))
										}
									}
								}
							}

							if detailed {
								comments, err := client.GetTaskComments(task.ID)
								if err == nil && len(comments) > 0 {
									limit := 3
									if len(comments) < limit {
										limit = len(comments)
									}
									for i := 0; i < limit; i++ {
										comment := comments[i]
										commentDate := format.FormatTaskDate(comment.Date)

										prefix := "├"
										if i == limit-1 {
											prefix = "└"
										}

										// Base indentation for the comment block
										blockIndent := 22
										headerText := fmt.Sprintf("%s %s %s: ", prefix, ui.DateStyle.Render(commentDate), comment.User.Username)
										// Header width without colors
										headerWidth := lipgloss.Width(headerText)

										contentWidth := width - blockIndent - headerWidth
										if contentWidth < 20 {
											contentWidth = 20
										}

										commentText := strings.TrimSpace(comment.CommentText)

										// Use lipgloss to wrap the text
										wrapped := lipgloss.NewStyle().Width(contentWidth).Render(commentText)
										lines := strings.Split(wrapped, "\n")

										// Render first line with header
										fmt.Printf("%s%s%s\n", strings.Repeat(" ", blockIndent), ui.CommentBaseStyle.Render(headerText), ui.CommentBaseStyle.Render(lines[0]))

										// Render subsequent lines with indentation
										indent := strings.Repeat(" ", blockIndent+headerWidth)
										for j := 1; j < len(lines); j++ {
											line := strings.TrimSpace(lines[j])
											if line != "" {
												fmt.Printf("%s%s\n", indent, ui.CommentBaseStyle.Render(line))
											}
										}
									}
								}
							}
						}
					}
				}
			}

			if !foundTasks {
				msg := "No active tasks found."
				if showAll {
					msg = "No open tasks found."
				}
				fmt.Println(ui.NoTasksStyle.Render(msg))
			}
		}
	},
}

func init() {
	tasksCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all open tasks (including backlog and scoping)")
	tasksCmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show the last 3 comments for each task")
	tasksCmd.Flags().BoolVarP(&summarize, "summarize", "s", false, "Generate an AI summary of each task")
	tasksCmd.Flags().BoolVar(&mine, "mine", true, "Only show tasks assigned to you")
	rootCmd.AddCommand(tasksCmd)
}
