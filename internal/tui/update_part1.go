package tui

import (
	"fmt"
	"github.com/bprendie/omasub/internal/remote"
	"github.com/bprendie/omasub/internal/subsonic"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	"time"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case remote.Command:
		var cmd tea.Cmd
		switch msg {
		case remote.Toggle:
			m.togglePause()
		case remote.Next:
			m, cmd = m.playNext()
		case remote.Previous:
			m, cmd = m.playPrevious()
		case remote.Stop:
			m.stop()
		case remote.CycleMode:
			m.cyclePlaybackMode()
			m.publishRemote(true)
		case remote.Quit:
			m.stop()
			m.closeVaultStore()
			return m, tea.Quit
		case remote.Visible:
			m.terminalVisible = true
		case remote.Hidden:
			m.terminalVisible = false
		}
		cmds = append(cmds, cmd)
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
	case tea.KeyMsg:
		next, cmd := m.handleKey(msg)
		m = next
		if cmd != nil {
			return m, cmd
		}
	case spinner.TickMsg:
		if m.searching || m.curating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	case llmQueueMsg:
		if msg.generation == m.curatorID {
			m = m.applyLLMQueue(msg)
		}
	case curatorPlaylistStartedMsg:
		if msg.generation == m.curatorID {
			m.applyCuratorPlaylistStarted(msg)
			cmds = append(cmds, waitCuratorEvent(m.curatorEvents))
		}
	case curatorPrivateStartedMsg:
		if msg.generation == m.curatorID {
			m.applyCuratorPrivateStarted(msg)
			cmds = append(cmds, waitCuratorEvent(m.curatorEvents))
		}
	case curatorAnchorsStartedMsg:
		if msg.generation == m.curatorID {
			var cmd tea.Cmd
			m, cmd = m.applyCuratorAnchorsStarted(msg)
			cmds = append(cmds, cmd, waitCuratorEvent(m.curatorEvents))
		}
	case curatorProgressMsg:
		if msg.generation == m.curatorID {
			m.applyCuratorProgress(msg)
			cmds = append(cmds, waitCuratorEvent(m.curatorEvents))
		}
	case curatorStageMsg:
		if msg.generation == m.curatorID {
			m.status = msg.status
			cmds = append(cmds, waitCuratorEvent(m.curatorEvents))
		}
	case playlistCopiedMsg:
		m.status = fmt.Sprintf("copied %s to %s", msg.name, msg.destination)
		m.err = ""
		m.searching = false
	case playlistDeletedMsg:
		m = m.applyPlaylistDeleted(msg)
	case llmModelsMsg:
		var cmd tea.Cmd
		m, cmd = m.handleLLMModelsMsg(msg)
		cmds = append(cmds, cmd)
	case cacheSyncMsg:
		m.cacheStatus = msg.status
		m.status = fmt.Sprintf("synced %d cached tracks", msg.tracks)
		m.err = ""
		m.searching = false
	case loadedMsg:
		m.mode = msg.mode
		if msg.mode != modeStation {
			m.station = nil
		}
		m.refreshTitle()
		m.list.SetItems(msg.items)
		m.status = msg.status
		m.err = ""
		m.searching = false
		m.input.Blur()
	case errMsg:
		if m.curating && strings.TrimSpace(m.status) != "" {
			m.err = m.status + ": " + msg.err.Error()
		} else {
			m.err = msg.err.Error()
		}
		m.searching = false
		m.curating = false
	case stationMsg:
		m.station = &msg.playlist
		m.mode = modeStation
		m.refreshTitle()
		m.list.SetItems(trackItems(msg.tracks))
		m.status = fmt.Sprintf("saved station %q with %d tracks", msg.playlist.Name, len(msg.tracks))
		m.err = ""
		m.searching = false
		m.input.Blur()
	case renamedMsg:
		m.renameLoadedPlaylist(msg.id, msg.name)
		m.status = "renamed playlist"
		m.err = ""
		m.searching = false
	case setupSavedMsg:
		m.cfg = msg.cfg
		m.client = subsonic.New(msg.cfg.Server, msg.cfg.Username, msg.cfg.Password)
		m.status = msg.status
		m.err = ""
		m.searching = false
		if msg.cfg.Ready() {
			if err := m.prepareVault(); err != nil {
				m.err = err.Error()
			}
			m.refreshTitle()
			return m, m.tick()
		}
	case coverArtMsg:
		if msg.id != m.coverID {
			break
		}
		if msg.err != nil {
			m.coverArt = nil
			m.coverErr = msg.err.Error()
			break
		}
		m.coverArt = msg.img
		m.coverErr = ""
		if msg.img != nil {
			m.coverCache[msg.id] = msg.img
		}
	case titleMsg:
		if msg.title != "" {
			m.trackTitle = msg.title
			m.publishRemote(true)
		}
	case scrobbleMsg:
		if msg.err != nil {
			m.err = "scrobble: " + msg.err.Error()
		}
	case tickMsg:
		m.refreshTheme(time.Time(msg))
		m.publishRemote()
		if m.playerStarted && m.isPlaying() && !m.paused && !m.player.Running() {
			finishedID := m.playing.ID
			m.playerStarted = false
			var cmd tea.Cmd
			m, cmd = m.playAfterFinished()
			cmds = append(cmds, m.scrobble(finishedID, true), cmd)
		}
		m.visualizer.Step(m.isPlaying() && !m.paused && m.terminalVisible)
		if m.isPlaying() && !m.paused && time.Time(msg).Sub(m.titlePoll) > 2*time.Second {
			m.titlePoll = time.Time(msg)
			cmds = append(cmds, fetchTitle(m.player))
		}
		cmds = append(cmds, m.tick())
	}
	next, cmd := m.updateFocused(msg)
	m = next
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}
