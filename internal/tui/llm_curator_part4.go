package tui

import (
	"github.com/bprendie/omasub/internal/subsonic"
)

func (m *Model) refreshStreamingCuratorView(accepted int) {
	if m.mode == modePlaylists {
		cursor := m.list.Index()
		items := m.list.Items()
		for i, row := range items {
			it, ok := row.(item)
			if !ok || it.kind != "playlist" || it.playlist.ID != m.curatorPlaylistID {
				continue
			}
			it.playlist.Count = accepted
			items[i] = it
			m.list.SetItems(items)
			m.list.Select(cursor)
			return
		}
	}
	if m.mode == modePrivatePlaylists {
		cursor := m.list.Index()
		items := m.list.Items()
		for i, row := range items {
			it, ok := row.(item)
			if !ok || it.kind != "private_playlist" || it.privatePlaylist.ID != m.curatorPlaylistID {
				continue
			}
			it.privatePlaylist.Tracks = append([]subsonic.Track(nil), m.curatorTracks...)
			it.privatePlaylist.Current = 0
			items[i] = it
			m.list.SetItems(items)
			m.list.Select(cursor)
			return
		}
	}
	if m.mode == modeTracks && m.playlistViewID == m.curatorPlaylistID {
		cursor := m.list.Index()
		m.list.SetItems(trackItems(m.curatorTracks))
		if len(m.curatorTracks) > 0 {
			m.list.Select(min(cursor, len(m.curatorTracks)-1))
		}
	}
}
func (m *Model) spliceCuratorTracks() {
	if len(m.curatorTracks) == 0 {
		return
	}
	old := m.queue.Tracks()
	current := m.queue.CurrentIndex()
	if m.playing != nil {
		for i, track := range old {
			if track.ID == m.playing.ID {
				current = i
				break
			}
		}
	}
	var prefix []subsonic.Track
	if current >= 0 && current < len(old) {
		prefix = append(prefix, old[:current+1]...)
	} else if m.playing != nil {
		prefix = append(prefix, *m.playing)
		current = 0
	}
	seen := map[string]bool{}
	for _, track := range prefix {
		seen[track.ID] = true
	}
	combined := append([]subsonic.Track(nil), prefix...)
	for _, track := range m.curatorTracks {
		if track.ID != "" && !seen[track.ID] {
			combined = append(combined, track)
			seen[track.ID] = true
		}
	}
	start := current + 1
	if start < 0 {
		start = 0
	}
	for _, track := range old[start:] {
		if track.ID != "" && !seen[track.ID] {
			combined = append(combined, track)
			seen[track.ID] = true
		}
	}
	if len(prefix) == 0 {
		current = 0
	}
	m.queue.Replace(combined, current)
	m.queueTitle = m.curatorPlaylistName + " (building)"
	m.persistQueue()
}
