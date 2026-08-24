package tui

import (
	"context"
	"fmt"
	"github.com/bprendie/omasub/internal/curator"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/subsonic"
	tea "github.com/charmbracelet/bubbletea"
	"time"
)

type llmQueueMsg struct {
	generation       string
	result           curator.Result
	run              localstore.RecommendationRun
	request          curatorRequest
	playlist         subsonic.Playlist
	playlists        []subsonic.Playlist
	privatePlaylist  localstore.PrivatePlaylist
	privatePlaylists []localstore.PrivatePlaylist
}
type curatorProgressMsg struct {
	generation string
	track      subsonic.Track
	accepted   int
	total      int
}
type curatorStageMsg struct {
	generation string
	status     string
}
type curatorPlaylistStartedMsg struct {
	generation string
	name       string
	playlist   subsonic.Playlist
	playlists  []subsonic.Playlist
}
type curatorPrivateStartedMsg struct {
	generation string
	name       string
	playlist   localstore.PrivatePlaylist
	playlists  []localstore.PrivatePlaylist
}
type curatorAnchorsStartedMsg struct {
	generation string
	name       string
	playlist   localstore.PrivatePlaylist
	playlists  []localstore.PrivatePlaylist
	anchors    []subsonic.Track
}

func (m Model) generateLLMQueue() (Model, tea.Cmd) {
	return m.startCuratorChoice()
}
func (m Model) generateMoodPlaylist() (Model, tea.Cmd) {
	seed, ok := m.moodSeed()
	if !ok {
		m.err = "play or queue a track before creating Mood"
		return m, noop
	}
	return m.generateLLMCuration(moodCuratorRequest(seed))
}
func (m Model) generateLLMCuration(request curatorRequest) (Model, tea.Cmd) {
	if !m.cfg.LLMReady() {
		m.err = "llm curator is not configured"
		return m, noop
	}
	if m.vaultStore == nil || !m.vaultStore.Unlocked() {
		m.err = "private vault is locked"
		return m, noop
	}
	m.beginSearch("curating AI mix with llm")
	if request.Mode == curator.ModeMood {
		m.status = "curating Mood from " + request.Seed.Title
	}
	m.curating = true
	m.curatorStarted = time.Now()
	if m.curatorCancel != nil {
		m.curatorCancel()
	}
	m.curatorID = fmt.Sprintf("%d", time.Now().UnixNano())
	m.curatorEvents = make(chan tea.Msg)
	m.curatorTracks = nil
	m.curatorPlaylistID = ""
	m.curatorPlaylistName = request.PlaylistName
	m.activeCurator = request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	m.curatorCancel = cancel
	spinnerCmd := m.spinner.Tick
	return m, tea.Batch(spinnerCmd, m.runLLMCurator(ctx, m.curatorID, m.curatorEvents, request), waitCuratorEvent(m.curatorEvents))
}
func (m Model) moodSeed() (subsonic.Track, bool) {
	if m.playing != nil && m.playing.ID != "" {
		return *m.playing, true
	}
	if track, ok := m.queue.Current(); ok {
		return track, true
	}
	return m.selectedTrack()
}
