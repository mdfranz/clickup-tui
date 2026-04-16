package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	showAll  bool
	detailed bool

	// Styles
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			MarginTop(1).
			Underline(true)

	folderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginTop(1).
			PaddingLeft(2)

	listStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("245")).
			PaddingLeft(4)

	taskStyle = lipgloss.NewStyle().
			PaddingLeft(6)

	statusStyle = lipgloss.NewStyle().
			Bold(true).
			Width(15)

	idStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	assigneeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("211"))

	dateStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	commentBaseStyle = lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("242"))

	noTasksStyle = lipgloss.NewStyle().
			Italic(true).
			Foreground(lipgloss.Color("240")).
			PaddingLeft(4)
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Show tasks in configured folders",
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

		pat := os.Getenv("CLICKUP_PAT")
		if pat == "" {
			fmt.Println("Error: CLICKUP_PAT environment variable not set")
			os.Exit(1)
		}

		client := clickup.NewClient(pat)

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

		title := "Active Tasks (In Progress/In Review)"
		if showAll {
			title = "All Open Tasks"
		}
		fmt.Println(headerStyle.Render(fmt.Sprintf("%s for Space: %s", title, cfg.SpaceName)))

		for _, folder := range cfg.Folders {
			fmt.Println(folderStyle.Render(fmt.Sprintf("Folder: %s", folder.Name)))

			lists, err := client.GetLists(folder.ID)
			if err != nil {
				fmt.Println(noTasksStyle.Render(fmt.Sprintf("Error getting lists: %v", err)))
				continue
			}

			if len(lists) == 0 {
				fmt.Println(noTasksStyle.Render("No lists found in this folder."))
				continue
			}

			foundTasks := false
			for _, list := range lists {
				tasks, err := client.GetTasks(list.ID)
				if err != nil {
					fmt.Println(noTasksStyle.Render(fmt.Sprintf("Error getting tasks for list %s: %v", list.Name, err)))
					continue
				}

				if len(tasks) > 0 {
					var filteredTasks []clickup.Task
					for _, task := range tasks {
						status := strings.ToLower(task.Status.Status)

						if showAll {
							if status != "completed" && status != "closed" {
								filteredTasks = append(filteredTasks, task)
							}
						} else {
							if status == "in progress" || status == "in review" {
								filteredTasks = append(filteredTasks, task)
							}
						}
					}

					if len(filteredTasks) > 0 {
						if !foundTasks {
							foundTasks = true
						}
						fmt.Println(listStyle.Render(fmt.Sprintf("List: %s", list.Name)))
						for _, task := range filteredTasks {
							status := task.Status.Status
							sColor := "245" // default gray
							switch strings.ToLower(status) {
							case "in progress":
								sColor = "42" // green
							case "scoping":
								sColor = "214" // orange
							case "in review":
								sColor = "99" // purple
							case "backlog":
								sColor = "240" // dark gray
							}

							// Format task update date
							var formattedDate string
							if task.DateUpdated != "" {
								ms, err := strconv.ParseInt(task.DateUpdated, 10, 64)
								if err == nil {
									t := time.Unix(0, ms*int64(time.Millisecond))
									formattedDate = t.Format("01/02")
								}
							}

							// Format assignees (excluding current user)
							var otherAssignees []string
							for _, a := range task.Assignees {
								if a.ID != currentUser.ID {
									otherAssignees = append(otherAssignees, a.Username)
								}
							}
							assigneesStr := ""
							if len(otherAssignees) > 0 {
								assigneesStr = assigneeStyle.Render(" @" + strings.Join(otherAssignees, ", @"))
							}

							styledStatus := statusStyle.Foreground(lipgloss.Color(sColor)).Render("[" + status + "]")
							styledID := idStyle.Render("(" + task.ID + ")")
							styledDate := dateStyle.Render(formattedDate)

							fmt.Println(taskStyle.Render(fmt.Sprintf("%s %s %s %s %s", styledStatus, task.Name, assigneesStr, styledID, styledDate)))

							if detailed {
								comments, err := client.GetTaskComments(task.ID)
								if err == nil && len(comments) > 0 {
									limit := 3
									if len(comments) < limit {
										limit = len(comments)
									}
									for i := 0; i < limit; i++ {
										comment := comments[i]
										var commentDate string
										if comment.Date != "" {
											ms, err := strconv.ParseInt(comment.Date, 10, 64)
											if err == nil {
												t := time.Unix(0, ms*int64(time.Millisecond))
												commentDate = t.Format("01/02")
											}
										}
										
										prefix := "├"
										if i == limit-1 {
											prefix = "└"
										}
										
										// Base indentation for the comment block
										blockIndent := 22
										headerText := fmt.Sprintf("%s %s %s: ", prefix, dateStyle.Render(commentDate), comment.User.Username)
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
										fmt.Printf("%s%s%s\n", strings.Repeat(" ", blockIndent), commentBaseStyle.Render(headerText), commentBaseStyle.Render(lines[0]))
										
										// Render subsequent lines with indentation
										indent := strings.Repeat(" ", blockIndent+headerWidth)
										for j := 1; j < len(lines); j++ {
											line := strings.TrimSpace(lines[j])
											if line != "" {
												fmt.Printf("%s%s\n", indent, commentBaseStyle.Render(line))
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
				fmt.Println(noTasksStyle.Render(msg))
			}
		}
	},
}

func init() {
	tasksCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all open tasks (including backlog and scoping)")
	tasksCmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show the last 3 comments for each task")
	rootCmd.AddCommand(tasksCmd)
}
