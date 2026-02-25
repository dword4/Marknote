package main

import (
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type App struct {
	fyneApp fyne.App
	window  fyne.Window

	rootPath    string
	currentFile string
	modified    bool
	loading     bool

	tree         *contextTree
	editor       *widget.Entry
	previewBox   *fyne.Container // VBox holding rendered markdown blocks
	statusLbl    *widget.Label
	mainSplit    *container.Split
	selectedPath string // most recently selected tree node
}

// toolbarSelect wraps a widget.Select so it can be placed inside a widget.Toolbar.
// widget.Toolbar accepts any value that implements the ToolbarItem interface
// (i.e. has a ToolbarObject() fyne.CanvasObject method).
type toolbarSelect struct{ *widget.Select }

func (ts *toolbarSelect) ToolbarObject() fyne.CanvasObject { return ts.Select }

func newApp() *App {
	a := app.NewWithID("io.marknote.app")
	w := a.NewWindow("Marknote")
	w.Resize(fyne.NewSize(1200, 800))
	return &App{
		fyneApp: a,
		window:  w,
	}
}

func (a *App) run() {
	a.buildUI()
	a.window.ShowAndRun()
}

func (a *App) buildUI() {
	// Editor: multiline entry with word-wrap + vertical scroll via container
	a.editor = widget.NewMultiLineEntry()
	a.editor.Wrapping = fyne.TextWrapWord
	a.editor.SetPlaceHolder("Select a note from the file tree to begin editing...")
	a.editor.OnChanged = func(text string) {
		a.updatePreview(text)
		if !a.loading {
			a.modified = true
			a.updateTitle()
		}
	}

	// Preview: a VBox of rendered blocks (RichText + table grids) inside a scroll.
	// Fyne's built-in ParseMarkdown doesn't support tables, so we split the
	// content and handle table blocks ourselves in preview.go.
	a.previewBox = container.NewVBox()

	// Editor/Preview horizontal split; editor wrapped in VScroll so it
	// scrolls vertically while still word-wrapping at the panel width.
	editorSplit := container.NewHSplit(
		container.NewVScroll(a.editor),
		container.NewVScroll(a.previewBox),
	)
	editorSplit.SetOffset(0.5)

	// File tree (widget.Tree manages its own internal scrolling)
	a.tree = a.buildFileTree()

	// Main split: tree ~22% | editor+preview ~78%
	a.mainSplit = container.NewHSplit(a.tree, editorSplit)
	a.mainSplit.SetOffset(0.22)

	// Theme selector — embedded in the toolbar via the ToolbarItem interface.
	themeSelect := widget.NewSelect([]string{"Light", "Dark", "Sepia"}, a.applyTheme)
	themeSelect.SetSelected("Light")

	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.DocumentCreateIcon(), a.promptNewFile),
		widget.NewToolbarAction(theme.FolderOpenIcon(), a.promptOpenFolder),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.DocumentSaveIcon(), a.saveFile),
		widget.NewToolbarSpacer(),
		&toolbarSelect{themeSelect},
	)

	// Status bar
	a.statusLbl = widget.NewLabel("Open a folder to get started  |  Ctrl+O")

	a.window.SetContent(container.NewBorder(toolbar, a.statusLbl, nil, nil, a.mainSplit))

	// Canvas-level shortcuts: these fire even when the editor has keyboard focus.
	a.window.Canvas().AddShortcut(
		&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl},
		func(_ fyne.Shortcut) { a.saveFile() },
	)
	a.window.Canvas().AddShortcut(
		&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl},
		func(_ fyne.Shortcut) { a.promptOpenFolder() },
	)
}

func (a *App) updateTitle() {
	if a.currentFile == "" {
		if a.rootPath != "" {
			a.window.SetTitle("Marknote — " + filepath.Base(a.rootPath))
		} else {
			a.window.SetTitle("Marknote")
		}
		return
	}

	name := filepath.Base(a.currentFile)
	if a.modified {
		a.window.SetTitle("Marknote — " + name + " *")
	} else {
		a.window.SetTitle("Marknote — " + name)
	}
	a.statusLbl.SetText(a.currentFile)
}
