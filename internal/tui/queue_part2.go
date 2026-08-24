package tui

import (
	"github.com/bprendie/omasub/internal/subsonic"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) moveQueueSelection(delta int) (Model, tea.Cmd) {
	if m.mode != modeQueue {
		return m, nil
	}
	it, ok := m.list.SelectedItem().(item)
	if !ok || it.kind != "queue" || !m.queue.Move(it.queueIndex, delta) {
		m.err = "queue track cannot move farther"
		return m, noop
	}
	m.persistQueue()
	m.refreshQueueView()
	m.list.Select(it.queueIndex + delta)
	m.status = "queue reordered"
	m.err = ""
	return m, noop
}
func (m Model) trackContext(selectedID string) ([]subsonic.Track, int) {
	items := m.list.Items()
	tracks := make([]subsonic.Track, 0, len(items))
	index := 0
	for _, row := range items {
		it, ok := row.(item)
		if !ok || it.kind != "song" || it.track.ID == "" {
			continue
		}
		if it.track.ID == selectedID {
			index = len(tracks)
		}
		tracks = append(tracks, it.track)
	}
	if len(tracks) == 0 {
		return []subsonic.Track{{ID: selectedID}}, 0
	}
	return tracks, index
}
func (m Model) selectedOrPlayingTrack() (subsonic.Track, bool) {
	if track, ok := m.selectedTrack(); ok {
		return track, true
	}
	if m.playing != nil {
		return *m.playing, true
	}
	return subsonic.Track{}, false
}
