package main

import (
	"image"
	"image/color"
	"path/filepath"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// App holds all application state and the main window.
type App struct {
	window app.Window
	th     *material.Theme

	// File state
	rootPath     string
	currentFile  string
	modified     bool
	loading      bool
	selectedPath string

	// Widgets
	editor   widget.Editor
	fileTree *FileTree

	// Split ratios [0..1]
	treeSplit   float32
	editorSplit float32
	mainWidth   int

	// Split drag state
	treeDrag   dragHandle
	editorDrag dragHandle

	// Preview
	previewBlocks []renderedBlock
	previewList   widget.List

	// Modal overlay (nil = none shown)
	modal *modalState

	// Status bar text
	status string

	// Toolbar buttons
	btnNew  widget.Clickable
	btnOpen widget.Clickable
	btnSave widget.Clickable

	// Theme buttons
	btnLight widget.Clickable
	btnDark  widget.Clickable
	btnSepia widget.Clickable

	// Global key shortcut tag (registered on background rect each frame)
	keyTag struct{}

	// Channel: zenity goroutine → frame loop
	openFolderCh chan string
}

// ---------------------------------------------------------------------------
// Drag-handle state
// ---------------------------------------------------------------------------

type dragHandle struct {
	active  bool
	lastPos float32
	tag     struct{}
}

// ---------------------------------------------------------------------------
// Modal state
// ---------------------------------------------------------------------------

type modalKind int

const (
	modalConfirm modalKind = iota
	modalInput
)

type modalState struct {
	kind      modalKind
	title     string
	message   string
	input     widget.Editor
	btnOK     widget.Clickable
	btnCancel widget.Clickable
	onOK      func(string)
	onCancel  func()
}

// ---------------------------------------------------------------------------
// Constructor + entry point
// ---------------------------------------------------------------------------

func newApp() *App {
	return &App{
		treeSplit:    0.22,
		editorSplit:  0.5,
		status:       "Open a folder to get started  |  Ctrl+O",
		openFolderCh: make(chan string, 1),
	}
}

func (a *App) run() error {
	a.window.Option(
		app.Title("Marknote"),
		app.Size(unit.Dp(1200), unit.Dp(800)),
	)

	a.th = material.NewTheme()
	a.th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	a.editor.SingleLine = false
	a.fileTree = newFileTree(a)
	a.previewList.Axis = layout.Vertical

	ops := new(op.Ops)
	for {
		switch e := a.window.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(ops, e)

			// Drain folder path from zenity goroutine.
			select {
			case p := <-a.openFolderCh:
				a.openFolder(p)
			default:
			}

			a.layout(gtx)
			e.Frame(ops)
		}
	}
}

// ---------------------------------------------------------------------------
// Top-level layout
// ---------------------------------------------------------------------------

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	// Background fill.
	paint.FillShape(gtx.Ops, a.th.Palette.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	// Register global key shortcut area.
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &a.keyTag)
	a.handleKeys(gtx)

	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(a.layoutToolbar),
		layout.Flexed(1, a.layoutMain),
		layout.Rigid(a.layoutStatusBar),
	)

	if a.modal != nil {
		a.layoutModal(gtx)
	}

	return dims
}

// ---------------------------------------------------------------------------
// Toolbar
// ---------------------------------------------------------------------------

func (a *App) layoutToolbar(gtx layout.Context) layout.Dimensions {
	if a.btnNew.Clicked(gtx) {
		a.promptNewFile()
	}
	if a.btnOpen.Clicked(gtx) {
		a.promptOpenFolder()
	}
	if a.btnSave.Clicked(gtx) {
		a.saveFile()
	}
	if a.btnLight.Clicked(gtx) {
		a.applyTheme(themeLight)
	}
	if a.btnDark.Clicked(gtx) {
		a.applyTheme(themeDark)
	}
	if a.btnSepia.Clicked(gtx) {
		a.applyTheme(themeSepia)
	}

	toolbarBg := darkenColor(a.th.Palette.Bg, 14)
	paint.FillShape(gtx.Ops, toolbarBg,
		clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, gtx.Dp(44))}.Op())

	return layout.Inset{
		Top: unit.Dp(6), Bottom: unit.Dp(6),
		Left: unit.Dp(8), Right: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.th, &a.btnNew, "New").Layout(gtx)
			}),
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.th, &a.btnOpen, "Open Folder").Layout(gtx)
			}),
			layout.Rigid(spacer(6)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.th, &a.btnSave, "Save").Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.th, &a.btnLight, "Light").Layout(gtx)
			}),
			layout.Rigid(spacer(4)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.th, &a.btnDark, "Dark").Layout(gtx)
			}),
			layout.Rigid(spacer(4)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Button(a.th, &a.btnSepia, "Sepia").Layout(gtx)
			}),
		)
	})
}

