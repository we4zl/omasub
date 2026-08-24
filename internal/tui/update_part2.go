package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.stop()
		m.closeVaultStore()
		return m, tea.Quit
	}
	if m.helpOpen {
		switch msg.String() {
		case "?", "esc", "left":
			m.helpOpen = false
			return m, noop
		case "q":
			m.stop()
			m.closeVaultStore()
			return m, tea.Quit
		default:
			return m, noop
		}
	}
	if m.mode == modeSetup {
		return m.handleSetupKey(msg)
	}
	if m.isLLMConfigMode() {
		return m.handleLLMConfigKey(msg)
	}
	if m.isCuratorInputMode() {
		return m.handleCuratorInputKey(msg)
	}
	if m.mode == modePlaylistDelete {
		return m.handlePlaylistDeleteKey(msg)
	}
	if m.mode == modeVault {
		return m.handleVaultKey(msg)
	}
	if msg.String() == "ctrl+l" {
		m.pushNav()
		return m.startLLMConfig()
	}
	if m.input.Focused() {
		switch msg.String() {
		case "enter":
			return m.handleEnter()
		case "esc":
			return m.back()
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	if m.width >= 64 && (msg.String() == "tab" || msg.String() == "shift+tab") {
		if m.focus == focusSidebar {
			m.focus = focusMain
		} else {
			m.focusSidebar()
		}
		return m, noop
	}
	if m.focus == focusSidebar {
		switch msg.String() {
		case "up", "k":
			m.sidebarIndex = (m.sidebarIndex - 1 + len(sidebarNavEntries)) % len(sidebarNavEntries)
		case "down", "j":
			m.sidebarIndex = (m.sidebarIndex + 1) % len(sidebarNavEntries)
		case "enter":
			return m.activateSidebarEntry()
		case "right", "esc":
			m.focus = focusMain
		}
		return m, noop
	}
	switch msg.String() {
	case "q":
		m.stop()
		m.closeVaultStore()
		return m, tea.Quit
	case "h":
		m.showHome()
		return m, noop
	case "1":
		m.clearNav()
		m.beginSearch("loading newest albums")
		return m, m.loadNewest()
	case "2":
		m.clearNav()
		m.beginSearch("loading playlists")
		return m, m.loadPlaylists()
	case "3":
		m.clearNav()
		m.beginSearch("loading random albums")
		return m, m.loadRandomAlbums()
	case "4":
		m.showQueue()
		return m, noop
	case "5":
		m.showPrivatePlaylists()
		return m, noop
	case "y":
		m.beginSearch("syncing Subsonic cache")
		return m, m.syncSubsonicCache()
	case "g":
		return m.generateRecommendedQueue()
	case "G":
		return m.startCuratorChoice()
	case "M":
		return m.generateMoodPlaylist()
	case "L":
		m.pushNav()
		return m.startLLMConfig()
	case "?":
		m.helpOpen = true
		return m, noop
	case "/":
		m.pushNav()
		m.mode = modeSearch
		m.refreshTitle()
		m.input.Focus()
		return m, noop
	case "enter":
		return m.handleEnter()
	case "w":
		return m.startSaveQueue()
	case "v":
		return m.copySelectedPlaylist()
	case "n":
		return m.playNext()
	case "p":
		return m.playPrevious()
	case "m":
		m.cyclePlaybackMode()
		return m, noop
	case "a":
		return m.enqueueSelected()
	case "x":
		return m.removeQueueSelection()
	case "delete":
		return m.startPlaylistDelete()
	case "c":
		return m.clearQueue()
	case "u":
		return m.moveQueueSelection(-1)
	case "d":
		return m.moveQueueSelection(1)
	case "r":
		return m.createStation()
	case "ctrl+r":
		return m.startRename()
	case "left", "esc":
		return m.back()
	case " ":
		m.togglePause()
	case "s":
		m.stop()
	}
	return m, nil
}
func (m Model) activateSidebarEntry() (Model, tea.Cmd) {
	if m.sidebarIndex < 0 || m.sidebarIndex >= len(sidebarNavEntries) {
		return m, noop
	}
	m.focus = focusMain
	switch sidebarNavEntries[m.sidebarIndex].key {
	case "h":
		m.showHome()
		return m, noop
	case "1":
		m.clearNav()
		m.beginSearch("loading newest albums")
		return m, m.loadNewest()
	case "2":
		m.clearNav()
		m.beginSearch("loading playlists")
		return m, m.loadPlaylists()
	case "3":
		m.clearNav()
		m.beginSearch("loading random albums")
		return m, m.loadRandomAlbums()
	case "4":
		m.showQueue()
		return m, noop
	case "5":
		m.showPrivatePlaylists()
		return m, noop
	case "y":
		m.beginSearch("syncing Subsonic cache")
		return m, m.syncSubsonicCache()
	case "g":
		return m.generateRecommendedQueue()
	case "G":
		return m.startCuratorChoice()
	case "M":
		return m.generateMoodPlaylist()
	case "ctrl+l":
		m.pushNav()
		return m.startLLMConfig()
	case "/":
		m.pushNav()
		m.mode = modeSearch
		m.refreshTitle()
		m.input.Focus()
		return m, noop
	}
	return m, noop
}
