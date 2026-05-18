package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	key     string
	desc    string
	section string // if non-empty, printed as a section header before this entry
}

var helpEntries = []helpEntry{
	{section: "Navigation"},
	{key: "1 / 2 / 3", desc: "Switch tab (My PRs / Needs Review / All Open)"},
	{key: "j / ↓", desc: "Move down / scroll down"},
	{key: "k / ↑", desc: "Move up / scroll up"},
	{key: "pgdn / pgup", desc: "Page scroll (detail screen)"},
	{key: "enter", desc: "Open PR detail"},
	{key: "b / esc", desc: "Back to list"},
	{key: "w", desc: "Open PR in browser"},
	{key: "f / F", desc: "Refresh  /  load more PRs"},
	{section: "Review actions"},
	{key: "a", desc: "Approve PR (asks for optional comment)"},
	{key: "r", desc: "Request changes (opens comment box)"},
	{key: "c", desc: "Post comment (opens comment box)"},
	{section: "Inline comments (detail screen)"},
	{key: "n", desc: "Enter line-select mode"},
	{key: "n / enter", desc: "Add comment on selected line"},
	{key: "s", desc: "Toggle unified / split diff view"},
	{section: "Comment box"},
	{key: "ctrl+d", desc: "Submit"},
	{key: "enter", desc: "New line"},
	{key: "esc", desc: "Cancel"},
	{section: "App"},
	{key: "?", desc: "Toggle this help"},
	{key: "q / ctrl+c", desc: "Quit"},
}

var styleHelpSection = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#00BFFF")).
	Bold(true).
	PaddingTop(1)

func renderHelp(width, height int) string {
	var sb strings.Builder
	for _, e := range helpEntries {
		if e.section != "" {
			sb.WriteString(styleHelpSection.Render(e.section) + "\n")
			continue
		}
		sb.WriteString(StyleHelpKey.Render(e.key))
		sb.WriteString(StyleHelpDesc.Render(e.desc))
		sb.WriteByte('\n')
	}
	sb.WriteString("\n" + StyleHelpDesc.Render("Press ? or esc to close"))

	box := StyleHelpBox.Render(sb.String())

	// center the box
	boxWidth := lipgloss.Width(box)
	boxHeight := lipgloss.Height(box)
	leftPad := (width - boxWidth) / 2
	topPad := (height - boxHeight) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	if topPad < 0 {
		topPad = 0
	}

	lines := strings.Split(box, "\n")
	var out strings.Builder
	for i := 0; i < topPad; i++ {
		out.WriteByte('\n')
	}
	pad := strings.Repeat(" ", leftPad)
	for _, line := range lines {
		out.WriteString(pad + line + "\n")
	}
	return out.String()
}
