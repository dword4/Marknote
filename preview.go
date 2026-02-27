package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	gmtext "github.com/yuin/goldmark/text"
)

// renderedBlock is a drawable block-level element of the preview pane.
type renderedBlock interface {
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
}

// ---------------------------------------------------------------------------
// Block types
// ---------------------------------------------------------------------------

type headingBlock struct {
	level int
	body  string
}

type paragraphBlock struct {
	body string
}

type codeBlock struct {
	code string
}

type hrBlock struct{}

type tableBlock struct {
	headers []string
	rows    [][]string
	numCols int
}

type listGroupBlock struct {
	items []listItemBlock
}

type listItemBlock struct {
	indent int
	bullet string
	body   string
}

type blockquoteBlock struct {
	body string
}

// ---------------------------------------------------------------------------
// Parser (package-level so it's allocated once)
// ---------------------------------------------------------------------------

var mdParser = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
	),
)

// renderMarkdown parses markdown and returns a slice of renderedBlocks.
func renderMarkdown(content string) []renderedBlock {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	src := []byte(content)
	reader := gmtext.NewReader(src)
	doc := mdParser.Parser().Parse(reader)

	var blocks []renderedBlock
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if b := nodeToBlock(n, src, 0); b != nil {
			blocks = append(blocks, b)
		}
	}
	return blocks
}

func nodeToBlock(n ast.Node, src []byte, listDepth int) renderedBlock {
	switch n := n.(type) {
	case *ast.Heading:
		return &headingBlock{level: n.Level, body: extractText(n, src)}

	case *ast.Paragraph:
		return &paragraphBlock{body: extractText(n, src)}

	case *ast.FencedCodeBlock:
		return &codeBlock{code: extractCodeLines(n, src)}

	case *ast.CodeBlock:
		return &codeBlock{code: extractCodeLines(n, src)}

	case *ast.ThematicBreak:
		return &hrBlock{}

	case *ast.Blockquote:
		return &blockquoteBlock{body: extractText(n, src)}

	case *ast.List:
		var items []listItemBlock
		counter := 1
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			li, ok := child.(*ast.ListItem)
			if !ok {
				continue
			}
			bullet := "• "
			if n.IsOrdered() {
				bullet = fmt.Sprintf("%d. ", counter)
				counter++
			}
			items = append(items, listItemBlock{
				indent: listDepth,
				bullet: bullet,
				body:   extractText(li, src),
			})
		}
		return &listGroupBlock{items: items}

	case *extast.Table:
		return buildTableBlock(n, src)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Text extraction
// ---------------------------------------------------------------------------

func extractText(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch tc := c.(type) {
		case *ast.Text:
			b.Write(tc.Segment.Value(src))
			if tc.HardLineBreak() || tc.SoftLineBreak() {
				b.WriteByte('\n')
			}
		case *ast.String:
			b.Write(tc.Value)
		case *ast.RawHTML:
			// skip
		default:
			b.WriteString(extractText(c, src))
		}
	}
	return strings.TrimSpace(b.String())
}

func extractCodeLines(n ast.Node, src []byte) string {
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		line := n.Lines().At(i)
		b.Write(line.Value(src))
	}
	return strings.TrimRight(b.String(), "\n")
}

// ---------------------------------------------------------------------------
// Table
// ---------------------------------------------------------------------------

func buildTableBlock(n *extast.Table, src []byte) *tableBlock {
	tb := &tableBlock{}
	for row := n.FirstChild(); row != nil; row = row.NextSibling() {
		var cells []string
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, extractText(cell, src))
		}
		if len(cells) > tb.numCols {
			tb.numCols = len(cells)
		}
		if _, ok := row.(*extast.TableHeader); ok {
			tb.headers = cells
		} else {
			tb.rows = append(tb.rows, cells)
		}
	}
	return tb
}

// ---------------------------------------------------------------------------
// Layout implementations
// ---------------------------------------------------------------------------

