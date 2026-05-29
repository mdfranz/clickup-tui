package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"clickup-tui/pkg/ai"
	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"
	"clickup-tui/pkg/format"
	"clickup-tui/pkg/ui"
	"clickup-tui/pkg/util"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	teamStatusDays      int
	teamStatusSummarize bool
	teamStatusRaw       bool
)

var teamStatusCmd = &cobra.Command{
	Use:   "team-status",
	Short: "View team activity summary across the last n days",
	Long:  `Retrieve and aggregate active contributions, comments, and completions across all configured ClickUp folders and lists, then optionally summarize them using AI.`,
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

		client, cleanup := newCachedClient(pat)
		defer cleanup()

		var summarizer *ai.Summarizer
		if teamStatusSummarize {
			summarizer, err = ai.NewSummarizer()
			if err != nil {
				fmt.Printf("Error initializing AI summarizer: %v\n", err)
				os.Exit(1)
			}
		}

		if os.Getenv("CLICKUP_TUI_MENU") == "1" {
			m := initialTeamStatusModel(client, cfg, summarizer, teamStatusDays, teamStatusSummarize, teamStatusRaw)
			p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
			if _, err := p.Run(); err != nil {
				fmt.Printf("Error running TUI: %v\n", err)
				os.Exit(1)
			}
		} else {
			loadSpinner := ui.NewConsoleSpinner("Fetching team activity...")
			loadSpinner.Start()

			userActs, acts, taskDetails, err := fetchTeamActivity(client, cfg, teamStatusDays)
			if err != nil {
				loadSpinner.Stop()
				fmt.Printf("Error fetching team activity: %v\n", err)
				os.Exit(1)
			}

			var summaryText string
			if teamStatusSummarize && len(acts) > 0 && summarizer != nil {
				loadSpinner.UpdateMessage("Generating team AI summary...")
				summaryText, err = summarizer.SummarizeTeamActivity(teamStatusDays, userActs, taskDetails)
				if err != nil {
					loadSpinner.Stop()
					fmt.Printf("Error generating AI summary: %v\n", err)
					os.Exit(1)
				}
			}

			loadSpinner.Stop()

			width, _, _ := term.GetSize(int(os.Stdout.Fd()))
			if width <= 0 {
				width = 80
			}

			fmt.Println(generateTeamStatusDisplayContent(width, teamStatusDays, summaryText, teamStatusRaw, acts))
		}
	},
}

