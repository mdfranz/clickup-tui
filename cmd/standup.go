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

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	standupAll  bool
	standupMine bool
)

var standupCmd = &cobra.Command{
	Use:   "standup",
	Short: "Walk through tasks and post updates",
	Long:  `Interactive standup workflow: select tasks, add comments, and change statuses.`,
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

		m := initialStandupModel(client, cfg, currentUser.ID.String(), standupAll, standupMine)
		
		var opts []tea.ProgramOption
		if os.Getenv("CLICKUP_TUI_MENU") == "1" {
			opts = append(opts, tea.WithAltScreen())
		}
		p := tea.NewProgram(m, opts...)

		finalModel, err := p.Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		
		finalStandupModel := finalModel.(standupModel)
		if os.Getenv("CLICKUP_TUI_MENU") != "1" && finalStandupModel.state == standupDone {
			fmt.Println(finalStandupModel.viewDoneContent())
		}
	},
}

// standupTask holds a task with its metadata and the list it belongs to
type standupTask struct {
	task       clickup.Task
	folderName string
	listName   string
	listID     string
	selected   bool
}

type standupState int

const (
	standupLoading standupState = iota
	standupSelect               // multi-select tasks
	standupUpdate               // comment + status for current task
	standupStatus               // status picker overlay
	standupPosting              // posting update
	standupDone                 // summary
)

type standupModel struct {
	client   *clickup.Client
	cfg      config.Config
	userID   string
	all      bool
	mine     bool
	state    standupState
	tasks    []standupTask
	cursor   int // cursor for task selection
	textarea textarea.Model
	width    int
	height   int
	err      error
	quitting bool

	// Per-task update state
	updateIdx    int              // index into selected tasks
	selected     []int            // indices of selected tasks
	statuses     []clickup.Status // available statuses for current task's list
	statusCursor int              // cursor in status picker
	newStatus    string           // chosen status (empty = no change)

	// Summary of posted updates
	posted  []standupResult
	spinner spinner.Model
}

type standupResult struct {
	taskName  string
	commented bool
	oldStatus string
	newStatus string
}

// Messages
type standupTasksLoaded []standupTask
type standupStatusesLoaded []clickup.Status
type standupUpdatePosted struct{}

func initialStandupModel(client *clickup.Client, cfg config.Config, userID string, all bool, mine bool) standupModel {
	return standupModel{
		client:  client,
		cfg:     cfg,
		userID:  userID,
		all:     all,
		mine:    mine,
		state:   standupLoading,
		spinner: ui.NewSpinnerModel(),
	}
}

