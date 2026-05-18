package diff

// SplitRow represents one rendered row in a side-by-side diff view.
type SplitRow struct {
	// Header rows (file headers, hunk headers) span both columns.
	IsHeader bool
	Header   *DiffLine

	// Content rows have a left side (old file) and/or right side (new file).
	// nil means the slot is empty (no matching line on that side).
	Left    *DiffLine
	Right   *DiffLine
	LeftIdx  int // index into source []DiffLine (-1 = empty slot)
	RightIdx int // index into source []DiffLine (-1 = empty slot)
}

// Split converts a flat slice of DiffLine into SplitRows for side-by-side rendering.
//
// Change blocks (consecutive removes followed by adds) are zipped into paired rows.
// Context lines appear on both sides. File/hunk headers span the full width.
func Split(lines []DiffLine) []SplitRow {
	var rows []SplitRow
	i := 0

	for i < len(lines) {
		line := lines[i]

		switch line.Type {
		case DiffFileHeader, DiffHunkHeader:
			rows = append(rows, SplitRow{IsHeader: true, Header: &lines[i], LeftIdx: -1, RightIdx: -1})
			i++

		case DiffContext:
			rows = append(rows, SplitRow{
				Left:     &lines[i],
				Right:    &lines[i],
				LeftIdx:  i,
				RightIdx: i,
			})
			i++

		case DiffRemoved, DiffAdded:
			// collect the full change block: all removes then all adds
			removes, removeIdxs := collectType(lines, i, DiffRemoved)
			i += len(removes)
			adds, addIdxs := collectType(lines, i, DiffAdded)
			i += len(adds)

			// zip removes and adds into paired rows
			maxLen := len(removes)
			if len(adds) > maxLen {
				maxLen = len(adds)
			}
			for j := 0; j < maxLen; j++ {
				row := SplitRow{LeftIdx: -1, RightIdx: -1}
				if j < len(removes) {
					row.Left = &lines[removeIdxs[j]]
					row.LeftIdx = removeIdxs[j]
				}
				if j < len(adds) {
					row.Right = &lines[addIdxs[j]]
					row.RightIdx = addIdxs[j]
				}
				rows = append(rows, row)
			}
		}
	}
	return rows
}

// collectType collects consecutive lines of the given type starting at idx,
// returning the lines and their indices in the source slice.
func collectType(lines []DiffLine, start int, t DiffLineType) ([]DiffLine, []int) {
	var result []DiffLine
	var idxs []int
	for i := start; i < len(lines) && lines[i].Type == t; i++ {
		result = append(result, lines[i])
		idxs = append(idxs, i)
	}
	return result, idxs
}
