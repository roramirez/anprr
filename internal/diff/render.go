package diff

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Force true-color output so ANSI codes are always emitted (including in tests and pipes).
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// Token is a text segment with an optional foreground color (hex string or empty).
type Token struct {
	Text  string
	Color string // hex e.g. "#FF6B6B", empty = use default
}

// Highlighter tokenizes a source line into colored segments.
type Highlighter interface {
	Tokenize(lang, line string) []Token
}

// NoopHighlighter returns the whole line as a single token with no color.
type NoopHighlighter struct{}

func (NoopHighlighter) Tokenize(_, line string) []Token {
	return []Token{{Text: line}}
}

// ChromaHighlighter uses chroma for per-language syntax highlighting.
type ChromaHighlighter struct {
	style *chroma.Style
}

func NewChromaHighlighter() *ChromaHighlighter {
	return &ChromaHighlighter{style: styles.Get("monokai")}
}

func (h *ChromaHighlighter) Tokenize(lang, line string) []Token {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// strip leading +/- from the line for tokenization, restore after
	prefix := ""
	src := line
	if len(line) > 0 && (line[0] == '+' || line[0] == '-' || line[0] == ' ') {
		prefix = string(line[0])
		src = line[1:]
	}

	iter, err := lexer.Tokenise(nil, src)
	if err != nil {
		return []Token{{Text: line}}
	}

	var result []Token
	if prefix != "" {
		result = append(result, Token{Text: prefix})
	}
	for _, t := range iter.Tokens() {
		color := ""
		if entry := h.style.Get(t.Type); entry.Colour.IsSet() {
			color = entry.Colour.String() // already includes leading #
		}
		result = append(result, Token{Text: t.Value, Color: color})
	}
	return result
}

// lipgloss styles for each diff line type
var (
	styleAdded = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF7F")).
			Background(lipgloss.Color("#003D00"))

	styleRemoved = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#3D0000"))

	styleHunkHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true)

	styleFileHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	styleContext = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Brighter prefix gutter colors
	styleAddedPrefix   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF7F")).Background(lipgloss.Color("#003D00")).Bold(true)
	styleRemovedPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Background(lipgloss.Color("#3D0000")).Bold(true)

	// Cursor highlight — applied on top of the line's own color
	styleCursor = lipgloss.NewStyle().
			Background(lipgloss.Color("#4A4A00")).
			Foreground(lipgloss.Color("#FFFF88")).
			Bold(true)

	// Pending comment marker shown in the gutter
	styleCommentMark = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF8C00")).
				Bold(true)
)

// Render converts parsed diff lines into a single ANSI-colored string.
// cursor is the index of the line to highlight (-1 = none).
// commented is the set of line indices that have pending inline comments.
// width is used to pad lines so backgrounds extend to the full terminal width.
func Render(lines []DiffLine, width int, hl Highlighter, cursor int, commented map[int]bool) string {
	if hl == nil {
		hl = NoopHighlighter{}
	}
	if commented == nil {
		commented = map[int]bool{}
	}
	var sb strings.Builder
	for i, line := range lines {
		rendered := renderLine(line, width, hl)
		// comment marker in gutter (leftmost 2 chars)
		mark := "  "
		if commented[i] {
			mark = styleCommentMark.Render("● ")
		}
		// cursor highlight overrides normal styling for commentable lines
		if i == cursor && line.Commentable {
			rendered = styleCursor.Render(padRight(stripANSI(rendered), width))
			mark = styleCursor.Render("> ")
		}
		sb.WriteString(mark + rendered)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// stripANSI removes ANSI escape codes so we can re-style a pre-rendered line.
func stripANSI(s string) string {
	var out strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func renderLine(line DiffLine, width int, hl Highlighter) string {
	switch line.Type {
	case DiffHunkHeader:
		return styleHunkHeader.Render(padRight(line.Text, width))
	case DiffFileHeader:
		return styleFileHeader.Render(padRight(line.Text, width))
	case DiffAdded:
		return renderColoredLine(line, width, hl, styleAdded, styleAddedPrefix)
	case DiffRemoved:
		return renderColoredLine(line, width, hl, styleRemoved, styleRemovedPrefix)
	default:
		tokens := hl.Tokenize(line.Lang, line.Text)
		return styleContext.Render(padRight(joinTokensNoColor(tokens), width))
	}
}

func renderColoredLine(line DiffLine, width int, hl Highlighter, base, prefixStyle lipgloss.Style) string {
	tokens := hl.Tokenize(line.Lang, line.Text)
	if len(tokens) == 0 {
		return base.Render(padRight(line.Text, width))
	}

	var sb strings.Builder
	for i, tok := range tokens {
		if i == 0 && len(tok.Text) == 1 && (tok.Text == "+" || tok.Text == "-") {
			sb.WriteString(prefixStyle.Render(tok.Text))
			continue
		}
		if tok.Color != "" {
			sb.WriteString(base.Copy().Foreground(lipgloss.Color(tok.Color)).Render(tok.Text))
		} else {
			sb.WriteString(base.Render(tok.Text))
		}
	}
	// pad to width with background
	text := sb.String()
	visibleLen := visibleLength(joinTokensNoColor(tokens))
	if visibleLen < width {
		sb.WriteString(base.Render(strings.Repeat(" ", width-visibleLen)))
	}
	_ = text
	return sb.String()
}

func joinTokensNoColor(tokens []Token) string {
	var sb strings.Builder
	for _, t := range tokens {
		sb.WriteString(t.Text)
	}
	return sb.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// visibleLength returns the rune count of a string (approximation for padding).
func visibleLength(s string) int {
	return len([]rune(s))
}