func (m standupModel) Init() tea.Cmd {
	loadCmd := func() tea.Msg {
		var tasks []standupTask
		for _, folder := range m.cfg.Folders {
			lists, err := m.client.GetLists(folder.ID)
			if err != nil {
				continue
			}
			for _, listObj := range lists {
				apiTasks, err := m.client.GetTasks(listObj.ID)
				if err != nil {
					continue
				}
				for _, task := range apiTasks {
					if filter.ShouldIncludeTask(task, m.userID, m.all, m.mine) {
						tasks = append(tasks, standupTask{
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
		sort.Slice(tasks, func(i, j int) bool {
			timeI, _ := strconv.ParseInt(tasks[i].task.DateUpdated, 10, 64)
			timeJ, _ := strconv.ParseInt(tasks[j].task.DateUpdated, 10, 64)
			return timeI > timeJ
		})
		
		return standupTasksLoaded(tasks)
	}
	return tea.Batch(loadCmd, m.spinner.Tick)
}

func (m standupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case standupTasksLoaded:
		m.tasks = msg
		if len(m.tasks) == 0 {
			m.state = standupDone
		} else {
			m.state = standupSelect
		}
		return m, nil

	case standupStatusesLoaded:
		m.statuses = msg
		m.statusCursor = 0
		// Pre-select current status
		currentStatus := strings.ToLower(m.tasks[m.selected[m.updateIdx]].task.Status.Status)
		for i, s := range m.statuses {
			if strings.ToLower(s.Status) == currentStatus {
				m.statusCursor = i
				break
			}
		}
		m.state = standupStatus
		return m, nil

	case standupUpdatePosted:
		m.updateIdx++
		if m.updateIdx >= len(m.selected) {
			m.state = standupDone
			if os.Getenv("CLICKUP_TUI_MENU") != "1" {
				return m, tea.Quit
			}
			return m, nil
		}
		m.state = standupUpdate
		return m, m.initTaskUpdate()

	case errMsg:
		m.err = msg
		return m, nil

	case spinner.TickMsg:
		if m.state == standupLoading || m.state == standupPosting {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case standupSelect:
			return m.updateSelect(msg)
		case standupUpdate:
			return m.updateTask(msg)
		case standupStatus:
			return m.updateStatusPicker(msg)
		case standupDone:
			if msg.String() == "q" || msg.String() == "esc" {
				return m, tea.Quit
			}
		}
	}

	// Pass through to textarea when in update mode
	if m.state == standupUpdate {
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m standupModel) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.tasks)-1 {
			m.cursor++
		}
	case " ":
		m.tasks[m.cursor].selected = !m.tasks[m.cursor].selected
	case "a":
		allSelected := true
		for _, t := range m.tasks {
			if !t.selected {
				allSelected = false
				break
			}
		}
		for i := range m.tasks {
			m.tasks[i].selected = !allSelected
		}
	case "enter":
		m.selected = nil
		for i, t := range m.tasks {
			if t.selected {
				m.selected = append(m.selected, i)
			}
		}
		if len(m.selected) == 0 {
			return m, tea.Quit
		}
		m.updateIdx = 0
		return m, m.initTaskUpdate()
	case "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m *standupModel) initTaskUpdate() tea.Cmd {
	ta := textarea.New()
	ta.Placeholder = "Add a comment (optional, leave empty to skip)..."
	ta.Focus()
	if m.width > 0 {
		ta.SetWidth(m.width - 10)
	}
	ta.SetHeight(5)
	m.textarea = ta
	m.newStatus = ""
	m.state = standupUpdate
	return textarea.Blink
}

func (m standupModel) updateTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Skip this task
		m.updateIdx++
		if m.updateIdx >= len(m.selected) {
			m.state = standupDone
			return m, nil
		}
		return m, m.initTaskUpdate()
	case "tab":
		// Open status picker
		task := m.tasks[m.selected[m.updateIdx]]
		return m, func() tea.Msg {
			list, err := m.client.GetList(task.listID)
			if err != nil {
				return errMsg(err)
			}
			return standupStatusesLoaded(list.Statuses)
		}
	case "ctrl+s":
		// Submit update for this task
		return m.submitUpdate()
	}

	// Let textarea handle the key
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m standupModel) updateStatusPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.statusCursor > 0 {
			m.statusCursor--
		}
	case "down", "j":
		if m.statusCursor < len(m.statuses)-1 {
			m.statusCursor++
		}
	case "enter":
		m.newStatus = m.statuses[m.statusCursor].Status
		m.state = standupUpdate
	case "esc":
		m.state = standupUpdate
	}
	return m, nil
}

func (m standupModel) submitUpdate() (tea.Model, tea.Cmd) {
	idx := m.selected[m.updateIdx]
	task := m.tasks[idx]
	commentText := strings.TrimSpace(m.textarea.Value())
	newStatus := m.newStatus
	oldStatus := task.task.Status.Status

	hasComment := commentText != ""
	hasStatusChange := newStatus != "" && strings.ToLower(newStatus) != strings.ToLower(oldStatus)

	if !hasComment && !hasStatusChange {
		// Nothing to do, move on
		m.updateIdx++
		if m.updateIdx >= len(m.selected) {
			m.state = standupDone
			if os.Getenv("CLICKUP_TUI_MENU") != "1" {
				m.quitting = true
				return m, tea.Quit
			}
			return m, nil
		}
		return m, m.initTaskUpdate()
	}

	m.state = standupPosting

	result := standupResult{
		taskName:  task.task.Name,
		commented: hasComment,
		oldStatus: oldStatus,
	}
	if hasStatusChange {
		result.newStatus = newStatus
	}
	m.posted = append(m.posted, result)

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		if hasComment {
			if err := m.client.CreateTaskComment(task.task.ID, commentText); err != nil {
				return errMsg(err)
			}
		}
		if hasStatusChange {
			if err := m.client.UpdateTaskStatus(task.task.ID, newStatus); err != nil {
				return errMsg(err)
			}
		}
		return standupUpdatePosted{}
	})
}

