package lib

import "fmt"

// FilterPlaylists returns a copy of the Library keeping only the playlists
// whose PlaylistID is in keep. A playlist that is not itself selected but has
// a selected descendant is kept as a folder: its own songs are dropped and
// only the selected descendants remain under it. Songs are copied unchanged;
// call PruneUnusedSongs on the result to drop songs no longer referenced.
func (l *Library) FilterPlaylists(keep map[int]bool) Library {
	return Library{
		Songs:     append([]Song(nil), l.Songs...),
		Playlists: filterPlaylists(l.Playlists, keep),
	}
}

func filterPlaylists(playlists []Playlist, keep map[int]bool) []Playlist {
	var out []Playlist
	for _, p := range playlists {
		children := filterPlaylists(p.SubPlaylists, keep)
		selected := keep[p.PlaylistID]
		if !selected && children == nil {
			continue
		}
		np := Playlist{
			PlaylistID:   p.PlaylistID,
			Name:         p.Name,
			SubPlaylists: children,
		}
		if selected {
			np.Songs = append([]int(nil), p.Songs...)
		}
		out = append(out, np)
	}
	return out
}

// PruneUnusedSongs removes songs that are not referenced by any playlist.
func (l *Library) PruneUnusedSongs() {
	used := make(map[int]bool)
	var walk func([]Playlist)
	walk = func(playlists []Playlist) {
		for _, p := range playlists {
			for _, id := range p.Songs {
				used[id] = true
			}
			walk(p.SubPlaylists)
		}
	}
	walk(l.Playlists)

	kept := make([]Song, 0, len(used))
	for _, s := range l.Songs {
		if used[s.SongID] {
			kept = append(kept, s)
		}
	}
	l.Songs = kept
}

// DedupePlaylistNames renames playlists that share a name with an earlier
// sibling by appending " (2)", " (3)" and so on. Some software (Rekordbox,
// for one) does not allow two playlists or folders with the same name at the
// same level. It returns the number of playlists renamed.
func (l *Library) DedupePlaylistNames() int {
	return dedupeNames(l.Playlists)
}

func dedupeNames(playlists []Playlist) int {
	renamed := 0
	seen := make(map[string]bool)
	for i := range playlists {
		name := playlists[i].Name
		if seen[name] {
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s (%d)", playlists[i].Name, n)
				if !seen[candidate] {
					name = candidate
					break
				}
			}
			playlists[i].Name = name
			renamed++
		}
		seen[name] = true
		renamed += dedupeNames(playlists[i].SubPlaylists)
	}
	return renamed
}
