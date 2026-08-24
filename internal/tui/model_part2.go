package tui

import (
	"github.com/bprendie/omasub/internal/config"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

func newSetupInputs(cfg config.Config) []textinput.Model {
	fields := []struct {
		label       string
		value       string
		placeholder string
		width       int
	}{{"server", cfg.Server, "https://your-navidrome.example", 52}, {"username", cfg.Username, "subsonic username", 32}, {"password", cfg.Password, "subsonic password", 32}}
	inputs := make([]textinput.Model, 0, len(fields))
	for i, field := range fields {
		in := textinput.New()
		in.Prompt = field.label + " > "
		in.Placeholder = field.placeholder
		in.SetValue(field.value)
		in.Width = field.width
		if field.label == "password" {
			in.EchoMode = textinput.EchoPassword
		}
		if i == 0 {
			in.Focus()
		}
		inputs = append(inputs, in)
	}
	return inputs
}
func newSpinner() spinner.Model {
	return spinner.New(spinner.WithSpinner(spinner.Jump), spinner.WithStyle(lipgloss.NewStyle().Foreground(accent)))
}
func (m *Model) refreshTitle() {
	switch m.mode {
	case modeHome:
		m.list.Title = "home"
	case modePlaylists:
		m.list.Title = "playlists"
	case modeRandomAlbums:
		m.list.Title = "random albums"
	case modeTracks:
		m.list.Title = "tracks"
	case modeSearch:
		m.list.Title = "search results"
	case modeQueue:
		m.list.Title = m.queueTitle
	case modePrivatePlaylists:
		m.list.Title = "private playlists"
	case modeStation:
		m.list.Title = "station"
	case modeLastPlayed:
		m.list.Title = "last played"
	case modeSetup:
		m.list.Title = "setup"
	case modeLLMProvider, modeLLMServer, modeLLMLoading, modeLLMModel:
		m.list.Title = "llm setup"
	case modeCuratorChoice, modeCuratorPrompt:
		m.list.Title = "ai curator"
	case modeVault:
		m.list.Title = "private vault"
	default:
		m.list.Title = "newest albums"
	}
}
