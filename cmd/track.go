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

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	trackSummarize   bool
	trackRawActivity bool
)

var trackCmd = &cobra.Command{
	Use:   "track [user_id]",
	Short: "Track user activity for the last 10 days",
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
		if trackSummarize {
			summarizer, err = ai.NewSummarizer()
			if err != nil {
				fmt.Printf("Error initializing AI summarizer: %v\n", err)
				os.Exit(1)
			}
		}

		var userID string
		if len(args) > 0 {
			userID = args[0]
		}

		m := initialTrackModel(client, cfg, summarizer, userID)

		var opts []tea.ProgramOption
		if os.Getenv("CLICKUP_TUI_MENU") == "1" {
			opts = append(opts, tea.WithAltScreen())
			opts = append(opts, tea.WithMouseCellMotion())
		}

		p := tea.NewProgram(m, opts...)

		finalModel, err := p.Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		finalTrackModel := finalModel.(trackModel)
		if os.Getenv("CLICKUP_TUI_MENU") != "1" && finalTrackModel.step == trackStepDisplay {
			fmt.Println(finalTrackModel.generateDisplayContent())
		}
	},
}

type trackStep int

const (
	trackStepUserSelect trackStep = iota
	trackStepLoading
	trackStepDisplay
)

type trackModel struct {
	client     clickup.API
	cfg        config.Config
	summarizer *ai.Summarizer
	userID     string
	user       *clickup.User
	step       trackStep
	userList   list.Model
	activities []clickup.Activity
	summaries  []string
	loading    bool
	spinner    spinner.Model
	viewport   viewport.Model
	ready      bool
	quitting   bool
	err        error
	width      int
	height     int
}

func initialTrackModel(client clickup.API, cfg config.Config, summarizer *ai.Summarizer, userID string) trackModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select User to Track"

	m := trackModel{
		client:     client,
		cfg:        cfg,
		summarizer: summarizer,
		userID:     userID,
		userList:   l,
		spinner:    ui.NewSpinnerModel(),
	}

	if userID != "" {
		m.step = trackStepLoading
	} else {
		m.step = trackStepUserSelect
	}

	return m
}

type trackUsersMsg []clickup.User
type trackActivityMsg struct {
	activities []clickup.Activity
	userInfo   *clickup.User
	summaries  []string
}

func (m trackModel) Init() tea.Cmd {
	if m.userID != "" {
		return tea.Batch(m.spinner.Tick, m.loadActivity(m.userID))
	}
	return tea.Batch(m.spinner.Tick, m.loadUsers)
}

func (m trackModel) loadUsers() tea.Msg {
	users, err := m.client.GetWorkspaceUsers(m.cfg.WorkspaceID)
	if err != nil {
		return errMsg(err)
	}
	return trackUsersMsg(users)
}

func isUserActor(userID string, comments []clickup.Comment, eventTime int64, defaultAssignees []clickup.User, creator clickup.User) bool {
	// 1. Check if any comment is very close to eventTime (within 5 seconds / 5000 ms)
	for _, c := range comments {
		cTime, err := strconv.ParseInt(c.Date, 10, 64)
		if err == nil {
			diff := eventTime - cTime
			if diff < 0 {
				diff = -diff
			}
			if diff <= 5000 {
				return c.User.ID.String() == userID
			}
		}
	}

	// 2. Fallback to assignees if any
	if len(defaultAssignees) > 0 {
		for _, assignee := range defaultAssignees {
			if assignee.ID.String() == userID {
				return true
			}
		}
		return false
	}

	// 3. Fallback to creator
	return creator.ID.String() == userID
}

