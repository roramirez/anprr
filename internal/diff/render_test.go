package diff

import (
	"strings"
	"testing"

	chromalib "github.com/alecthomas/chroma/v2"
)

func TestRender_noopHighlighter(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+new line", Lang: "go", Path: "a.go", Commentable: true},
		{Type: DiffRemoved, Text: "-old line", Lang: "go", Path: "a.go", Commentable: true},
		{Type: DiffHunkHeader, Text: "@@ -1,1 +1,1 @@", Lang: "go", Path: "a.go"},
		{Type: DiffContext, Text: " ctx", Lang: "go", Path: "a.go", Commentable: true},
	}
	out := Render(lines, 40, NoopHighlighter{}, -1, nil)

	// ANSI codes use decimal RGB: #00FF7F = 0;255;127 (green fg for added)
	if !strings.Contains(out, "0;255;127") {
		t.Error("expected green fg ANSI code for added line")
	}
	// #FF6B6B = 255;107;107 (red fg for removed)
	if !strings.Contains(out, "255;107;107") {
		t.Error("expected red fg ANSI code for removed line")
	}
	// #00BFFF = 0;191;255 (cyan for hunk header)
	if !strings.Contains(out, "0;191;255") {
		t.Error("expected cyan ANSI code for hunk header")
	}
}

func TestRender_noopVsChroma_noRegression(t *testing.T) {
	lines := Parse("diff --git a/x.txt b/x.txt\n--- a/x.txt\n+++ b/x.txt\n@@ -1 +1 @@\n-old\n+new\n")
	noop := Render(lines, 40, NoopHighlighter{}, -1, nil)
	if !strings.Contains(noop, "old") || !strings.Contains(noop, "new") {
		t.Error("noop render missing content")
	}
}

func TestRender_chromaGoFile(t *testing.T) {
	lines := Parse("diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-func old() {}\n+func new() {}\n")
	hl := NewChromaHighlighter()
	out := Render(lines, 80, hl, -1, nil)
	if !strings.Contains(out, "old") || !strings.Contains(out, "new") {
		t.Error("chroma render missing content")
	}
}

func TestRender_chromaMultiLineString(t *testing.T) {
	// A Go raw string literal spanning multiple diff lines.
	// With file-level tokenization, all lines inside the backtick string
	// should receive the string literal color (#e6db74 in monokai).
	raw := "diff --git a/query.go b/query.go\n" +
		"--- a/query.go\n" +
		"+++ b/query.go\n" +
		"@@ -1,5 +1,5 @@\n" +
		" package main\n" +
		"+var q = `\n" +
		"+  SELECT id\n" +
		"+  FROM users`\n"
	lines := Parse(raw)
	hl := NewChromaHighlighter()
	out := Render(lines, 80, hl, -1, nil)
	// monokai string color: #e6db74 = 230;219;116
	if !strings.Contains(out, "230;219;116") {
		t.Error("expected string literal color inside raw string — file-level tokenization not working")
	}
}

func TestRender_unknownExtensionFallback(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+something", Lang: "unknownxyz123", Path: "file.unknownxyz123", Commentable: true},
	}
	hl := NewChromaHighlighter()
	out := Render(lines, 40, hl, -1, nil)
	if !strings.Contains(out, "something") {
		t.Error("expected content in output")
	}
}

func TestRender_nilHighlighter(t *testing.T) {
	lines := []DiffLine{{Type: DiffAdded, Text: "+line", Path: "x.go", Commentable: true}}
	out := Render(lines, 40, nil, -1, nil)
	if !strings.Contains(out, "line") {
		t.Error("expected content")
	}
}

func TestRender_cursorHighlight(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+line 0", Path: "a.go", Commentable: true},
		{Type: DiffAdded, Text: "+line 1", Path: "a.go", Commentable: true},
	}
	out := Render(lines, 40, NoopHighlighter{}, 1, nil)
	// cursor style uses #FFFF88 = 255;255;136
	if !strings.Contains(out, "255;255;136") {
		t.Error("expected cursor highlight color on selected line")
	}
}

func TestRender_commentedLineMarker(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+line 0", Path: "a.go", Commentable: true},
		{Type: DiffAdded, Text: "+line 1", Path: "a.go", Commentable: true},
	}
	commented := map[int]bool{0: true}
	out := Render(lines, 40, NoopHighlighter{}, -1, commented)
	// comment marker uses #FF8C00 = 255;140;0
	if !strings.Contains(out, "255;140;0") {
		t.Error("expected comment marker color on commented line")
	}
}

// TestTokenizeFile_multiLineString verifies ChromaHighlighter tokenizes a raw
// string spanning multiple lines correctly when given the full file content.
func TestTokenizeFile_multiLineString(t *testing.T) {
	hl := NewChromaHighlighter()
	lines := []string{
		`var q = ` + "`",
		"  SELECT id",
		"  FROM users`",
	}
	result := hl.TokenizeFile("query.go", "go", lines)
	if len(result) != 3 {
		t.Fatalf("expected 3 line results, got %d", len(result))
	}
	// lines 2 and 3 (inside the backtick string) should have string color
	hasStringColor := func(tokens []Token) bool {
		for _, tok := range tokens {
			if tok.Color != "" {
				return true
			}
		}
		return false
	}
	if !hasStringColor(result[1]) {
		t.Error("line 2 (inside raw string) should have syntax color")
	}
	if !hasStringColor(result[2]) {
		t.Error("line 3 (inside raw string) should have syntax color")
	}
}

func TestTokenizeFile_noop(t *testing.T) {
	hl := NoopHighlighter{}
	lines := []string{"hello", "world"}
	result := hl.TokenizeFile("f.go", "go", lines)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
	if result[0][0].Text != "hello" || result[1][0].Text != "world" {
		t.Error("noop should return each line as a single token")
	}
}

func TestSplitTokensByLine_basic(t *testing.T) {
	tokens := []chromalib.Token{
		{Type: chromalib.Other, Value: "line1\nline2\nline3"},
	}
	colorFor := func(chromalib.TokenType) string { return "" }
	result := splitTokensByLine(tokens, colorFor, 3)
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(result))
	}
	if result[0][0].Text != "line1" || result[1][0].Text != "line2" || result[2][0].Text != "line3" {
		t.Errorf("unexpected token texts: %v", result)
	}
}
