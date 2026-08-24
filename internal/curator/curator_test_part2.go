package curator

import (
	"context"
	"fmt"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/subsonic"
	"strings"
	"testing"
)

func TestAIMixSelectionEnforcesClassArtistAlbumAndAdjacencyCaps(t *testing.T) {
	byID := map[string]localstore.CachedTrack{}
	accepted := []string{"a1", "a2", "b1"}
	byID["a1"] = localstore.CachedTrack{Track: subsonic.Track{ID: "a1", Artist: "Artist A", Album: "Album A"}, New: true}
	byID["a2"] = localstore.CachedTrack{Track: subsonic.Track{ID: "a2", Artist: "Artist A", Album: "Album B"}, New: true}
	byID["b1"] = localstore.CachedTrack{Track: subsonic.Track{ID: "b1", Artist: "Artist B", Album: "Album C"}}
	byID["a0"] = localstore.CachedTrack{Track: subsonic.Track{ID: "a0", Artist: "Artist A", Album: "Album Z"}}
	artistCap := localstore.CachedTrack{Track: subsonic.Track{ID: "a3", Artist: "Artist A", Album: "Album D"}}
	byID["a3"] = artistCap
	if selectionAllowed(artistCap, append(accepted, "a0"), byID, ModeAIMix, 40) {
		t.Fatal("allowed fourth track by artist")
	}
	adjacent := localstore.CachedTrack{Track: subsonic.Track{ID: "b2", Artist: "Artist B", Album: "Album D"}}
	if selectionAllowed(adjacent, accepted, byID, ModeAIMix, 40) {
		t.Fatal("allowed adjacent same artist")
	}
	albumCap := localstore.CachedTrack{Track: subsonic.Track{ID: "a4", Artist: "Artist C", Album: "Album A"}}
	byID["a0"] = localstore.CachedTrack{Track: subsonic.Track{ID: "a0", Artist: "Artist D", Album: "Album A"}}
	if selectionAllowed(albumCap, append(accepted, "a0"), byID, ModeAIMix, 40) {
		t.Fatal("allowed third track from album")
	}
	var newAccepted []string
	for i := 0; i < 24; i++ {
		id := fmt.Sprintf("new-cap-%d", i)
		byID[id] = localstore.CachedTrack{Track: subsonic.Track{ID: id, Artist: fmt.Sprintf("Artist %d", i), Album: fmt.Sprintf("Album %d", i)}, New: true}
		newAccepted = append(newAccepted, id)
	}
	tooNew := localstore.CachedTrack{Track: subsonic.Track{ID: "too-new", Artist: "Different", Album: "Different"}, New: true}
	if selectionAllowed(tooNew, newAccepted, byID, ModeAIMix, 40) {
		t.Fatal("allowed more than 24 NEW tracks")
	}
}
func TestGenerateStreamingStopsAfterRequiredValidIDs(t *testing.T) {
	var candidates []localstore.CachedTrack
	var emitted []string
	for i := 0; i < 120; i++ {
		id := fmt.Sprintf("opaque-navidrome-id-%03d", i)
		emitted = append(emitted, id)
		candidates = append(candidates, localstore.CachedTrack{Track: subsonic.Track{ID: id, Title: id}})
	}
	client := &fakeStreamingClient{response: jsonIDs(t, emitted)}
	var progress []string
	result, err := GenerateStreaming(context.Background(), client, Request{Candidates: candidates, Limit: 20}, func(track subsonic.Track, _, _ int) error {
		progress = append(progress, track.ID)
		return nil
	})
	if err != nil {
		t.Fatalf("GenerateStreaming: %v", err)
	}
	if len(result.IDs) != 20 || len(progress) != 20 {
		t.Fatalf("ids=%d progress=%d", len(result.IDs), len(progress))
	}
	if result.IDs[19] != emitted[19] {
		t.Fatalf("last id=%q want=%q", result.IDs[19], emitted[19])
	}
	if client.calls != 1 {
		t.Fatalf("calls=%d", client.calls)
	}
}
func TestGenerateStreamingKeepsPublishedAnchorsAndRequestsOnlyBackfill(t *testing.T) {
	anchors := []subsonic.Track{{ID: "anchor-a", Artist: "Oasis", Album: "Definitely Maybe"}, {ID: "anchor-b", Artist: "Blur", Album: "Parklife"}, {ID: "anchor-c", Artist: "The Verve", Album: "Urban Hymns"}}
	candidates := make([]localstore.CachedTrack, 0, 40)
	for _, anchor := range anchors {
		candidates = append(candidates, localstore.CachedTrack{Track: anchor})
	}
	var backfill []string
	for i := 0; i < 37; i++ {
		id := fmt.Sprintf("fill-%02d", i)
		backfill = append(backfill, id)
		candidates = append(candidates, localstore.CachedTrack{Track: subsonic.Track{ID: id, Artist: fmt.Sprintf("Artist %02d", i), Album: fmt.Sprintf("Album %02d", i)}})
	}
	client := &fakeStreamingClient{response: jsonIDs(t, backfill)}
	var progress []string
	result, err := GenerateStreaming(context.Background(), client, Request{Mode: ModeAIMix, Candidates: candidates, Initial: anchors, Limit: 40}, func(track subsonic.Track, _, _ int) error {
		progress = append(progress, track.ID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.IDs) != 40 || len(progress) != 37 || result.IDs[0] != "anchor-a" || progress[0] != "fill-00" {
		t.Fatalf("result=%v progress=%v", result.IDs, progress)
	}
	if !strings.Contains(result.Attempts[0].Prompt, "exactly 37 track IDs") {
		t.Fatalf("prompt=%s", result.Attempts[0].Prompt)
	}
}
func TestStreamedStringParserWaitsForClosingQuote(t *testing.T) {
	p := &streamedStringParser{}
	if got := p.Feed(`["partial`); len(got) != 0 {
		t.Fatalf("premature values=%v", got)
	}
	got := p.Feed(`-id","complete"]`)
	if len(got) != 2 || got[0] != "partial-id" || got[1] != "complete" {
		t.Fatalf("values=%v", got)
	}
}
func TestCandidateListBlacklistsShortSilenceArtifacts(t *testing.T) {
	candidates := []localstore.CachedTrack{{Track: subsonic.Track{ID: "spacer", Title: "[silence]", Artist: "Bowling for Soup", Duration: 5}}, {Track: subsonic.Track{ID: "real", Title: "Silence", Artist: "Portishead", Duration: 300}}, {Track: subsonic.Track{ID: "song", Title: "A Real Song", Duration: 4}}}
	got := candidateList(candidates, nil, "", nil)
	if len(got) != 2 || got[0].Track.ID == "spacer" || got[1].Track.ID == "spacer" {
		t.Fatalf("candidate list = %#v", got)
	}
}
func TestShortSilenceArtifactCannotServeAsMoodSeed(t *testing.T) {
	seed := subsonic.Track{ID: "spacer", Title: "[silence]", Duration: 5}
	_, err := Generate(context.Background(), &fakeClient{responses: []string{`[]`}}, Request{Mode: ModeMood, Seed: seed, Limit: 1, Candidates: []localstore.CachedTrack{{Track: seed}}})
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("err=%v", err)
	}
}
