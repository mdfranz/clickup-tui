package cmd

import (
	"fmt"
	"io"
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
	"github.com/charmbracelet/lipgloss"
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

		client, cleanup := newCachedClient(pat)
		defer cleanup()

		currentUser, err := client.GetUser()
		if err != nil {
			fmt.Printf("Error getting current user: %v\n", err)
			os.Exit(1)
		}

		m := initialBrowseModel(client, cfg, currentUser, browseAll, browseMine)
		
		var opts []tea.ProgramOption
		if os.Getenv("CLICKUP_TUI_MENU") == "1" {
			opts = append(opts, tea.WithAltScreen())
		}
		p := tea.NewProgram(m, opts...)

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
	listID      string
	workspaceID string
}

func (i taskItem) Title() string {
	formattedDate := format.FormatTaskDate(i.task.DateUpdated)
	return fmt.Sprintf("[%s] %s (%s)", i.task.Status.Status, i.task.Name, formattedDate)
}

func (i taskItem) Description() string {
	return ""
}

func (i taskItem) FilterValue() string {
	return i.task.Name + " " + i.task.Status.Status + " " + i.folderName + " " + i.listName
}

type taskDelegate struct {
	list.DefaultDelegate
}

func (d taskDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(taskItem)
	if !ok {
		return
	}

	status := i.task.Status.Status
	name := i.task.Name
	formattedDate := format.FormatTaskDate(i.task.DateUpdated)

	sColor, ok := ui.StatusColors[strings.ToLower(status)]
	if !ok {
		sColor = ui.ColorGray
	}
	statusDisplay := lipgloss.NewStyle().Bold(true).Foreground(sColor).Render(fmt.Sprintf("[%s]", status))

	content := fmt.Sprintf("%s %s (%s)", statusDisplay, name, formattedDate)

	if index == m.Index() {
		fmt.Fprint(w, d.Styles.SelectedTitle.Render(content))
	} else {
		fmt.Fprint(w, d.Styles.NormalTitle.Render(content))
	}
}

type browseState int

const (
	stateList browseState = iota
	stateComment
	stateStatus
	stateNewTask
)

type commentPostedMsg struct{}
type statusUpdatedMsg struct {
	status string
}

type browseModel struct {
	client            clickup.API
	cfg               config.Config
	currentUser       clickup.User
	all               bool
	mine              bool
	list              list.Model
	statusList        list.Model
	viewport          viewport.Model
	textarea          textarea.Model
	newTaskModel      *newModel
	state             browseState
	selectedTask      *taskItem
	comments          []clickup.Comment
	availableStatuses []clickup.Status
	loading           bool
	posting           bool
	spinner           spinner.Model
	err               error
	width             int
	height            int
}

func initialBrowseModel(client clickup.API, cfg config.Config, currentUser clickup.User, all bool, mine bool) browseModel {
	delegate := taskDelegate{list.NewDefaultDelegate()}
	delegate.ShowDescription = false
	l := list.New([]list.Item{}, delegate, 0, 0)
	title := "Active Tasks (n: New Task)"
	if all {
		title = "All Open Tasks (n: New Task)"
	}
	if mine {
		title = strings.Replace(title, " (n: New Task)", " (Mine) (n: New Task)", 1)
	}
	l.Title = title

	slDelegate := list.NewDefaultDelegate()
	slDelegate.ShowDescription = false
	slDelegate.SetHeight(1)
	slDelegate.SetSpacing(0)
	sl := list.New([]list.Item{}, slDelegate, 0, 0)
	sl.SetShowTitle(false)
	sl.SetShowStatusBar(false)
	sl.SetShowHelp(false)
	sl.SetFilteringEnabled(false)

	return browseModel{
		client:      client,
		cfg:         cfg,
		currentUser: currentUser,
		all:         all,
		mine:        mine,
		list:        l,
		statusList:  sl,
		textarea:    textarea.New(),
		state:       stateList,
		loading:     true,
		spinner:     ui.NewSpinnerModel(),
	}
}

