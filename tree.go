package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// contextTree extends widget.Tree with a secondary-tap (right-click) handler.
// Calling ExtendBaseWidget(ct) at construction time redirects Fyne's event
// dispatch to the outer struct, so TappedSecondary is invoked on right-click.
type contextTree struct {
	widget.Tree
	app *App
}

// TappedSecondary shows the tree context menu when the user right-clicks.
func (ct *contextTree) TappedSecondary(ev *fyne.PointEvent) {
	ct.app.showTreeContextMenu(ev.AbsolutePosition)
}

func (a *App) buildFileTree() *contextTree {
	ct := &contextTree{app: a}

	ct.ChildUIDs = func(uid widget.TreeNodeID) []widget.TreeNodeID {
		if a.rootPath == "" {
			return nil
		}
		dir := uid
		if uid == "" {
			dir = a.rootPath
		}
		return a.listDir(dir)
	}

	ct.IsBranch = func(uid widget.TreeNodeID) bool {
		if uid == "" {
			return a.rootPath != ""
		}
		info, err := os.Stat(uid)
		return err == nil && info.IsDir()
	}

	// BorderLayout keeps the icon pinned left while the label fills remaining width.
	ct.CreateNode = func(branch bool) fyne.CanvasObject {
		icon := widget.NewIcon(theme.DocumentIcon())
		label := widget.NewLabel("")
		label.Truncation = fyne.TextTruncateEllipsis
		return container.New(layout.NewBorderLayout(nil, nil, icon, nil), icon, label)
	}

	ct.UpdateNode = func(uid widget.TreeNodeID, branch bool, node fyne.CanvasObject) {
		c := node.(*fyne.Container)
		icon := c.Objects[0].(*widget.Icon)
		label := c.Objects[1].(*widget.Label)
		if branch {
			icon.SetResource(theme.FolderIcon())
		} else {
			icon.SetResource(theme.DocumentIcon())
		}
		label.SetText(filepath.Base(uid))
	}

	ct.OnSelected = func(uid widget.TreeNodeID) {
		// Always track the selection so the context menu and new-file action
		// know which directory to use.
		a.selectedPath = uid
		if uid == "" {
			return
		}
		info, err := os.Stat(uid)
		if err != nil || info.IsDir() {
			return // directories just expand/collapse; files get opened
		}
		a.confirmSwitch(uid)
	}

	// Must be called last: overrides widget.Tree's own ExtendBaseWidget call
	// so that Fyne dispatches events (including TappedSecondary) to ct.
	ct.ExtendBaseWidget(ct)
	return ct
}

// listDir returns the direct children of path visible in the tree:
// directories first (alphabetical), then .md files (alphabetical).
// Entries whose names begin with "." are hidden.
func (a *App) listDir(path string) []widget.TreeNodeID {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var dirs, files []widget.TreeNodeID
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