func spacer(dp float32) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: image.Pt(gtx.Dp(unit.Dp(dp)), 1)}
	}
}

// ---------------------------------------------------------------------------
// Main split area
// ---------------------------------------------------------------------------

func (a *App) layoutMain(gtx layout.Context) layout.Dimensions {
	total := gtx.Constraints.Max.X
	a.mainWidth = total
	handleW := gtx.Dp(5)

	restForEditorSplit := total - int(float32(total)*a.treeSplit) - handleW*2
	a.processDrag(gtx, &a.treeDrag, &a.treeSplit, total)
	a.processDrag(gtx, &a.editorDrag, &a.editorSplit, restForEditorSplit)

	treeW := int(float32(total) * a.treeSplit)
	rest := total - treeW - handleW*2
	if rest < 80 {
		rest = 80
	}
	editorW := int(float32(rest) * a.editorSplit)

	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(treeW, gtx.Constraints.Max.Y))
			return a.fileTree.Layout(gtx, a.th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutSplitBar(gtx, &a.treeDrag, handleW)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(editorW, gtx.Constraints.Max.Y))
			return a.layoutEditor(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutSplitBar(gtx, &a.editorDrag, handleW)
		}),
		layout.Flexed(1, a.layoutPreview),
	)
}

func (a *App) processDrag(gtx layout.Context, h *dragHandle, ratio *float32, totalPx int) {
	for {
		e, ok := gtx.Event(pointer.Filter{
			Target: &h.tag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release,
		})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			h.active = true
			h.lastPos = pe.Position.X
		case pointer.Drag:
			if h.active && totalPx > 0 {
				delta := pe.Position.X - h.lastPos
				h.lastPos = pe.Position.X
				*ratio += delta / float32(totalPx)
				if *ratio < 0.1 {
					*ratio = 0.1
				}
				if *ratio > 0.85 {
					*ratio = 0.85
				}
				a.window.Invalidate()
			}
		case pointer.Release:
			h.active = false
		}
	}
}

func (a *App) layoutSplitBar(gtx layout.Context, h *dragHandle, w int) layout.Dimensions {
	size := image.Pt(w, gtx.Constraints.Max.Y)

	barColor := mulAlpha(a.th.Palette.Fg, 40)
	if h.active {
		barColor = mulAlpha(a.th.Palette.ContrastBg, 200)
	}
	paint.FillShape(gtx.Ops, barColor, clip.Rect{Max: size}.Op())

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &h.tag)

	return layout.Dimensions{Size: size}
}

// ---------------------------------------------------------------------------
// Editor panel
// ---------------------------------------------------------------------------

func (a *App) layoutEditor(gtx layout.Context) layout.Dimensions {
	// Poll editor for text changes.
	for {
		ev, ok := a.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			if !a.loading {
				a.modified = true
				a.updateTitle()
				a.previewBlocks = renderMarkdown(a.editor.Text())
			}
		}
	}

	paint.FillShape(gtx.Ops, a.th.Palette.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ed := material.Editor(a.th, &a.editor, "Select a file to start editing…")
		ed.TextSize = unit.Sp(14)
		return ed.Layout(gtx)
	})
}

// ---------------------------------------------------------------------------
// Preview panel
// ---------------------------------------------------------------------------

func (a *App) layoutPreview(gtx layout.Context) layout.Dimensions {
	paint.FillShape(gtx.Ops, previewBg(a.th.Palette.Bg), clip.Rect{Max: gtx.Constraints.Max}.Op())

	blocks := a.previewBlocks
	return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(a.th, &a.previewList).Layout(gtx, len(blocks),
			func(gtx layout.Context, i int) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return blocks[i].Layout(gtx, a.th)
				})
			},
		)
	})
}

func previewBg(bg color.NRGBA) color.NRGBA {
	sub := func(a, b uint8) uint8 {
		if a < b {
			return 0
		}
		return a - b
	}
	return color.NRGBA{R: sub(bg.R, 10), G: sub(bg.G, 10), B: sub(bg.B, 8), A: 255}
}

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------