type commentsMsg []clickup.Comment
type statusesMsg []clickup.Status

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
					if filter.ShouldIncludeTask(task, m.currentUser.ID.String(), m.all, m.mine) {
						allItems = append(allItems, taskItem{
							task:       task,
							folderName: folder.Name,
							listName:   listObj.Name,
							listID:     listObj.ID,
						})
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

type taskStatusItem struct {
	status clickup.Status
}

func (i taskStatusItem) Title() string       { return i.status.Status }
func (i taskStatusItem) Description() string { return "" }
func (i taskStatusItem) FilterValue() string { return i.status.Status }

func (m browseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case taskCreatedMsg:
		if m.state == stateNewTask {
			// Let sub-model handle it first
			if m.newTaskModel != nil {
				nm, _ := m.newTaskModel.Update(msg)
				updatedModel := nm.(newModel)
				m.newTaskModel = &updatedModel
			}
			return m, nil
		}

	case tea.KeyMsg:
		// Handle new task state
		if m.state == stateNewTask {
			if m.newTaskModel.step == stepNewDone {
				switch msg.String() {
				case "q", "enter", "esc":
					m.state = stateList
					m.newTaskModel = nil
					m.loading = true
					return m, m.Init()
				}
			}
			if msg.String() == "esc" && m.newTaskModel.step == stepFolderSelect {
				m.state = stateList
				m.newTaskModel = nil
				return m, nil
			}
			nm, cmd := m.newTaskModel.Update(msg)
			updatedModel := nm.(newModel)
			m.newTaskModel = &updatedModel
			return m, cmd
		}

		// In comment mode, only handle ctrl+c (quit), esc (cancel), and ctrl+s (submit)
		if m.state == stateComment {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.state = stateList
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

		if m.state == stateStatus {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc", "q":
				m.state = stateList
				return m, nil
			case "enter":
				if it, ok := m.statusList.SelectedItem().(taskStatusItem); ok {
					m.posting = true
					taskID := m.selectedTask.task.ID
					status := it.status.Status
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						if err := m.client.UpdateTaskStatus(taskID, status); err != nil {
							return errMsg(err)
						}
						return statusUpdatedMsg{status: status}
					})
				}
			}
			m.statusList, cmd = m.statusList.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.list.FilterState() != list.Filtering {
				return m, tea.Quit
			}
		case "c":
			if m.state == stateList && m.selectedTask != nil {
				ta := textarea.New()
				ta.Placeholder = "Type your comment..."
				ta.Focus()
				ta.SetWidth(m.viewport.Width / 2)
				ta.SetHeight(6)
				m.textarea = ta
				m.state = stateComment
				return m, textarea.Blink
			}
		case "s":
			if m.state == stateList && m.selectedTask != nil {
				m.state = stateStatus
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					list, err := m.client.GetList(m.selectedTask.listID)
					if err != nil {
						return errMsg(err)
					}
					return statusesMsg(list.Statuses)
				})
			}
		case "n":
			if m.state == stateList {
				nm := initialNewModel(m.client, m.cfg, m.currentUser)
				
				// Initialize sub-model with full size for overlay
				subModel, _ := nm.Update(tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.height,
				})
				nm = subModel.(newModel)

				var preseedCmd tea.Cmd
				if m.selectedTask != nil {
					for _, f := range m.cfg.Folders {
						if f.Name == m.selectedTask.folderName {
							nm.selectedFolder = f
							nm.targetListID = m.selectedTask.listID
							nm.step = stepListSelect
							nm.loading = true
							folderID := f.ID
							preseedCmd = func() tea.Msg {
								lists, err := m.client.GetLists(folderID)
								if err != nil {
									return errMsg(err)
								}
								return listsMsg(lists)
							}
							break
						}
					}
				}

				m.newTaskModel = &nm
				m.state = stateNewTask
				
				if preseedCmd != nil {
					return m, tea.Batch(nm.Init(), preseedCmd)
				}
				return m, nm.Init()
			}
		}

	case commentPostedMsg:
		m.posting = false
		m.state = stateList
		m.loading = true
		taskID := m.selectedTask.task.ID
		return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
			comments, err := m.client.GetTaskComments(taskID)
			if err != nil {
				return errMsg(err)
			}
			return commentsMsg(comments)
		})

	case statusUpdatedMsg:
		m.posting = false
		m.state = stateList
		m.selectedTask.task.Status.Status = msg.status
		m.viewport.SetContent(m.renderDetail())
		// Also update in the main list
		items := m.list.Items()
		for i, item := range items {
			if ti, ok := item.(taskItem); ok && ti.task.ID == m.selectedTask.task.ID {
				ti.task.Status.Status = msg.status
				items[i] = ti
				break
			}
		}
		m.list.SetItems(items)

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
		m.list.Title = fmt.Sprintf("%s (%d) (n: New Task)", title, len(items))

		// Auto-select first task and load comments
		if len(items) > 0 {
			if it, ok := items[0].(taskItem); ok {
				m.selectedTask = &it
				m.loading = true
				m.viewport.SetContent(m.renderDetail())
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					comments, err := m.client.GetTaskComments(it.task.ID)
					if err != nil {
						return errMsg(err)
					}
					return commentsMsg(comments)
				})
			}
		}

	case commentsMsg:
		m.loading = false
		m.comments = msg
		m.viewport.SetContent(m.renderDetail())

	case statusesMsg:
		m.loading = false
		m.availableStatuses = msg
		items := make([]list.Item, len(msg))
		maxW := 5 // Minimal initial width
		for i, v := range msg {
			items[i] = taskStatusItem{status: v}
			if len(v.Status) > maxW {
				maxW = len(v.Status)
			}
		}
		m.statusList.SetItems(items)

		// Adjust height and width dynamically
		newHeight := len(items)
		if maxH := m.height - 6; newHeight > maxH {
			newHeight = maxH
		}
		
		newWidth := maxW + 4 // Cursor and small margin
		if maxW_ := m.width - 10; newWidth > maxW_ {
			newWidth = maxW_
		}

		m.statusList.SetSize(newWidth, newHeight)

	case spinner.TickMsg:
		if m.state == stateNewTask {
			nm, cmd := m.newTaskModel.Update(msg)
			updatedModel := nm.(newModel)
			m.newTaskModel = &updatedModel
			return m, cmd
		}
		if m.loading || m.posting {
			m.spinner, cmd = m.spinner.Update(msg)
			if m.state == stateList {
				m.viewport.SetContent(m.renderDetail())
			}
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := ui.DocStyle.GetFrameSize()
		
		if m.newTaskModel != nil {
			// If we are in stateNewTask, give it the full size as it's an overlay
			if m.state == stateNewTask {
				nm, _ := m.newTaskModel.Update(msg)
				updatedModel := nm.(newModel)
				m.newTaskModel = &updatedModel
			} else {
				// Otherwise size for the potential right-pane modal
				// ... (actually it's better to just give it the full size or correct size later)
			}
		}

		// Ratio 1/3 for list, 2/3 for details
		listWidth := msg.Width / 3
		if listWidth < 35 {
			listWidth = 35
		}
		
		// Space for the vertical separator and padding
		separatorWidth := 3 
		viewportWidth := msg.Width - listWidth - h - separatorWidth

		m.list.SetSize(listWidth, msg.Height-v)
		
		// Status list popup size
		sw := msg.Width / 2
		sh := msg.Height / 2
		if sw < 40 {
			sw = 40
		}
		if sh < 15 {
			sh = 15
		}
		m.statusList.SetSize(sw-4, sh-4)

		m.viewport = viewport.New(viewportWidth, msg.Height-v)
		m.viewport.SetContent(m.renderDetail())
		m.textarea.SetWidth(viewportWidth / 2)

	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	// Handle other messages for newTaskModel if it's active
	if m.state == stateNewTask {
		nm, cmd := m.newTaskModel.Update(msg)
		updatedModel := nm.(newModel)
		m.newTaskModel = &updatedModel
		return m, cmd
	}

	if m.state == stateList {
		prevID := ""
		if m.selectedTask != nil {
			prevID = m.selectedTask.task.ID
		}

		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		if it, ok := m.list.SelectedItem().(taskItem); ok {
			if it.task.ID != prevID {
				m.selectedTask = &it
				m.loading = true
				m.viewport.SetContent(m.renderDetail())
				cmds = append(cmds, tea.Batch(m.spinner.Tick, func() tea.Msg {
					comments, err := m.client.GetTaskComments(it.task.ID)
					if err != nil {
						return errMsg(err)
					}
					return commentsMsg(comments)
				}))
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}


func (m browseModel) renderDetail() string {
	if m.selectedTask == nil {
		return ""
	}

	var b strings.Builder

	// Header
	b.WriteString(ui.HeaderStyle.Render(m.selectedTask.task.Name) + "\n")
	
	status := m.selectedTask.task.Status.Status
	sColor, ok := ui.StatusColors[strings.ToLower(status)]
	if !ok {
		sColor = ui.ColorGray
	}
	statusDisplay := lipgloss.NewStyle().Bold(true).Foreground(sColor).Render(status)
	b.WriteString(fmt.Sprintf("Status: %s\n", statusDisplay))
	
	b.WriteString(fmt.Sprintf("Folder: %s | List: %s\n", m.selectedTask.folderName, m.selectedTask.listName))

	assignees := []string{}
	for _, a := range m.selectedTask.task.Assignees {
		assignees = append(assignees, a.Username)
	}
	// Note: assignee filtering would use a.ID.String() == currentUser.ID.String()
	if len(assignees) > 0 {
		b.WriteString(fmt.Sprintf("Assignees: %s\n", ui.AssigneeStyle.Render(strings.Join(assignees, ", "))))
	}
	
	width := m.viewport.Width - 4
	if width < 0 {
		width = 0
	}
	b.WriteString("\n" + strings.Repeat("─", width) + "\n\n")

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
			wrapWidth := m.viewport.Width - 6
			if wrapWidth < 20 {
				wrapWidth = 20
			}
			wrapped := lipgloss.NewStyle().Width(wrapWidth).PaddingLeft(2).Render(strings.TrimSpace(c.CommentText))
			b.WriteString(wrapped + "\n\n")
		}
	}

	b.WriteString("\n\n(c: comment | s: status | q: quit)")
	return b.String()
}

func (m browseModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}

	if m.state == stateNewTask {
		return m.newTaskModel.View()
	}

	// 1. Prepare Left Pane (List)
	leftPane := m.list.View()
	if m.state == stateList && m.loading && len(m.list.Items()) == 0 {
		leftPane = ui.DocStyle.Render(ui.SpinnerView("Loading tasks...", m.spinner))
	}

	// 2. Prepare Right Pane Content
	var rightPane string
	if m.state == stateComment || m.state == stateStatus {
		var modalContent string
		if m.state == stateComment {
			var b strings.Builder
			b.WriteString(ui.HeaderStyle.Render("Add Comment: "+m.selectedTask.task.Name) + "\n\n")
			if m.posting {
				b.WriteString(ui.SpinnerView("Posting comment...", m.spinner))
			} else {
				b.WriteString(m.textarea.View())
				b.WriteString("\n\n(Ctrl+S: submit | Esc: cancel)")
			}
			modalContent = b.String()
		} else if m.state == stateStatus {
			if m.loading {
				modalContent = ui.SpinnerView("Loading statuses...", m.spinner)
			} else if m.posting {
				modalContent = ui.SpinnerView("Updating status...", m.spinner)
			} else {
				modalContent = m.statusList.View()
			}
		}

		modal := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorPurple).
			Padding(1, 2).
			Render(modalContent)
		
		// Center modal within the right pane's dimensions
		rightPane = lipgloss.Place(m.viewport.Width, m.viewport.Height, lipgloss.Center, lipgloss.Center, modal)
	} else {
		rightPane = m.viewport.View()
	}

	// 3. Assemble Layout
	separator := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(ui.ColorGray).
		Height(m.viewport.Height).
		Margin(0, 1).
		Render("")

	return ui.DocStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftPane,
			separator,
			rightPane,
		),
	)
}

func init() {
	browseCmd.Flags().BoolVarP(&browseAll, "all", "a", false, "Browse all open tasks (including backlog and scoping)")
	browseCmd.Flags().BoolVar(&browseMine, "mine", true, "Only browse tasks assigned to you")
	rootCmd.AddCommand(browseCmd)
}
