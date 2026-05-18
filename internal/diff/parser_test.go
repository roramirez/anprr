package diff

import (
	"testing"
)

const sampleDiff = `diff --git a/auth/token.go b/auth/token.go
--- a/auth/token.go
+++ b/auth/token.go
@@ -23,7 +23,9 @@
 func ValidateToken(t *Token) error {
- if token == nil {
+ if token == nil || token.Expired() {
+   return ErrTokenExpired
 }
`

func TestParse_lineTypes(t *testing.T) {
	lines := Parse(sampleDiff)

	want := []struct {
		typ  DiffLineType
		text string
	}{
		{DiffFileHeader, "diff --git a/auth/token.go b/auth/token.go"},
		{DiffFileHeader, "--- a/auth/token.go"},
		{DiffFileHeader, "+++ b/auth/token.go"},
		{DiffHunkHeader, "@@ -23,7 +23,9 @@"},
		{DiffContext, " func ValidateToken(t *Token) error {"},
		{DiffRemoved, "- if token == nil {"},
		{DiffAdded, "+ if token == nil || token.Expired() {"},
		{DiffAdded, "+   return ErrTokenExpired"},
		{DiffContext, " }"},
	}

	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, w := range want {
		if lines[i].Type != w.typ {
			t.Errorf("line %d: type got %d want %d (text: %q)", i, lines[i].Type, w.typ, lines[i].Text)
		}
		if lines[i].Text != w.text {
			t.Errorf("line %d: text got %q want %q", i, lines[i].Text, w.text)
		}
	}
}

func TestParse_emptyDiff(t *testing.T) {
	lines := Parse("")
	if len(lines) != 0 {
		t.Errorf("expected empty, got %d lines", len(lines))
	}
}

func TestParse_langDetection(t *testing.T) {
	d := `diff --git a/main.go b/main.go
@@ -1,1 +1,1 @@
-old
+new
`
	lines := Parse(d)
	for _, l := range lines {
		if l.Type == DiffAdded || l.Type == DiffRemoved || l.Type == DiffHunkHeader {
			if l.Lang != "go" {
				t.Errorf("expected lang=go, got %q for line %q", l.Lang, l.Text)
			}
		}
	}
}

func TestParse_pathDetection(t *testing.T) {
	d := `diff --git a/auth/token.go b/auth/token.go
@@ -1,1 +1,1 @@
-old
+new
`
	lines := Parse(d)
	for _, l := range lines {
		if l.Type == DiffAdded || l.Type == DiffRemoved {
			if l.Path != "auth/token.go" {
				t.Errorf("expected path=auth/token.go, got %q", l.Path)
			}
		}
	}
}

func TestParse_lineNumbers(t *testing.T) {
	d := `diff --git a/foo.go b/foo.go
@@ -10,3 +10,4 @@
 ctx1
-removed
+added1
+added2
 ctx2
`
	lines := Parse(d)
	// find specific lines by type
	type check struct {
		typ     DiffLineType
		oldLine int
		newLine int
	}
	checks := []check{
		{DiffContext, 10, 10}, // ctx1
		{DiffRemoved, 11, 0},  // removed (old line 11)
		{DiffAdded, 0, 11},    // added1 (new line 11)
		{DiffAdded, 0, 12},    // added2 (new line 12)
		{DiffContext, 12, 13}, // ctx2
	}
	var commentable []DiffLine
	for _, l := range lines {
		if l.Commentable {
			commentable = append(commentable, l)
		}
	}
	if len(commentable) != len(checks) {
		t.Fatalf("expected %d commentable lines, got %d", len(checks), len(commentable))
	}
	for i, c := range checks {
		l := commentable[i]
		if l.Type != c.typ {
			t.Errorf("line %d: type got %d want %d", i, l.Type, c.typ)
		}
		if l.OldLine != c.oldLine {
			t.Errorf("line %d: oldLine got %d want %d", i, l.OldLine, c.oldLine)
		}
		if l.NewLine != c.newLine {
			t.Errorf("line %d: newLine got %d want %d", i, l.NewLine, c.newLine)
		}
	}
}

func TestParse_commentable(t *testing.T) {
	lines := Parse(sampleDiff)
	for _, l := range lines {
		switch l.Type {
		case DiffFileHeader, DiffHunkHeader:
			if l.Commentable {
				t.Errorf("file/hunk header should not be commentable: %q", l.Text)
			}
		case DiffAdded, DiffRemoved, DiffContext:
			if !l.Commentable {
				t.Errorf("code line should be commentable: %q", l.Text)
			}
		}
	}
}

func TestParse_multipleFiles(t *testing.T) {
	d := `diff --git a/foo.go b/foo.go
@@ -1 +1 @@
-old
diff --git a/bar.py b/bar.py
@@ -1 +1 @@
+new
`
	lines := Parse(d)
	var goCnt, pyCnt int
	for _, l := range lines {
		switch l.Lang {
		case "go":
			goCnt++
		case "py":
			pyCnt++
		}
	}
	if goCnt == 0 || pyCnt == 0 {
		t.Errorf("expected both go and py lines, go=%d py=%d", goCnt, pyCnt)
	}
}

func TestFormatPosition(t *testing.T) {
	cases := []struct {
		line DiffLine
		want string
	}{
		{DiffLine{Type: DiffAdded, Path: "auth/token.go", NewLine: 42}, "auth/token.go:42 (new)"},
		{DiffLine{Type: DiffRemoved, Path: "auth/token.go", OldLine: 10}, "auth/token.go:10 (old)"},
		{DiffLine{Type: DiffContext, Path: "main.go", NewLine: 5, OldLine: 5}, "main.go:5"},
		{DiffLine{Type: DiffFileHeader, Path: ""}, ""},
	}
	for _, c := range cases {
		got := FormatPosition(c.line)
		if got != c.want {
			t.Errorf("FormatPosition(%+v) = %q, want %q", c.line, got, c.want)
		}
	}
}

func TestCommentSideAndLine(t *testing.T) {
	added := DiffLine{Type: DiffAdded, NewLine: 42}
	removed := DiffLine{Type: DiffRemoved, OldLine: 10}
	ctx := DiffLine{Type: DiffContext, NewLine: 5, OldLine: 5}

	if CommentSide(added) != "RIGHT" {
		t.Error("added should be RIGHT")
	}
	if CommentLine(added) != 42 {
		t.Errorf("added line: got %d", CommentLine(added))
	}
	if CommentSide(removed) != "LEFT" {
		t.Error("removed should be LEFT")
	}
	if CommentLine(removed) != 10 {
		t.Errorf("removed line: got %d", CommentLine(removed))
	}
	if CommentSide(ctx) != "RIGHT" {
		t.Error("context should be RIGHT")
	}
	if CommentLine(ctx) != 5 {
		t.Errorf("context line: got %d", CommentLine(ctx))
	}
}
