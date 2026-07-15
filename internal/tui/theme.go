package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent  = lipgloss.AdaptiveColor{Light: "25", Dark: "39"}   // blue
	colorSubtle  = lipgloss.AdaptiveColor{Light: "245", Dark: "241"} // gray
	colorGood    = lipgloss.AdaptiveColor{Light: "28", Dark: "40"}   // green
	colorBad     = lipgloss.AdaptiveColor{Light: "124", Dark: "196"} // red
	colorWarn    = lipgloss.AdaptiveColor{Light: "130", Dark: "214"} // orange
	colorCluster = lipgloss.AdaptiveColor{Light: "26", Dark: "33"}
	colorUser    = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
	colorContext = lipgloss.AdaptiveColor{Light: "94", Dark: "220"}

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "231", Dark: "231"}).
			Background(colorAccent).
			Padding(0, 1)

	styleHeaderDirty = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "231", Dark: "231"}).
				Background(colorWarn).
				Padding(0, 1)

	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Underline(true).
			Padding(0, 1)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(colorSubtle).
				Padding(0, 1)

	styleFooter = lipgloss.NewStyle().Foreground(colorSubtle)

	styleToastInfo    = lipgloss.NewStyle().Foreground(colorAccent)
	styleToastSuccess = lipgloss.NewStyle().Foreground(colorGood).Bold(true)
	styleToastError   = lipgloss.NewStyle().Foreground(colorBad).Bold(true)
	styleToastWarn    = lipgloss.NewStyle().Foreground(colorWarn)

	styleCurrentMark = lipgloss.NewStyle().Foreground(colorGood).Bold(true)
	styleClusterName = lipgloss.NewStyle().Foreground(colorCluster)
	styleUserName    = lipgloss.NewStyle().Foreground(colorUser)
	styleContextName = lipgloss.NewStyle().Foreground(colorContext)

	styleModalBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2)

	styleModalTitle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	styleDetailKey = lipgloss.NewStyle().Foreground(colorSubtle).Width(22)
)
