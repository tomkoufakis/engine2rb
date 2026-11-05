package main

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"engine2rb/internal/convert"

	"github.com/nateranda/djtools/lib"
)

func newTestUI(t *testing.T) *ui {
	test.NewTempApp(t)
	u := &ui{checked: map[string]bool{}}
	u.win = test.NewTempWindow(t, nil)
	u.win.SetContent(u.build())
	u.setLibrary(&convert.Loaded{Library: lib.Library{Playlists: []lib.Playlist{
		{PlaylistID: 1, Name: "House", Songs: []int{1}, SubPlaylists: []lib.Playlist{
			{PlaylistID: 2, Name: "Deep", Songs: []int{2}},
			{PlaylistID: 3, Name: "Tech", Songs: []int{3}},
		}},
		{PlaylistID: 4, Name: "Other", Songs: []int{4}},
	}}})
	return u
}

func checkFor(u *ui, id string) *widget.Check {
	c := u.createNode(false).(*widget.Check)
	u.updateNode(id, false, c)
	return c
}

func TestTickingFolderSelectsChildren(t *testing.T) {
	u := newTestUI(t)
	if !u.convertBtn.Disabled() {
		t.Fatal("convert should start disabled")
	}
	test.Tap(checkFor(u, "1"))
	if got := u.selectedIDs(); !reflect.DeepEqual(got, map[int]bool{1: true, 2: true, 3: true}) {
		t.Fatalf("selected %v", got)
	}
	if u.convertBtn.Disabled() {
		t.Fatal("convert should be enabled after selecting")
	}

	// unticking one child leaves the parent partially checked
	test.Tap(checkFor(u, "3"))
	parent := checkFor(u, "1")
	if parent.Checked || !parent.Partial {
		t.Fatalf("parent checked=%v partial=%v", parent.Checked, parent.Partial)
	}
	if got := u.selectedIDs(); !reflect.DeepEqual(got, map[int]bool{1: true, 2: true}) {
		t.Fatalf("selected %v", got)
	}

	u.setAll(false)
	if len(u.selectedIDs()) != 0 || !u.convertBtn.Disabled() {
		t.Fatal("select none failed")
	}
	u.setAll(true)
	if len(u.selectedIDs()) != 4 {
		t.Fatal("select all failed")
	}
	if u.summary.Text != "4 of 4 playlists selected" {
		t.Fatalf("summary %q", u.summary.Text)
	}
}
