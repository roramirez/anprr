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

// Highlighter tokenizes source lines into colored segments.
// TokenizeFile receives all lines of a single file at once (stripped of +/-/space diff prefixes),
// enabling multi-line constructs (raw strings, block comments) to be recognized correctly.
// It returns one []Token per input line: len(result) == len(lines).
type Highlighter interface {
	TokenizeFile(path, lang string, lines []string) [][]Token
}

// NoopHighlighter returns each line as a single token with no color.
type NoopHighlighter struct{}

func (NoopHighlighter) TokenizeFile(_, _ string, lines []string) [][]Token {
	result := make([][]Token, len(lines))
	for i, line := range lines {
		result[i] = []Token{{Text: line}}
	}
	return result
}

// ChromaHighlighter uses chroma for per-language syntax highlighting.
// It tokenizes all lines of a file as a single string so multi-line constructs
// (raw string literals, block comments, heredocs) are colored correctly.
type ChromaHighlighter struct {
	style *chroma.Style
}

func NewChromaHighlighter() *ChromaHighlighter {
	return &ChromaHighlighter{style: styles.Get("monokai")}
}

func (h *ChromaHighlighter) TokenizeFile(path, lang string, lines []string) [][]Token {
	lexer := lexers.Get(lang)
	if lexer == nil {
		// try matching by filename
		lexer = lexers.Match(path)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	// join all lines into a single string for full-file tokenization
	full := strings.Join(lines, "\n")
	iter, err := lexer.Tokenise(nil, full)
	if err != nil {
		// fallback: return each line as-is
		result := make([][]Token, len(lines))
		for i, l := range lines {
			result[i] = []Token{{Text: l}}
		}
		return result
	}

	colorFor := func(t chroma.TokenType) string {
		if entry := h.style.Get(t); entry.Colour.IsSet() {
			return entry.Colour.String()
		}
		return ""
	}

	return splitTokensByLine(iter.Tokens(), colorFor, len(lines))
}

// splitTokensByLine distributes flat chroma tokens back into per-line slices.
// Tokens whose text contains newlines are split at each newline boundary.
func splitTokensByLine(rawTokens []chroma.Token, colorFor func(chroma.TokenType) string, numLines int) [][]Token {
	result := make([][]Token, 0, numLines)
	var current []Token
	for _, t := range rawTokens {
		current, result = splitAtNewlines(t.Value, colorFor(t.Type), current, result)
	}
	if len(current) > 0 {
		result = append(result, current)
	}
	for len(result) < numLines {
		result = append(result, []Token{})
	}
	return result[:numLines]
}

// splitAtNewlines appends segments of text (split at newlines) to current and result.
func splitAtNewlines(text, color string, current []Token, result [][]Token) ([]Token, [][]Token) {
	for {
		nl := strings.Index(text, "\n")
		if nl < 0 {
			if text != "" {
				current = append(current, Token{Text: text, Color: color})
			}
			return current, result
		}
		if nl > 0 {
			current = append(current, Token{Text: text[:nl], Color: color})
		}
		result = append(result, current)
		current = nil
		text = text[nl+1:]
	}
}

const (
	colorAddedFg   = "#00FF7F"
	colorAddedBg   = "#003D00"
	colorRemovedBg = "#3D0000"
)

// lipgloss styles for each diff line type
var (
	styleAdded = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorAddedFg)).
			Background(lipgloss.Color(colorAddedBg))

	styleRemoved = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color(colorRemovedBg))

	styleHunkHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF")).
			Bold(true)

	styleFileHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	styleContext = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	// Brighter prefix gutter colors
	styleAddedPrefix   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorAddedFg)).Background(lipgloss.Color(colorAddedBg)).Bold(true)
	styleRemovedPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Background(lipgloss.Color(colorRemovedBg)).Bold(true)

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

type fileGroup struct {
	path string
	lang string
	idxs []int
}

// preTokenize groups diff lines by file path and calls TokenizeFile once per file,
// returning a map of line-index → pre-computed tokens.
func preTokenize(lines []DiffLine, hl Highlighter) map[int][]Token {
	result := make(map[int][]Token, len(lines))
	for _, g := range groupByPath(lines) {
		applyGroupTokens(lines, g, hl, result)
	}
	return result
}

