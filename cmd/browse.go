package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"
	"clickup-tui/pkg/filter"
	"clickup-tui/pkg/format"
	"clickup-tui/pkg/ui"
	"clickup-tui/pkg/util"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type browseTasksMsg []taskItem

var (
	browseAll  bool
	browseMine bool
)

var browseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse tasks in an interactive TUI",
	Long:  `Interactively browse tasks from your configured ClickUp workspace.\n\nFlags:\n  --all, -a: Browse all open tasks (includes backlog and scoping)\n  --mine: Only browse tasks assigned to you (default true)`,
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

		currentUser, err := client.GetUser()
		if err != nil {
			fmt.Printf("Error getting current user: %v\n", err)
			os.Exit(1)
		}

		m := initialBrowseModel(client, cfg, currentUser.ID.String(), browseAll, browseMine)
		p := tea.NewProgram(m, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

type taskItem struct {
	task        clickup.Task
	folderName  string
	listName    string
	workspaceID string
}

func (i taskItem) Title() string {
	return fmt.Sprintf("[%s] %s", i.task.Status.Status, i.task.Name)
}

func (i taskItem) Description() string {
	formattedDate := format.FormatTaskDate(i.task.DateUpdated)
	return fmt.Sprintf("Folder: %s | List: %s | ID: %s | Updated: %s", i.folderName, i.listName, i.task.ID, formattedDate)
}

func (i taskItem) FilterValue() string {
	return i.task.Name + " " + i.task.Status.Status + " " + i.folderName + " " + i.listName
}

type browseState int

const (
	stateList browseState = iota
	stateDetail
	stateComment
)

type commentPostedMsg struct{}

type browseModel struct {
	client       *clickup.Client
	cfg          config.Config
	userID       string
	all          bool
	mine         bool
	list         list.Model
	viewport     viewport.Model
	textarea     textarea.Model
	state        browseState
	selectedTask *taskItem
	comments     []clickup.Comment
	loading      bool
	posting      bool
	spinner      spinner.Model
	err          error
	width        int
	height       int
}

func initialBrowseModel(client *clickup.Client, cfg config.Config, userID string, all bool, mine bool) browseModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	title := "Active Tasks"
	if all {
		title = "All Open Tasks"
	}
	if mine {
		title += " (Mine)"
	}
	l.Title = title

	return browseModel{
		client:  client,
		cfg:     cfg,
		userID:  userID,
		all:     all,
		mine:    mine,
		list:    l,
		state:   stateList,
		loading: true,
		spinner: ui.NewSpinnerModel(),
	}
}

type commentsMsg []clickup.Comment

func (m browseModel) Init() tea.Cmd {
	loadCmd := func() tea.Msg {
		var allItems []taskItem
		for _, folder := range m.cfg.Folders {
			lists, err := m.client.GetLists(folder.ID)
			if err != nil {
				continue
			}
			for _, listObj := range lists {
				tasks, err := m.client.GetTasks(listObj.ID)
				if err != nil {
					continue
				}
				for _, task := range tasks {
					if filter.ShouldIncludeTask(task, m.userID, m.all, m.mine) {
						allItems = append(allItems, taskItem{task: task, folderName: folder.Name, listName: listObj.Name})
					}
				}
			}
		}

		// Sort by DateUpdated descending
		sort.Slice(allItems, func(i, j int) bool {
			timeI, _ := strconv.ParseInt(allItems[i].task.DateUpdated, 10, 64)
			timeJ, _ := strconv.ParseInt(allItems[j].task.DateUpdated, 10, 64)
			return timeI > timeJ
		})

		return browseTasksMsg(allItems)
	}
	return tea.Batch(loadCmd, m.spinner.Tick)
}

