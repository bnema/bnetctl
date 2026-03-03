package styles

import "github.com/charmbracelet/lipgloss"

// Color palette -- Battle.net blue theme
var (
	ColorPrimary   = lipgloss.Color("#00AEFF") // Battle.net blue
	ColorSecondary = lipgloss.Color("#148EFF")
	ColorSuccess   = lipgloss.Color("#00D97E")
	ColorError     = lipgloss.Color("#FF4D6A")
	ColorWarning   = lipgloss.Color("#FFB347")
	ColorMuted     = lipgloss.Color("#666666")
	ColorWhite     = lipgloss.Color("#FFFFFF")
)

// Styles
var (
	Title = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	Success = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	Error = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)

	Warning = lipgloss.NewStyle().
		Foreground(ColorWarning)

	Muted = lipgloss.NewStyle().
		Foreground(ColorMuted)

	Bold = lipgloss.NewStyle().
		Bold(true)

	StepPrefix = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)
)
