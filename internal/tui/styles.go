package tui

import "github.com/charmbracelet/lipgloss"

var (
	accentColor  = lipgloss.Color("39")
	cyanColor    = lipgloss.Color("45")
	greenColor   = lipgloss.Color("42")
	yellowColor  = lipgloss.Color("220")
	redColor     = lipgloss.Color("203")
	magentaColor = lipgloss.Color("171")
	panelColor   = lipgloss.Color("238")
	subtleColor  = lipgloss.Color("240")
	textColor    = lipgloss.Color("252")

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(panelColor).
			Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(panelColor).
			Padding(0, 1)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(accentColor).
			Padding(1, 2).
			Background(lipgloss.Color("235")).
			Foreground(textColor)

	statusStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("250"))

	sepStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	labelStyle = lipgloss.NewStyle().
			Foreground(cyanColor).
			Bold(true)

	keyStyle = lipgloss.NewStyle().
			Foreground(cyanColor).
			Bold(true)

	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")).
			Bold(true)

	assistantStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("114")).
			Bold(true)

	wrapStyle = lipgloss.NewStyle().
			Foreground(textColor)

	emptyStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Align(lipgloss.Center, lipgloss.Center)
)
