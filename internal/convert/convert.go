// Package convert loads an Engine DJ library (via a temporary snapshot) and
// exports a selection of its playlists to a Rekordbox XML file. It contains
// no UI code so it can be tested on its own.
package convert

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nateranda/djtools/engine"
	"github.com/nateranda/djtools/lib"
	"github.com/nateranda/djtools/rbxml"
)

// DefaultFileName is used when the target is a folder rather than a file.
const DefaultFileName = "rekordbox.xml"

// Loaded is an Engine library read from a snapshot.
type Loaded struct {
	Root    string      // absolute path of the source Engine Library folder
	Schema  string      // Engine database schema version, "" if unknown
	Library lib.Library // tracks and playlists, paths resolved against Root
}

// Options control an export.
type Options struct {
	Selected           map[int]bool // PlaylistIDs to export
	OnlySelectedTracks bool         // drop tracks not in any exported playlist
}

// Result summarises an export.
type Result struct {
	Path         string   // XML file written
	Playlists    int      // playlists (excluding folders) written
	Tracks       int      // tracks written to the collection
	Renamed      int      // playlists renamed to avoid duplicate names
	MissingFiles []string // track files that could not be found on disk
}

// ResolveRoot accepts either the "Engine Library" folder or its Database2
// subfolder and returns the Engine Library folder.
func ResolveRoot(path string) string {
	path = filepath.Clean(path)
	if strings.EqualFold(filepath.Base(path), "Database2") {
		return filepath.Dir(path)
	}
	return path
}

// Load copies the Engine database at source to a temporary directory, reads
// the copy, and deletes it. The source database is never opened by SQLite.
func Load(source string) (*Loaded, error) {
	root, err := filepath.Abs(ResolveRoot(source))
	if err != nil {
		return nil, err
	}

	tempDir, err := engine.Snapshot(root)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	schema, _ := engine.SchemaVersion(tempDir) // informational only

	library, err := engine.Import(tempDir, engine.ImportOptions{PathBase: root})
	if err != nil {
		return nil, fmt.Errorf("reading Engine library: %w", err)
	}
	return &Loaded{Root: root, Schema: schema, Library: library}, nil
}

// TargetPath turns the user's target into an XML file path: an existing
// folder gets DefaultFileName appended, and a missing .xml extension is added.
func TargetPath(target string) string {
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return filepath.Join(target, DefaultFileName)
	}
	if !strings.EqualFold(filepath.Ext(target), ".xml") {
		return target + ".xml"
	}
	return target
}

// Export writes the selected playlists of l to a Rekordbox XML file.
func Export(l *Loaded, target string, opts Options) (Result, error) {
	if len(opts.Selected) == 0 {
		return Result{}, errors.New("no playlists selected")
	}
	path, err := filepath.Abs(TargetPath(target))
	if err != nil {
		return Result{}, err
	}
	if isInside(path, l.Root) {
		return Result{}, errors.New("choose a target outside the Engine Library folder")
	}

	out := l.Library.FilterPlaylists(opts.Selected)
	if opts.OnlySelectedTracks {
		out.PruneUnusedSongs()
	}
	res := Result{Path: path, Tracks: len(out.Songs), Playlists: countPlaylists(out.Playlists)}
	res.Renamed = out.DedupePlaylistNames()

	for _, s := range out.Songs {
		if _, err := os.Stat(s.Path); err != nil {
			res.MissingFiles = append(res.MissingFiles, s.Path)
		}
	}

	// write to a temporary file in the same folder, then rename, so a failed
	// export never leaves a half-written XML in place of a good one
	tmp, err := os.CreateTemp(filepath.Dir(path), ".engine2rb-*.xml")
	if err != nil {
		return Result{}, fmt.Errorf("creating output file: %w", err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName) // no-op after a successful rename

	if err := rbxml.Export(&out, tmpName, rbxml.ExportOptions{}); err != nil {
		return Result{}, fmt.Errorf("writing Rekordbox XML: %w", err)
	}
	if err := replaceFile(tmpName, path); err != nil {
		return Result{}, fmt.Errorf("saving %s: %w", path, err)
	}
	// os.CreateTemp makes the file owner-only (0600); use normal permissions
	_ = os.Chmod(path, 0o644)
	return res, nil
}

// replaceFile renames src over dst. os.Rename replaces an existing file on
// macOS/Linux; on Windows it can fail if dst exists, so remove dst first.
func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(src, dst)
}

func countPlaylists(playlists []lib.Playlist) int {
	n := 0
	for _, p := range playlists {
		if p.Songs != nil {
			n++
		}
		n += countPlaylists(p.SubPlaylists)
	}
	return n
}

func isInside(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
