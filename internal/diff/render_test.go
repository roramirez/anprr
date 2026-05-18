package diff

import (
	"strings"
	"testing"
)

func TestRender_noopHighlighter(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+new line", Lang: "go"},
		{Type: DiffRemoved, Text: "-old line", Lang: "go"},
		{Type: DiffHunkHeader, Text: "@@ -1,1 +1,1 @@", Lang: "go"},
		{Type: DiffContext, Text: " ctx", Lang: "go"},
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

func TestRender_unknownExtensionFallback(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+something", Lang: "unknownxyz123"},
	}
	hl := NewChromaHighlighter()
	out := Render(lines, 40, hl, -1, nil)
	if !strings.Contains(out, "something") {
		t.Error("expected content in output")
	}
}

func TestRender_nilHighlighter(t *testing.T) {
	lines := []DiffLine{{Type: DiffAdded, Text: "+line"}}
	out := Render(lines, 40, nil, -1, nil)
	if !strings.Contains(out, "line") {
		t.Error("expected content")
	}
}

func TestRender_cursorHighlight(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+line 0", Commentable: true},
		{Type: DiffAdded, Text: "+line 1", Commentable: true},
	}
	// cursor on line 1
	out := Render(lines, 40, NoopHighlighter{}, 1, nil)
	// cursor style uses #FFFF88 = 255;255;136
	if !strings.Contains(out, "255;255;136") {
		t.Error("expected cursor highlight color on selected line")
	}
}

func TestRender_commentedLineMarker(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+line 0", Commentable: true},
		{Type: DiffAdded, Text: "+line 1", Commentable: true},
	}
	commented := map[int]bool{0: true}
	out := Render(lines, 40, NoopHighlighter{}, -1, commented)
	// comment marker uses #FF8C00 = 255;140;0
	if !strings.Contains(out, "255;140;0") {
		t.Error("expected comment marker color on commented line")
	}
}
