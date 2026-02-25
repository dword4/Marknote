package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// updatePreview rebuilds the preview VBox from the given markdown content.
func (a *App) updatePreview(content string) {
	a.previewBox.Objects = renderMarkdown(content)
	a.previewBox.Refresh()
}

// renderMarkdown splits content into table and non-table blocks, renders each,
// and returns the combined slice of Fyne canvas objects.
func renderMarkdown(content string) []fyne.CanvasObject {
	if strings.TrimSpace(content) == "" {
		return nil
	}

	var objects []fyne.CanvasObject
	for _, block := range splitMarkdownBlocks(content) {
		if block.isTable {
			if obj := renderTableBlock(block.lines); obj != nil {
				objects = append(objects, obj)
			}
		} else {
			text := strings.TrimSpace(strings.Join(block.lines, "\n"))
			if text != "" {
				rt := widget.NewRichTextFromMarkdown(text)
				rt.Wrapping = fyne.TextWrapWord
				objects = append(objects, rt)
			}
		}
	}
	return objects
}

// mdBlock is a contiguous run of lines that are either all table or all non-table.
type mdBlock struct {
	isTable bool
	lines   []string
}

// splitMarkdownBlocks partitions the markdown source into alternating table and
// non-table blocks. Tables are identified by their GFM separator row (cells of
// dashes/colons separated by |). Code fences are tracked so that | inside a
// fenced block is never mistaken for a table separator.
func splitMarkdownBlocks(content string) []mdBlock {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Pass 1: track which lines are inside a code fence.
	inFence := make([]bool, len(lines))
	fenceActive := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenceActive = !fenceActive
		}
		inFence[i] = fenceActive
	}

	// Pass 2: find separator rows outside fences.
	separatorAt := make(map[int]bool)
	for i, line := range lines {
		if !inFence[i] && isSeparatorRow(line) {
			separatorAt[i] = true
		}
	}

	// Pass 3: mark table lines — the separator row plus adjacent non-blank lines
	// above (header) and below (data rows).
	isTableLine := make([]bool, len(lines))
	for sepIdx := range separatorAt {
		isTableLine[sepIdx] = true
		for i := sepIdx - 1; i >= 0; i-- {
			if strings.TrimSpace(lines[i]) == "" {
				break
			}
			isTableLine[i] = true
		}
		for i := sepIdx + 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "" {
				break
			}
			isTableLine[i] = true
		}
	}

	// Pass 4: group consecutive same-type lines into blocks.
	var blocks []mdBlock
	current := []string{lines[0]}
	currentIsTable := isTableLine[0]

	for i := 1; i < len(lines); i++ {
		if isTableLine[i] != currentIsTable {
			blocks = append(blocks, mdBlock{isTable: currentIsTable, lines: current})
			current = nil
			currentIsTable = isTableLine[i]
		}
		current = append(current, lines[i])
	}
	if len(current) > 0 {
		blocks = append(blocks, mdBlock{isTable: currentIsTable, lines: current})
	}
	return blocks
}

// isSeparatorRow returns true when the line is a GFM table separator such as
// |---|:---:|---:| or --- | --- (with or without surrounding pipes).
func isSeparatorRow(line string) bool {
	t := strings.TrimSpace(line)
	if !strings.Contains(t, "-") {
		return false
	}
	t = strings.Trim(t, "|")
	if t == "" {
		return false
	}
	for _, cell := range strings.Split(t, "|") {
		cell = strings.TrimSpace(cell)
		if cell == "" {
			return false // empty cell — not a valid separator
		}
		for _, ch := range cell {
			if ch != '-' && ch != ':' {
				return false
			}
		}
		if !strings.Contains(cell, "-") {
			return false // must have at least one dash
		}
	}
	return true
}

// renderTableBlock parses the raw table lines and returns a Fyne widget that
// renders the table with bold headers, a separator, and plain data rows.
func renderTableBlock(lines []string) fyne.CanvasObject {
	headers, dataRows, numCols := parseTable(lines)
	if numCols == 0 {
		return nil
	}

	var rows []fyne.CanvasObject

	// Header row — bold labels
	headerCells := make([]fyne.CanvasObject, numCols)
	for i := range headerCells {
		lbl := widget.NewLabel(cellAt(headers, i))
		lbl.TextStyle = fyne.TextStyle{Bold: true}
		lbl.Wrapping = fyne.TextWrapWord
		headerCells[i] = lbl
	}
	rows = append(rows, container.NewGridWithColumns(numCols, headerCells...))
	rows = append(rows, widget.NewSeparator())

	// Data rows
	for _, dataRow := range dataRows {
		cells := make([]fyne.CanvasObject, numCols)
		for i := range cells {
			lbl := widget.NewLabel(cellAt(dataRow, i))
			lbl.Wrapping = fyne.TextWrapWord
			cells[i] = lbl
		}
		rows = append(rows, container.NewGridWithColumns(numCols, cells...))
	}

	return container.NewPadded(container.NewVBox(rows...))
}

// parseTable splits the raw table lines into headers, data rows, and the
// maximum column count. The separator row is consumed but not stored.
func parseTable(lines []string) (headers []string, dataRows [][]string, numCols int) {
	separatorSeen := false
	for _, line := range lines {
		if !strings.Contains(line, "|") {
			continue
		}
		if isSeparatorRow(line) {
			separatorSeen = true
			continue
		}
		cells := parseTableRow(line)
		if !separatorSeen {
			headers = cells
		} else {
			dataRows = append(dataRows, cells)
		}
		if len(cells) > numCols {
			numCols = len(cells)
		}
	}
	if len(headers) > numCols {
		numCols = len(headers)
	}
	return
}

// parseTableRow splits a raw table row string (e.g. "| a | b | c |") into
// a slice of trimmed cell strings.
func parseTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// cellAt returns cells[i] or "" if i is out of range.
func cellAt(cells []string, i int) string {
	if i < len(cells) {
		return cells[i]
	}
	return ""
}
