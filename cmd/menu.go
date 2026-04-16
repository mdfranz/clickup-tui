package cmd

import (
	"fmt"
	"os"

	"clickup-tui/pkg/ui"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive menu to launch commands",
	Run: func(cmd *cobra.Command, args []string) {
		m := initialMenuModel()
		p := tea.NewProgram(m, tea.WithAltScreen())

		res, err := p.Run()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		if finalModel, ok := res.(menuModel); ok && finalModel.choice != "" {
			// Find the subcommand and execute it
			root := cmd.Root()
			for _, sub := range root.Commands() {
				if sub.Name() == finalModel.choice {
					root.SetArgs([]string{finalModel.choice})
					if err := root.Execute(); err != nil {
						fmt.Printf("Error executing %s: %v\n", finalModel.choice, err)
						os.Exit(1)
					}
					return
				}
			}
		}
	},
}

type menuItem struct {
	cmd         string
	title       string
	description string
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.description }
func (i menuItem) FilterValue() string { return i.title + " " + i.description }

type menuModel struct {
	list   list.Model
	choice string
}

func initialMenuModel() menuModel {
	items := []list.Item{
		menuItem{cmd: "tasks", title: "Tasks", description: "Display tasks from your workspace"},
		menuItem{cmd: "browse", title: "Browse", description: "Interactively browse tasks"},
		menuItem{cmd: "new", title: "New Task", description: "Create a new task"},
		menuItem{cmd: "standup", title: "Standup", description: "Walk through tasks and post updates"},
		menuItem{cmd: "setup", title: "Setup", description: "Configure your workspace, space, and folders"},
		menuItem{cmd: "show", title: "Show Config", description: "Display current configuration"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "ClickUp TUI Menu"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return menuModel{
		list: l,
	}
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if i, ok := m.list.SelectedItem().(menuItem); ok {
				m.choice = i.cmd
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		h, v := ui.DocStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m menuModel) View() string {
	return ui.DocStyle.Render(m.list.View())
}

func init() {
	rootCmd.AddCommand(menuCmd)
}