func fetchTeamActivity(client clickup.API, cfg config.Config, days int) (map[string][]clickup.Activity, []clickup.Activity, map[string]clickup.Task, error) {
	dateFrom := time.Now().AddDate(0, 0, -days).UnixNano() / int64(time.Millisecond)

	var activities []clickup.Activity
	var taskDetails = make(map[string]clickup.Task)

	// Fetch all users in the workspace to map user details accurately
	users, _ := client.GetWorkspaceUsers(cfg.WorkspaceID)
	userMap := make(map[string]clickup.User)
	for _, u := range users {
		userMap[u.ID.String()] = u
	}

	for _, folder := range cfg.Folders {
		lists, err := client.GetLists(folder.ID)
		if err != nil {
			continue
		}

		for _, listObj := range lists {
			tasks, err := client.GetRecentTasks(listObj.ID, dateFrom)
			if err != nil {
				continue
			}

			for _, task := range tasks {
				taskDateCreated, _ := strconv.ParseInt(task.DateCreated, 10, 64)
				taskDateUpdated, _ := strconv.ParseInt(task.DateUpdated, 10, 64)
				taskDateDone, _ := strconv.ParseInt(task.DateDone, 10, 64)
				taskDateClosed, _ := strconv.ParseInt(task.DateClosed, 10, 64)

				taskDetails[task.ID] = task

				// 1. Check if task was created within time window by a workspace user
				if taskDateCreated >= dateFrom {
					creatorID := task.Creator.ID.String()
					creator, exists := userMap[creatorID]
					if !exists {
						creator = task.Creator
					}
					activities = append(activities, clickup.Activity{
						ID:     "create-" + task.ID,
						User:   creator,
						Type:   fmt.Sprintf("created task [%s]", task.Status.Status),
						Date:   task.DateCreated,
						TaskID: task.ID,
						Source: task.Name,
					})
				}

				// 2. Check for completions, closures, or general updates by assignees
				for _, assignee := range task.Assignees {
					assigneeID := assignee.ID.String()
					actualAssignee, exists := userMap[assigneeID]
					if !exists {
						actualAssignee = assignee
					}

					if taskDateDone >= dateFrom {
						activities = append(activities, clickup.Activity{
							ID:     "done-" + task.ID + "-" + task.DateDone + "-" + assigneeID,
							User:   actualAssignee,
							Type:   fmt.Sprintf("completed task [%s]", task.Status.Status),
							Date:   task.DateDone,
							TaskID: task.ID,
							Source: task.Name,
						})
					} else if taskDateClosed >= dateFrom {
						activities = append(activities, clickup.Activity{
							ID:     "closed-" + task.ID + "-" + task.DateClosed + "-" + assigneeID,
							User:   actualAssignee,
							Type:   fmt.Sprintf("closed task [%s]", task.Status.Status),
							Date:   task.DateClosed,
							TaskID: task.ID,
							Source: task.Name,
						})
					} else if taskDateUpdated >= dateFrom && taskDateUpdated > taskDateCreated {
						activities = append(activities, clickup.Activity{
							ID:     "update-" + task.ID + "-" + task.DateUpdated + "-" + assigneeID,
							User:   actualAssignee,
							Type:   fmt.Sprintf("updated task [%s]", task.Status.Status),
							Date:   task.DateUpdated,
							TaskID: task.ID,
							Source: task.Name,
						})
					}
				}

				// 3. Fetch task comments to attribute to respective authors
				// Only fetch if task was updated within the window (avoids N+1 on stale tasks)
				if taskDateUpdated >= dateFrom {
					comments, err := client.GetTaskComments(task.ID)
					if err == nil {
						for _, comment := range comments {
							commentDate, _ := strconv.ParseInt(comment.Date, 10, 64)
							if commentDate >= dateFrom {
								commenterID := comment.User.ID.String()
								commenter, exists := userMap[commenterID]
								if !exists {
									commenter = comment.User
								}
								activities = append(activities, clickup.Activity{
									ID:     "comment-" + comment.ID,
									User:   commenter,
									Type:   "commented on task",
									Date:   comment.Date,
									TaskID: task.ID,
									Source: task.Name,
									Detail: comment.CommentText,
								})
							}
						}
					}
				}
			}
		}
	}

	// Sort activities by date descending
	sort.Slice(activities, func(i, j int) bool {
		timeI, _ := strconv.ParseInt(activities[i].Date, 10, 64)
		timeJ, _ := strconv.ParseInt(activities[j].Date, 10, 64)
		return timeI > timeJ
	})

	// Group activities by username
	userActivities := make(map[string][]clickup.Activity)
	for _, a := range activities {
		username := a.User.Username
		if username == "" {
			username = "Unknown User"
		}
		userActivities[username] = append(userActivities[username], a)
	}

	return userActivities, activities, taskDetails, nil
}

func generateTeamStatusDisplayContent(width int, days int, summaryText string, raw bool, activities []clickup.Activity) string {
	if width <= 0 {
		width = 80
	}

	var b strings.Builder
	title := fmt.Sprintf("Team Status for the last %d days", days)
	b.WriteString(ui.HeaderStyle.Render(title) + "\n\n")

	if summaryText != "" {
		glamourStyle := "dark"
		if !lipgloss.HasDarkBackground() {
			glamourStyle = "light"
		}

		r, _ := glamour.NewTermRenderer(
			glamour.WithStandardStyle(glamourStyle),
			glamour.WithWordWrap(width-10),
		)

		out, err := r.Render(summaryText)
		if err != nil {
			b.WriteString(summaryText)
		} else {
			b.WriteString(strings.TrimSpace(out))
		}
		b.WriteString("\n\n")
	}

	if len(activities) == 0 {
		b.WriteString(fmt.Sprintf("No team activity found in the last %d days.", days))
	} else if summaryText == "" || raw {
		if summaryText != "" {
			b.WriteString(ui.HeaderStyle.Render("Raw Team Activity Log") + "\n\n")
		}

		activityWrapStyle := lipgloss.NewStyle().Width(width - 6)

		for _, a := range activities {
			date := format.FormatCommentDate(a.Date)
			activityLine := fmt.Sprintf("%s %s: %s", ui.DateStyle.Render(date), ui.AssigneeStyle.Render(a.User.Username), a.Type)
			if a.Source != "" {
				activityLine += fmt.Sprintf(" (%s)", a.Source)
			}
			if a.Detail != "" {
				// Flatten newlines and collapse spaces for a clean single-line output
				detailClean := strings.Join(strings.Fields(a.Detail), " ")
				activityLine += fmt.Sprintf("\n      └ %s", detailClean)
			}
			b.WriteString(activityWrapStyle.Render(activityLine))
			b.WriteString("\n")
		}
	}
	return b.String()
}

