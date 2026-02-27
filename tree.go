package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// treeNode is one visible row in the flat file tree list.
type treeNode struct {
	path  string
	name  string
	isDir bool
	depth int
}

// rowTag is a unique pointer-event tag per tree row.
type rowTag struct{ idx int }

// FileTree renders the folder/file hierarchy as a scrollable flat list.
type FileTree struct {
	app      *App
	expanded map[string]bool
	visible  []treeNode

	list       widget.List
	rowTags    []rowTag
	hoveredIdx int // index of hovered row, -1 if none
}

func newFileTree(a *App) *FileTree {
	ft := &FileTree{
		app:        a,
		expanded:   make(map[string]bool),
		hoveredIdx: -1,
	}
	ft.list.Axis = layout.Vertical
	return ft
}

// rebuild recomputes the visible flat list from the filesystem.
func (ft *FileTree) rebuild() {
	ft.visible = nil
	if ft.app.rootPath == "" {
		return
	}
	ft.appendChildren(ft.app.rootPath, 0)
}

func (ft *FileTree) appendChildren(dir string, depth int) {
	children := ft.app.listDir(dir)
	for _, p := range children {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		ft.visible = append(ft.visible, treeNode{
			path:  p,
			name:  filepath.Base(p),
			isDir: info.IsDir(),
			depth: depth,
		})
		if info.IsDir() && ft.expanded[p] {
			ft.appendChildren(p, depth+1)
		}
	}
}

// Reset clears expanded state and rebuilds.
func (ft *FileTree) Reset() {
	ft.expanded = make(map[string]bool)
	ft.hoveredIdx = -1
	ft.rebuild()
}

// Refresh rebuilds without clearing expanded state.
func (ft *FileTree) Refresh() {
	ft.rebuild()
}

// Layout draws the file tree and processes user interaction.
func (ft *FileTree) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	treeBg := darkenColor(th.Palette.Bg, 8)
	paint.FillShape(gtx.Ops, treeBg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	n := len(ft.visible)

	// Grow per-row tag slice as the list gains entries.
	for len(ft.rowTags) < n {
		ft.rowTags = append(ft.rowTags, rowTag{idx: len(ft.rowTags)})
	}

	rowH := gtx.Dp(28)

	return material.List(th, &ft.list).Layout(gtx, n, func(gtx layout.Context, i int) layout.Dimensions {
		if i >= len(ft.visible) {
			return layout.Dimensions{}
		}
		node := ft.visible[i]
		rowSize := image.Pt(gtx.Constraints.Max.X, rowH)

		// --- poll pointer events for this row ---
		for {
			e, ok := gtx.Event(pointer.Filter{
				Target: &ft.rowTags[i],
				Kinds:  pointer.Press | pointer.Enter | pointer.Leave,
			})
			if !ok {
				break
			}
			pe, ok := e.(pointer.Event)
			if !ok {
				continue
			}
			switch pe.Kind {
			case pointer.Enter:
				ft.hoveredIdx = i
				ft.app.window.Invalidate()
			case pointer.Leave:
				if ft.hoveredIdx == i {
					ft.hoveredIdx = -1
					ft.app.window.Invalidate()
				}
			case pointer.Press:
				if pe.Buttons&pointer.ButtonPrimary != 0 {
					if node.isDir {
						ft.expanded[node.path] = !ft.expanded[node.path]
						ft.rebuild()
					} else {
						ft.app.selectedPath = node.path
						ft.app.confirmSwitch(node.path)
					}
					ft.app.window.Invalidate()
				} else if pe.Buttons&pointer.ButtonSecondary != 0 {
					ft.app.selectedPath = node.path
					ft.app.promptNewFile()
					ft.app.window.Invalidate()
				}
			}
		}

		// --- row background ---
		isSelected := node.path == ft.app.currentFile || node.path == ft.app.selectedPath
		var rowBg color.NRGBA
		if isSelected {
			rowBg = mulAlpha(th.Palette.ContrastBg, 200)
		} else if ft.hoveredIdx == i {
			rowBg = mulAlpha(th.Palette.ContrastBg, 60)
		}
		paint.FillShape(gtx.Ops, rowBg, clip.Rect{Max: rowSize}.Op())

		// --- register event area for this row (single tag handles all pointer events) ---
		rcStack := clip.Rect{Max: rowSize}.Push(gtx.Ops)
		event.Op(gtx.Ops, &ft.rowTags[i])
		rcStack.Pop()

		// --- draw row content: indent + arrow/space + name ---
		fg := th.Palette.Fg
		if isSelected {
			fg = th.Palette.ContrastFg
		}

		layout.Inset{
			Left:   unit.Dp(float32(node.depth*14 + 8)),
			Top:    unit.Dp(5),
			Bottom: unit.Dp(5),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var arrow string
					if node.isDir {
						if ft.expanded[node.path] {
							arrow = "▼ "
						} else {
							arrow = "▶ "
						}
					} else {
						arrow = "  "
					}
					lbl := material.Label(th, unit.Sp(10), arrow)
					lbl.Color = mulAlpha(fg, 160)
					return lbl.Layout(gtx)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Label(th, unit.Sp(13), node.name)
					lbl.Color = fg
					if node.isDir {
						lbl.Font = font.Font{Weight: font.SemiBold}
					}
					return lbl.Layout(gtx)
				}),
			)
		})

		return layout.Dimensions{Size: rowSize}
	})
}

// ---------------------------------------------------------------------------
// listDir — shared by FileTree and actions
// ---------------------------------------------------------------------------

// listDir returns direct children of path: dirs first (alpha), then .md files
// (alpha). Hidden entries (name starts with ".") are excluded.
func (a *App) listDir(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var dirs, files []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(path, e.Name())
		if e.IsDir() {
			dirs = append(dirs, full)
		} else if strings.ToLower(filepath.Ext(e.Name())) == ".md" {
			files = append(files, full)
		}
	}

	sort.Slice(dirs, func(i, j int) bool { return dirs[i] < dirs[j] })
	sort.Slice(files, func(i, j int) bool { return files[i] < files[j] })

	return append(dirs, files...)
}

// ---------------------------------------------------------------------------
// Color helpers
// ---------------------------------------------------------------------------

// darkenColor subtracts `by` from each RGB channel (clamps to 0).
func darkenColor(c color.NRGBA, by uint8) color.NRGBA {
	sub := func(a, b uint8) uint8 {
		if a < b {
			return 0
		}
		return a - b
	}
	return color.NRGBA{R: sub(c.R, by), G: sub(c.G, by), B: sub(c.B, by), A: c.A}
}
