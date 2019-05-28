package handler

import (
	"fmt"
	"net/http"

	"github.com/jinzhu/gorm"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func (c *Controller) GetArtists(w http.ResponseWriter, r *http.Request) {
	var artists []model.Artist
	c.DB.Find(&artists)
	var indexMap = make(map[rune]*subsonic.Index)
	var indexes subsonic.Artists
	for _, artist := range artists {
		i := indexOf(artist.Name)
		index, ok := indexMap[i]
		if !ok {
			index = &subsonic.Index{
				Name:    string(i),
				Artists: []*subsonic.Artist{},
			}
			indexMap[i] = index
			indexes.List = append(indexes.List, index)
		}
		index.Artists = append(index.Artists,
			makeArtistFromArtist(&artist))
	}
	sub := subsonic.NewResponse()
	sub.Artists = &indexes
	respond(w, r, sub)
}

func (c *Controller) GetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var artist model.Artist
	c.DB.
		Preload("Albums").
		First(&artist, id)
	sub := subsonic.NewResponse()
	sub.Artist = makeArtistFromArtist(&artist)
	for _, album := range artist.Albums {
		sub.Artist.Albums = append(sub.Artist.Albums,
			makeAlbumFromAlbum(&album, &artist))
	}
	respond(w, r, sub)
}

func (c *Controller) GetAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var album model.Album
	err = c.DB.
		Preload("Artist").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.track_number")
		}).
		First(&album, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		respondError(w, r, 10, "couldn't find an album with that id")
		return
	}
	sub := subsonic.NewResponse()
	sub.Album = makeAlbumFromAlbum(&album, &album.Artist)
	for _, track := range album.Tracks {
		sub.Album.Tracks = append(sub.Album.Tracks,
			makeTrackFromTrack(&track, &album))
	}
	respond(w, r, sub)
}

// changes to this function should be reflected in in _by_folder.go's
// getAlbumList() function
func (c *Controller) GetAlbumListTwo(w http.ResponseWriter, r *http.Request) {
	listType := getStrParam(r, "type")
	if listType == "" {
		respondError(w, r, 10, "please provide a `type` parameter")
		return
	}
	q := c.DB
	switch listType {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN artists
			ON albums.artist_id = artists.id`)
		q = q.Order("artists.name")
	case "alphabeticalByName":
		q = q.Order("title")
	case "byYear":
		q = q.Where(
			"year BETWEEN ? AND ?",
			getIntParamOr(r, "fromYear", 1800),
			getIntParamOr(r, "toYear", 2200))
		q = q.Order("year")
	case "frequent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("updated_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(contextUserKey).(*model.User)
		q = q.Joins(`
			JOIN plays
			ON albums.id = plays.album_id AND plays.user_id = ?`,
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		respondError(w, r, 10, fmt.Sprintf(
			"unknown value `%s` for parameter 'type'", listType,
		))
		return
	}
	var albums []model.Album
	q.
		Offset(getIntParamOr(r, "offset", 0)).
		Limit(getIntParamOr(r, "size", 10)).
		Preload("Artist").
		Find(&albums)
	sub := subsonic.NewResponse()
	sub.AlbumsTwo = &subsonic.Albums{}
	for _, album := range albums {
		sub.AlbumsTwo.List = append(sub.AlbumsTwo.List,
			makeAlbumFromAlbum(&album, &album.Artist))
	}
	respond(w, r, sub)
}