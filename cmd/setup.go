package cmd

import (
	"fmt"
	"io"
	"os"

	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/config"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure the default ClickUp workspace, space, and folders",
	Run: func(cmd *cobra.Command, args []string) {
		pat := os.Getenv("CLICKUP_PAT")
		if pat == "" {
			fmt.Println("Error: CLICKUP_PAT environment variable not set")
			os.Exit(1)
		}

		client := clickup.NewClient(pat)

		m := initialModel(client)
		p := tea.NewProgram(m, tea.WithAltScreen())

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
		style := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
		fmt.Fprint(w, style.Render("> "+str))
	} else {
		style := lipgloss.NewStyle().PaddingLeft(4)
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
	}
}

type teamsMsg []clickup.Team
type spacesMsg []clickup.Space
type foldersMsg []clickup.Folder
type errMsg error

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		teams, err := m.client.GetTeams()
		if err != nil {
			return errMsg(err)
		}
		return teamsMsg(teams)
	}
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
				return m, func() tea.Msg {
					spaces, err := m.client.GetSpaces(m.selectedWorkspace.ID)
					if err != nil {
						return errMsg(err)
					}
					return spacesMsg(spaces)
				}
			case stepSpace:
				m.selectedSpace = clickup.Space{ID: selected.id, Name: selected.name}
				m.step = stepFolder
				m.loading = true
				m.list.Title = "Loading folders..."
				return m, func() tea.Msg {
					folders, err := m.client.GetFolders(m.selectedSpace.ID)
					if err != nil {
						return errMsg(err)
					}
					return foldersMsg(folders)
				}
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

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
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
	return docStyle.Render(m.list.View())
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)

func init() {
	rootCmd.AddCommand(setupCmd)
}
