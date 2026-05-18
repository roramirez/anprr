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

	rows := Split(lines)

	// column widths: each side gets equal space, separator takes 3 chars (" │ ")
	colW := (width - 5) / 2 // -5: 2 for gutter marks + 3 for separator
	if colW < 10 {
		colW = 10
	}

	var sb strings.Builder
	for _, row := range rows {
		if row.IsHeader {
			// header spans full width
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
		leftMark, leftCell := renderSplitCell(row.Left, row.LeftIdx, colW, hl, cursor, commented,
			styleRemoved, styleRemovedPrefix)

		// right side
		rightMark, rightCell := renderSplitCell(row.Right, row.RightIdx, colW, hl, cursor, commented,
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
	hl Highlighter,
	cursor int,
	commented map[int]bool,
	base, prefixSt lipgloss.Style,
) (mark, cell string) {
	if line == nil {
		// empty slot
		return "  ", styleEmpty.Render(strings.Repeat("░", colW))
	}

	// gutter mark
	mark = "  "
	if commented[idx] {
		mark = styleCommentMark.Render("● ")
	}

	// cursor highlight
	if idx == cursor && line.Commentable {
		text := padRight(stripANSI(renderLine(*line, colW, hl)), colW)
		cell = styleCursor.Render(text)
		mark = styleCursor.Render("> ")
		return mark, cell
	}

	// normal render
	switch line.Type {
	case DiffAdded:
		cell = renderColoredLine(*line, colW, hl, styleAdded, styleAddedPrefix)
	case DiffRemoved:
		cell = renderColoredLine(*line, colW, hl, styleRemoved, styleRemovedPrefix)
	default:
		// context line — use the appropriate base style based on which side we're on
		tokens := hl.Tokenize(line.Lang, line.Text)
		cell = styleContext.Render(padRight(joinTokensNoColor(tokens), colW))
	}
	return mark, cell
}
