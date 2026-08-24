package tui

import (
	"github.com/bprendie/omasub/internal/subsonic"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleEnter() (Model, tea.Cmd) {
	if m.input.Focused() {
		if m.savingQueue {
			name := m.input.Value()
			if name == "" {
				return m, nil
			}
			return m.saveQueueAsPrivatePlaylist(name)
		}
		if m.privateRenaming != "" {
			name := m.input.Value()
			if name == "" {
				return m, nil
			}
			return m.renamePrivatePlaylist(name)
		}
		if m.renaming != nil {
			name := m.input.Value()
			if name == "" {
				return m, nil
			}
			playlist := *m.renaming
			m.renaming = nil
			m.resetInput()
			m.beginSearch("renaming playlist")
			return m, m.renamePlaylist(playlist.ID, name)
		}
		q := m.input.Value()
		if q == "" {
			return m, nil
		}
		m.beginSearch("searching")
		return m, m.search(q)
	}
	if it, ok := m.list.SelectedItem().(item); ok {
		switch it.kind {
		case "home":
			return m.handleHomeAction(it)
		case "album":
			m.playlistViewID = ""
			m.pushNav()
			m.beginSearch("loading album")
			return m, m.loadAlbum(it.album.ID)
		case "playlist":
			m.playlistViewID = it.playlist.ID
			m.pushNav()
			m.beginSearch("loading playlist")
			return m, m.loadPlaylist(it.playlist.ID)
		case "private_playlist":
			return m.loadPrivatePlaylist(it)
		case "song", "queue":
			return m.playSelectedTrack(it)
		default:
			return m, nil
		}
	}
	return m, nil
}
func (m Model) createStation() (Model, tea.Cmd) {
	seed, ok := m.selectedTrack()
	if !ok && m.playing != nil {
		seed, ok = *m.playing, true
	}
	if !ok {
		m.err = "select or play a song first"
		return m, noop
	}
	m.beginSearch("creating station")
	return m, m.createStationFrom(seed)
}
func (m Model) startRename() (Model, tea.Cmd) {
	if m.mode == modePrivatePlaylists {
		return m.startPrivatePlaylistRename()
	}
	if it, ok := m.list.SelectedItem().(item); ok && it.kind == "playlist" {
		playlist := it.playlist
		m.renaming = &playlist
		m.input.Prompt = "rename > "
		m.input.SetValue(playlist.Name)
		m.input.Focus()
		m.status = "enter playlist name"
		m.err = ""
		return m, noop
	}
	if m.station != nil {
		playlist := *m.station
		m.renaming = &playlist
		m.input.Prompt = "rename > "
		m.input.SetValue(playlist.Name)
		m.input.Focus()
		m.status = "enter station playlist name"
		m.err = ""
		return m, noop
	}
	m.err = "select a playlist to rename"
	return m, noop
}
func (m Model) selectedTrack() (subsonic.Track, bool) {
	it, ok := m.list.SelectedItem().(item)
	if !ok || (it.kind != "song" && it.kind != "queue") {
		return subsonic.Track{}, false
	}
	return it.track, true
}
func (m *Model) renameLoadedPlaylist(id, name string) {
	if m.station != nil && m.station.ID == id {
		m.station.Name = name
	}
	items := m.list.Items()
	for i, row := range items {
		it, ok := row.(item)
		if !ok || it.kind != "playlist" || it.playlist.ID != id {
			continue
		}
		it.playlist.Name = name
		items[i] = it
		m.list.SetItems(items)
		return
	}
}
func stationName(seed subsonic.Track) string {
	if seed.Artist == "" {
		return "Radio - " + seed.Title
	}
	return "Radio - " + seed.Artist + " - " + seed.Title
}
func (m *Model) beginSearch(status string) tea.Cmd {
	m.status = status
	m.err = ""
	m.searching = true
	return m.spinner.Tick
}
