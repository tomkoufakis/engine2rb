# Engine2RB

A small desktop app (Windows and macOS) that converts playlists from an
Engine DJ library into a Rekordbox XML file. Rekordbox can then import the XML
and export a USB stick for CDJs/XDJs.

Built on [djtools](https://github.com/nateranda/djtools) (MIT, Go), extended
in `third_party/djtools`.

## How it works

1. **Load playlists.** The app copies `Database2/m.db` and `Database2/hm.db`
   (plus any `-wal`, `-shm` or `-journal` files next to them) from your Engine
   Library folder to a temporary folder, reads the copy, and deletes it. SQLite
   never opens your real database. Music files are not copied.
2. **Pick playlists.** Ticking a folder ticks everything inside it. If you
   tick a child but not its parent, the parent is kept as a folder only.
3. **Convert.** Writes the XML to a temporary file next to the target and then
   renames it, so a failed run never leaves a half-written file.

Close Engine DJ before loading. Copying a database while Engine is writing to
it could give an inconsistent snapshot. Your source is still not modified, but
the export might be incomplete.

## Using the XML in rekordbox

As far as I know (verify against your rekordbox version, menu names move
between releases): enable the *rekordbox xml* view in Preferences > View, then
set Preferences > Advanced > Database > rekordbox xml > Imported Library to the
exported file. The playlists then appear under "rekordbox xml" in the browser
and can be imported into the collection and exported to USB.

Test with a small playlist first, and check that hot cues and beatgrids land
where you expect on your own files (see Known limitations).

## Building

Requirements: Go (version in `go.mod`) and a C compiler, because both the
SQLite driver (`mattn/go-sqlite3`) and Fyne use cgo. That also means you build
on each OS rather than cross-compiling.

```sh
go mod tidy          # first time; commit the resulting go.sum
go test ./...
go run ./cmd/engine2rb
```

Packaging with the Fyne CLI:

```sh
go install fyne.io/tools/cmd/fyne@v1.7.4
fyne package --target darwin  --src cmd/engine2rb --app-id engine2rb.converter --release   # on a Mac
fyne package --target windows --src cmd/engine2rb --app-id engine2rb.converter --release   # on Windows
```

`.github/workflows/build.yml` does this on GitHub Actions: tests on Linux,
a Windows `.exe`, and a macOS `.app` (universal if the Intel build succeeds,
otherwise Apple Silicon only). Download the results from the run's Artifacts.

The builds are unsigned. Windows SmartScreen will warn on first launch ("More
info" > "Run anyway"). On macOS, right-click the app and choose Open, or run
`xattr -d com.apple.quarantine Engine2RB.app`.

## Changes to djtools

Applied as two commits (also in `patches/` for upstreaming or your own fork):

- `engine.Snapshot`, `engine.ImportSnapshot`, `engine.ValidateLibrary`,
  `engine.SchemaVersion`: copy-then-read import.
- `engine.ImportOptions.PathBase`: resolve Engine's relative track paths
  against the real library folder when reading a copy. Absolute paths are kept.
- `lib.Library.FilterPlaylists`, `PruneUnusedSongs`, `DedupePlaylistNames`.
- Fixes: databases are now closed after import (needed on Windows to delete the
  temporary copy); `CheckCorruptedSongs` no longer skips or panics when
  removing several songs; macOS paths no longer become `file://localhost//Users/…`;
  performance data for a track missing from the Track table no longer crashes;
  the XML now starts with an `<?xml …?>` declaration.

To use a published fork instead of the vendored copy, change the `replace`
line in `go.mod`.

## Known limitations

- Smart playlists are not converted (djtools reads them but does not convert them).
- Empty playlists are skipped.
- MP3 cue offset correction is not implemented in djtools. Some MP3s may show
  cues slightly early or late in rekordbox.
- Tested only against djtools' sample databases (Engine schema 3.0.1). Check
  the schema version shown in the log after loading your own library.
