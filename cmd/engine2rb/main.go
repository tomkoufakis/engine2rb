// Command engine2rb is a small desktop app that converts playlists from an
// Engine DJ library into a Rekordbox XML file.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"engine2rb/internal/convert"

	"github.com/nateranda/djtools/lib"
)

// ui holds the window state. All fields are only touched on the UI goroutine;
// background work hands results back with fyne.Do.
type ui struct {
	win fyne.Window

	sourceEntry *widget.Entry
	targetEntry *widget.Entry
	loadBtn     *widget.Button
	convertBtn  *widget.Button
	onlyTracks  *widget.Check
	progress    *widget.ProgressBarInfinite
	logEntry    *widget.Entry
	tree        *widget.Tree
	summary     *widget.Label

	loaded   *convert.Loaded
	children map[string][]string // tree node ID -> child IDs ("" is the root)
	names    map[string]string   // tree node ID -> label
	checked  map[string]bool     // tree node ID -> selected for export
}

func main() {
	a := app.NewWithID("engine2rb.converter")
	w := a.NewWindow("Engine DJ → Rekordbox XML")
	u := &ui{win: w, checked: map[string]bool{}}
	w.SetContent(u.build())
	w.Resize(fyne.NewSize(760, 640))
	w.ShowAndRun()
}

func (u *ui) build() fyne.CanvasObject {
	u.sourceEntry = widget.NewEntry()
	u.sourceEntry.SetPlaceHolder("Engine Library folder (contains Database2)")
	if def := defaultEngineLibrary(); def != "" {
		u.sourceEntry.SetText(def)
	}
	sourceBrowse := widget.NewButtonWithIcon("Browse…", theme.FolderOpenIcon(), u.pickSource)
	u.loadBtn = widget.NewButtonWithIcon("Load playlists", theme.ViewRefreshIcon(), u.load)

	u.targetEntry = widget.NewEntry()
	u.targetEntry.SetPlaceHolder("Where to save the Rekordbox XML file")
	targetBrowse := widget.NewButtonWithIcon("Browse…", theme.DocumentSaveIcon(), u.pickTarget)

	u.tree = widget.NewTree(u.childIDs, u.isBranch, u.createNode, u.updateNode)
	selectAll := widget.NewButton("Select all", func() { u.setAll(true) })
	selectNone := widget.NewButton("Select none", func() { u.setAll(false) })
	u.summary = widget.NewLabel("No library loaded.")

	u.onlyTracks = widget.NewCheck("Only include tracks from the selected playlists", nil)
	u.onlyTracks.SetChecked(true)

	u.convertBtn = widget.NewButtonWithIcon("Convert", theme.ConfirmIcon(), u.convert)
	u.convertBtn.Importance = widget.HighImportance
	u.convertBtn.Disable()

	u.progress = widget.NewProgressBarInfinite()
	u.progress.Stop()
	u.progress.Hide()

	u.logEntry = widget.NewMultiLineEntry()
	u.logEntry.SetPlaceHolder("Log")

	form := container.NewVBox(
		widget.NewLabel("Engine library (source)"),
		container.NewBorder(nil, nil, nil, container.NewHBox(sourceBrowse, u.loadBtn), u.sourceEntry),
		widget.NewLabel("Rekordbox XML (target)"),
		container.NewBorder(nil, nil, nil, targetBrowse, u.targetEntry),
	)
	treeHeader := container.NewBorder(nil, nil, widget.NewLabel("Playlists to export"),
		container.NewHBox(selectAll, selectNone))
	treePane := container.NewBorder(treeHeader, u.summary, nil, nil, u.tree)
	bottom := container.NewVBox(u.onlyTracks, u.progress, u.convertBtn)

	split := container.NewVSplit(treePane, u.logEntry)
	split.Offset = 0.7

	return container.NewBorder(form, bottom, nil, nil, split)
}

// --- source / target selection ------------------------------------------

func (u *ui) pickSource() {
	d := dialog.NewFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if dir == nil {
			return // cancelled
		}
		u.sourceEntry.SetText(dir.Path())
		u.load()
	}, u.win)
	setDialogLocation(d, u.sourceEntry.Text)
	d.Show()
}

