package diff

import (
	"fmt"
	"strconv"
	"strings"
)

type DiffLineType int

const (
	DiffContext    DiffLineType = iota
	DiffAdded                   // lines starting with +
	DiffRemoved                 // lines starting with -
	DiffHunkHeader              // lines starting with @@
	DiffFileHeader              // lines starting with diff --git / --- / +++
)

type DiffLine struct {
	Type DiffLineType
	Text string // includes the +/- prefix character
	Lang string // detected from diff --git header
	Path string // file path (b-side), e.g. "auth/token.go"
	// Line numbers in the respective file versions.
	// 0 means "not applicable" (e.g. file headers, hunk headers).
	OldLine int // line number in the old file (relevant for DiffRemoved and DiffContext)
	NewLine int // line number in the new file (relevant for DiffAdded and DiffContext)
	// Commentable is true for lines that can receive inline review comments
	// (added, removed, context — not file/hunk headers).
	Commentable bool
}

// Parse converts a raw unified diff string into a slice of DiffLine.
func Parse(raw string) []DiffLine {
	lines := strings.Split(raw, "\n")
	result := make([]DiffLine, 0, len(lines))

	currentLang := ""
	currentPath := ""
	oldLine := 0
	newLine := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		dl := DiffLine{Lang: currentLang, Path: currentPath}

		switch {
		case strings.HasPrefix(line, "diff --git"):
			dl.Type = DiffFileHeader
			currentLang = detectLang(line)
			currentPath = detectPath(line)
			dl.Lang = currentLang
			dl.Path = currentPath
			oldLine = 0
			newLine = 0

		case strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ "):
			dl.Type = DiffFileHeader

		case strings.HasPrefix(line, "@@"):
			dl.Type = DiffHunkHeader
			o, n := parseHunkHeader(line)
			oldLine = o
			newLine = n

		case strings.HasPrefix(line, "+"):
			dl.Type = DiffAdded
			dl.NewLine = newLine
			dl.Commentable = true
			newLine++

		case strings.HasPrefix(line, "-"):
			dl.Type = DiffRemoved
			dl.OldLine = oldLine
			dl.Commentable = true
			oldLine++

		default:
			// context line
			dl.Type = DiffContext
			dl.OldLine = oldLine
			dl.NewLine = newLine
			dl.Commentable = true
			oldLine++
			newLine++
		}

		dl.Text = line
		result = append(result, dl)
	}
	return result
}

// parseHunkHeader extracts the starting line numbers from a hunk header like
// "@@ -23,7 +23,9 @@ func foo() {".
func parseHunkHeader(line string) (oldStart, newStart int) {
	// format: @@ -<old>[,<count>] +<new>[,<count>] @@
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return 1, 1
	}
	oldStart = parseHunkNum(parts[1]) // "-23,7"
	newStart = parseHunkNum(parts[2]) // "+23,9"
	return
}

func parseHunkNum(s string) int {
	// strip leading - or +
	s = strings.TrimLeft(s, "-+")
	// take only the number before the comma
	if idx := strings.Index(s, ","); idx >= 0 {
		s = s[:idx]
	}
	n, _ := strconv.Atoi(s)
	return n
}

// detectLang extracts a file extension from "diff --git a/path/file.ext b/path/file.ext".
func detectLang(line string) string {
	path := detectPath(line)
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx+1:]
	}
	return ""
}

// detectPath extracts the file path from "diff --git a/path b/path" (b-side).
func detectPath(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	p := parts[3] // "b/auth/token.go"
	p = strings.TrimPrefix(p, "b/")
	return p
}

// CommentSide returns the GitHub review comment side for a diff line:
// "RIGHT" for added/context (new file), "LEFT" for removed (old file).
func CommentSide(dl DiffLine) string {
	if dl.Type == DiffRemoved {
		return "LEFT"
	}
	return "RIGHT"
}

// CommentLine returns the file line number to use in a GitHub review comment.
func CommentLine(dl DiffLine) int {
	if dl.Type == DiffRemoved {
		return dl.OldLine
	}
	return dl.NewLine
}

// FormatPosition returns a human-readable position label for a diff line.
func FormatPosition(dl DiffLine) string {
	if dl.Path == "" {
		return ""
	}
	switch dl.Type {
	case DiffAdded:
		return fmt.Sprintf("%s:%d (new)", dl.Path, dl.NewLine)
	case DiffRemoved:
		return fmt.Sprintf("%s:%d (old)", dl.Path, dl.OldLine)
	default:
		return fmt.Sprintf("%s:%d", dl.Path, dl.NewLine)
	}
}
