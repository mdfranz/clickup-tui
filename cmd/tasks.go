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

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	showAll   bool
	detailed  bool
	summarize bool
	mine      bool
	team      bool
	showID    bool
)

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Show tasks in configured folders",
	Long:  `Display tasks from your configured ClickUp workspace.\n\nFlags:\n  --all, -a: Show all open tasks (includes backlog and scoping)\n  --detailed, -d: Show the last 3 comments for each task\n  --summarize, -s: Generate an AI summary of each task\n  --team: Show tasks for the entire team (overrides --mine)\n  --mine: Only show tasks assigned to you (default true)`,
	Run: func(cmd *cobra.Command, args []string) {
		if team {
			mine = false
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

		client, cleanup := newCachedClient(pat)
		defer cleanup()

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
			loadSpinner := ui.NewConsoleSpinner(fmt.Sprintf("Loading tasks for folder: %s", folder.Name))
			loadSpinner.Start()
			var output strings.Builder
			output.WriteString(ui.FolderStyle.Render(fmt.Sprintf("Folder: %s", folder.Name)) + "\n")

			lists, err := client.GetLists(folder.ID)
			if err != nil {
				loadSpinner.Stop()
				fmt.Println(ui.NoTasksStyle.Render(fmt.Sprintf("Error getting lists: %v", err)))
				continue
			}

			if len(lists) == 0 {
				loadSpinner.Stop()
				fmt.Println(ui.NoTasksStyle.Render("No lists found in this folder."))
				continue
			}

			foundTasks := false
			for _, list := range lists {
				tasks, err := client.GetTasks(list.ID, showAll)
				if err != nil {
					output.WriteString(ui.NoTasksStyle.Render(fmt.Sprintf("Error getting tasks for list %s: %v", list.Name, err)) + "\n")
					continue
				}

				if len(tasks) > 0 {
					// Map tasks by ID for easy lookup
					taskMap := make(map[string]clickup.Task)
					for _, t := range tasks {
						taskMap[t.ID] = t
					}

					// Group subtasks by parent for easier lookup
					subtasksByParent := make(map[string][]clickup.Task)
					for _, t := range tasks {
						if t.ParentID != "" {
							subtasksByParent[t.ParentID] = append(subtasksByParent[t.ParentID], t)
						}
					}

					// Identify top-level tasks and filtered tasks
					var topLevelTasks []clickup.Task
					for _, task := range tasks {
						_, hasParent := taskMap[task.ParentID]
						if task.ParentID == "" || !hasParent {
							if filter.ShouldIncludeTask(task, currentUser.ID.String(), showAll, mine) {
								topLevelTasks = append(topLevelTasks, task)
							} else {
								// If top-level task is filtered out, check if any of its subtasks should be included
								hasVisibleSubtask := false
								for _, st := range subtasksByParent[task.ID] {
									if filter.ShouldIncludeTask(st, currentUser.ID.String(), showAll, mine) {
										hasVisibleSubtask = true
										break
									}
								}
								if hasVisibleSubtask {
									topLevelTasks = append(topLevelTasks, task)
								}
							}
						}
					}

					if len(topLevelTasks) > 0 {
						if !foundTasks {
							foundTasks = true
						}

						// Sort top-level tasks by date, newest first
						util.SortTasksByDateDesc(topLevelTasks)

						output.WriteString(ui.ListStyle.Render(fmt.Sprintf("List: %s", list.Name)) + "\n")

						// Recursive function to render task and its subtasks
						var renderTask func(t clickup.Task, depth int)
						renderTask = func(task clickup.Task, depth int) {
							status := task.Status.Status
							sColor, ok := ui.StatusColors[strings.ToLower(status)]
							if !ok {
								sColor = ui.ColorGray
							}

							formattedDate := format.FormatTaskDate(task.DateUpdated)

							// Format assignees
							var otherAssignees []string
							for _, a := range task.Assignees {
								if a.ID.String() != currentUser.ID.String() {
									displayName := a.Username
									if showID {
										displayName = fmt.Sprintf("%s (ID: %s)", a.Username, a.ID.String())
									}
									otherAssignees = append(otherAssignees, displayName)
								}
							}
							assigneesStr := ""
							if len(otherAssignees) > 0 {
								assigneesStr = ui.AssigneeStyle.Render(" @" + strings.Join(otherAssignees, ", @"))
							}

							styledStatus := ui.StatusStyle.
								Foreground(sColor).
								Render("[" + status + "]")
							styledName := ui.TaskNameStyle.Render(task.Name)
							styledDate := ui.DateStyle.Render(formattedDate)

							if depth > 0 {
								subtaskLine := fmt.Sprintf("%s%s (%s)%s", strings.Repeat(" ", 16), task.Name, formattedDate, assigneesStr)
								output.WriteString(ui.TaskStyle.Render(ui.DateStyle.Render(subtaskLine)) + "\n")
							} else {
								output.WriteString(ui.TaskStyle.Render(fmt.Sprintf("%s %s (%s)%s", styledStatus, styledName, styledDate, assigneesStr)) + "\n")
							}

							if summarize {
								fullTask, err := client.GetTask(task.ID)
								if err == nil {
									comments, _ := client.GetTaskComments(task.ID)
									// Also include subtask comments in summary
									for _, st := range subtasksByParent[task.ID] {
										stComments, _ := client.GetTaskComments(st.ID)
										for _, sc := range stComments {
											sc.CommentText = fmt.Sprintf("[%s] %s", st.Name, sc.CommentText)
											comments = append(comments, sc)
										}
									}
									util.SortCommentsByDateDesc(comments)

									summary, err := summarizer.SummarizeTask(fullTask, comments)
									if err == nil {
										indent := 22
										summaryWidth := width - indent
										if summaryWidth < 40 {
											summaryWidth = 40
										}
										wrapped := lipgloss.NewStyle().Width(summaryWidth).Render(strings.TrimSpace(summary))
										lines := strings.Split(wrapped, "\n")
										for _, line := range lines {
											output.WriteString(fmt.Sprintf("%s%s\n", strings.Repeat(" ", indent), ui.SummaryStyle.Render(line)))
										}
									}
								}
							}

							if detailed {
								comments, err := client.GetTaskComments(task.ID)
								if err == nil {
									// Also get comments for subtasks
									for _, st := range subtasksByParent[task.ID] {
										stComments, _ := client.GetTaskComments(st.ID)
										for _, sc := range stComments {
											sc.CommentText = fmt.Sprintf("[%s] %s", st.Name, sc.CommentText)
											comments = append(comments, sc)
										}
									}

									if len(comments) > 0 {
										util.SortCommentsByDateDesc(comments)

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
											output.WriteString(fmt.Sprintf("%s%s%s\n", strings.Repeat(" ", blockIndent), ui.CommentBaseStyle.Render(headerText), ui.CommentBaseStyle.Render(lines[0])))

											// Render subsequent lines with indentation
											indentStr := strings.Repeat(" ", blockIndent+headerWidth)
											for j := 1; j < len(lines); j++ {
												line := strings.TrimSpace(lines[j])
												if line != "" {
													output.WriteString(fmt.Sprintf("%s%s\n", indentStr, ui.CommentBaseStyle.Render(line)))
												}
											}
										}
									}
								}
							}

							// Render subtasks
							for _, st := range subtasksByParent[task.ID] {
								// Only show subtasks if they match filter OR if showAll is true
								if showAll || filter.ShouldIncludeTask(st, currentUser.ID.String(), showAll, mine) {
									renderTask(st, depth+1)
								}
							}
						}

						for _, task := range topLevelTasks {
							renderTask(task, 0)
							output.WriteString("\n")
						}
					}
				}
			}



			if !foundTasks {
				msg := "No active tasks found."
				if showAll {
					msg = "No open tasks found."
				}
				output.WriteString(ui.NoTasksStyle.Render(msg) + "\n")
			}
			loadSpinner.Stop()
			fmt.Print(output.String())
		}
	},
}

func init() {
	tasksCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all open tasks (including backlog)")
	tasksCmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show the last 3 comments for each task")
	tasksCmd.Flags().BoolVarP(&summarize, "summarize", "s", false, "Generate an AI summary of each task")
	tasksCmd.Flags().BoolVar(&team, "team", false, "Show tasks for the whole team")
	tasksCmd.Flags().BoolVar(&mine, "mine", true, "Only show tasks assigned to you")
	tasksCmd.Flags().BoolVar(&showID, "id", false, "Show user IDs and emails next to assignees")
	rootCmd.AddCommand(tasksCmd)
}