func (m trackModel) loadActivity(userID string) tea.Cmd {
	return func() tea.Msg {
		// Last 10 days
		dateFrom := time.Now().AddDate(0, 0, -10).UnixNano() / int64(time.Millisecond)

		var activities []clickup.Activity
		var taskDetails = make(map[string]clickup.Task)
		var taskComments = make(map[string][]clickup.Comment)
		var userInfo *clickup.User

		// 1. Fetch user info to populate the activity records
		users, _ := m.client.GetWorkspaceUsers(m.cfg.WorkspaceID)
		for _, u := range users {
			if u.ID.String() == userID {
				userInfo = &u
				break
			}
		}

		if userInfo == nil {
			return errMsg(fmt.Errorf("user %s not found", userID))
		}

		// 2. Iterate through configured folders and lists
		for _, folder := range m.cfg.Folders {
			lists, err := m.client.GetLists(folder.ID)
			if err != nil {
				continue
			}

			for _, listObj := range lists {
				// 3. Get tasks updated in the last 10 days
				tasks, err := m.client.GetRecentTasks(listObj.ID, dateFrom)
				if err != nil {
					continue
				}

				// Populate FolderName and ListName
				for i := range tasks {
					tasks[i].FolderName = folder.Name
					tasks[i].ListName = listObj.Name
				}

				for _, task := range tasks {
					taskDateCreated, _ := strconv.ParseInt(task.DateCreated, 10, 64)
					taskDateUpdated, _ := strconv.ParseInt(task.DateUpdated, 10, 64)
					taskDateDone, _ := strconv.ParseInt(task.DateDone, 10, 64)
					taskDateClosed, _ := strconv.ParseInt(task.DateClosed, 10, 64)

					isAssignee := false
					for _, a := range task.Assignees {
						if a.ID.String() == userID {
							isAssignee = true
							break
						}
					}

					// Fetch task comments first if task was updated in the window
					var comments []clickup.Comment
					if taskDateUpdated >= dateFrom {
						comments, _ = m.client.GetTaskComments(task.ID)
						if len(comments) > 0 {
							taskComments[task.ID] = comments
						}
					}

					// 1. Check if created by user in window
					if taskDateCreated >= dateFrom {
						if task.Creator.ID.String() == userID {
							activities = append(activities, clickup.Activity{
								ID:         "create-" + task.ID,
								User:       *userInfo,
								Type:       fmt.Sprintf("created task [%s]", task.Status.Status),
								Date:       task.DateCreated,
								TaskID:     task.ID,
								Source:     task.Name,
								FolderName: task.FolderName,
								ListName:   task.ListName,
							})
							taskDetails[task.ID] = task
						}

						// Also check if assigned to task at creation time
						if task.Creator.ID.String() != userID && isAssignee {
							activities = append(activities, clickup.Activity{
								ID:         "assign-create-" + task.ID + "-" + userID,
								User:       *userInfo,
								Type:       "assigned to task",
								Date:       task.DateCreated,
								TaskID:     task.ID,
								Source:     task.Name,
								FolderName: task.FolderName,
								ListName:   task.ListName,
							})
							taskDetails[task.ID] = task
						}
					}

					// 2. Check for completions, closures, or general updates where the user was the actor
					if taskDateDone >= dateFrom {
						if isUserActor(userID, comments, taskDateDone, task.Assignees, task.Creator) {
							activities = append(activities, clickup.Activity{
								ID:         "done-" + task.ID + "-" + task.DateDone,
								User:       *userInfo,
								Type:       fmt.Sprintf("completed task [%s]", task.Status.Status),
								Date:       task.DateDone,
								TaskID:     task.ID,
								Source:     task.Name,
								FolderName: task.FolderName,
								ListName:   task.ListName,
							})
							taskDetails[task.ID] = task
						}
					} else if taskDateClosed >= dateFrom {
						if isUserActor(userID, comments, taskDateClosed, task.Assignees, task.Creator) {
							activities = append(activities, clickup.Activity{
								ID:         "closed-" + task.ID + "-" + task.DateClosed,
								User:       *userInfo,
								Type:       fmt.Sprintf("closed task [%s]", task.Status.Status),
								Date:       task.DateClosed,
								TaskID:     task.ID,
								Source:     task.Name,
								FolderName: task.FolderName,
								ListName:   task.ListName,
							})
							taskDetails[task.ID] = task
						}
					} else if taskDateUpdated >= dateFrom && taskDateUpdated > taskDateCreated {
						if isUserActor(userID, comments, taskDateUpdated, task.Assignees, task.Creator) {
							activities = append(activities, clickup.Activity{
								ID:         "update-" + task.ID + "-" + task.DateUpdated,
								User:       *userInfo,
								Type:       fmt.Sprintf("updated task [%s]", task.Status.Status),
								Date:       task.DateUpdated,
								TaskID:     task.ID,
								Source:     task.Name,
								FolderName: task.FolderName,
								ListName:   task.ListName,
							})
							taskDetails[task.ID] = task
						}
					}

					// 3. Log user comments
					for _, comment := range comments {
						commentDate, _ := strconv.ParseInt(comment.Date, 10, 64)
						if commentDate >= dateFrom && comment.User.ID.String() == userID {
							activities = append(activities, clickup.Activity{
								ID:         "comment-" + comment.ID,
								User:       *userInfo,
								Type:       "commented on task",
								Date:       comment.Date,
								TaskID:     task.ID,
								Source:     task.Name,
								Detail:     comment.CommentText,
								FolderName: task.FolderName,
								ListName:   task.ListName,
							})
							taskDetails[task.ID] = task
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

		var summaries []string
		if m.summarizer != nil && len(activities) > 0 {
			dailyActivities := make(map[string][]clickup.Activity)
			var days []string

			for _, a := range activities {
				t, _ := strconv.ParseInt(a.Date, 10, 64)
				dayStr := time.Unix(t/1000, 0).Format("2006-01-02")
				if _, exists := dailyActivities[dayStr]; !exists {
					days = append(days, dayStr)
				}
				dailyActivities[dayStr] = append(dailyActivities[dayStr], a)
			}

			sort.Sort(sort.Reverse(sort.StringSlice(days)))

			for _, day := range days {
				acts := dailyActivities[day]
				summaryText, err := m.summarizer.SummarizeUserActivity(userInfo.Username, day, acts, taskDetails, taskComments)
				if err != nil {
					summaries = append(summaries, fmt.Sprintf("## %s\nError generating summary: %v\n", day, err))
				} else {
					summaries = append(summaries, fmt.Sprintf("## %s\n%s\n", day, summaryText))
				}
			}
		}

		return trackActivityMsg{activities: activities, userInfo: userInfo, summaries: summaries}

	}
}

func (m trackModel) generateDisplayContent() string {
	width := m.width
	if width <= 0 {
		width = 80
	}

	var b strings.Builder
	title := "Activity for the last 10 days"
	if m.user != nil {
		title = fmt.Sprintf("Activity for last 10 days for (%s: %s)", m.user.Username, m.user.ID.String())
	}
	b.WriteString(ui.HeaderStyle.Render(title) + "\n\n")

	if len(m.summaries) > 0 {
		b.WriteString(ui.HeaderStyle.Render("AI Daily Summary"))
		b.WriteString("\n\n")

		// Render Markdown using glamour
		glamourStyle := "dark"
		if !lipgloss.HasDarkBackground() {
			glamourStyle = "light"
		}

		r, _ := glamour.NewTermRenderer(
			glamour.WithStandardStyle(glamourStyle),
			glamour.WithWordWrap(width-10),
		)

		for _, s := range m.summaries {
			out, err := r.Render(s)
			if err != nil {
				b.WriteString(s) // Fallback
			} else {
				b.WriteString(strings.TrimSpace(out))
			}
			b.WriteString("\n\n")
		}
	}

	if len(m.activities) == 0 {
		b.WriteString("No activity found in the last 10 days.")
	} else if len(m.summaries) == 0 || trackRawActivity {
		if len(m.summaries) > 0 {
			b.WriteString(ui.HeaderStyle.Render("Raw Activity Log"))
			b.WriteString("\n\n")
		}
		// Activity wrap style
		activityWrapStyle := lipgloss.NewStyle().Width(width - 6)

		for _, a := range m.activities {
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

func (m trackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 3 // for the "(q: quit | esc: back to users)" + margins

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}

		h, v := ui.DocStyle.GetFrameSize()
		m.userList.SetSize(msg.Width-h, msg.Height-v)

		if m.step == trackStepDisplay {
			m.viewport.SetContent(ui.DocStyle.Width(m.width).Render(m.generateDisplayContent()))
		}

		return m, nil

	case spinner.TickMsg:
		if m.step == trackStepLoading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case trackUsersMsg:
		items := make([]list.Item, len(msg))
		for i, u := range msg {
			uCopy := u
			items[i] = assigneeItem{user: &uCopy}
		}
		m.userList.SetItems(items)
		return m, nil

	case trackActivityMsg:
		m.activities = msg.activities
		m.user = msg.userInfo
		m.summaries = msg.summaries
		m.step = trackStepDisplay
		m.loading = false

		if os.Getenv("CLICKUP_TUI_MENU") != "1" {
			m.quitting = true
			return m, tea.Quit
		}

		if m.ready {
			m.viewport.SetContent(ui.DocStyle.Width(m.width).Render(m.generateDisplayContent()))
			m.viewport.GotoTop()
		}
		return m, nil

	case errMsg:
		m.err = msg
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

		switch m.step {
		case trackStepUserSelect:
			if msg.String() == "enter" {
				if it, ok := m.userList.SelectedItem().(assigneeItem); ok && it.user != nil {
					m.userID = it.user.ID.String()
					m.step = trackStepLoading
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, m.loadActivity(m.userID))
				}
			}
		case trackStepDisplay:
			if msg.String() == "esc" {
				m.step = trackStepUserSelect
				return m, nil
			}
		}
	}

	if m.step == trackStepUserSelect {
		m.userList, cmd = m.userList.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.step == trackStepDisplay {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m trackModel) View() string {
	if m.err != nil {
		return ui.DocStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	}

	if m.quitting {
		return ""
	}

	switch m.step {
	case trackStepUserSelect:
		return ui.DocStyle.Render(m.userList.View())
	case trackStepLoading:
		return ui.DocStyle.Render(ui.SpinnerView("Loading activity...", m.spinner))
	case trackStepDisplay:
		if !m.ready {
			return "\n  Initializing..."
		}
		footer := "\n\n(q: quit | esc: back to users | ↑/↓: scroll)"
		return fmt.Sprintf("%s%s", m.viewport.View(), footer)
	}
	return ""
}

func init() {
	trackCmd.Flags().BoolVarP(&trackSummarize, "summarize", "s", false, "Generate an AI summary of user activity")
	trackCmd.Flags().BoolVarP(&trackRawActivity, "raw", "c", false, "Show raw activity log even if summarizing")
	rootCmd.AddCommand(trackCmd)
}