// groupByPath groups diff line indices by their file path, preserving order.
func groupByPath(lines []DiffLine) []fileGroup {
	seen := map[string]int{}
	var groups []fileGroup
	for i, dl := range lines {
		if dl.Path == "" {
			continue
		}
		gi, ok := seen[dl.Path]
		if !ok {
			gi = len(groups)
			seen[dl.Path] = gi
			groups = append(groups, fileGroup{path: dl.Path, lang: dl.Lang})
		}
		groups[gi].idxs = append(groups[gi].idxs, i)
	}
	return groups
}

// applyGroupTokens tokenizes one file group and stores results into the map.
func applyGroupTokens(lines []DiffLine, g fileGroup, hl Highlighter, result map[int][]Token) {
	rawLines := make([]string, len(g.idxs))
	for j, idx := range g.idxs {
		rawLines[j] = stripDiffPrefix(lines[idx].Text)
	}
	tokensByLine := hl.TokenizeFile(g.path, g.lang, rawLines)
	for j, idx := range g.idxs {
		if j < len(tokensByLine) {
			result[idx] = tokensByLine[j]
		} else {
			result[idx] = []Token{}
		}
	}
}

// stripDiffPrefix removes the leading +, -, or space from a diff line.
func stripDiffPrefix(line string) string {
	if len(line) > 0 && (line[0] == '+' || line[0] == '-' || line[0] == ' ') {
		return line[1:]
	}
	return line
}

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

	fileTokens := preTokenize(lines, hl)

	var sb strings.Builder
	for i, line := range lines {
		tokens := fileTokens[i] // nil for headers/lines without a path
		rendered := renderLineWithTokens(line, width, tokens)

		mark := "  "
		if commented[i] {
			mark = styleCommentMark.Render("● ")
		}
		if i == cursor && line.Commentable {
			rendered = styleCursor.Render(padRight(stripANSI(rendered), width))
			mark = styleCursor.Render("> ")
		}
		sb.WriteString(mark + rendered)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// renderLineWithTokens renders a single diff line using pre-computed tokens.
// tokens may be nil for lines without a path (headers).
func renderLineWithTokens(line DiffLine, width int, tokens []Token) string {
	switch line.Type {
	case DiffHunkHeader:
		return styleHunkHeader.Render(padRight(line.Text, width))
	case DiffFileHeader:
		return styleFileHeader.Render(padRight(line.Text, width))
	case DiffAdded:
		return renderColoredLine(line, width, tokens, styleAdded, styleAddedPrefix)
	case DiffRemoved:
		return renderColoredLine(line, width, tokens, styleRemoved, styleRemovedPrefix)
	default:
		// context line — apply syntax colors on gray background
		return renderContextLine(line, width, tokens)
	}
}

// appendTokens writes each token to sb, applying its color on top of base style.
func appendTokens(sb *strings.Builder, tokens []Token, base lipgloss.Style) {
	for _, tok := range tokens {
		if tok.Color != "" {
			sb.WriteString(base.Foreground(lipgloss.Color(tok.Color)).Render(tok.Text))
		} else {
			sb.WriteString(base.Render(tok.Text))
		}
	}
}

func renderContextLine(line DiffLine, width int, tokens []Token) string {
	if len(tokens) == 0 {
		return styleContext.Render(padRight(line.Text, width))
	}
	var sb strings.Builder
	appendTokens(&sb, tokens, styleContext)
	visibleLen := visibleLength(joinTokensNoColor(tokens))
	if visibleLen < width {
		sb.WriteString(styleContext.Render(strings.Repeat(" ", width-visibleLen)))
	}
	return sb.String()
}

func renderColoredLine(line DiffLine, width int, tokens []Token, base, prefixStyle lipgloss.Style) string {
	if len(tokens) == 0 {
		return base.Render(padRight(line.Text, width))
	}
	var sb strings.Builder
	sb.WriteString(prefixStyle.Render(line.Text[:1]))
	appendTokens(&sb, tokens, base)
	visibleLen := 1 + visibleLength(joinTokensNoColor(tokens)) // +1 for the prefix char
	if visibleLen < width {
		sb.WriteString(base.Render(strings.Repeat(" ", width-visibleLen)))
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
