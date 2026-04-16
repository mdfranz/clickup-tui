package cmd

import (
	"fmt"
	"io"
	"os"

	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"
	"clickup-tui/pkg/ui"
	"clickup-tui/pkg/util"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure the default ClickUp workspace, space, and folders",
	Run: func(cmd *cobra.Command, args []string) {
		pat, err := util.GetClickUpPAT()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		client := clickup.NewClient(pat)

		m := initialModel(client)
		
		var opts []tea.ProgramOption
		if os.Getenv("CLICKUP_TUI_MENU") == "1" {
			opts = append(opts, tea.WithAltScreen())
		}
		p := tea.NewProgram(m, opts...)

		finalModel, err := p.Run()
		if err != nil {
			fmt.Printf("Error: %v", err)
			os.Exit(1)
		}

		m = finalModel.(model)
		if m.err != nil {
			fmt.Printf("Error: %v\n", m.err)
			os.Exit(1)
		}

		if m.step == stepDone {
			folders := []config.FolderConfig{}
			for id, name := range m.selectedFolders {
				folders = append(folders, config.FolderConfig{ID: id, Name: name})
			}

			cfg := config.Config{
				WorkspaceID:   m.selectedWorkspace.ID,
				WorkspaceName: m.selectedWorkspace.Name,
				SpaceID:       m.selectedSpace.ID,
				SpaceName:     m.selectedSpace.Name,
				Folders:       folders,
			}
			err := config.SaveConfig(cfg)
			if err != nil {
				fmt.Printf("Error saving config: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Configuration saved successfully!")
			fmt.Printf("Workspace: %s\nSpace:     %s\nFolders:   %d selected\n", cfg.WorkspaceName, cfg.SpaceName, len(cfg.Folders))
		}
	},
}

type step int

const (
	stepWorkspace step = iota
	stepSpace
	stepFolder
	stepDone
)

type item struct {
	id       string
	name     string
	desc     string
	selected bool
}

func (i item) Title() string       { return i.name }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.name }

type itemDelegate struct {
	list.DefaultDelegate
	isMultiSelect bool
}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := i.name
	if d.isMultiSelect {
		checkbox := "[ ] "
		if i.selected {
			checkbox = "[x] "
		}
		str = checkbox + str
	}

	if index == m.Index() {
		style := ui.DocStyle.PaddingLeft(2).Foreground(ui.DocStyle.GetForeground())
		fmt.Fprint(w, style.Render("> "+str))
	} else {
		style := ui.DocStyle.PaddingLeft(4)
		fmt.Fprint(w, style.Render(str))
	}
}

func (d itemDelegate) Height() int {
	return 1
}

func (d itemDelegate) Spacing() int {
	return 0
}

func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return d.DefaultDelegate.Update(msg, m)
}

type model struct {
	client            *clickup.Client
	step              step
	list              list.Model
	selectedWorkspace clickup.Team
	selectedSpace     clickup.Space
	selectedFolders   map[string]string // id -> name
	err               error
	loading           bool
	spinner           spinner.Model
}

func initialModel(client *clickup.Client) model {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Loading workspaces..."
	return model{
		client:          client,
		step:            stepWorkspace,
		list:            l,
		loading:         true,
		selectedFolders: make(map[string]string),
		spinner:         ui.NewSpinnerModel(),
	}
}

type teamsMsg []clickup.Team
type spacesMsg []clickup.Space
type foldersMsg []clickup.Folder
type errMsg error

func (m model) Init() tea.Cmd {
	loadCmd := func() tea.Msg {
		teams, err := m.client.GetTeams()
		if err != nil {
			return errMsg(err)
		}
		return teamsMsg(teams)
	}
	return tea.Batch(loadCmd, m.spinner.Tick)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case " ":
			if m.step == stepFolder && !m.loading {
				idx := m.list.Index()
				items := m.list.Items()
				if it, ok := items[idx].(item); ok {
					it.selected = !it.selected
					if it.selected {
						m.selectedFolders[it.id] = it.name
					} else {
						delete(m.selectedFolders, it.id)
					}
					items[idx] = it
					m.list.SetItems(items)
					return m, nil
				}
			}
		case "enter":
			if m.loading {
				return m, nil
			}

			if m.step == stepFolder {
				m.step = stepDone
				return m, tea.Quit
			}

			selected := m.list.SelectedItem().(item)
			switch m.step {
			case stepWorkspace:
				m.selectedWorkspace = clickup.Team{ID: selected.id, Name: selected.name}
				m.step = stepSpace
				m.loading = true
				m.list.Title = "Loading spaces..."
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					spaces, err := m.client.GetSpaces(m.selectedWorkspace.ID)
					if err != nil {
						return errMsg(err)
					}
					return spacesMsg(spaces)
				})
			case stepSpace:
				m.selectedSpace = clickup.Space{ID: selected.id, Name: selected.name}
				m.step = stepFolder
				m.loading = true
				m.list.Title = "Loading folders..."
				return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
					folders, err := m.client.GetFolders(m.selectedSpace.ID)
					if err != nil {
						return errMsg(err)
					}
					return foldersMsg(folders)
				})
			}
		}

	case teamsMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, team := range msg {
			items[i] = item{id: team.ID, name: team.Name}
		}
		m.list.SetItems(items)
		m.list.Title = "Select Workspace"

	case spacesMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, space := range msg {
			items[i] = item{id: space.ID, name: space.Name}
		}
		m.list.SetItems(items)
		m.list.Title = "Select Space"

	case foldersMsg:
		m.loading = false
		items := make([]list.Item, len(msg))
		for i, folder := range msg {
			items[i] = item{id: folder.ID, name: folder.Name}
		}
		m.list.SetItems(items)
		m.list.Title = "Select Folder (Space to toggle, Enter to confirm)"
		// Switch to a custom delegate that shows checkboxes
		d := itemDelegate{DefaultDelegate: list.NewDefaultDelegate(), isMultiSelect: true}
		d.ShowDescription = false
		m.list.SetDelegate(d)

	case errMsg:
		m.err = msg
		return m, tea.Quit

	case spinner.TickMsg:
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.WindowSizeMsg:
		h, v := ui.DocStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}
	if m.step == stepDone {
		return "Setup complete!\n"
	}
	if m.loading {
		return ui.DocStyle.Render(ui.SpinnerView(m.list.Title, m.spinner))
	}
	return ui.DocStyle.Render(m.list.View())
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
