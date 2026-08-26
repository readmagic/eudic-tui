package theme

import "github.com/charmbracelet/lipgloss"

// 色彩定义（暗色主题）
var (
	Primary   = lipgloss.Color("#7D56F4")
	Accent    = lipgloss.Color("#FF79C6")
	Muted     = lipgloss.Color("#6272A4")
	Success   = lipgloss.Color("#50FA7B")
	Warning   = lipgloss.Color("#FFB86C")
	ErrorCol  = lipgloss.Color("#FF5555")
	Highlight = lipgloss.Color("#F1FA8C")
)

// 视图样式
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Primary).
			Padding(0, 2)

	ActiveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Accent).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().Foreground(Muted)

	SentenceActive = lipgloss.NewStyle().
			Foreground(Highlight).
			Bold(true)

	SentenceDone = lipgloss.NewStyle().Foreground(Success)

	SentencePending = lipgloss.NewStyle().Foreground(Muted)

	TranslationStyle = lipgloss.NewStyle().Foreground(Accent)

	Border = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary)

	HelpStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorCol).
			Bold(true)
)