func (m standupModel) View() string {
	if m.err != nil {
		return ui.DocStyle.Render(fmt.Sprintf("Error: %v\n\nPress ctrl+c to quit.", m.err))
	}
	
	if m.quitting {
		return ""
	}

	switch m.state {
	case standupLoading:
		return ui.DocStyle.Render(ui.SpinnerView("Loading tasks...", m.spinner))

	case standupSelect:
		return ui.DocStyle.Render(m.viewSelect())

	case standupUpdate:
		return ui.DocStyle.Render(m.viewUpdate())

	case standupStatus:
		return ui.DocStyle.Render(m.viewStatusPicker())

	case standupPosting:
		return ui.DocStyle.Render(ui.SpinnerView("Posting update...", m.spinner))

	case standupDone:
		return ui.DocStyle.Render(m.viewDone())
	}

	return ""
}

func (m standupModel) viewDoneContent() string {
	var b strings.Builder

	b.WriteString(ui.HeaderStyle.Render("Standup Complete") + "\n\n")

	if len(m.posted) == 0 {
		if len(m.tasks) == 0 {
			b.WriteString("No tasks found.\n")
		} else {
			b.WriteString("No updates posted.\n")
		}
	} else {
		for _, r := range m.posted {
			b.WriteString(fmt.Sprintf("  %s\n", lipgloss.NewStyle().Bold(true).Render(r.taskName)))
			if r.commented {
				b.WriteString("    + comment added\n")
			}
			if r.newStatus != "" {
				b.WriteString(fmt.Sprintf("    + status: %s → %s\n", r.oldStatus, r.newStatus))
			}
			b.WriteString("\n")
		}
	}
	
	return b.String()
}

func (m standupModel) viewDone() string {
	content := m.viewDoneContent()
	return fmt.Sprintf("%s(Press q or esc to quit)", content)
}

func (m standupModel) viewSelect() string {
	var b strings.Builder

	b.WriteString(ui.HeaderStyle.Render("Standup: Select tasks to update") + "\n\n")

	// Calculate visible window
	maxVisible := m.height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.tasks) {
		end = len(m.tasks)
	}

	// Measure column widths
	maxStatus := 0
	maxName := 0
	maxDate := 0
	maxFolder := 0
	for i := start; i < end; i++ {
		t := m.tasks[i]
		if w := len(t.task.Status.Status) + 2; w > maxStatus {
			maxStatus = w
		}
		if w := len(t.task.Name); w > maxName {
			maxName = w
		}
		if w := len(format.FormatTaskDate(t.task.DateUpdated)); w > maxDate {
			maxDate = w
		}
		folder := t.folderName + "/" + t.listName
		if w := len(folder); w > maxFolder {
			maxFolder = w
		}
	}

	// Header row
	prefixWidth := 7 // "> [x] "
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ui.ColorGray)).Underline(true)
	header := fmt.Sprintf("%s%s  %s  %s  %s",
		strings.Repeat(" ", prefixWidth),
		headerStyle.Width(maxStatus).Render("Status"),
		headerStyle.Width(maxName).Render("Name"),
		headerStyle.Width(maxDate).Render("Date"),
		headerStyle.Render("Folder"),
	)
	b.WriteString(header + "\n")

	// Task rows
	statusCol := lipgloss.NewStyle().Bold(true).Width(maxStatus)
	nameCol := lipgloss.NewStyle().Width(maxName)
	dateCol := lipgloss.NewStyle().Width(maxDate)
	folderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorDarkGray))

	for i := start; i < end; i++ {
		t := m.tasks[i]
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		check := "[ ]"
		if t.selected {
			check = "[x]"
		}

		status := t.task.Status.Status
		sColor := ui.StatusColors[strings.ToLower(status)]
		if sColor == "" {
			sColor = ui.ColorGray
		}

		line := fmt.Sprintf("%s%s %s  %s  %s  %s",
			cursor, check,
			statusCol.Foreground(lipgloss.Color(sColor)).Render("["+status+"]"),
			nameCol.Render(t.task.Name),
			dateCol.Foreground(lipgloss.Color(ui.ColorGray)).Render(format.FormatTaskDate(t.task.DateUpdated)),
			folderStyle.Render(t.folderName+"/"+t.listName),
		)
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
	selectedCount := 0
	for _, t := range m.tasks {
		if t.selected {
			selectedCount++
		}
	}
	b.WriteString(fmt.Sprintf("%d/%d selected\n", selectedCount, len(m.tasks)))
	b.WriteString("(space: toggle | a: toggle all | enter: start updates | q: quit)")

	return b.String()
}