type teamStatusStep int

const (
	teamStatusStepLoading teamStatusStep = iota
	teamStatusStepDisplay
)

type teamStatusModel struct {
	client         clickup.API
	cfg            config.Config
	summarizer     *ai.Summarizer
	days           int
	summarize      bool
	raw            bool
	step           teamStatusStep
	activities     []clickup.Activity
	userActivities map[string][]clickup.Activity
	taskDetails    map[string]clickup.Task
	summaryText    string
	spinner        spinner.Model
	viewport       viewport.Model
	ready          bool
	quitting       bool
	err            error
	width          int
	height         int
}

type teamStatusLoadedMsg struct {
	userActivities map[string][]clickup.Activity
	activities     []clickup.Activity
	taskDetails    map[string]clickup.Task
	summaryText    string
}

func initialTeamStatusModel(client clickup.API, cfg config.Config, summarizer *ai.Summarizer, days int, summarize bool, raw bool) teamStatusModel {
	return teamStatusModel{
		client:     client,
		cfg:        cfg,
		summarizer: summarizer,
		days:       days,
		summarize:  summarize,
		raw:        raw,
		step:       teamStatusStepLoading,
		spinner:    ui.NewSpinnerModel(),
	}
}

func (m teamStatusModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadData)
}

func (m teamStatusModel) loadData() tea.Msg {
	userActs, acts, taskDetails, err := fetchTeamActivity(m.client, m.cfg, m.days)
	if err != nil {
		return errMsg(err)
	}

	var summaryText string
	if m.summarize && len(acts) > 0 && m.summarizer != nil {
		var err error
		summaryText, err = m.summarizer.SummarizeTeamActivity(m.days, userActs, taskDetails)
		if err != nil {
			return errMsg(err)
		}
	}

	return teamStatusLoadedMsg{
		userActivities: userActs,
		activities:     acts,
		taskDetails:    taskDetails,
		summaryText:    summaryText,
	}
}

func (m teamStatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}

		if m.step == teamStatusStepDisplay {
			content := generateTeamStatusDisplayContent(m.width, m.days, m.summaryText, m.raw, m.activities)
			m.viewport.SetContent(ui.DocStyle.Width(m.width).Render(content))
		}
		return m, nil

	case spinner.TickMsg:
		if m.step == teamStatusStepLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case teamStatusLoadedMsg:
		m.activities = msg.activities
		m.userActivities = msg.userActivities
		m.taskDetails = msg.taskDetails
		m.summaryText = msg.summaryText
		m.step = teamStatusStepDisplay

		if m.ready {
			content := generateTeamStatusDisplayContent(m.width, m.days, m.summaryText, m.raw, m.activities)
			m.viewport.SetContent(ui.DocStyle.Width(m.width).Render(content))
			m.viewport.GotoTop()
		}
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" || msg.String() == "esc" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	if m.step == teamStatusStepDisplay {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m teamStatusModel) View() string {
	if m.err != nil {
		return ui.DocStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	}

	if m.quitting {
		return ""
	}

	switch m.step {
	case teamStatusStepLoading:
		return ui.DocStyle.Render(ui.SpinnerView("Fetching team activity & generating AI summary...", m.spinner))
	case teamStatusStepDisplay:
		if !m.ready {
			return "\n  Initializing..."
		}
		footer := "\n\n(q/esc: back to menu | ↑/↓: scroll)"
		return fmt.Sprintf("%s%s", m.viewport.View(), footer)
	}
	return ""
}

func init() {
	teamStatusCmd.Flags().IntVarP(&teamStatusDays, "days", "d", 7, "Window of activity in days")
	teamStatusCmd.Flags().BoolVarP(&teamStatusSummarize, "summarize", "s", true, "Generate an AI summary of team activity")
	teamStatusCmd.Flags().BoolVarP(&teamStatusRaw, "raw", "c", false, "Show raw activity log in addition to the AI summary")
	rootCmd.AddCommand(teamStatusCmd)
}
