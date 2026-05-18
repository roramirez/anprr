package diff

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	styleSeparator = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
	styleEmpty     = lipgloss.NewStyle().Foreground(lipgloss.Color("#222222")).Background(lipgloss.Color("#111111"))
)

// RenderSplit renders a side-by-side diff into a single ANSI string.
// cursor is an index into the original []DiffLine (-1 = no cursor).
// commented is the set of diffLine indices with pending inline comments.
func RenderSplit(lines []DiffLine, width int, hl Highlighter, cursor int, commented map[int]bool) string {
	if hl == nil {
		hl = NoopHighlighter{}
	}
	if commented == nil {
		commented = map[int]bool{}
	}

	// pre-tokenize all files at once (same as unified Render)
	fileTokens := preTokenize(lines, hl)

	rows := Split(lines)

	// column widths: each side gets equal space, separator takes 3 chars (" │ ")
	colW := (width - 5) / 2 // -5: 2 for gutter marks + 3 for separator
	if colW < 10 {
		colW = 10
	}

	var sb strings.Builder
	for _, row := range rows {
		if row.IsHeader {
			mark := "  "
			line := row.Header
			var rendered string
			switch line.Type {
			case DiffHunkHeader:
				rendered = styleHunkHeader.Render(padRight(line.Text, width-2))
			default:
				rendered = styleFileHeader.Render(padRight(line.Text, width-2))
			}
			sb.WriteString(mark + rendered + "\n")
			continue
		}

		// left side
		var leftTokens []Token
		if row.LeftIdx >= 0 {
			leftTokens = fileTokens[row.LeftIdx]
		}
		leftMark, leftCell := renderSplitCell(row.Left, row.LeftIdx, colW, leftTokens, cursor, commented,
			styleRemoved, styleRemovedPrefix)

		// right side
		var rightTokens []Token
		if row.RightIdx >= 0 {
			rightTokens = fileTokens[row.RightIdx]
		}
		rightMark, rightCell := renderSplitCell(row.Right, row.RightIdx, colW, rightTokens, cursor, commented,
			styleAdded, styleAddedPrefix)

		sep := styleSeparator.Render(" │ ")
		sb.WriteString(leftMark + leftCell + sep + rightMark + rightCell + "\n")
	}
	return sb.String()
}

func renderSplitCell(
	line *DiffLine,
	idx int,
	colW int,
	tokens []Token,
	cursor int,
	commented map[int]bool,
	base, prefixSt lipgloss.Style,
) (mark, cell string) {
	if line == nil {
		return "  ", styleEmpty.Render(strings.Repeat("░", colW))
	}

	mark = "  "
	if commented[idx] {
		mark = styleCommentMark.Render("● ")
	}

	if idx == cursor && line.Commentable {
		rendered := renderLineWithTokens(*line, colW, tokens)
		cell = styleCursor.Render(padRight(stripANSI(rendered), colW))
		mark = styleCursor.Render("> ")
		return mark, cell
	}

	switch line.Type {
	case DiffAdded:
		cell = renderColoredLine(*line, colW, tokens, styleAdded, styleAddedPrefix)
	case DiffRemoved:
		cell = renderColoredLine(*line, colW, tokens, styleRemoved, styleRemovedPrefix)
	default:
		cell = renderContextLine(*line, colW, tokens)
	}
	return mark, cell
}
