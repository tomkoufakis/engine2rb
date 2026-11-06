package engine

import "testing"

func TestPopulatePlaylistsSkipsForeignAndMissingTracks(t *testing.T) {
	resolver := trackResolver{
		databaseUuid: "local",
		known:        map[int]struct{}{1: {}, 2: {}, 3: {}},
	}
	entities := []playlistEntity{
		{id: 10, listId: 1, trackId: 1, nextEntityId: 11, databaseUuid: "local"},
		// same trackId as a local track but from another library: must not match
		{id: 11, listId: 1, trackId: 2, nextEntityId: 12, databaseUuid: "other"},
		{id: 12, listId: 1, trackId: 3, nextEntityId: 13, databaseUuid: "local"},
		// local library but the track no longer exists
		{id: 13, listId: 1, trackId: 99, nextEntityId: 0, databaseUuid: "local"},
		// playlist whose entries all dangle
		{id: 20, listId: 2, trackId: 98, nextEntityId: 0, databaseUuid: "local"},
	}
	playlists := []playlist{{id: 1}, {id: 2}}

	got, err := populatePlaylists(entities, playlists, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 3}; !equal(got[0].songs, want) {
		t.Fatalf("songs = %v, want %v", got[0].songs, want)
	}
	if got[0].unresolved != 2 {
		t.Fatalf("unresolved = %d, want 2", got[0].unresolved)
	}
	if len(got[1].songs) != 0 || got[1].unresolved != 1 {
		t.Fatalf("playlist 2: songs %v unresolved %d", got[1].songs, got[1].unresolved)
	}
}

func TestTrackResolverUnknownUuidAcceptsKnownTracks(t *testing.T) {
	r := trackResolver{known: map[int]struct{}{5: {}}}
	if !r.resolves(playlistEntity{trackId: 5, databaseUuid: "anything"}) {
		t.Fatal("with no library uuid, known tracks should resolve")
	}
	if r.resolves(playlistEntity{trackId: 6}) {
		t.Fatal("unknown track must not resolve")
	}
}
