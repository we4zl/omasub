package curator

import (
	"github.com/bprendie/omasub/internal/subsonic"
	"strings"
)

func uniqueTerms(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := normalized(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}
func normalized(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
func recordingVariant(track subsonic.Track) bool {
	title, album := normalized(track.Title), normalized(track.Album)
	text := title + " " + album
	for _, marker := range []string{"remix", "acoustic", "karaoke", "tribute", "cover version"} {
		if strings.Contains(" "+text, marker) {
			return true
		}
	}
	return album == "live" || strings.Contains(title, "(live") || strings.Contains(title, "[live") || strings.Contains(title, " live at ") || strings.Contains(title, " live from ") || strings.HasSuffix(title, " - live") || strings.HasSuffix(title, " live")
}
func anchorTrackLine(track subsonic.Track) string {
	return strings.Join([]string{"id=" + track.ID, "title=" + clean(track.Title), "artist=" + clean(track.Artist), "album=" + clean(track.Album), "genre=" + clean(track.Genre)}, " | ")
}
