package cmd

import (
	"fmt"
	"os"
	"strings"

	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"
	"clickup-tui/pkg/ui"
	"clickup-tui/pkg/util"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new task in a saved folder",
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

		if len(cfg.Folders) == 0 {
			fmt.Println("No folders configured. Run 'clickup-tui setup' to select folders.")
			return
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

		m := initialNewModel(client, cfg, currentUser)
		
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
		
		finalNewModel := finalModel.(newModel)
		if os.Getenv("CLICKUP_TUI_MENU") != "1" && finalNewModel.step == stepNewDone {
			fmt.Println(finalNewModel.viewDoneContent())
		}
	},
}

type newStep int

const (
	stepFolderSelect newStep = iota
	stepListSelect
	stepStatusSelect
	stepNameInput
	stepDescriptionInput
	stepAssigneeSelect
	stepConfirm
	stepCreating
	stepNewDone
)

type folderItem struct {
	folder config.FolderConfig
}

func (i folderItem) Title() string       { return i.folder.Name }
func (i folderItem) Description() string { return i.folder.ID }
func (i folderItem) FilterValue() string { return i.folder.Name }

type listItem struct {
	list clickup.List
}

func (i listItem) Title() string       { return i.list.Name }
func (i listItem) Description() string { return i.list.ID }
func (i listItem) FilterValue() string { return i.list.Name }

type statusItem struct {
	status clickup.Status
}

func (i statusItem) Title() string       { return i.status.Status }
func (i statusItem) Description() string { return "Status" }
func (i statusItem) FilterValue() string { return i.status.Status }

type assigneeItem struct {
	user clickup.User
}

func (i assigneeItem) Title() string       { return i.user.Username }
func (i assigneeItem) Description() string { return fmt.Sprintf("%s (ID: %s)", i.user.Email, i.user.ID.String()) }
func (i assigneeItem) FilterValue() string { return i.user.Username + " " + i.user.ID.String() }

type listsMsg []clickup.List
type listMsg clickup.List
type usersMsg []clickup.User
type taskCreatedMsg clickup.Task

type newModel struct {
	client           *clickup.Client
	cfg              config.Config
	currentUser      clickup.User
	step             newStep
	folderList       list.Model
	listList         list.Model
	statusList       list.Model
	assigneeList     list.Model
	selectedFolder   config.FolderConfig
	selectedList     clickup.List
	selectedStatus   clickup.Status
	selectedAssignee *clickup.User
	nameInput        textinput.Model
	descInput        textarea.Model
	loading          bool
	spinner          spinner.Model
	createdTask      *clickup.Task
	err              error
	quitting         bool
	width            int
	height           int
}

func initialNewModel(client *clickup.Client, cfg config.Config, currentUser clickup.User) newModel {
	folders := make([]list.Item, len(cfg.Folders))
	for i, f := range cfg.Folders {
		folders[i] = folderItem{folder: f}
	}

	folderList := list.New(folders, list.NewDefaultDelegate(), 0, 0)
	folderList.Title = "Select Folder"

	listList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	listList.Title = "Select List"

	statusList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	statusList.Title = "Select Status"

	assigneeList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	assigneeList.Title = "Select Assignee"

	nameInput := textinput.New()
	nameInput.Placeholder = "Task name"
	nameInput.Focus()

	descInput := textarea.New()
	descInput.Placeholder = "Description (optional)"
	descInput.SetHeight(6)

	return newModel{
		client:           client,
		cfg:              cfg,
		currentUser:      currentUser,
		selectedAssignee: &currentUser,
		step:             stepFolderSelect,
		folderList:       folderList,
		listList:         listList,
		statusList:       statusList,
		assigneeList:     assigneeList,
		nameInput:        nameInput,
		descInput:        descInput,
		spinner:          ui.NewSpinnerModel(),
	}
}

