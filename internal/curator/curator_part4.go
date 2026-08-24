package curator

import (
	_ "embed"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func extractIDs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty llm response")
	}
	var obj struct {
		TrackIDs []string `json:"track_ids"`
		IDs      []string `json:"ids"`
	}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		if len(obj.TrackIDs) > 0 {
			return obj.TrackIDs, nil
		}
		if len(obj.IDs) > 0 {
			return obj.IDs, nil
		}
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err == nil && len(ids) > 0 {
		return ids, nil
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return extractIDs(raw[start : end+1])
	}
	return nil, errors.New("llm response did not contain track_ids JSON")
}
func RunPayload(provider, model string, result Result) map[string]any {
	return map[string]any{"provider": provider, "model": model, "track_ids": result.IDs, "rejected_ids": result.Rejected, "prompt": result.Prompt, "response": result.Raw, "attempts": result.Attempts, "fallback_ids": result.Fallback, "created_at": time.Now().UTC().Format(time.RFC3339)}
}
