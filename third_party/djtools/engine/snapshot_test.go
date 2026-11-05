package engine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nateranda/djtools/engine"
)

func TestSnapshotLeavesSourceUntouched(t *testing.T) {
	src := generateDatabase(t, filepath.Join(fixturesDir, "nestedPlaylists"))
	before, _ := os.ReadFile(filepath.Join(src, "Database2", "m.db"))

	tmp, err := engine.Snapshot(src)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	if v, err := engine.SchemaVersion(tmp); err != nil || v == "" {
		t.Fatalf("schema version: %q %v", v, err)
	}

	lib, err := engine.ImportSnapshot(src, engine.ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Playlists) == 0 {
		t.Fatal("expected playlists")
	}
	for _, s := range lib.Songs {
		if !filepath.IsAbs(s.Path) || filepath.Dir(filepath.Dir(s.Path)) != filepath.Dir(src) {
			t.Fatalf("path %q not resolved against source %q", s.Path, src)
		}
	}
	after, _ := os.ReadFile(filepath.Join(src, "Database2", "m.db"))
	if string(before) != string(after) {
		t.Fatal("source m.db changed")
	}
}

func TestSnapshotRejectsNonLibrary(t *testing.T) {
	if _, err := engine.Snapshot(t.TempDir()); err != engine.ErrNotEngineLibrary {
		t.Fatalf("got %v", err)
	}
}