func (u *ui) pickTarget() {
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		if w == nil {
			return // cancelled
		}
		path := w.URI().Path()
		w.Close()
		// The save dialog creates an empty file; Export replaces it later.
		// Remove it now so cancelling the conversion leaves nothing behind.
		os.Remove(path)
		u.targetEntry.SetText(path)
	}, u.win)
	d.SetFileName(convert.DefaultFileName)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".xml"}))
	d.Show()
}

func setDialogLocation(d *dialog.FileDialog, path string) {
	for dir := path; dir != "" && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			if l, err := storage.ListerForURI(storage.NewFileURI(dir)); err == nil {
				d.SetLocation(l)
			}
			return
		}
	}
}

// defaultEngineLibrary returns ~/Music/Engine Library if it exists. As far
// as I know this is Engine DJ desktop's default on both macOS and Windows,
// but users can move it, so it is only a convenience.
func defaultEngineLibrary() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, "Music", "Engine Library")
	if _, err := os.Stat(filepath.Join(p, "Database2", "m.db")); err == nil {
		return p
	}
	return ""
}

// --- loading --------------------------------------------------------------

func (u *ui) load() {
	src := u.sourceEntry.Text
	if src == "" {
		dialog.ShowInformation("No source", "Choose your Engine Library folder first.", u.win)
		return
	}
	u.busy(true)
	u.logf("Copying Engine database from %s to a temporary folder and reading it…", src)

	go func() {
		loaded, err := convert.Load(src)
		fyne.Do(func() {
			u.busy(false)
			if err != nil {
				u.logf("Load failed: %v", err)
				dialog.ShowError(err, u.win)
				return
			}
			u.setLibrary(loaded)
		})
	}()
}

func (u *ui) setLibrary(l *convert.Loaded) {
	u.loaded = l
	u.children = map[string][]string{}
	u.names = map[string]string{}
	u.checked = map[string]bool{}

	partial, unresolved := 0, 0
	var add func(parent string, playlists []lib.Playlist)
	add = func(parent string, playlists []lib.Playlist) {
		for _, p := range playlists {
			id := strconv.Itoa(p.PlaylistID)
			u.children[parent] = append(u.children[parent], id)
			if p.UnresolvedTracks > 0 {
				// some entries point to tracks that are not in this library
				u.names[id] = fmt.Sprintf("%s  (%d of %d)", p.Name, len(p.Songs), len(p.Songs)+p.UnresolvedTracks)
				partial++
				unresolved += p.UnresolvedTracks
			} else {
				u.names[id] = fmt.Sprintf("%s  (%d)", p.Name, len(p.Songs))
			}
			add(id, p.SubPlaylists)
		}
	}
	add("", l.Library.Playlists)

	schema := l.Schema
	if schema == "" {
		schema = "unknown"
	}
	u.logf("Loaded %d tracks and %d playlists (Engine database schema %s).",
		len(l.Library.Songs), len(u.names), schema)
	if unresolved > 0 {
		u.logf("%d playlist(s) reference %d track(s) that are not in this library "+
			"(tracks from another library or deleted tracks); these are skipped. "+
			"Such playlists show \"found of total\" counts.", partial, unresolved)
	}
	if len(u.names) == 0 {
		u.logf("No playlists found. Smart playlists (smartlists) are not converted by djtools.")
	}
	u.tree.Refresh()
	u.tree.OpenAllBranches()
	u.refreshSelection()
}

// --- playlist tree ----------------------------------------------------------

func (u *ui) childIDs(id widget.TreeNodeID) []widget.TreeNodeID { return u.children[id] }

func (u *ui) isBranch(id widget.TreeNodeID) bool { return id == "" || len(u.children[id]) > 0 }

func (u *ui) createNode(bool) fyne.CanvasObject { return widget.NewCheck("", nil) }

func (u *ui) updateNode(id widget.TreeNodeID, _ bool, o fyne.CanvasObject) {
	c := o.(*widget.Check)
	all, some := u.subtreeState(id)

	// set state directly so OnChanged doesn't fire while redrawing
	c.OnChanged = nil
	c.Text = u.names[id]
	c.Checked = all
	c.Partial = some && !all
	c.Refresh()
	c.OnChanged = func(on bool) {
		u.setSubtree(id, on)
		u.tree.Refresh()
		u.refreshSelection()
	}
}