func (a *App) layoutStatusBar(gtx layout.Context) layout.Dimensions {
	statusBg := darkenColor(a.th.Palette.Bg, 14)
	paint.FillShape(gtx.Ops, statusBg,
		clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, gtx.Dp(24))}.Op())

	return layout.Inset{Top: unit.Dp(4), Bottom: unit.Dp(4), Left: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.Label(a.th, unit.Sp(12), a.status).Layout(gtx)
		},
	)
}

// ---------------------------------------------------------------------------
// Modal overlay
// ---------------------------------------------------------------------------

func (a *App) layoutModal(gtx layout.Context) layout.Dimensions {
	// Scrim.
	paint.FillShape(gtx.Ops, color.NRGBA{A: 150},
		clip.Rect{Max: gtx.Constraints.Max}.Op())
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, a.modal)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		cardW := gtx.Dp(420)
		gtx.Constraints = layout.Constraints{
			Min: image.Pt(cardW, 0),
			Max: image.Pt(cardW, gtx.Constraints.Max.Y),
		}
		return a.layoutModalCard(gtx)
	})
}

func (a *App) layoutModalCard(gtx layout.Context) layout.Dimensions {
	m := a.modal
	if m.btnOK.Clicked(gtx) {
		input := m.input.Text()
		onOK := m.onOK
		a.modal = nil
		if onOK != nil {
			onOK(input)
		}
	}
	if m.btnCancel.Clicked(gtx) {
		onCancel := m.onCancel
		a.modal = nil
		if onCancel != nil {
			onCancel()
		}
	}

	paint.FillShape(gtx.Ops, a.th.Palette.Bg, clip.Rect{Max: gtx.Constraints.Max}.Op())

	return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(a.th, unit.Sp(16), m.title)
				lbl.Font = font.Font{Weight: font.Bold}
				return lbl.Layout(gtx)
			}),
			layout.Rigid(spacer(8)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Label(a.th, unit.Sp(13), m.message)
				lbl.MaxLines = 6
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if m.kind != modalInput {
					return layout.Dimensions{}
				}
				return layout.Inset{Top: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(a.th, &m.input, "filename.md")
					ed.TextSize = unit.Sp(13)
					return ed.Layout(gtx)
				})
			}),
			layout.Rigid(spacer(20)),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Button(a.th, &m.btnCancel, "Cancel").Layout(gtx)
					}),
					layout.Rigid(spacer(8)),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := "OK"
						if m.kind == modalConfirm {
							label = "Discard"
						}
						return material.Button(a.th, &m.btnOK, label).Layout(gtx)
					}),
				)
			}),
		)
	})
}

// ---------------------------------------------------------------------------
// Keyboard shortcuts
// ---------------------------------------------------------------------------

func (a *App) handleKeys(gtx layout.Context) {
	for {
		e, ok := gtx.Event(
			key.Filter{Focus: &a.keyTag, Name: "S", Required: key.ModCtrl},
			key.Filter{Focus: &a.keyTag, Name: "O", Required: key.ModCtrl},
			key.Filter{Focus: &a.keyTag, Name: "N", Required: key.ModCtrl},
		)
		if !ok {
			break
		}
		ke, ok := e.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case "S":
			a.saveFile()
		case "O":
			a.promptOpenFolder()
		case "N":
			a.promptNewFile()
		}
	}
}

// ---------------------------------------------------------------------------
// State helpers
// ---------------------------------------------------------------------------

func (a *App) updateTitle() {
	if a.currentFile == "" {
		if a.rootPath != "" {
			a.window.Option(app.Title("Marknote — " + filepath.Base(a.rootPath)))
		} else {
			a.window.Option(app.Title("Marknote"))
		}
		return
	}
	name := filepath.Base(a.currentFile)
	if a.modified {
		a.window.Option(app.Title("Marknote — " + name + " *"))
	} else {
		a.window.Option(app.Title("Marknote — " + name))
	}
	a.status = a.currentFile
}

func (a *App) showConfirmModal(title, message string, onOK func(), onCancel func()) {
	a.modal = &modalState{
		kind:     modalConfirm,
		title:    title,
		message:  message,
		onOK:     func(_ string) { onOK() },
		onCancel: onCancel,
	}
	a.window.Invalidate()
}

func (a *App) showInputModal(title, message string, onOK func(string)) {
	m := &modalState{
		kind:    modalInput,
		title:   title,
		message: message,
		onOK:    onOK,
	}
	m.input.SingleLine = true
	a.modal = m
	a.window.Invalidate()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mulAlpha(c color.NRGBA, alpha uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: alpha}
}
