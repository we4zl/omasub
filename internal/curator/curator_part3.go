package curator

import (
	_ "embed"
	"errors"
	"fmt"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/subsonic"
	"sort"
	"strings"
)

func trackLine(candidate localstore.CachedTrack) string {
	track := candidate.Track
	parts := []string{"id=" + track.ID, "title=" + clean(track.Title), "artist=" + clean(track.Artist), "album=" + clean(track.Album)}
	if track.Genre != "" {
		parts = append(parts, "genre="+clean(track.Genre))
	}
	if track.Year > 0 {
		parts = append(parts, fmt.Sprintf("year=%d", track.Year))
	}
	if candidate.New {
		parts = append(parts, "class=NEW")
		if candidate.NewestRank > 0 {
			parts[len(parts)-1] = fmt.Sprintf("class=NEW rank=%d", candidate.NewestRank)
		}
	} else {
		parts = append(parts, "class=BACK-NINE")
	}
	if candidate.Starred {
		parts = append(parts, "starred=true")
	}
	return strings.Join(parts, " | ")
}
func clean(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.TrimSpace(value)
}
func candidateList(cached []localstore.CachedTrack, recent map[string]bool, seedID string, preferred map[string]bool) []localstore.CachedTrack {
	seen := map[string]bool{}
	out := make([]localstore.CachedTrack, 0, len(cached))
	for _, candidate := range cached {
		id := candidate.Track.ID
		if id == "" || seen[id] || curatorBlacklisted(candidate.Track) || (recent[id] && id != seedID) {
			continue
		}
		seen[id] = true
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		iPreferred, jPreferred := preferred[out[i].Track.ID], preferred[out[j].Track.ID]
		if iPreferred != jPreferred {
			return iPreferred
		}
		if out[i].New != out[j].New {
			return out[i].New
		}
		if out[i].New && out[i].NewestRank != out[j].NewestRank {
			return out[i].NewestRank < out[j].NewestRank
		}
		if out[i].Starred != out[j].Starred {
			return out[i].Starred
		}
		return sortKey(out[i].Track) < sortKey(out[j].Track)
	})
	return out
}
func curatorBlacklisted(track subsonic.Track) bool {
	title := strings.ToLower(strings.TrimSpace(track.Title))
	title = strings.Trim(title, "[](){}<>")
	title = strings.TrimSpace(title)
	return title == "silence" && track.Duration > 0 && track.Duration <= 10
}
func containsCandidate(candidates []localstore.CachedTrack, id string) bool {
	for _, candidate := range candidates {
		if candidate.Track.ID == id {
			return true
		}
	}
	return false
}
func remainingCandidates(candidates []localstore.CachedTrack, accepted map[string]bool, mode Mode, preferred map[string]bool) []localstore.CachedTrack {
	var preferredTracks, newest, backNine []localstore.CachedTrack
	for _, candidate := range candidates {
		if accepted[candidate.Track.ID] {
			continue
		}
		switch {
		case mode == ModeMood && preferred[candidate.Track.ID]:
			preferredTracks = append(preferredTracks, candidate)
		case candidate.New:
			newest = append(newest, candidate)
		default:
			backNine = append(backNine, candidate)
		}
	}
	out := make([]localstore.CachedTrack, 0, maxCandidateTracks)
	seen := map[string]bool{}
	appendFrom := func(source []localstore.CachedTrack, limit int) {
		for _, candidate := range source {
			if len(out) == maxCandidateTracks || limit == 0 {
				return
			}
			if seen[candidate.Track.ID] {
				continue
			}
			seen[candidate.Track.ID] = true
			out = append(out, candidate)
			limit--
		}
	}
	if mode == ModeMood {
		appendFrom(preferredTracks, 100)
		appendFrom(newest, 70)
		appendFrom(backNine, 70)
	} else {
		for len(out) < maxCandidateTracks && (len(newest) > 0 || len(backNine) > 0) {
			appendFrom(newest, min(3, len(newest)))
			if len(newest) >= 3 {
				newest = newest[3:]
			} else {
				newest = nil
			}
			appendFrom(backNine, min(2, len(backNine)))
			if len(backNine) >= 2 {
				backNine = backNine[2:]
			} else {
				backNine = nil
			}
		}
	}
	appendFrom(preferredTracks, maxCandidateTracks)
	appendFrom(newest, maxCandidateTracks)
	appendFrom(backNine, maxCandidateTracks)
	return out
}
func selectionAllowed(candidate localstore.CachedTrack, accepted []string, byID map[string]localstore.CachedTrack, mode Mode, limit int) bool {
	if mode != ModeAIMix {
		return true
	}
	artist, album := normalized(candidate.Track.Artist), normalized(candidate.Track.Album)
	if artist == "" {
		return false
	}
	artistCount, albumCount, newCount := 0, 0, 0
	for _, id := range accepted {
		prior := byID[id]
		if prior.New {
			newCount++
		}
		if artist != "" && normalized(prior.Track.Artist) == artist {
			artistCount++
		}
		if album != "" && normalized(prior.Track.Album) == album {
			albumCount++
		}
	}
	if candidate.New && newCount >= limit*3/5 {
		return false
	}
	if artistCount >= 3 || albumCount >= 2 {
		return false
	}
	if len(accepted) > 0 && artist != "" && normalized(byID[accepted[len(accepted)-1]].Track.Artist) == artist {
		return false
	}
	return true
}
func acceptedClassCounts(candidates []localstore.CachedTrack, accepted []string) (int, int) {
	byID := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.Track.ID] = candidate.New
	}
	newCount, backCount := 0, 0
	for _, id := range accepted {
		if byID[id] {
			newCount++
		} else {
			backCount++
		}
	}
	return newCount, backCount
}
func validateMixCompletion(mode Mode, accepted []string, byID map[string]localstore.CachedTrack) error {
	if mode != ModeAIMix {
		return nil
	}
	artists := map[string]bool{}
	for _, id := range accepted {
		artist := normalized(byID[id].Track.Artist)
		if artist != "" {
			artists[artist] = true
		}
	}
	if len(artists) < 8 {
		return fmt.Errorf("curator produced only %d distinct artists; need at least 8", len(artists))
	}
	return nil
}
func candidateIDSet(candidates []localstore.CachedTrack) map[string]bool {
	ids := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		ids[candidate.Track.ID] = true
	}
	return ids
}
func sortKey(track subsonic.Track) string {
	return strings.ToLower(track.Artist) + "\x00" + strings.ToLower(track.Album) + "\x00" + strings.ToLower(track.Title) + "\x00" + track.ID
}
func validTrackIDs(raw string, candidates []localstore.CachedTrack, limit int) ([]string, []string, error) {
	allowed := map[string]bool{}
	for _, candidate := range candidates {
		allowed[candidate.Track.ID] = true
	}
	ids, err := extractIDs(raw)
	if err != nil {
		return nil, nil, err
	}
	seen := map[string]bool{}
	valid := make([]string, 0, min(len(ids), limit))
	var rejected []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if !allowed[id] {
			rejected = append(rejected, id)
			continue
		}
		valid = append(valid, id)
		if len(valid) >= limit {
			break
		}
	}
	if len(valid) == 0 {
		return nil, rejected, errors.New("llm returned no valid cached track IDs")
	}
	return valid, rejected, nil
}