// subtreeState reports whether every / any node in id's subtree is checked.
func (u *ui) subtreeState(id string) (all, some bool) {
	all, some = u.checked[id], u.checked[id]
	for _, child := range u.children[id] {
		ca, cs := u.subtreeState(child)
		all = all && ca
		some = some || cs
	}
	return all, some
}

func (u *ui) setSubtree(id string, on bool) {
	if id != "" {
		u.checked[id] = on
	}
	for _, child := range u.children[id] {
		u.setSubtree(child, on)
	}
}

func (u *ui) setAll(on bool) {
	if u.loaded == nil {
		return
	}
	u.setSubtree("", on)
	u.tree.Refresh()
	u.refreshSelection()
}

func (u *ui) selectedIDs() map[int]bool {
	sel := map[int]bool{}
	for id, on := range u.checked {
		if on {
			n, _ := strconv.Atoi(id)
			sel[n] = true
		}
	}
	return sel
}

func (u *ui) refreshSelection() {
	n := len(u.selectedIDs())
	if u.loaded == nil {
		u.summary.SetText("No library loaded.")
	} else {
		u.summary.SetText(fmt.Sprintf("%d of %d playlists selected", n, len(u.names)))
	}
	if n > 0 && u.progress.Hidden {
		u.convertBtn.Enable()
	} else {
		u.convertBtn.Disable()
	}
}

// --- conversion -------------------------------------------------------------

func (u *ui) convert() {
	if u.loaded == nil {
		return
	}
	target := u.targetEntry.Text
	if target == "" {
		dialog.ShowInformation("No target", "Choose where to save the Rekordbox XML file.", u.win)
		return
	}
	if convert.ResolveRoot(u.sourceEntry.Text) != convert.ResolveRoot(u.loaded.Root) {
		u.logf("Note: the source path changed since loading; converting the library loaded from %s. Press Load to reload.", u.loaded.Root)
	}
	opts := convert.Options{Selected: u.selectedIDs(), OnlySelectedTracks: u.onlyTracks.Checked}
	loaded := u.loaded

	u.busy(true)
	u.logf("Converting %d selected playlists (empty playlists are skipped)…", len(opts.Selected))
	go func() {
		start := time.Now()
		res, err := convert.Export(loaded, target, opts)
		fyne.Do(func() {
			u.busy(false)
			if err != nil {
				u.logf("Convert failed: %v", err)
				dialog.ShowError(err, u.win)
				return
			}
			u.report(res, time.Since(start))
		})
	}()
}

func (u *ui) report(res convert.Result, took time.Duration) {
	u.logf("Wrote %s: %d playlist(s), %d track(s) (%.1fs).", res.Path, res.Playlists, res.Tracks, took.Seconds())
	if res.Renamed > 0 {
		u.logf("Renamed %d playlists that shared a name with a sibling.", res.Renamed)
	}
	if n := len(res.MissingFiles); n > 0 {
		sort.Strings(res.MissingFiles)
		u.logf("Warning: %d track files were not found at the paths Engine recorded:", n)
		for i, p := range res.MissingFiles {
			if i == 20 {
				u.logf("  …and %d more", n-20)
				break
			}
			u.logf("  %s", p)
		}
	}
	dialog.ShowInformation("Done",
		fmt.Sprintf("Saved %d playlist(s) and %d track(s).\n\nNext: point rekordbox's rekordbox xml setting at this file and import the playlists (see README).", res.Playlists, res.Tracks),
		u.win)
}

// --- helpers ------------------------------------------------------------------

func (u *ui) busy(on bool) {
	if on {
		u.loadBtn.Disable()
		u.convertBtn.Disable()
		u.progress.Show()
		u.progress.Start()
		return
	}
	u.progress.Stop()
	u.progress.Hide()
	u.loadBtn.Enable()
	u.refreshSelection()
}

func (u *ui) logf(format string, args ...any) {
	u.logEntry.Append(time.Now().Format("15:04:05 ") + fmt.Sprintf(format, args...) + "\n")
}
