package tui

import (
	"context"
	"errors"
	"fmt"
	"github.com/bprendie/omasub/internal/curator"
	"github.com/bprendie/omasub/internal/llm"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/playqueue"
	"github.com/bprendie/omasub/internal/subsonic"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

func (m Model) runLLMCurator(ctx context.Context, generation string, events chan tea.Msg, request curatorRequest) tea.Cmd {
	store := m.vaultStore
	cfg := m.cfg.LLM
	client := m.client
	return func() tea.Msg {
		defer close(events)
		var err error
		cfg, err = llm.ResolveConfig(ctx, cfg)
		if err != nil {
			return errMsg{err: err}
		}
		cached, err := store.CachedSubsonicTracks(0)
		if err != nil {
			return errMsg{err: err}
		}
		if len(cached) == 0 {
			return errMsg{err: fmt.Errorf("sync the Subsonic cache first")}
		}
		recent, err := store.RecentSubsonicTrackIDs(100)
		if err != nil {
			return errMsg{err: err}
		}
		preferred := map[string]bool{}
		llmClient := llm.New(cfg)
		var groundingAnchors []subsonic.Track
		var groundingIntent curator.Intent
		groundingMethod := "mood_seed"
		candidateSnapshotID := curator.CandidateSnapshotID(cached)
		playlistName := request.PlaylistName
		playlistLimit := request.Limit
		var priorPrivate *localstore.PrivatePlaylist
		var partialPrivateID string
		published := false
		if request.Destination == curatorDestinationVault {
			playlists, listErr := store.PrivatePlaylists()
			if listErr != nil {
				return errMsg{err: listErr}
			}
			for _, playlist := range playlists {
				if strings.EqualFold(strings.TrimSpace(playlist.Name), strings.TrimSpace(playlistName)) {
					copy := playlist
					priorPrivate = &copy
					break
				}
			}
		}
		rollbackPrivate := func() {
			if request.Destination != curatorDestinationVault || partialPrivateID == "" {
				return
			}
			if priorPrivate != nil {
				_, _ = store.SaveOrReplacePrivatePlaylist(priorPrivate.Name, priorPrivate.Snapshot())
			} else {
				_ = store.DeletePrivatePlaylist(partialPrivateID)
			}
		}
		if request.Mode == curator.ModeMood {
			if request.Seed.ID == "" {
				return errMsg{err: fmt.Errorf("play a track before creating a Mood playlist")}
			}
			if similar, err := client.Similar(ctx, request.Seed, 80); err == nil {
				for _, track := range similar {
					preferred[track.ID] = true
				}
			}
		} else {
			query := strings.TrimSpace(request.Prompt)
			intent := curator.Intent{Purpose: "cohesive discovery without taxing the gig"}
			cachedIDs := make(map[string]bool, len(cached))
			for _, candidate := range cached {
				cachedIDs[candidate.Track.ID] = true
			}
			events <- curatorStageMsg{generation: generation, status: "grounding three playable anchors"}
			primary, fastGrounded := curator.DiscoveryPrimarySeed(cached, recent)
			if query != "" {
				primary, fastGrounded = curator.ResolvePrimarySeed(query, cached, recent)
			}
			var anchors []subsonic.Track
			var authoritativeNeighborhood []subsonic.Track
			if fastGrounded {
				groundingMethod = "deterministic_cache_and_navidrome"
				similar, similarErr := client.Similar(ctx, primary, 160)
				if similarErr == nil {
					anchors, similarErr = curator.SelectDeterministicAnchors(primary, similar, cachedIDs, recent)
					authoritativeNeighborhood = similar
				}
				fastGrounded = similarErr == nil
			}
			if !fastGrounded && query != "" {
				groundingMethod = "llm_fallback"
				events <- curatorStageMsg{generation: generation, status: "interpreting your request"}
				intent, err = curator.InterpretIntent(ctx, llmClient, query)
				if err != nil {
					return errMsg{err: fmt.Errorf("interpret AI mix request: %w", err)}
				}
				groundingIntent = intent
				seen := map[string]bool{}
				var anchorCandidates []subsonic.Track
				for _, term := range intent.SearchTerms {
					matches, searchErr := store.CachedSubsonicSearch(term, 40)
					if searchErr != nil {
						return errMsg{err: fmt.Errorf("search anchor candidates: %w", searchErr)}
					}
					for _, track := range matches {
						if !seen[track.ID] {
							seen[track.ID] = true
							anchorCandidates = append(anchorCandidates, track)
						}
					}
				}
				events <- curatorStageMsg{generation: generation, status: fmt.Sprintf("judging %d fallback anchor candidates", len(anchorCandidates))}
				anchors, err = curator.SelectAnchors(ctx, llmClient, query, intent, anchorCandidates)
				if err != nil {
					return errMsg{err: err}
				}
				authoritativeNeighborhood, err = client.Similar(ctx, anchors[0], 200)
				if err != nil {
					return errMsg{err: fmt.Errorf("find music similar to authoritative seed %s: %w", anchors[0].Title, err)}
				}
			} else if !fastGrounded {
				return errMsg{err: errors.New("could not establish three discovery anchors")}
			}
			groundingAnchors = anchors
			playlist, saveErr := store.SaveOrReplacePrivatePlaylist(playlistName, playqueue.Snapshot{Tracks: anchors, Current: 0})
			if saveErr != nil {
				return errMsg{err: fmt.Errorf("publish anchor playlist: %w", saveErr)}
			}
			partialPrivateID = playlist.ID
			playlists, listErr := store.PrivatePlaylists()
			if listErr != nil {
				rollbackPrivate()
				return errMsg{err: fmt.Errorf("refresh anchor playlist: %w", listErr)}
			}
			events <- curatorAnchorsStartedMsg{generation: generation, name: playlistName, playlist: playlist, playlists: playlists, anchors: anchors}
			published = true
			events <- curatorStageMsg{generation: generation, status: "building from authoritative seed " + anchors[0].Artist + " - " + anchors[0].Title}
			crate := curator.MergeSimilarityCrate(anchors, [][]subsonic.Track{authoritativeNeighborhood})
			cachedByID := make(map[string]localstore.CachedTrack, len(cached))
			for _, candidate := range cached {
				cachedByID[candidate.Track.ID] = candidate
			}
			grounded := make([]localstore.CachedTrack, 0, len(crate))
			for _, track := range crate {
				candidate, ok := cachedByID[track.ID]
				if !ok {
					continue
				}
				grounded = append(grounded, candidate)
				preferred[track.ID] = true
			}
			if len(grounded) < playlistLimit {
				return errMsg{err: preservedPartialCuratorError(fmt.Errorf("authoritative seed produced only %d cached candidates; need %d", len(grounded), playlistLimit), len(anchors), playlistLimit)}
			}
			cached = grounded
			candidateSnapshotID = curator.CandidateSnapshotID(cached)
			events <- curatorStageMsg{generation: generation, status: fmt.Sprintf("curating a grounded crate of %d tracks", len(grounded))}
		}
		progressive := append([]subsonic.Track(nil), groundingAnchors...)
		started := false
		if request.Destination == curatorDestinationServer && request.Mode == curator.ModeMood {
			playlist, saveErr := client.SaveOrReplacePlaylist(ctx, playlistName, []subsonic.Track{request.Seed})
			if saveErr != nil {
				return errMsg{err: fmt.Errorf("start server %s playlist: %w", playlistName, saveErr)}
			}
			playlists, listErr := client.Playlists(ctx)
			if listErr != nil {
				return errMsg{err: fmt.Errorf("refresh server playlists: %w", listErr)}
			}
			events <- curatorPlaylistStartedMsg{generation: generation, name: playlistName, playlist: playlist, playlists: playlists}
			started = true
		}
		result, err := curator.GenerateStreaming(ctx, llmClient, curator.Request{Seed: request.Seed, Candidates: cached, RecentIDs: recent, Preferred: preferred, Limit: playlistLimit, Mode: request.Mode, UserPrompt: request.Prompt, Anchors: authoritativeGrounding(groundingAnchors), Initial: groundingAnchors}, func(track subsonic.Track, accepted, total int) error {
			progressive = append(progressive, track)
			if request.Destination == curatorDestinationVault {
				playlist, saveErr := store.SaveOrReplacePrivatePlaylist(playlistName, playqueue.Snapshot{Tracks: progressive, Current: 0})
				if saveErr != nil {
					return saveErr
				}
				partialPrivateID = playlist.ID
				if accepted == 1 {
					playlists, listErr := store.PrivatePlaylists()
					if listErr != nil {
						return listErr
					}
					events <- curatorPrivateStartedMsg{generation: generation, name: playlistName, playlist: playlist, playlists: playlists}
				}
			}
			if request.Destination == curatorDestinationServer && !started {
				playlist, saveErr := client.SaveOrReplacePlaylist(ctx, playlistName, progressive)
				if saveErr != nil {
					return saveErr
				}
				playlists, listErr := client.Playlists(ctx)
				if listErr != nil {
					return listErr
				}
				events <- curatorPlaylistStartedMsg{generation: generation, name: playlistName, playlist: playlist, playlists: playlists}
				started = true
			}
			events <- curatorProgressMsg{generation: generation, track: track, accepted: accepted, total: total}
			if request.Destination == curatorDestinationServer && accepted > 1 && (accepted%4 == 0 || accepted == total) {
				_, err := client.SaveOrReplacePlaylist(ctx, playlistName, progressive)
				return err
			}
			return nil
		})
		if err != nil {
			if !published {
				rollbackPrivate()
			}
			payload := enrichedCuratorRunPayload(cfg.Provider, cfg.Model, result, request, groundingMethod, groundingIntent, groundingAnchors, candidateSnapshotID, len(cached))
			payload["status"] = "failed"
			payload["error"] = err.Error()
			_, _ = store.SaveRecommendationRun(localstore.RecommendationRun{Provider: cfg.Provider, Model: cfg.Model, TrackIDs: result.IDs, Status: "failed", Payload: payload})
			if errors.Is(err, context.DeadlineExceeded) {
				err = fmt.Errorf("Omasub Curator timed out; check the LLM server and try again")
			}
			if published {
				err = preservedPartialCuratorError(err, len(result.IDs), playlistLimit)
			}
			return errMsg{err: err}
		}
		run := localstore.RecommendationRun{Provider: cfg.Provider, Model: cfg.Model, TrackIDs: result.IDs, Status: "complete", Payload: enrichedCuratorRunPayload(cfg.Provider, cfg.Model, result, request, groundingMethod, groundingIntent, groundingAnchors, candidateSnapshotID, len(cached))}
		run, err = store.SaveRecommendationRun(run)
		if err != nil {
			if !published {
				rollbackPrivate()
			}
			if published {
				err = preservedPartialCuratorError(err, len(result.IDs), playlistLimit)
			}
			return errMsg{err: err}
		}
		msg := llmQueueMsg{generation: generation, result: result, run: run, request: request}
		if request.Destination == curatorDestinationServer {
			msg.playlist, err = client.SaveOrReplacePlaylist(ctx, playlistName, result.Tracks)
			if err != nil {
				return errMsg{err: fmt.Errorf("save server %s playlist: %w", playlistName, err)}
			}
			msg.playlists, err = client.Playlists(ctx)
			if err != nil {
				return errMsg{err: fmt.Errorf("refresh server playlists: %w", err)}
			}
		}
		if request.Destination == curatorDestinationVault {
			msg.privatePlaylist, err = store.SaveOrReplacePrivatePlaylist(playlistName, playqueue.Snapshot{Tracks: result.Tracks, Current: 0})
			if err != nil {
				if !published {
					rollbackPrivate()
				}
				err = fmt.Errorf("save private %s playlist: %w", playlistName, err)
				if published {
					err = preservedPartialCuratorError(err, len(result.IDs), playlistLimit)
				}
				return errMsg{err: err}
			}
			msg.privatePlaylists, err = store.PrivatePlaylists()
			if err != nil {
				if !published {
					rollbackPrivate()
				}
				err = fmt.Errorf("refresh private playlists: %w", err)
				if published {
					err = preservedPartialCuratorError(err, len(result.IDs), playlistLimit)
				}
				return errMsg{err: err}
			}
		}
		return msg
	}
}
