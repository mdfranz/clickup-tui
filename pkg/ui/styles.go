package ui

import "github.com/charmbracelet/lipgloss"

// Color constants
const (
	ColorPurple   = "170"
	ColorBlue     = "39"
	ColorGray     = "245"
	ColorDarkGray = "240"
	ColorPink     = "211"
	ColorGreen    = "42"
	ColorOrange   = "214"
	ColorIndigo   = "99"
)

// Status colors mapping
var StatusColors = map[string]string{
	"in progress": ColorGreen,
	"scoping":     ColorOrange,
	"in review":   ColorIndigo,
	"backlog":     ColorDarkGray,
}

// Style definitions
var (
	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorPurple)).
		MarginTop(1).
		Underline(true)

	FolderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorBlue)).
		MarginTop(1).
		PaddingLeft(2)

	ListStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color(ColorGray)).
		PaddingLeft(4)

	TaskStyle = lipgloss.NewStyle().
		PaddingLeft(6)

	StatusStyle = lipgloss.NewStyle().
		Bold(true).
		Width(15)

	IDStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDarkGray))

	AssigneeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorPink))

	DateStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorGray))

	CommentBaseStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("242"))

	NoTasksStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color(ColorDarkGray)).
		PaddingLeft(4)

	DocStyle = lipgloss.NewStyle().Margin(1, 2)
)
