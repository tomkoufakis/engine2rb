package engine

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nateranda/djtools/lib"
)

// databaseFiles are the Engine database files djtools reads.
var databaseFiles = []string{"m.db", "hm.db"}

// sidecarSuffixes are SQLite companion files that may hold data not yet
// written back into the main database file (write-ahead log, shared memory,
// rollback journal). They are copied alongside the database if present.
var sidecarSuffixes = []string{"-wal", "-shm", "-journal"}

// ErrNotEngineLibrary is returned when a directory does not contain
// Database2/m.db and Database2/hm.db.
var ErrNotEngineLibrary = errors.New("not an Engine library: expected Database2/m.db and Database2/hm.db")

// ValidateLibrary checks that path looks like an Engine library folder
// without opening (and so without risking modifying) the database.
func ValidateLibrary(path string) error {
	for _, name := range databaseFiles {
		info, err := os.Stat(filepath.Join(path, "Database2", name))
		if err != nil || info.IsDir() {
			return ErrNotEngineLibrary
		}
	}
	return nil
}

// Snapshot copies the Engine database files at path (an "Engine Library"
// folder) into a new temporary directory with the same Database2 layout,
// and returns that directory. Music files are not copied. The caller is
// responsible for removing the directory, for example with os.RemoveAll.
func Snapshot(path string) (string, error) {
	if err := ValidateLibrary(path); err != nil {
		return "", err
	}

	tempDir, err := os.MkdirTemp("", "djtools-engine-*")
	if err != nil {
		return "", fmt.Errorf("error creating temporary directory: %v", err)
	}
	dstDir := filepath.Join(tempDir, "Database2")
	if err := os.Mkdir(dstDir, 0o755); err != nil {
		os.RemoveAll(tempDir)
		return "", fmt.Errorf("error creating temporary directory: %v", err)
	}

	srcDir := filepath.Join(path, "Database2")
	for _, name := range databaseFiles {
		if err := copyFile(filepath.Join(srcDir, name), filepath.Join(dstDir, name)); err != nil {
			os.RemoveAll(tempDir)
			return "", err
		}
		for _, suffix := range sidecarSuffixes {
			src := filepath.Join(srcDir, name+suffix)
			if _, err := os.Stat(src); err != nil {
				continue // sidecar not present
			}
			if err := copyFile(src, filepath.Join(dstDir, name+suffix)); err != nil {
				os.RemoveAll(tempDir)
				return "", err
			}
		}
	}
	return tempDir, nil
}

// ImportSnapshot copies the Engine library at path to a temporary location,
// imports the copy, and removes the copy. The source database is only ever
// read as a plain file, never opened by SQLite.
//
// Unless importOptions.PathBase is already set, relative track paths are
// resolved against the original library location, not the temporary copy.
func ImportSnapshot(path string, importOptions ImportOptions) (lib.Library, error) {
	tempDir, err := Snapshot(path)
	if err != nil {
		return lib.Library{}, err
	}
	defer os.RemoveAll(tempDir)

	if importOptions.PathBase == "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return lib.Library{}, err
		}
		importOptions.PathBase = absPath
	}
	return Import(tempDir, importOptions)
}

// SchemaVersion returns the Engine database schema version
// ("major.minor.patch") recorded in m.db's Information table at path.
// Only call this on a snapshot, never on a live library.
func SchemaVersion(path string) (string, error) {
	db, err := sql.Open("sqlite3", filepath.Join(path, "Database2", "m.db"))
	if err != nil {
		return "", err
	}
	defer db.Close()

	var major, minor, patch int
	err = db.QueryRow(`SELECT schemaVersionMajor, schemaVersionMinor, schemaVersionPatch
		FROM Information ORDER BY id LIMIT 1`).Scan(&major, &minor, &patch)
	if err != nil {
		return "", fmt.Errorf("error reading schema version: %v", err)
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening %s: %v", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("error copying %s: %v", src, err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("error copying %s: %v", src, err)
	}
	return out.Close()
}
