package diff

import (
	"strings"
	"testing"
)

func TestSplit_contextLine(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffContext, Text: " ctx", Commentable: true, NewLine: 1, OldLine: 1},
	}
	rows := Split(lines)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Left == nil || rows[0].Right == nil {
		t.Error("context line should appear on both sides")
	}
	if rows[0].LeftIdx != 0 || rows[0].RightIdx != 0 {
		t.Errorf("indices should both be 0: left=%d right=%d", rows[0].LeftIdx, rows[0].RightIdx)
	}
}

func TestSplit_header(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffFileHeader, Text: "diff --git a/x b/x"},
		{Type: DiffHunkHeader, Text: "@@ -1 +1 @@"},
	}
	rows := Split(lines)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if !row.IsHeader {
			t.Error("expected header row")
		}
	}
}

func TestSplit_equalRemovesAndAdds(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffRemoved, Text: "-old1", Commentable: true, OldLine: 1},
		{Type: DiffRemoved, Text: "-old2", Commentable: true, OldLine: 2},
		{Type: DiffAdded, Text: "+new1", Commentable: true, NewLine: 1},
		{Type: DiffAdded, Text: "+new2", Commentable: true, NewLine: 2},
	}
	rows := Split(lines)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (paired), got %d", len(rows))
	}
	if rows[0].Left == nil || rows[0].Right == nil {
		t.Error("row 0: both sides should be filled")
	}
	if rows[0].Left.Text != "-old1" || rows[0].Right.Text != "+new1" {
		t.Errorf("row 0 mismatch: left=%q right=%q", rows[0].Left.Text, rows[0].Right.Text)
	}
}

func TestSplit_moreAddsThanRemoves(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffRemoved, Text: "-old", Commentable: true, OldLine: 1},
		{Type: DiffAdded, Text: "+new1", Commentable: true, NewLine: 1},
		{Type: DiffAdded, Text: "+new2", Commentable: true, NewLine: 2},
		{Type: DiffAdded, Text: "+new3", Commentable: true, NewLine: 3},
	}
	rows := Split(lines)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	// first row: paired
	if rows[0].Left == nil || rows[0].Right == nil {
		t.Error("row 0 should be fully paired")
	}
	// remaining rows: left empty
	if rows[1].Left != nil {
		t.Error("row 1 left should be empty")
	}
	if rows[1].LeftIdx != -1 {
		t.Errorf("row 1 leftIdx should be -1, got %d", rows[1].LeftIdx)
	}
}

func TestSplit_moreRemovesThanAdds(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffRemoved, Text: "-old1", Commentable: true, OldLine: 1},
		{Type: DiffRemoved, Text: "-old2", Commentable: true, OldLine: 2},
		{Type: DiffAdded, Text: "+new", Commentable: true, NewLine: 1},
	}
	rows := Split(lines)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[1].Right != nil {
		t.Error("row 1 right should be empty")
	}
	if rows[1].RightIdx != -1 {
		t.Errorf("row 1 rightIdx should be -1, got %d", rows[1].RightIdx)
	}
}

func TestSplit_fullDiff(t *testing.T) {
	lines := Parse("diff --git a/x.go b/x.go\n@@ -1,3 +1,4 @@\n ctx1\n-old\n+new1\n+new2\n ctx2\n")
	rows := Split(lines)

	var headerCount, contentCount int
	for _, r := range rows {
		if r.IsHeader {
			headerCount++
		} else {
			contentCount++
		}
	}
	if headerCount != 2 { // file header + hunk header
		t.Errorf("expected 2 headers, got %d", headerCount)
	}
	// ctx1 + change block (2 rows: old+new1, nil+new2) + ctx2 = 4 content rows
	if contentCount != 4 {
		t.Errorf("expected 4 content rows, got %d", contentCount)
	}
}

func TestRenderSplit_containsBothSides(t *testing.T) {
	lines := Parse("diff --git a/x.go b/x.go\n@@ -1,2 +1,2 @@\n-old line\n+new line\n ctx\n")
	out := RenderSplit(lines, 120, NoopHighlighter{}, -1, nil)

	if !strings.Contains(out, "old line") {
		t.Error("expected old line on left side")
	}
	if !strings.Contains(out, "new line") {
		t.Error("expected new line on right side")
	}
	if !strings.Contains(out, "│") {
		t.Error("expected separator │ in output")
	}
}

func TestRenderSplit_emptySlotFiller(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffAdded, Text: "+only add", Commentable: true, NewLine: 1},
	}
	out := RenderSplit(lines, 80, NoopHighlighter{}, -1, nil)
	// empty left side should contain filler characters
	if !strings.Contains(out, "░") {
		t.Error("expected empty slot filler ░ on left side")
	}
}

func TestRenderSplit_cursorHighlight(t *testing.T) {
	lines := Parse("diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-removed\n+added\n")
	// find index of the added line
	addedIdx := -1
	for i, l := range lines {
		if l.Type == DiffAdded {
			addedIdx = i
			break
		}
	}
	if addedIdx < 0 {
		t.Fatal("no added line found")
	}
	out := RenderSplit(lines, 120, NoopHighlighter{}, addedIdx, nil)
	// cursor style uses #FFFF88 = 255;255;136
	if !strings.Contains(out, "255;255;136") {
		t.Error("expected cursor color in split output")
	}
}