func (m standupModel) viewUpdate() string {
	var b strings.Builder

	idx := m.selected[m.updateIdx]
	task := m.tasks[idx]
	progress := fmt.Sprintf("Task %d/%d", m.updateIdx+1, len(m.selected))

	b.WriteString(ui.HeaderStyle.Render(progress) + "\n\n")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(task.task.Name) + "\n")

	// Current status
	currentStatus := task.task.Status.Status
	sColor := ui.StatusColors[strings.ToLower(currentStatus)]
	if sColor == "" {
		sColor = ui.ColorGray
	}
	statusDisplay := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(sColor)).Render(currentStatus)

	if m.newStatus != "" && strings.ToLower(m.newStatus) != strings.ToLower(currentStatus) {
		newColor := ui.StatusColors[strings.ToLower(m.newStatus)]
		if newColor == "" {
			newColor = ui.ColorGray
		}
		newDisplay := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(newColor)).Render(m.newStatus)
		b.WriteString(fmt.Sprintf("Status: %s → %s\n", statusDisplay, newDisplay))
	} else {
		b.WriteString(fmt.Sprintf("Status: %s\n", statusDisplay))
	}

	b.WriteString(fmt.Sprintf("Folder: %s | List: %s\n", task.folderName, task.listName))

	assignees := []string{}
	for _, a := range task.task.Assignees {
		assignees = append(assignees, a.Username)
	}
	if len(assignees) > 0 {
		b.WriteString(fmt.Sprintf("Assignees: %s\n", ui.AssigneeStyle.Render(strings.Join(assignees, ", "))))
	}

	b.WriteString("\n")
	b.WriteString(m.textarea.View())
	b.WriteString("\n\n(Tab: change status | Ctrl+S: submit | Esc: skip)")

	return b.String()
}

func (m standupModel) viewStatusPicker() string {
	var b strings.Builder

	idx := m.selected[m.updateIdx]
	task := m.tasks[idx]
	b.WriteString(ui.HeaderStyle.Render("Change Status: "+task.task.Name) + "\n\n")

	currentStatus := strings.ToLower(task.task.Status.Status)
	for i, s := range m.statuses {
		cursor := "  "
		if i == m.statusCursor {
			cursor = "> "
		}

		sColor := ui.StatusColors[strings.ToLower(s.Status)]
		if sColor == "" {
			sColor = ui.ColorGray
		}
		styled := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(sColor)).Render(s.Status)

		current := ""
		if strings.ToLower(s.Status) == currentStatus {
			current = " (current)"
		}

		b.WriteString(fmt.Sprintf("%s%s%s\n", cursor, styled, current))
	}

	b.WriteString("\n(enter: select | esc: cancel)")
	return b.String()
}

func init() {
	standupCmd.Flags().BoolVarP(&standupAll, "all", "a", false, "Include all open tasks (including backlog and scoping)")
	standupCmd.Flags().BoolVar(&standupMine, "mine", true, "Only show tasks assigned to you")
	rootCmd.AddCommand(standupCmd)
}
