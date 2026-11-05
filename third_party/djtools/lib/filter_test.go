package lib

import (
	"reflect"
	"testing"
)

func sampleLibrary() Library {
	return Library{
		Songs: []Song{{SongID: 1}, {SongID: 2}, {SongID: 3}, {SongID: 4}},
		Playlists: []Playlist{
			{PlaylistID: 1, Name: "House", Songs: []int{1}, SubPlaylists: []Playlist{
				{PlaylistID: 2, Name: "Deep", Songs: []int{2}},
				{PlaylistID: 3, Name: "Tech", Songs: []int{3}},
			}},
			{PlaylistID: 4, Name: "Other", Songs: []int{4}},
		},
	}
}

func TestFilterKeepsAncestorsAsFolders(t *testing.T) {
	l := sampleLibrary()
	out := l.FilterPlaylists(map[int]bool{3: true})
	want := []Playlist{{PlaylistID: 1, Name: "House", SubPlaylists: []Playlist{
		{PlaylistID: 3, Name: "Tech", Songs: []int{3}},
	}}}
	if !reflect.DeepEqual(out.Playlists, want) {
		t.Fatalf("got %+v", out.Playlists)
	}
	out.PruneUnusedSongs()
	if len(out.Songs) != 1 || out.Songs[0].SongID != 3 {
		t.Fatalf("prune: got %+v", out.Songs)
	}
	// original untouched
	if len(l.Playlists) != 2 || len(l.Songs) != 4 {
		t.Fatal("source library was modified")
	}
}

func TestFilterNothingSelected(t *testing.T) {
	l := sampleLibrary()
	out := l.FilterPlaylists(map[int]bool{})
	if out.Playlists != nil {
		t.Fatalf("expected no playlists, got %+v", out.Playlists)
	}
}

func TestDedupe(t *testing.T) {
	l := Library{Playlists: []Playlist{
		{Name: "A"}, {Name: "A"}, {Name: "B", SubPlaylists: []Playlist{{Name: "x"}, {Name: "x"}}}, {Name: "A"},
	}}
	if n := l.DedupePlaylistNames(); n != 3 {
		t.Fatalf("renamed %d, want 3", n)
	}
	got := []string{l.Playlists[0].Name, l.Playlists[1].Name, l.Playlists[3].Name, l.Playlists[2].SubPlaylists[1].Name}
	want := []string{"A", "A (2)", "A (3)", "x (2)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestCheckCorruptedSongsAdjacent(t *testing.T) {
	l := Library{
		Songs:     []Song{{SongID: 1}, {SongID: 2, Corrupt: true}, {SongID: 3, Corrupt: true}},
		Playlists: []Playlist{{Name: "p", Songs: []int{1, 2, 3}}},
	}
	l.CheckCorruptedSongs()
	if len(l.Songs) != 1 || l.Songs[0].SongID != 1 || !reflect.DeepEqual(l.Playlists[0].Songs, []int{1}) {
		t.Fatalf("got %+v %+v", l.Songs, l.Playlists)
	}
}
