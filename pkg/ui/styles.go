package ui

import "github.com/charmbracelet/lipgloss"

// Color constants
var (
	ColorPurple   = lipgloss.AdaptiveColor{Light: "99", Dark: "170"}
	ColorBlue     = lipgloss.AdaptiveColor{Light: "27", Dark: "39"}
	ColorGray     = lipgloss.AdaptiveColor{Light: "241", Dark: "245"}
	ColorDarkGray = lipgloss.AdaptiveColor{Light: "237", Dark: "240"}
	ColorPink     = lipgloss.AdaptiveColor{Light: "161", Dark: "211"}
	ColorGreen    = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
	ColorOrange   = lipgloss.AdaptiveColor{Light: "166", Dark: "214"}
	ColorIndigo   = lipgloss.AdaptiveColor{Light: "57", Dark: "99"}
	ColorRed      = lipgloss.AdaptiveColor{Light: "160", Dark: "196"}
)

// Status colors mapping
var StatusColors = map[string]lipgloss.AdaptiveColor{
	"in progress": ColorGreen,
	"scoping":     ColorOrange,
	"in review":   ColorIndigo,
	"blocked":     ColorRed,
	"backlog":     ColorDarkGray,
}

// Style definitions
var (
	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPurple).
		MarginTop(1).
		Underline(true)

	FolderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorBlue).
		MarginTop(1).
		PaddingLeft(2)

	ListStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(ColorGray).
		PaddingLeft(4)

	TaskStyle = lipgloss.NewStyle().
		PaddingLeft(6)

	TaskNameStyle = lipgloss.NewStyle().
		Bold(true)

	StatusStyle = lipgloss.NewStyle().
		Bold(true).
		Width(15)

	IDStyle = lipgloss.NewStyle().
		Foreground(ColorDarkGray)

	AssigneeStyle = lipgloss.NewStyle().
		Foreground(ColorPink)

	DateStyle = lipgloss.NewStyle().
		Foreground(ColorGray)

	CommentBaseStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.AdaptiveColor{Light: "239", Dark: "242"})

	SubtaskIndicatorStyle = lipgloss.NewStyle().
		Foreground(ColorGray).
		Bold(true)

	SummaryStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(ColorBlue).
		PaddingLeft(2)

	NoTasksStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(ColorDarkGray).
		PaddingLeft(4)

	SelectedTaskStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPurple).
		Background(lipgloss.AdaptiveColor{Light: "252", Dark: "235"}).
		PaddingLeft(2)

	TaskItemStyle = lipgloss.NewStyle().
		PaddingLeft(4)

	DocStyle = lipgloss.NewStyle().Margin(1, 2)
)
