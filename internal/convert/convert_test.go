package convert

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nateranda/djtools/rbxml"
)

var fixture = filepath.Join("..", "..", "third_party", "djtools", "engine", "testdata",
	"import", "fixtures", "nestedPlaylists")

// makeLibrary builds <tmp>/Engine Library/Database2/{m,hm}.db from the
// djtools fixture and returns the Engine Library folder.
func makeLibrary(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Engine Library")
	if err := os.MkdirAll(filepath.Join(root, "Database2"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"m", "hm"} {
		query, err := os.ReadFile(filepath.Join(fixture, name+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite3", filepath.Join(root, "Database2", name+".db"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(query)); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
	return root
}

func TestLoadAndExportSelection(t *testing.T) {
	root := makeLibrary(t)

	// one of the two tracks in playlist3 exists on disk, the other doesn't
	music := filepath.Join(filepath.Dir(root), "DJ Music")
	os.MkdirAll(music, 0o755)
	os.WriteFile(filepath.Join(music, "Four Tet - Parallel 4.mp3"), []byte("x"), 0o644)

	mBefore, _ := os.ReadFile(filepath.Join(root, "Database2", "m.db"))

	// passing the Database2 folder should also work
	loaded, err := Load(filepath.Join(root, "Database2"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Root != root {
		t.Fatalf("root %q, want %q", loaded.Root, root)
	}
	if loaded.Schema == "" {
		t.Error("schema version not read")
	}

	target := t.TempDir() // a folder: DefaultFileName is appended
	res, err := Export(loaded, target, Options{Selected: map[int]bool{3: true}, OnlySelectedTracks: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Path != filepath.Join(target, DefaultFileName) || res.Playlists != 1 || res.Tracks != 2 {
		t.Fatalf("unexpected result %+v", res)
	}
	if len(res.MissingFiles) != 1 {
		t.Fatalf("missing files %v", res.MissingFiles)
	}

	// read the XML back with djtools and check structure and paths
	back, err := rbxml.Import(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Playlists) != 1 || back.Playlists[0].Name != "playlist1" ||
		len(back.Playlists[0].SubPlaylists) != 1 ||
		len(back.Playlists[0].SubPlaylists[0].SubPlaylists) != 1 ||
		back.Playlists[0].SubPlaylists[0].SubPlaylists[0].Name != "playlist3" {
		t.Fatalf("unexpected playlists %+v", back.Playlists)
	}
	xmlBytes, _ := os.ReadFile(res.Path)
	if !strings.Contains(string(xmlBytes), "Four%20Tet%20-%20Parallel%204.mp3") {
		t.Error("expected track location in XML")
	}
	if strings.Contains(string(xmlBytes), "file://localhost//") {
		t.Error("double slash in track location")
	}
	for _, s := range back.Songs {
		if filepath.Dir(s.Path) != music {
			t.Errorf("track path %q not under %q", s.Path, music)
		}
	}

	mAfter, _ := os.ReadFile(filepath.Join(root, "Database2", "m.db"))
	if string(mBefore) != string(mAfter) {
		t.Error("source database was modified")
	}
	leftovers, _ := filepath.Glob(filepath.Join(target, ".engine2rb-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestExportRejectsTargetInsideLibrary(t *testing.T) {
	root := makeLibrary(t)
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Export(loaded, filepath.Join(root, "out.xml"), Options{Selected: map[int]bool{1: true}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRejectsNonLibrary(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}
