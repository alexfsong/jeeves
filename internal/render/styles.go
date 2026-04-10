package render

import "github.com/charmbracelet/lipgloss"

// Jeeves aesthetic: muted navy, cream, understated
var (
	Navy       = lipgloss.Color("#1B2A4A")
	Cream      = lipgloss.Color("#F5F0E8")
	Slate      = lipgloss.Color("#8899AA")
	Gold       = lipgloss.Color("#C9A84C")
	DimWhite   = lipgloss.Color("#D0D0D0")
	ErrorRed   = lipgloss.Color("#CC6666")
	SuccessGrn = lipgloss.Color("#66CC99")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Gold)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Cream)

	BodyStyle = lipgloss.NewStyle().
			Foreground(DimWhite)

	URLStyle = lipgloss.NewStyle().
			Foreground(Slate).
			Underline(true)

	MetaStyle = lipgloss.NewStyle().
			Foreground(Slate).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorRed)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessGrn)

	ScoreStyle = lipgloss.NewStyle().
			Foreground(Gold).
			Bold(true)

	DividerStyle = lipgloss.NewStyle().
			Foreground(Slate)
)

func Divider() string {
	return DividerStyle.Render("────────────────────────────────────────")
}