var headingSizes = [7]unit.Sp{0, 22, 19, 16, 15, 14, 13}

func (b *headingBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	lvl := b.level
	if lvl < 1 {
		lvl = 1
	}
	if lvl > 6 {
		lvl = 6
	}
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Label(th, headingSizes[lvl], b.body)
		lbl.Font = font.Font{Weight: font.Bold}
		return lbl.Layout(gtx)
	})
}

func (b *paragraphBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	lbl := material.Label(th, unit.Sp(14), b.body)
	lbl.MaxLines = 0
	return lbl.Layout(gtx)
}

func (b *codeBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return withBackground(gtx, darkenColor(th.Palette.Bg, 18), unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(th, unit.Sp(12), b.code)
			lbl.MaxLines = 0
			lbl.Font = font.Font{Typeface: "Go Mono"}
			return lbl.Layout(gtx)
		})
	})
}

func (b *hrBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
		paint.FillShape(gtx.Ops, mulAlpha(th.Palette.Fg, 80), clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	})
}

func (b *listGroupBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	items := b.items
	var children []layout.FlexChild
	for i := range items {
		it := &items[i]
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return it.Layout(gtx, th)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (b *listItemBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	indent := unit.Dp(float32(b.indent*16 + 8))
	return layout.Inset{Left: indent}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Label(th, unit.Sp(14), b.bullet).Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, unit.Sp(14), b.body)
				lbl.MaxLines = 0
				return lbl.Layout(gtx)
			}),
		)
	})
}

func (b *blockquoteBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Dp(4), 1)
			paint.FillShape(gtx.Ops, mulAlpha(th.Palette.ContrastBg, 200),
				clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: image.Pt(gtx.Dp(12), size.Y)}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Label(th, unit.Sp(13), b.body)
			lbl.Color = mulAlpha(th.Palette.Fg, 180)
			lbl.MaxLines = 0
			return lbl.Layout(gtx)
		}),
	)
}

func (b *tableBlock) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if b.numCols == 0 {
			return layout.Dimensions{}
		}
		colW := gtx.Constraints.Max.X / b.numCols

		// Capture header row height using op.Record, so we can draw a separator.
		var rows []layout.FlexChild

		headerCells := b.headers
		numCols := b.numCols
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return tableRow(gtx, th, headerCells, numCols, colW, true)
		}))
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
			paint.FillShape(gtx.Ops, mulAlpha(th.Palette.Fg, 100), clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: image.Pt(size.X, gtx.Dp(4))}
		}))
		for _, dr := range b.rows {
			cells := dr
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return tableRow(gtx, th, cells, numCols, colW, false)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

func tableRow(gtx layout.Context, th *material.Theme, cells []string, numCols, colW int, header bool) layout.Dimensions {
	var cols []layout.FlexChild
	for i := 0; i < numCols; i++ {
		idx := i
		cell := ""
		if idx < len(cells) {
			cell = cells[idx]
		}
		cols = append(cols, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = colW
			gtx.Constraints.Min.X = colW
			return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(th, unit.Sp(13), cell)
				if header {
					lbl.Font = font.Font{Weight: font.Bold}
				}
				lbl.MaxLines = 0
				return lbl.Layout(gtx)
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cols...)
}

// ---------------------------------------------------------------------------
// withBackground draws w on top of a filled background rect.
// Uses op.Record to capture widget dimensions before painting the fill.
// ---------------------------------------------------------------------------

func withBackground(gtx layout.Context, bg color.NRGBA, pad unit.Dp, w layout.Widget) layout.Dimensions {
	// Record the widget ops to learn the size, then replay with background.
	rec := op.Record(gtx.Ops)
	dims := layout.UniformInset(pad).Layout(gtx, w)
	call := rec.Stop()

	paint.FillShape(gtx.Ops, bg, clip.Rect{Max: dims.Size}.Op())
	call.Add(gtx.Ops)
	return dims
}
