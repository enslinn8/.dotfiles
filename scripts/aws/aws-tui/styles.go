package main

import "github.com/charmbracelet/lipgloss"

var (
	colorEnabled     = lipgloss.Color("#43BF6D")
	colorDisabled    = lipgloss.Color("#E84855")
	colorUnconfirmed = lipgloss.Color("#F5A623")
	colorForceChange = lipgloss.Color("#4ECDC4")

	appNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(2).
			PaddingRight(2)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E84855")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#43BF6D")).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F5A623")).
			Bold(true)

	keyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4ECDC4")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#7D56F4")).
				Bold(true)
)

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "CONFIRMED":
		return lipgloss.NewStyle().Foreground(colorEnabled)
	case "UNCONFIRMED":
		return lipgloss.NewStyle().Foreground(colorUnconfirmed)
	case "FORCE_CHANGE_PASSWORD":
		return lipgloss.NewStyle().Foreground(colorForceChange)
	case "RESET_REQUIRED":
		return lipgloss.NewStyle().Foreground(colorForceChange)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	}
}

func enabledStyle(enabled bool) lipgloss.Style {
	if enabled {
		return lipgloss.NewStyle().Foreground(colorEnabled).Bold(true)
	}
	return lipgloss.NewStyle().Foreground(colorDisabled).Bold(true)
}
