package tui

import "github.com/charmbracelet/lipgloss"

var (
	// App frame
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444"))

	StyleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#1A1A2E")).
			PaddingLeft(1).PaddingRight(1)

	StyleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			PaddingLeft(1)

	StyleFooterKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true)

	// Tabs
	StyleTabActive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#0077CC")).
			Bold(true).
			PaddingLeft(1).PaddingRight(1)

	StyleTabInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				PaddingLeft(1).PaddingRight(1)

	// PR list items
	StyleCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF7F")).
			Bold(true)

	StylePRTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD"))

	StylePRTitleDraft = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))

	StylePRRepo = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	StylePRAge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	// PR status dots
	StyleStatusPending = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	StyleStatusApproved = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF7F"))

	StyleStatusChanges = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFBF00"))

	StyleStatusConflict = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444"))

	StyleStatusDraft = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))

	// Status bar
	StyleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			PaddingLeft(1)

	StyleStatusBarError = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444")).
				PaddingLeft(1)

	StyleStatusBarOK = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF7F")).
				PaddingLeft(1)

	StyleStatusBarWarn = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFBF00")).
				PaddingLeft(1)

	// Rate limit warning
	StyleRateLimit = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFBF00"))

	// Help overlay
	StyleHelpBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00BFFF")).
			Padding(1, 2)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true).
			Width(14)

	StyleHelpDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	// Too small message
	StyleTooSmall = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4444")).
			Bold(true).
			Padding(1, 2)
)
