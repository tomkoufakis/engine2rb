package engine

import "testing"

func ids(ps []playlist) []int {
	var r []int
	for _, p := range ps {
		r = append(r, p.id)
	}
	return r
}

func equal(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortPlaylistsMultipleGroups(t *testing.T) {
	in := []playlist{
		{id: 3, parentListId: 1, nextListId: 0},
		{id: 2, parentListId: 1, nextListId: 3},
		{id: 5, parentListId: 4, nextListId: 6},
		{id: 6, parentListId: 4, nextListId: 0},
	}
	got, err := sortPlaylists(in)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{2, 3, 5, 6}; !equal(ids(got), want) {
		t.Fatalf("got %v want %v", ids(got), want)
	}
}

func TestSortPlaylistsEmpty(t *testing.T) {
	got, err := sortPlaylists(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSortPlaylistsBrokenChains(t *testing.T) {
	// cycle with no head, plus a link to a missing sibling
	in := []playlist{
		{id: 1, parentListId: 0, nextListId: 2},
		{id: 2, parentListId: 0, nextListId: 1},
		{id: 7, parentListId: 9, nextListId: 99},
	}
	got, err := sortPlaylists(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("lost playlists: %v", ids(got))
	}
}