func (m browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// In comment mode, only handle ctrl+c (quit), esc (cancel), and ctrl+s (submit)
		if m.state == stateComment {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.state = stateDetail
				return m, nil
			case "ctrl+s":
				text := strings.TrimSpace(m.textarea.Value())
				if text == "" {
					return m, nil
				}
				m.posting = true
				taskID := m.selectedTask.task.ID
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					if err := m.client.CreateTaskComment(taskID, text); err != nil {
						return errMsg(err)
					}
					return commentPostedMsg{}
				})
			}
			// Let textarea handle all other keys
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.state == stateDetail {
				m.state = stateList
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if m.state == stateList {
				if it, ok := m.list.SelectedItem().(taskItem); ok {
					m.selectedTask = &it
					m.state = stateDetail
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						comments, err := m.client.GetTaskComments(it.task.ID)
						if err != nil {
							return errMsg(err)
						}
						return commentsMsg(comments)
					})
				}
			}
		case "c":
			if m.state == stateDetail && m.selectedTask != nil {
				ta := textarea.New()
				ta.Placeholder = "Type your comment..."
				ta.Focus()
				ta.SetWidth(m.width - 10)
				ta.SetHeight(6)
				m.textarea = ta
				m.state = stateComment
				return m, textarea.Blink
			}
		case "esc":
			if m.state == stateDetail {
				m.state = stateList
				return m, nil
			}
		case " ":
			if m.state == stateDetail {
				// Move to next task
				m.list.CursorDown()
				if it, ok := m.list.SelectedItem().(taskItem); ok {
					// Only load if it's actually a different task (in case we're at the bottom)
					if m.selectedTask == nil || m.selectedTask.task.ID != it.task.ID {
						m.selectedTask = &it
						m.loading = true
						return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
							comments, err := m.client.GetTaskComments(it.task.ID)
							if err != nil {
								return errMsg(err)
							}
							return commentsMsg(comments)
						})
					}
				}
			}
		}

	case commentPostedMsg:		m.posting = false
		m.state = stateDetail
		m.loading = true
		taskID := m.selectedTask.task.ID
		return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
			comments, err := m.client.GetTaskComments(taskID)
			if err != nil {
				return errMsg(err)
			}
			return commentsMsg(comments)
		})

	case browseTasksMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, v := range msg {
			items[i] = v
		}
		m.list.SetItems(items)
		title := "Active Tasks"
		if m.all {
			title = "All Open Tasks"
		}
		if m.mine {
			title += " (Mine)"
		}
		m.list.Title = fmt.Sprintf("%s (%d)", title, len(items))

	case commentsMsg:
		m.loading = false
		m.comments = msg
		m.viewport.SetContent(m.renderDetail())

	case spinner.TickMsg:
		if m.loading || m.posting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := ui.DocStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		m.viewport = viewport.New(msg.Width-h, msg.Height-v-10)
		if m.state == stateDetail {
			m.viewport.SetContent(m.renderDetail())
		}

	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	if m.state == stateList {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m browseModel) renderDetail() string {
	if m.selectedTask == nil {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(ui.HeaderStyle.Render(m.selectedTask.task.Name) + "\n")
	b.WriteString(fmt.Sprintf("Status: %s\n", m.selectedTask.task.Status.Status))
	b.WriteString(fmt.Sprintf("Folder: %s | List: %s\n", m.selectedTask.folderName, m.selectedTask.listName))

	assignees := []string{}
	for _, a := range m.selectedTask.task.Assignees {
		assignees = append(assignees, a.Username)
	}
	// Note: assignee filtering would use a.ID.String() == currentUser.ID.String()
	if len(assignees) > 0 {
		b.WriteString(fmt.Sprintf("Assignees: %s\n", strings.Join(assignees, ", ")))
	}
	b.WriteString("\n" + strings.Repeat("-", m.width-10) + "\n\n")

	// Comments
	if m.loading {
		b.WriteString(ui.SpinnerView("Loading comments...", m.spinner))
	} else if len(m.comments) == 0 {
		b.WriteString("No comments found.")
	} else {
		b.WriteString("Recent Comments:\n\n")
		for _, c := range m.comments {
			date := format.FormatCommentDate(c.Date)
			b.WriteString(fmt.Sprintf("%s %s:\n", ui.DateStyle.Render(date), ui.AssigneeStyle.Render(c.User.Username)))

			// Wrap comment text
			wrapWidth := m.width - 15
			if wrapWidth < 20 {
				wrapWidth = 20
			}
			wrapped := ui.DocStyle.Width(wrapWidth).PaddingLeft(2).Render(strings.TrimSpace(c.CommentText))
			b.WriteString(wrapped + "\n\n")
		}
	}

	b.WriteString("\n\n(c: add comment | space: next task | Esc/q: go back)")
	return b.String()
}

func (m browseModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}
	if m.state == stateComment {
		var b strings.Builder
		b.WriteString(ui.HeaderStyle.Render("Add Comment: "+m.selectedTask.task.Name) + "\n\n")
		if m.posting {
			b.WriteString(ui.SpinnerView("Posting comment...", m.spinner))
		} else {
			b.WriteString(m.textarea.View())
			b.WriteString("\n\n(Ctrl+S: submit | Esc: cancel)")
		}
		return ui.DocStyle.Render(b.String())
	}
	if m.state == stateList && m.loading && len(m.list.Items()) == 0 {
		return ui.DocStyle.Render(ui.SpinnerView("Loading tasks...", m.spinner))
	}
	if m.state == stateDetail {
		return ui.DocStyle.Render(m.viewport.View())
	}
	return ui.DocStyle.Render(m.list.View())
}

func init() {
	browseCmd.Flags().BoolVarP(&browseAll, "all", "a", false, "Browse all open tasks (including backlog and scoping)")
	browseCmd.Flags().BoolVar(&browseMine, "mine", true, "Only browse tasks assigned to you")
	rootCmd.AddCommand(browseCmd)
}
