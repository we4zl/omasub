package curator

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/bprendie/omasub/internal/llm"
	"github.com/bprendie/omasub/internal/localstore"
	"github.com/bprendie/omasub/internal/subsonic"
	"strings"
)

func GenerateStreaming(ctx context.Context, client StreamingClient, req Request, onAccept func(subsonic.Track, int, int) error) (Result, error) {
	if client == nil {
		return Result{}, errors.New("streaming llm client is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	candidates := candidateList(req.Candidates, req.RecentIDs, req.Seed.ID, req.Preferred)
	if req.Mode == ModeMood && (req.Seed.ID == "" || !containsCandidate(candidates, req.Seed.ID)) {
		return Result{}, errors.New("currently playing track is not available in the synced cache")
	}
	if len(candidates) < limit {
		return Result{}, fmt.Errorf("need %d eligible cached tracks; found %d", limit, len(candidates))
	}
	byID := map[ // GenerateStreaming validates opaque IDs as soon as their closing quote
	// arrives. The provider request is cancelled once the playlist is full.
	string]subsonic.Track{}
	candidateByID := map[string]localstore.CachedTrack{}
	for _, candidate := range candidates {
		byID[candidate.Track.ID] = candidate.Track
		candidateByID[candidate.Track.ID] = candidate
	}
	accepted := make([]string, 0, limit)
	acceptedSet := map[string]bool{}
	if req.Mode == ModeAIMix {
		for _, track := range req.Initial {
			if track.ID == "" || acceptedSet[track.ID] || !containsCandidate(candidates, track.ID) || len(accepted) == limit {
				continue
			}
			accepted = append(accepted, track.ID)
			acceptedSet[track.ID] = true
		}
	}
	if req.Mode == ModeMood {
		accepted = append(accepted, req.Seed.ID)
		acceptedSet[req.Seed.ID] = true
		if onAccept != nil {
			if err := onAccept(req.Seed, 1, limit); err != nil {
				return Result{}, err
			}
		}
	}
	var attempts []Attempt
	var allRejected []string
	for attempt := 1; attempt <= maxAttempts && len(accepted) < limit; attempt++ {
		remaining := remainingCandidates(candidates, acceptedSet, req.Mode, req.Preferred)
		prompt := promptText(req, remaining, limit-len(accepted), attempt, accepted)
		allowed := candidateIDSet(remaining)
		parser := &streamedStringParser{}
		var raw strings.Builder
		var added, rejected []string
		full := false
		roundCtx, cancel := context.WithCancel(ctx)
		err := client.StreamComplete(roundCtx, []llm.Message{{Role: "system", Content: systemPrompt(req.Mode)}, {Role: "user", Content: prompt}}, 3000, func(delta string) error {
			raw.WriteString(delta)
			for _, id := range parser.Feed(delta) {
				id = strings.TrimSpace(id)
				candidate, exists := candidateByID[id]
				if id == "" || acceptedSet[id] || !allowed[id] || !exists || !selectionAllowed(candidate, accepted, candidateByID, req.Mode, limit) {
					if id != "" {
						rejected = append(rejected, id)
						allRejected = append(allRejected, id)
						if exists {
							acceptedSet[id] = true
						}
					}
					continue
				}
				acceptedSet[id] = true
				accepted = append(accepted, id)
				added = append(added, id)
				if onAccept != nil {
					if err := onAccept(byID[id], len(accepted), limit); err != nil {
						return err
					}
				}
				if len(accepted) == limit {
					full = true
					cancel()
					break
				}
			}
			return nil
		})
		cancel()
		attempts = append(attempts, Attempt{Prompt: prompt, Response: raw.String(), Accepted: added, Rejected: rejected})
		if err != nil && !(full && errors.Is(err, context.Canceled)) {
			return resultFrom(candidates, accepted, attempts, allRejected), err
		}
	}
	var fallback []string
	if len(accepted) < limit {
		for _, candidate := range remainingCandidates(candidates, acceptedSet, req.Mode, req.Preferred) {
			id := candidate.Track.ID
			if acceptedSet[id] || !selectionAllowed(candidate, accepted, candidateByID, req.Mode, limit) {
				continue
			}
			acceptedSet[id] = true
			accepted = append(accepted, id)
			fallback = append(fallback, id)
			if onAccept != nil {
				if err := onAccept(candidate.Track, len(accepted), limit); err != nil {
					return resultFrom(candidates, accepted, attempts, allRejected), err
				}
			}
			if len(accepted) == limit {
				break
			}
		}
	}
	if len(accepted) != limit {
		return resultFrom(candidates, accepted, attempts, allRejected), fmt.Errorf("curator could only fill %d/%d unique cached tracks", len(accepted), limit)
	}
	if err := validateMixCompletion(req.Mode, accepted, candidateByID); err != nil {
		return resultFrom(candidates, accepted, attempts, allRejected), err
	}
	result := resultFrom(candidates, accepted, attempts, allRejected)
	result.Fallback = fallback
	return result, nil
}

type streamedStringParser struct {
	inString, escaped bool
	value             strings.Builder
}

func (p *streamedStringParser) Feed(delta string) []string {
	var values []string
	for _, r := range delta {
		if !p.inString {
			if r == '"' {
				p.inString = true
				p.value.Reset()
			}
			continue
		}
		if p.escaped {
			p.value.WriteRune(r)
			p.escaped = false
			continue
		}
		if r == '\\' {
			p.escaped = true
			continue
		}
		if r == '"' {
			values = append(values, p.value.String())
			p.inString = false
			continue
		}
		p.value.WriteRune(r)
	}
	return values
}
func resultFrom(candidates []localstore.CachedTrack, accepted []string, attempts []Attempt, rejected []string) Result {
	tracksByID := map[string]subsonic.Track{}
	for _, candidate := range candidates {
		tracksByID[candidate.Track.ID] = candidate.Track
	}
	tracks := make([]subsonic.Track, 0, len(accepted))
	for _, id := range accepted {
		tracks = append(tracks, tracksByID[id])
	}
	result := Result{Tracks: tracks, IDs: accepted, Rejected: rejected, Attempts: attempts}
	if len(attempts) > 0 {
		result.Prompt = attempts[0].Prompt
		result.Raw = attempts[len(attempts)-1].Response
	}
	return result
}
func systemPrompt(mode Mode) string {
	if mode == ModeMood {
		return strings.TrimSpace(moodSystemPromptText)
	}
	return strings.TrimSpace(aiMixSystemPromptText)
}
func promptText(req Request, candidates []localstore.CachedTrack, limit, attempt int, accepted []string) string {
	rows := make([]string, 0, min(len(candidates), maxCandidateTracks)+8)
	mode := req.Mode
	if mode == "" {
		mode = ModeAIMix
	}
	rows = append(rows, "MODE INSTRUCTION:")
	rows = append(rows, modeInstruction(req, limit, attempt))
	if mode == ModeAIMix {
		newAccepted, backAccepted := acceptedClassCounts(req.Candidates, accepted)
		remainingNew := max(0, req.Limit*3/5-newAccepted)
		remainingBack := max(0, req.Limit*2/5-backAccepted)
		rows = append(rows, fmt.Sprintf("APPLICATION LIMITS: at most %d additional NEW selections; at least %d additional BACK-NINE selections are still required.", remainingNew, remainingBack))
	}
	if attempt > 1 {
		rows = append(rows, fmt.Sprintf("REPAIR ROUND %d: return exactly %d replacement track IDs.", attempt, limit))
		rows = append(rows, "Previously accepted IDs (never repeat): "+strings.Join(accepted, ","))
	} else {
		rows = append(rows, fmt.Sprintf("Create a playlist by returning exactly %d track IDs.", limit))
	}
	if req.Seed.ID != "" {
		rows = append(rows, "Seed: "+trackLine(localstore.CachedTrack{Track: req.Seed}))
	}
	if len(req.Anchors) > 0 {
		rows = append(rows, "AUTHORITATIVE SEED (this recording owns the mix's musical identity):")
		rows = append(rows, trackLine(localstore.CachedTrack{Track: req.Anchors[0]}))
	}
	if len(req.Initial) > 1 {
		rows = append(rows, "PROVISIONAL LAUNCH TRACKS (already playing; protect their transitions but do not use them to broaden the requested style):")
		for _, track := range req.Initial[1:] {
			rows = append(rows, trackLine(localstore.CachedTrack{Track: track}))
		}
	}
	if strings.TrimSpace(req.UserPrompt) != "" {
		rows = append(rows, "USER REQUEST:")
		rows = append(rows, clean(req.UserPrompt))
	}
	if mode == ModeMood {
		rows = append(rows, "Preserve the seed's mood, energy, texture, and momentum. Include compatible NEW material and BACK-NINE deep cuts.")
	}
	rows = append(rows, "Return only a JSON array shaped like [\"id1\",\"id2\"].")
	rows = append(rows, "CANDIDATE POOL:")
	for i, candidate := range candidates {
		if i >= maxCandidateTracks {
			break
		}
		rows = append(rows, trackLine(candidate))
	}
	return strings.Join(rows, "\n")
}
func modeInstruction(req Request, limit, attempt int) string {
	if req.Mode == ModeMood {
		return fmt.Sprintf("Continue the seed's mood, energy, texture, and momentum with exactly %d additional selections.", limit)
	}
	if attempt > 1 {
		return fmt.Sprintf("REPAIR ROUND: return exactly %d replacement IDs that still satisfy the user request. Never return previously accepted or rejected IDs.", limit)
	}
	if strings.TrimSpace(req.UserPrompt) == "" {
		return fmt.Sprintf("ZERO_TAX_GRINDAGE: construct an opinionated, discovery-oriented, momentum-protected mix of exactly %d tracks. Weave relevant NEW uploads with BACK-NINE deep cuts; do not copy candidate order or album blocks.", limit)
	}
	return fmt.Sprintf("PROMPTED MIX: return exactly %d tracks. The user request governs eligibility; NEW is only a tie-breaker among equally relevant candidates.", limit)
}
