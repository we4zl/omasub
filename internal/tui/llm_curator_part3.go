package tui

import (
	"fmt"
	"github.com/bprendie/omasub/internal/curator"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/playqueue"
	"github.com/bprendie/omasub/internal/subsonic"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func discoveryAnchorCandidates(cached []localstore.CachedTrack, limit int) []subsonic.Track {
	var newest, backNine []subsonic.Track
	for _, candidate := range cached {
		if candidate.New {
			newest = append(newest, candidate.Track)
		} else {
			backNine = append(backNine, candidate.Track)
		}
	}
	out := make([]subsonic.Track, 0, min(limit, len(cached)))
	for i, j := 0, 0; len(out) < limit && (i < len(newest) || j < len(backNine)); {
		if i < len(newest) {
			out = append(out, newest[i])
			i++
		}
		if len(out) == limit {
			break
		}
		if j < len(backNine) {
			out = append(out, backNine[j])
			j++
		}
	}
	return out
}
func anchorSummary(anchors []subsonic.Track) string {
	names := make([]string, 0, len(anchors))
	for _, anchor := range anchors {
		label := strings.TrimSpace(anchor.Artist + " - " + anchor.Title)
		if label != "-" && label != "" {
			names = append(names, label)
		}
	}
	return strings.Join(names, ", ")
}
func authoritativeGrounding(anchors []subsonic.Track) []subsonic.Track {
	if len(anchors) == 0 {
		return nil
	}
	return anchors[:1]
}
func preservedPartialCuratorError(err error, accepted, total int) error {
	return fmt.Errorf("curation stopped at %d/%d; playable partial playlist preserved: %w", accepted, total, err)
}
func enrichedCuratorRunPayload(provider, model string, result curator.Result, request curatorRequest, method string, intent curator.Intent, launchTracks []subsonic.Track, candidateSnapshotID string, candidateCount int) map[string]any {
	payload := curator.RunPayload(provider, model, result)
	payload["curator_mode"] = request.Mode
	payload["user_prompt"] = request.Prompt
	payload["grounding_method"] = method
	payload["interpreted_intent"] = intent
	payload["candidate_snapshot_id"] = candidateSnapshotID
	payload["candidate_count"] = candidateCount
	if len(launchTracks) > 0 {
		payload["authoritative_seed"] = launchTracks[0]
		payload["launch_tracks"] = launchTracks
	}
	return payload
}
func waitCuratorEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}
func curatorPlaylistName(mode curator.Mode) string {
	if mode == curator.ModeMood {
		return "Mood"
	}
	return "AI Mix"
}
func curatorPlaylistLimit(mode curator.Mode) int {
	if mode == curator.ModeMood {
		return 20
	}
	return 40
}
func (m Model) applyLLMQueue(msg llmQueueMsg) Model {
	snapshot := playqueue.Snapshot{Tracks: msg.result.Tracks, Current: 0}
	if msg.request.Destination == curatorDestinationServer && m.playing != nil {
		current := -1
		for i, track := range snapshot.Tracks {
			if track.ID == m.playing.ID {
				current = i
				break
			}
		}
		if current >= 0 {
			snapshot.Current = current
		} else {
			snapshot.Tracks = append([]subsonic.Track{*m.playing}, snapshot.Tracks...)
			snapshot.Current = 0
		}
	}
	if msg.request.Destination == curatorDestinationServer {
		m.queue = playqueue.FromSnapshot(snapshot)
		m.queueTitle = msg.request.PlaylistName
		m.persistQueue()
	}
	if msg.request.Destination == curatorDestinationVault {
		m.queueTitle = msg.request.PlaylistName
		m.persistQueue()
		items := make([]list.Item, 0, len(msg.privatePlaylists))
		for _, playlist := range msg.privatePlaylists {
			items = append(items, item{kind: "private_playlist", privatePlaylist: playlist})
		}
		m.clearNav()
		m.mode = modePrivatePlaylists
		m.refreshTitle()
		m.list.SetItems(items)
		for i, row := range m.list.Items() {
			if it, ok := row.(item); ok && it.kind == "private_playlist" && it.privatePlaylist.ID == msg.privatePlaylist.ID {
				m.list.Select(i)
				break
			}
		}
	}
	if msg.request.Destination == curatorDestinationServer {
		if m.mode == modeTracks && m.playlistViewID == msg.playlist.ID {
			cursor := m.list.Index()
			m.list.SetItems(trackItems(msg.result.Tracks))
			if len(msg.result.Tracks) > 0 {
				m.list.Select(min(cursor, len(msg.result.Tracks)-1))
			}
		} else {
			items := make([]list.Item, 0, len(msg.playlists))
			for _, playlist := range msg.playlists {
				items = append(items, item{kind: "playlist", playlist: playlist})
			}
			m.clearNav()
			m.mode = modePlaylists
			m.refreshTitle()
			m.list.SetItems(items)
			for i, row := range m.list.Items() {
				if it, ok := row.(item); ok && it.kind == "playlist" && it.playlist.ID == msg.playlist.ID {
					m.list.Select(i)
					break
				}
			}
		}
	}
	m.status = fmt.Sprintf("Omasub Curator curated %d tracks", len(msg.result.Tracks))
	if len(msg.result.Rejected) > 0 {
		m.status += fmt.Sprintf("; rejected %d invented ids", len(msg.result.Rejected))
	}
	if len(msg.result.Fallback) > 0 {
		m.status += fmt.Sprintf("; library-filled %d slots", len(msg.result.Fallback))
	}
	m.err = ""
	m.searching = false
	m.curating = false
	m.curatorCancel = nil
	m.curatorID = ""
	return m
}
func (m *Model) applyCuratorPlaylistStarted(msg curatorPlaylistStartedMsg) {
	m.curatorPlaylistID = msg.playlist.ID
	m.curatorPlaylistName = msg.name
	items := make([]list.Item, 0, len(msg.playlists))
	for _, playlist := range msg.playlists {
		items = append(items, item{kind: "playlist", playlist: playlist})
	}
	m.clearNav()
	m.mode = modePlaylists
	m.refreshTitle()
	m.list.SetItems(items)
	for i, row := range m.list.Items() {
		if it, ok := row.(item); ok && it.kind == "playlist" && it.playlist.ID == msg.playlist.ID {
			m.list.Select(i)
			break
		}
	}
	m.status = fmt.Sprintf("%s created on server; curating", msg.name)
}
func (m *Model) applyCuratorPrivateStarted(msg curatorPrivateStartedMsg) {
	m.curatorPlaylistID = msg.playlist.ID
	m.curatorPlaylistName = msg.name
	items := make([]list.Item, 0, len(msg.playlists))
	for _, playlist := range msg.playlists {
		items = append(items, item{kind: "private_playlist", privatePlaylist: playlist})
	}
	m.clearNav()
	m.mode = modePrivatePlaylists
	m.refreshTitle()
	m.list.SetItems(items)
	for i, row := range m.list.Items() {
		if it, ok := row.(item); ok && it.kind == "private_playlist" && it.privatePlaylist.ID == msg.playlist.ID {
			m.list.Select(i)
			break
		}
	}
	m.status = fmt.Sprintf("%s created in private vault; curating", msg.name)
}
func (m Model) applyCuratorAnchorsStarted(msg curatorAnchorsStartedMsg) (Model, tea.Cmd) {
	m.applyCuratorPrivateStarted(curatorPrivateStartedMsg{generation: msg.generation, name: msg.name, playlist: msg.playlist, playlists: msg.playlists})
	m.curatorTracks = append([]subsonic.Track(nil), msg.anchors...)
	m.queue.Replace(msg.anchors, 0)
	m.queueTitle = msg.name + " (building)"
	m.queueSourceID = msg.playlist.ID
	m.resetPlaybackTraversal()
	m.persistQueue()
	m.status = fmt.Sprintf("playing anchor 1/3; curating %s 3/40", msg.name)
	if len(msg.anchors) == 0 {
		return m, noop
	}
	cmd := m.play(msg.anchors[0])
	return m, cmd
}
func (m *Model) applyCuratorProgress(msg curatorProgressMsg) {
	seen := false
	for _, track := range m.curatorTracks {
		if track.ID == msg.track.ID {
			seen = true
			break
		}
	}
	if !seen {
		m.curatorTracks = append(m.curatorTracks, msg.track)
	}
	m.refreshStreamingCuratorView(msg.accepted)
	if m.activeCurator.Destination == curatorDestinationServer || (m.activeCurator.Destination == curatorDestinationVault && len(m.curatorTracks) >= 3) {
		m.spliceCuratorTracks()
	}
	m.status = fmt.Sprintf("curating %s %d/%d", m.curatorPlaylistName, msg.accepted, msg.total)
}
