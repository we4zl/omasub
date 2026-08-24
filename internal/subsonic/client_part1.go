package subsonic

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	base     string
	user     string
	password string
	http     *http.Client
}
type Track struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	AlbumID  string `json:"albumId"`
	CoverID  string `json:"coverArt"`
	Duration int    `json:"duration"`
	Genre    string `json:"genre"`
	Year     int    `json:"year"`
}
type Album struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Artist  string `json:"artist"`
	CoverID string `json:"coverArt"`
	Year    int    `json:"year"`
}
type Playlist struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"songCount"`
}

func New(base, user, password string) Client {
	return Client{base: base, user: user, password: password, http: &http.Client{Timeout: 20 * time.Second}}
}
func (c Client) Ping(ctx context.Context) error {
	var out struct {
		Response response `json:"subsonic-response"`
	}
	return c.get(ctx, "ping", nil, &out)
}
func (c Client) Newest(ctx context.Context) ([]Album, error) {
	return c.albumList(ctx, "newest", 40)
}
func (c Client) RandomAlbums(ctx context.Context) ([]Album, error) {
	return c.albumList(ctx, "random", 40)
}
func (c Client) albumList(ctx context.Context, listType string, size int) ([]Album, error) {
	var out struct {
		Response struct {
			response
			AlbumList2 struct {
				Album []Album `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	v := url.Values{"type": {listType}, "size": {strconv.Itoa(size)}}
	if err := c.get(ctx, "getAlbumList2", v, &out); err != nil {
		return nil, err
	}
	return out.Response.AlbumList2.Album, nil
}
func (c Client) Album(ctx context.Context, id string) ([]Track, error) {
	var out struct {
		Response struct {
			response
			Album struct {
				Song []Track `json:"song"`
			} `json:"album"`
		} `json:"subsonic-response"`
	}
	if err := c.get(ctx, "getAlbum", url.Values{"id": {id}}, &out); err != nil {
		return nil, err
	}
	return out.Response.Album.Song, nil
}
func (c Client) Playlists(ctx context.Context) ([]Playlist, error) {
	var out struct {
		Response struct {
			response
			Playlists struct {
				Playlist []Playlist `json:"playlist"`
			} `json:"playlists"`
		} `json:"subsonic-response"`
	}
	if err := c.get(ctx, "getPlaylists", nil, &out); err != nil {
		return nil, err
	}
	return out.Response.Playlists.Playlist, nil
}
func (c Client) Playlist(ctx context.Context, id string) ([]Track, error) {
	var out struct {
		Response struct {
			response
			Playlist struct {
				Entry []Track `json:"entry"`
			} `json:"playlist"`
		} `json:"subsonic-response"`
	}
	if err := c.get(ctx, "getPlaylist", url.Values{"id": {id}}, &out); err != nil {
		return nil, err
	}
	return out.Response.Playlist.Entry, nil
}
func (c Client) Search(ctx context.Context, query string) ([]Track, error) {
	var out struct {
		Response struct {
			response
			Search struct {
				Song []Track `json:"song"`
			} `json:"searchResult3"`
		} `json:"subsonic-response"`
	}
	v := url.Values{"query": {query}, "songCount": {"80"}, "albumCount": {"0"}, "artistCount": {"0"}}
	if err := c.get(ctx, "search3", v, &out); err != nil {
		return nil, err
	}
	return out.Response.Search.Song, nil
}
func (c Client) Similar(ctx context.Context, seed Track, limit int) ([]Track, error) {
	seen := map[string]bool{seed.ID: true}
	tracks := []Track{seed}
	if seed.ID != "" {
		similar, err := c.SimilarSongs(ctx, seed.ID, limit)
		if err == nil {
			tracks = appendUnique(tracks, similar, seen, limit)
			if len(tracks) >= limit {
				return tracks, nil
			}
		}
	}
	for _, query := range []string{seed.Artist, seed.Album} {
		if query == "" {
			continue
		}
		found, err := c.Search(ctx, query)
		if err != nil {
			return nil, err
		}
		tracks = appendUnique(tracks, found, seen, limit)
		if len(tracks) >= limit {
			return tracks, nil
		}
	}
	if seed.Genre != "" {
		genreTracks, err := c.RandomSongsByGenre(ctx, seed.Genre, limit)
		if err == nil {
			tracks = appendUnique(tracks, genreTracks, seen, limit)
			if len(tracks) >= limit {
				return tracks, nil
			}
		}
	}
	if seed.Year > 0 {
		yearTracks, err := c.RandomSongsByYear(ctx, seed.Year, limit)
		if err == nil {
			tracks = appendUnique(tracks, yearTracks, seen, limit)
			if len(tracks) >= limit {
				return tracks, nil
			}
		}
	}
	random, err := c.RandomSongs(ctx, limit)
	if err != nil {
		return tracks, nil
	}
	return appendUnique(tracks, random, seen, limit), nil
}
func (c Client) SimilarSongs(ctx context.Context, id string, limit int) ([]Track, error) {
	var out struct {
		Response struct {
			response
			SimilarSongs struct {
				Song []Track `json:"song"`
			} `json:"similarSongs2"`
		} `json:"subsonic-response"`
	}
	v := url.Values{"id": {id}, "count": {strconv.Itoa(limit)}}
	if err := c.get(ctx, "getSimilarSongs2", v, &out); err != nil {
		return nil, err
	}
	return out.Response.SimilarSongs.Song, nil
}
func (c Client) RandomSongs(ctx context.Context, limit int) ([]Track, error) {
	return c.randomSongs(ctx, url.Values{"size": {strconv.Itoa(limit)}})
}
func (c Client) RandomSongsByGenre(ctx context.Context, genre string, limit int) ([]Track, error) {
	return c.randomSongs(ctx, url.Values{"size": {strconv.Itoa(limit)}, "genre": {genre}})
}
func (c Client) RandomSongsByYear(ctx context.Context, year, limit int) ([]Track, error) {
	from := strconv.Itoa(year - 2)
	to := strconv.Itoa(year + 2)
	return c.randomSongs(ctx, url.Values{"size": {strconv.Itoa(limit)}, "fromYear": {from}, "toYear": {to}})
}
func (c Client) randomSongs(ctx context.Context, v url.Values) ([]Track, error) {
	var out struct {
		Response struct {
			response
			RandomSongs struct {
				Song []Track `json:"song"`
			} `json:"randomSongs"`
		} `json:"subsonic-response"`
	}
	if err := c.get(ctx, "getRandomSongs", v, &out); err != nil {
		return nil, err
	}
	return out.Response.RandomSongs.Song, nil
}
func (c Client) CreatePlaylist(ctx context.Context, name string, tracks []Track) (Playlist, error) {
	var out struct {
		Response struct {
			response
			Playlist Playlist `json:"playlist"`
		} `json:"subsonic-response"`
	}
	v := url.Values{"name": {name}}
	for _, track := range tracks {
		if track.ID != "" {
			v.Add("songId", track.ID)
		}
	}
	if err := c.get(ctx, "createPlaylist", v, &out); err != nil {
		return Playlist{}, err
	}
	return out.Response.Playlist, nil
}
func (c Client) RenamePlaylist(ctx context.Context, id, name string) error {
	var out struct {
		Response response `json:"subsonic-response"`
	}
	return c.get(ctx, "updatePlaylist", url.Values{"playlistId": {id}, "name": {name}}, &out)
}
func (c Client) DeletePlaylist(ctx context.Context, id string) error {
	var out struct {
		Response response `json:"subsonic-response"`
	}
	return c.get(ctx, "deletePlaylist", url.Values{"id": {id}}, &out)
}
