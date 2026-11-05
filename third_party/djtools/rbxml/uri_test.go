package rbxml

import (
	"runtime"
	"testing"
)

func TestPathToURIAbsoluteUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix path test")
	}
	got := pathToURI("/Users/tom/Music/A & B.mp3")
	want := "file://localhost/Users/tom/Music/A%20&%20B.mp3"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	back, err := uriToPath(got)
	if err != nil || back != "/Users/tom/Music/A & B.mp3" {
		t.Fatalf("round trip got %q %v", back, err)
	}
}
