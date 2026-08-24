package subsonic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (c Client) SaveOrReplacePlaylist(ctx context.Context, name string, tracks []Track) ( // SaveOrReplacePlaylist creates a named server playlist or replaces every track
	// in the existing playlist with the supplied sequence.
	Playlist, error) {
	playlists, err := c.Playlists(ctx)
	if err != nil {
		return Playlist{}, err
	}
	for _, playlist := range playlists {
		if !strings.EqualFold(strings.TrimSpace(playlist.Name), strings.TrimSpace(name)) {
			continue
		}
		existing, err := c.Playlist(ctx, playlist.ID)
		if err != nil {
			return Playlist{}, err
		}
		v := url.Values{"playlistId": {playlist.ID}, "name": {name}}
		for i := range existing {
			v.Add("songIndexToRemove", strconv.Itoa(i))
		}
		for _, track := range tracks {
			if track.ID != "" {
				v.Add("songIdToAdd", track.ID)
			}
		}
		var out struct {
			Response response `json:"subsonic-response"`
		}
		if err := c.get(ctx, "updatePlaylist", v, &out); err != nil {
			return Playlist{}, err
		}
		playlist.Name = name
		playlist.Count = len(tracks)
		return playlist, nil
	}
	return c.CreatePlaylist(ctx, name, tracks)
}

func (c Client) Scrobble(ctx context.Context, id string, submission bool) error {
	if id == "" {
		return errors.New("missing track id")
	}
	var out struct {
		Response response `json:"subsonic-response"`
	}
	v := url.Values{"id": {id}, "submission": {strconv.FormatBool(submission)}}
	return c.get(ctx, "scrobble", v, &out)
}
func (c Client) StreamURL(id string) string {
	return c.endpoint("stream", url.Values{"id": {id}}).String()
}
func (c Client) CoverURL(id string) string {
	if id == "" {
		return ""
	}
	return c.endpoint("getCoverArt", url.Values{"id": {id}, "size": {"600"}}).String()
}
func (c Client) CoverArt(ctx context.Context, id string, size int) ([]byte, // Scrobble reports either the start of playback or a completed listen.
	// A false submission is the Subsonic "now playing" notification.
	error) {
	if id == "" {
		return nil, errors.New("missing cover art id")
	}
	if size <= 0 {
		size = 600
	}
	u := c.endpoint("getCoverArt", url.Values{"id": {id}, "size": {strconv.Itoa(size)}})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("subsonic http %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, 8<<20))
}
func (c Client) get(ctx context.Context, method string, values url.Values, target any) error {
	u := c.endpoint(method, values)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("subsonic http %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		return err
	}
	if r, ok := findResponse(target); ok && r.Status == "failed" {
		return errors.New(r.Error.Message)
	}
	return nil
}
func (c Client) endpoint(method string, values url.Values) *url.URL {
	u, _ := url.Parse(c.base + "/rest/" + method + ".view")
	q := u.Query()
	for key, vals := range values {
		for _, value := range vals {
			q.Add(key, value)
		}
	}
	salt := strconv.FormatInt(time.Now().UnixNano(), 36)
	sum := md5.Sum([]byte(c.password + salt))
	q.Set("u", c.user)
	q.Set("s", salt)
	q.Set("t", hex.EncodeToString(sum[:]))
	q.Set("v", "1.16.1")
	q.Set("c", "omasub")
	q.Set("f", "json")
	u.RawQuery = q.Encode()
	return u
}

type response struct {
	Status string `json:"status"`
	Error  struct {
		Message string `json:"message"`
	} `json:"error"`
}

func findResponse(target any) (response, bool) {
	b, _ := json.Marshal(target)
	var out struct {
		Response response `json:"subsonic-response"`
	}
	if json.Unmarshal(b, &out) != nil {
		return response{}, false
	}
	return out.Response, true
}
func appendUnique(dst, src []Track, seen map[string]bool, limit int) []Track {
	for _, track := range src {
		if track.ID == "" || seen[track.ID] {
			continue
		}
		dst = append(dst, track)
		seen[track.ID] = true
		if len(dst) >= limit {
			return dst
		}
	}
	return dst
}