func (m newModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m newModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := ui.DocStyle.GetFrameSize()
		m.folderList.SetSize(msg.Width-h, msg.Height-v)
		m.listList.SetSize(msg.Width-h, msg.Height-v)
		m.statusList.SetSize(msg.Width-h, msg.Height-v)
		m.assigneeList.SetSize(msg.Width-h, msg.Height-v)
		m.nameInput.Width = msg.Width - h - 6
		m.descInput.SetWidth(msg.Width - h - 6)
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.step == stepCreating {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case usersMsg:
		m.loading = false
		if len(msg) == 0 {
			m.err = fmt.Errorf("no users found in workspace %s", m.cfg.WorkspaceName)
			m.step = stepConfirm
			return m, nil
		}
		items := make([]list.Item, len(msg))
		for i, u := range msg {
			items[i] = assigneeItem{user: u}
		}
		m.assigneeList.SetItems(items)
		m.step = stepAssigneeSelect
		return m, nil

	case listsMsg:
		m.loading = false
		if len(msg) == 0 {
			m.err = fmt.Errorf("no lists found in folder %s", m.selectedFolder.Name)
			m.step = stepFolderSelect
			return m, nil
		}

		// Auto-select "List" if it exists, or if there's only one list
		var autoSelectedList *clickup.List
		if len(msg) == 1 {
			autoSelectedList = &msg[0]
		} else {
			for _, l := range msg {
				if strings.ToLower(l.Name) == "list" {
					autoSelectedList = &l
					break
				}
			}
		}

		if autoSelectedList != nil {
			m.selectedList = *autoSelectedList
			m.loading = true
			m.step = stepStatusSelect
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				list, err := m.client.GetList(autoSelectedList.ID)
				if err != nil {
					return errMsg(err)
				}
				return listMsg(list)
			})
		}

		items := make([]list.Item, len(msg))
		for i, l := range msg {
			items[i] = listItem{list: l}
		}
		m.listList.SetItems(items)
		m.listList.Title = fmt.Sprintf("Select List (%s)", m.selectedFolder.Name)
		m.step = stepListSelect
		return m, nil

	case listMsg:
		m.loading = false
		m.selectedList = clickup.List(msg)

		// Populate status list
		statusItems := make([]list.Item, len(m.selectedList.Statuses))
		for i, s := range m.selectedList.Statuses {
			statusItems[i] = statusItem{status: s}
		}
		m.statusList.SetItems(statusItems)
		m.statusList.Title = fmt.Sprintf("Select Status (%s)", m.selectedList.Name)
		m.step = stepStatusSelect
		return m, nil

	case taskCreatedMsg:
		task := clickup.Task(msg)
		m.createdTask = &task
		m.step = stepNewDone
		m.loading = false
		if os.Getenv("CLICKUP_TUI_MENU") != "1" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.err != nil && msg.String() == "esc" {
			m.err = nil
			return m, nil
		}

		switch m.step {
		case stepFolderSelect:
			switch msg.String() {
			case "enter":
				if it, ok := m.folderList.SelectedItem().(folderItem); ok {
					m.selectedFolder = it.folder
					m.loading = true
					m.step = stepListSelect
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						lists, err := m.client.GetLists(it.folder.ID)
						if err != nil {
							return errMsg(err)
						}
						return listsMsg(lists)
					})
				}
			}

		case stepListSelect:
			switch msg.String() {
			case "esc":
				m.step = stepFolderSelect
				return m, nil
			case "enter":
				if it, ok := m.listList.SelectedItem().(listItem); ok {
					m.selectedList = it.list
					m.loading = true
					m.step = stepStatusSelect
					return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
						list, err := m.client.GetList(it.list.ID)
						if err != nil {
							return errMsg(err)
						}
						return listMsg(list)
					})
				}
			}

		case stepStatusSelect:
			switch msg.String() {
			case "esc":
				m.step = stepListSelect
				return m, nil
			case "enter":
				if it, ok := m.statusList.SelectedItem().(statusItem); ok {
					m.selectedStatus = it.status
					m.step = stepNameInput
					m.nameInput.Focus()
					return m, nil
				}
			}

		case stepNameInput:
			switch msg.String() {
			case "esc":
				m.step = stepStatusSelect
				return m, nil
			case "enter":
				if strings.TrimSpace(m.nameInput.Value()) == "" {
					return m, nil
				}
				m.step = stepDescriptionInput
				m.descInput.Focus()
				return m, textarea.Blink
			}

		case stepDescriptionInput:
			switch msg.String() {
			case "esc":
				m.step = stepNameInput
				m.nameInput.Focus()
				return m, nil
			case "ctrl+s":
				m.step = stepConfirm
				return m, nil
			}

		case stepConfirm:
			switch msg.String() {
			case "esc":
				m.step = stepDescriptionInput
				m.descInput.Focus()
				return m, nil
			case "n":
				m.step = stepNameInput
				m.nameInput.Focus()
				return m, nil
			case "d":
				m.step = stepDescriptionInput
				m.descInput.Focus()
				return m, nil
			case "a":
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					users, err := m.client.GetWorkspaceUsers(m.cfg.WorkspaceID)
					if err != nil {
						return errMsg(err)
					}
					return usersMsg(users)
				})
			case "enter":
				m.step = stepCreating
				m.loading = true
				name := strings.TrimSpace(m.nameInput.Value())
				desc := strings.TrimSpace(m.descInput.Value())
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					var assignees []int64
					if m.selectedAssignee != nil {
						assignees = []int64{int64(m.selectedAssignee.ID)}
					}
					task, err := m.client.CreateTask(m.selectedList.ID, name, desc, m.selectedStatus.Status, assignees)
					if err != nil {
						return errMsg(err)
					}
					return taskCreatedMsg(task)
				})
			}

		case stepAssigneeSelect:
			switch msg.String() {
			case "esc":
				m.step = stepConfirm
				return m, nil
			case "enter":
				if it, ok := m.assigneeList.SelectedItem().(assigneeItem); ok {
					userCopy := it.user
					m.selectedAssignee = &userCopy
					m.step = stepConfirm
					return m, nil
				}
			}

		case stepNewDone:
			switch msg.String() {
			case "q", "enter", "esc":
				return m, tea.Quit
			}
		}
	}

	switch m.step {
	case stepFolderSelect:
		m.folderList, cmd = m.folderList.Update(msg)
		return m, cmd
	case stepListSelect:
		m.listList, cmd = m.listList.Update(msg)
		return m, cmd
	case stepStatusSelect:
		m.statusList, cmd = m.statusList.Update(msg)
		return m, cmd
	case stepNameInput:
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	case stepDescriptionInput:
		m.descInput, cmd = m.descInput.Update(msg)
		return m, cmd
	case stepAssigneeSelect:
		m.assigneeList, cmd = m.assigneeList.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m newModel) View() string {
	if m.err != nil {
		return ui.DocStyle.Render(fmt.Sprintf("Error: %v\n\nPress esc to go back or ctrl+c to quit.", m.err))
	}
	
	if m.quitting {
		return ""
	}

	switch m.step {
	case stepFolderSelect:
		return ui.DocStyle.Render(m.folderList.View())
	case stepListSelect:
		if m.loading {
			return ui.DocStyle.Render(ui.SpinnerView("Loading lists...", m.spinner))
		}
		return ui.DocStyle.Render(m.listList.View())
	case stepStatusSelect:
		if m.loading {
			return ui.DocStyle.Render(ui.SpinnerView("Loading statuses...", m.spinner))
		}
		return ui.DocStyle.Render(m.statusList.View())
	case stepNameInput:
		var b strings.Builder
		b.WriteString(ui.HeaderStyle.Render("New Task") + "\n\n")
		b.WriteString("Folder: " + m.selectedFolder.Name + "\n")
		b.WriteString("List:   " + m.selectedList.Name + "\n")
		b.WriteString("Status: " + m.selectedStatus.Status + "\n\n")
		b.WriteString("Task name:\n")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n\n(Enter: next | Esc: back)")
		return ui.DocStyle.Render(b.String())
	case stepDescriptionInput:
		var b strings.Builder
		b.WriteString(ui.HeaderStyle.Render("New Task") + "\n\n")
		b.WriteString("Folder: " + m.selectedFolder.Name + "\n")
		b.WriteString("List:   " + m.selectedList.Name + "\n")
		b.WriteString("Status: " + m.selectedStatus.Status + "\n\n")
		b.WriteString("Description:\n")
		b.WriteString(m.descInput.View())
		b.WriteString("\n\n(Ctrl+S: continue | Esc: back)")
		return ui.DocStyle.Render(b.String())
	case stepConfirm:
		if m.loading {
			return ui.DocStyle.Render(ui.SpinnerView("Loading users...", m.spinner))
		}
		var b strings.Builder
		b.WriteString(ui.HeaderStyle.Render("Confirm Task") + "\n\n")
		b.WriteString("Folder:   " + m.selectedFolder.Name + "\n")
		b.WriteString("List:     " + m.selectedList.Name + "\n")
		b.WriteString("Status:   " + m.selectedStatus.Status + "\n")
		assigneeName := "None"
		if m.selectedAssignee != nil {
			assigneeName = m.selectedAssignee.Username
		}
		b.WriteString("Assignee: " + assigneeName + "\n")
		b.WriteString("Name:     " + strings.TrimSpace(m.nameInput.Value()) + "\n\n")
		desc := strings.TrimSpace(m.descInput.Value())
		if desc == "" {
			desc = "(none)"
		}
		wrapWidth := m.width - 6
		if wrapWidth < 20 {
			wrapWidth = 20
		}
		b.WriteString("Description:\n")
		b.WriteString(ui.DocStyle.Width(wrapWidth).Render(desc) + "\n\n")
		b.WriteString("(Enter: create | Esc: back | n: edit name | d: edit description | a: edit assignee)")
		return ui.DocStyle.Render(b.String())
	case stepAssigneeSelect:
		return ui.DocStyle.Render(m.assigneeList.View())
	case stepCreating:
		return ui.DocStyle.Render(ui.SpinnerView("Creating task...", m.spinner))
	case stepNewDone:
		content := m.viewDoneContent()
		return ui.DocStyle.Render(fmt.Sprintf("%s\n(Press enter to exit)", content))
	}

	return ""
}

func (m newModel) viewDoneContent() string {
	var b strings.Builder
	b.WriteString(ui.HeaderStyle.Render("Task Created") + "\n\n")
	if m.createdTask != nil {
		b.WriteString(fmt.Sprintf("Name: %s\n", m.createdTask.Name))
		b.WriteString(fmt.Sprintf("ID:   %s\n", m.createdTask.ID))
	}
	return b.String()
}

func init() {
	rootCmd.AddCommand(newCmd)
}
