package tui

import (
	"context"
	"fmt"
	"github.com/bprendie/omasub/internal/audio"
	"github.com/bprendie/omasub/internal/config"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/player"
	"github.com/bprendie/omasub/internal/playqueue"
	"github.com/bprendie/omasub/internal/state"
	"github.com/bprendie/omasub/internal/subsonic"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"image"
	"time"
)

type mode int
type playbackMode int
type paneFocus int

const (
	focusMain paneFocus = iota
	focusSidebar
)
const (
	playbackNormal playbackMode = iota
	playbackShuffle
	playbackShuffleRepeat
	playbackRepeatAll
	playbackRepeatOne
)
const (
	modeHome mode = iota
	modeNewest
	modeRandomAlbums
	modePlaylists
	modeTracks
	modeSearch
	modeQueue
	modePrivatePlaylists
	modeStation
	modeLastPlayed
	modeSetup
	modeLLMProvider
	modeLLMServer
	modeLLMLoading
	modeLLMModel
	modeCuratorChoice
	modeCuratorPrompt
	modePlaylistDelete
	modeVault
)
const searchPrompt = "stream > "

type Model struct {
	cfg                 config.Config
	styles              styles
	client              subsonic.Client
	player              *player.Player
	meter               *audio.Meter
	list                list.Model
	input               textinput.Model
	setup               []textinput.Model
	setupFocus          int
	llmDraft            llmConfigDraft
	curatorDraft        curatorDraft
	playlistDelete      playlistDeleteDraft
	activeCurator       curatorRequest
	vaultInput          textinput.Model
	vaultStore          *localstore.Store
	vaultStage          string
	vaultPass           string
	spinner             spinner.Model
	mode                mode
	nav                 []navSnapshot
	appState            state.State
	status              string
	err                 string
	searching           bool
	curating            bool
	curatorStarted      time.Time
	curatorID           string
	curatorEvents       chan tea.Msg
	curatorCancel       context.CancelFunc
	curatorTracks       []subsonic.Track
	curatorPlaylistID   string
	curatorPlaylistName string
	playlistViewID      string
	playing             *subsonic.Track
	playSource          string
	queue               playqueue.Queue
	queueTitle          string
	queueSourceID       string
	cacheStatus         localstore.CacheStatus
	paused              bool
	playbackMode        playbackMode
	shuffleUpcoming     []int
	shuffleReady        bool
	playHistory         []int
	playerStarted       bool
	trackTitle          string
	titlePoll           time.Time
	coverID             string
	coverArt            image.Image
	coverErr            string
	coverCache          map[string]image.Image
	energy              audio.Sample
	renaming            *subsonic.Playlist
	savingQueue         bool
	privateRenaming     string
	station             *subsonic.Playlist
	width               int
	height              int
	helpOpen            bool
	focus               paneFocus
	sidebarIndex        int
	visualizer          Visualizer
	remoteEnabled       bool
	remotePublished     time.Time
	themeSignature      string
	themeChecked        time.Time
}

type item struct {
	kind            string
	title           string
	desc            string
	track           subsonic.Track
	album           subsonic.Album
	playlist        subsonic.Playlist
	privatePlaylist localstore.PrivatePlaylist
	action          string
	queueIndex      int
}
type navSnapshot struct {
	mode   mode
	items  []list.Item
	status string
	cursor int
}
type llmConfigDraft struct {
	ProviderType  string
	ServerURL     string
	Model         string
	Models        []string
	FetchErr      string
	ProviderIndex int
	ModelIndex    int
	PreviousInput string
}

func (i item) Title() string {
	switch i.kind {
	case "album":
		return i.album.Name
	case "playlist":
		return i.playlist.Name
	case "private_playlist":
		return i.privatePlaylist.Name
	case "empty", "home", "queue":
		return i.title
	default:
		return i.track.Title
	}
}
func (i item) Description() string {
	switch i.kind {
	case "album":
		return fmt.Sprintf("%s  %d", i.album.Artist, i.album.Year)
	case "playlist":
		return fmt.Sprintf("%d tracks", i.playlist.Count)
	case "private_playlist":
		return fmt.Sprintf("private vault playlist  %d tracks", len(i.privatePlaylist.Tracks))
	case "empty", "home", "queue":
		return i.desc
	default:
		return fmt.Sprintf("%s  %s", i.track.Artist, i.track.Album)
	}
}
func (i item) FilterValue() string {
	return i.Title() + " " + i.Description()
}

type loadedMsg struct {
	items  []list.Item
	status string
	mode   mode
}
type stationMsg struct {
	playlist subsonic.Playlist
	tracks   []subsonic.Track
}
type errMsg struct{ err error }
type renamedMsg struct {
	id   string
	name string
}
type titleMsg struct{ title string }
type scrobbleMsg struct {
	trackID   string
	submitted bool
	err       error
}
type tickMsg time.Time
type setupSavedMsg struct {
	cfg    config.Config
	status string
}
type llmModelsMsg struct {
	models []string
	err    error
}
type coverArtMsg struct {
	id  string
	img image.Image
	err error
}

func New(cfg config.Config) Model {
	input := textinput.New()
	input.Placeholder = "song, artist, or album"
	input.Prompt = searchPrompt
	input.CharLimit = 240
	input.Width = 42
	vaultInput := textinput.New()
	vaultInput.EchoMode = textinput.EchoPassword
	vaultInput.CharLimit = 240
	vaultInput.Width = 42
	appStyles := newStyles()
	l := list.New(nil, delegate{styles: appStyles}, 80, 20)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	appState, stateErr := state.Load()
	m := Model{cfg: cfg, styles: appStyles, client: subsonic.New(cfg.Server, cfg.Username, cfg.Password), player: player.New(), list: l, input: input, vaultInput: vaultInput, spinner: newSpinner(), mode: modeHome, status: "ready", appState: appState, coverCache: map[string]image.Image{}, queue: playqueue.New(), queueTitle: "queue", visualizer: NewVisualizer(harmonica.FPS(30)), themeSignature: activeThemeSignature}
	if stateErr != nil {
		m.err = stateErr.Error()
	}
	m.setup = newSetupInputs(cfg)
	m.applyThemeStyles()
	if !cfg.Ready() {
		m.mode = modeSetup
		m.status = "connect a Subsonic server"
		m.refreshTitle()
		return m
	}
	if err := m.prepareVault(); err != nil {
		m.err = err.Error()
	}
	if m.mode != modeVault {
		m.restoreQueueSnapshot()
		m.refreshCacheStatus()
		m.showHome()
	}
	m.refreshTitle()
	return m
}
func (m Model) Init() tea.Cmd {
	if !m.cfg.Ready() || m.mode == modeVault {
		return tick()
	}
	return tick()
}
