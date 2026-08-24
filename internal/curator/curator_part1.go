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

const (
	maxCandidateTracks = 240
	maxAttempts        = 5
)

//go:embed prompts/omasub_curator.md
var moodSystemPromptText string

//go:embed prompts/ai_mix.md
var aiMixSystemPromptText string

type Mode string

const (
	ModeAIMix Mode = "ai_mix"
	ModeMood  Mode = "mood"
)

type Client interface {
	Complete(ctx context.Context, messages []llm.Message, maxTokens int) (string, error)
}
type StreamingClient interface {
	StreamComplete(ctx context.Context, messages []llm.Message, maxTokens int, onDelta func(string) error) error
}
type Request struct {
	Seed       subsonic.Track
	Candidates []localstore.CachedTrack
	RecentIDs  map[string]bool
	Preferred  map[string]bool
	Limit      int
	Mode       Mode
	UserPrompt string
	Anchors    []subsonic.Track
	Initial    []subsonic.Track
}
type Attempt struct {
	Prompt   string   `json:"prompt"`
	Response string   `json:"response"`
	Accepted []string `json:"accepted_ids"`
	Rejected []string `json:"rejected_ids"`
}
type Result struct {
	Tracks   []subsonic.Track
	IDs      []string
	Prompt   string
	Raw      string
	Rejected []string
	Attempts []Attempt
	Fallback []string
}

func Generate(ctx context.Context, client Client, req Request) (Result, error) {
	if client == nil {
		return Result{}, errors.New("llm client is required")
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
	candidateByID := make(map[string]localstore.CachedTrack, len(candidates))
	for _, candidate := range candidates {
		candidateByID[candidate.Track.ID] = candidate
	}
	if req.Mode == ModeMood {
		accepted = append(accepted, req.Seed.ID)
		acceptedSet[req.Seed.ID] = true
	}
	var attempts []Attempt
	var allRejected []string
	for attempt := 1; attempt <= maxAttempts && len(accepted) < limit; attempt++ {
		remaining := remainingCandidates(candidates, acceptedSet, req.Mode, req.Preferred)
		need := limit - len(accepted)
		prompt := promptText(req, remaining, need, attempt, accepted)
		raw, err := client.Complete(ctx, []llm.Message{{Role: "system", Content: systemPrompt(req.Mode)}, {Role: "user", Content: prompt}}, 400)
		if err != nil {
			return resultFrom(candidates, accepted, attempts, allRejected), err
		}
		ids, err := extractIDs(raw)
		if err != nil {
			attempts = append(attempts, Attempt{Prompt: prompt, Response: raw, Rejected: []string{"invalid_json"}})
			allRejected = append(allRejected, "invalid_json")
			continue
		}
		allowed := candidateIDSet(remaining)
		var added, rejected []string
		seenResponse := map[string]bool{}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			candidate, exists := candidateByID[id]
			if id == "" || seenResponse[id] || acceptedSet[id] || !allowed[id] || !exists || !selectionAllowed(candidate, accepted, candidateByID, req.Mode, limit) {
				if id != "" {
					rejected = append(rejected, id)
					if exists {
						acceptedSet[id] = true
					}
				}
				continue
			}
			seenResponse[id] = true
			acceptedSet[id] = true
			accepted = append(accepted, id)
			added = append(added, id)
			if len(accepted) == limit {
				break
			}
		}
		attempts = append(attempts, Attempt{Prompt: prompt, Response: raw, Accepted: added, Rejected: rejected})
		allRejected = append(allRejected, rejected...)
	}
	var fallback []string
	if len(accepted) != limit {
		for _, candidate := range remainingCandidates(candidates, acceptedSet, req.Mode, req.Preferred) {
			if acceptedSet[candidate.Track.ID] || !selectionAllowed(candidate, accepted, candidateByID, req.Mode, limit) {
				continue
			}
			acceptedSet[candidate.Track.ID] = true
			accepted = append(accepted, candidate.Track.ID)
			fallback = append(fallback, candidate.Track.ID)
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
