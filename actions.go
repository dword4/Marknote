package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ncruces/zenity"
)

// promptOpenFolder launches the OS folder picker via zenity in a goroutine.
// The result is delivered through openFolderCh so the frame loop can pick it up.
func (a *App) promptOpenFolder() {
	go func() {
		path, err := zenity.SelectFile(
			zenity.Title("Open Folder"),
			zenity.Directory(),
		)
		if err != nil {
			// zenity.ErrCanceled is returned when user dismisses — not a real error.
			return
		}
		if path != "" {
			a.openFolderCh <- path
			a.window.Invalidate()
		}
	}()
}

// openFolder sets rootPath and resets the file tree.
func (a *App) openFolder(path string) {
	a.rootPath = path
	a.currentFile = ""
	a.modified = false

	a.loading = true
	a.editor.SetText("")
	a.loading = false

	a.previewBlocks = nil
	a.fileTree.Reset()

	a.status = "Folder: " + path
	a.updateTitle()
}

// confirmSwitch opens targetPath, prompting about unsaved changes if needed.
func (a *App) confirmSwitch(targetPath string) {
	if !a.modified {
		a.loadFile(targetPath)
		return
	}
	prev := a.currentFile
	a.showConfirmModal(
		"Unsaved Changes",
		"Discard changes to '"+filepath.Base(prev)+"' and open '"+filepath.Base(targetPath)+"'?",
		func() {
			a.loadFile(targetPath)
		},
		func() {
			// User cancelled — keep the current file selected.
			_ = prev
		},
	)
}

// loadFile reads the file at path and loads it into the editor and preview.
func (a *App) loadFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		a.status = "Error: " + err.Error()
		return
	}

	a.currentFile = path
	a.selectedPath = path

	a.loading = true
	a.editor.SetText(string(data))
	a.loading = false

	a.modified = false
	a.previewBlocks = renderMarkdown(string(data))
	a.updateTitle()
}

// targetDir returns the directory to use for new-file operations.
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

// promptNewFile shows an input modal asking for a filename then creates the file.
func (a *App) promptNewFile() {
	dir := a.targetDir()
	if dir == "" {
		a.showConfirmModal(
			"No Folder Open",
			"Open a folder first (Ctrl+O).",
			func() {}, nil,
		)
		return
	}

	a.showInputModal("New File", "Enter a filename:", func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			name += ".md"
		}
		a.createNewFile(filepath.Join(dir, name))
	})
}

// createNewFile creates a file at path, refreshes the tree, and opens it.
func (a *App) createNewFile(path string) {
	if _, err := os.Stat(path); err == nil {
		a.status = fmt.Sprintf("Error: '%s' already exists", filepath.Base(path))
		return
	}
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		a.status = "Error: " + err.Error()
		return
	}
	a.fileTree.Refresh()
	a.loadFile(path)
}

// saveFile writes the editor content to the current file.
func (a *App) saveFile() {
	if a.currentFile == "" {
		return
	}
	if err := os.WriteFile(a.currentFile, []byte(a.editor.Text()), 0644); err != nil {
		a.status = "Error: " + err.Error()
		return
	}
	a.modified = false
	a.updateTitle()
	a.status = "Saved: " + a.currentFile
}
