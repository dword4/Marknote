package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// promptOpenFolder shows the OS folder picker dialog.
func (a *App) promptOpenFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, a.window)
			return
		}
		if uri == nil {
			return // user cancelled
		}
		// fyne.URI.Path() returns the filesystem path directly (e.g. /home/user/notes)
		a.openFolder(uri.Path())
	}, a.window)
}

// openFolder sets rootPath and refreshes the file tree.
func (a *App) openFolder(path string) {
	a.rootPath = path
	a.currentFile = ""
	a.modified = false

	// Clear the editor without marking the document modified.
	a.loading = true
	a.editor.SetText("")
	a.loading = false

	// Reset tree state: close all expanded nodes, clear selection, re-fetch.
	a.tree.CloseAllBranches()
	a.tree.UnselectAll()
	a.tree.Refresh()

	a.statusLbl.SetText("Folder: " + path)
	a.window.SetTitle("Marknote — " + filepath.Base(path))
}

// confirmSwitch opens targetPath, first asking about unsaved changes if needed.
func (a *App) confirmSwitch(targetPath string) {
	if !a.modified {
		a.loadFile(targetPath)
		return
	}
	dialog.ShowConfirm(
		"Unsaved Changes",
		"You have unsaved changes in '"+filepath.Base(a.currentFile)+"'.\n"+
			"Discard and open '"+filepath.Base(targetPath)+"'?",
		func(discard bool) {
			if discard {
				a.loadFile(targetPath)
			} else if a.currentFile != "" {
				// Re-highlight the previously open file in the tree.
				a.tree.Select(a.currentFile)
			}
		},
		a.window,
	)
}

// loadFile reads the file at path and loads it into the editor and preview.
func (a *App) loadFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	a.currentFile = path

	// SetText triggers OnChanged which updates the preview. The loading guard
	// prevents OnChanged from setting the modified flag during this programmatic load.
	a.loading = true
	a.editor.SetText(string(data))
	a.loading = false

	a.modified = false
	a.updateTitle()
}

// targetDir returns the directory to use for new-file operations.
// Priority: selected tree node (or its parent if it's a file) → root folder.
func (a *App) targetDir() string {
	if a.selectedPath != "" {
		info, err := os.Stat(a.selectedPath)
		if err == nil {
			if info.IsDir() {
				return a.selectedPath
			}
			return filepath.Dir(a.selectedPath)
		}
	}
	return a.rootPath
}

// promptNewFile shows a dialog asking for a filename and creates the file.
func (a *App) promptNewFile() {
	dir := a.targetDir()
	if dir == "" {
		dialog.ShowInformation("No Folder Open", "Open a folder first (Ctrl+O).", a.window)
		return
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder("filename.md")

	d := dialog.NewForm("New File", "Create", "Cancel",
		[]*widget.FormItem{{Text: "Name", Widget: entry}},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(entry.Text)
			if name == "" {
				return
			}
			if !strings.HasSuffix(strings.ToLower(name), ".md") {
				name += ".md"
			}
			a.createNewFile(filepath.Join(dir, name))
		},
		a.window,
	)
	d.Resize(fyne.NewSize(360, 160))
	d.Show()
}

// createNewFile creates a file at path, refreshes the tree, and opens it.
func (a *App) createNewFile(path string) {
	if _, err := os.Stat(path); err == nil {
		dialog.ShowError(fmt.Errorf("'%s' already exists", filepath.Base(path)), a.window)
		return
	}
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.tree.Refresh()
	a.loadFile(path)
	a.tree.Select(path)
}

// showTreeContextMenu pops up a context menu at the given canvas position.
func (a *App) showTreeContextMenu(pos fyne.Position) {
	if a.rootPath == "" {
		return // no folder open — nothing to do
	}
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("New File", a.promptNewFile),
	)
	widget.ShowPopUpMenuAtPosition(menu, a.window.Canvas(), pos)
}

// saveFile writes the editor content back to the current file.
func (a *App) saveFile() {
	if a.currentFile == "" {
		return
	}
	if err := os.WriteFile(a.currentFile, []byte(a.editor.Text), 0644); err != nil {
		dialog.ShowError(err, a.window)
		return
	}
	a.modified = false
	a.updateTitle()
	a.statusLbl.SetText("Saved: " + a.currentFile)
}
