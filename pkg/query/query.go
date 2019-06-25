package query

import (
	"database/sql"
	"strings"
)

type ArtistResult struct {
	Name      string   `json:"artist"`
	PlayCount int      `json:"count"` // XXX rename in json also
	ImageURLs []string `json:"urls"`
}

// TopArtists finds the most popular artists by play count over
// a bounded time period
// XXX start & end should probably be time.Time?
func TopArtists(db *sql.DB, start string, end string, limit int) ([]ArtistResult, error) {

	var artists []ArtistResult

	query := `select a.artist, count(*) as plays, group_concat(distinct i.url)
	from activity a
	join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist
	order by plays desc limit ?;`

	rows, err := db.Query(query, start, end, limit)
	if err != nil {
		return artists, err
	}
	defer rows.Close()

	for rows.Next() {
		groupConcat := "" // temp var for comma separated group concat result
		res := ArtistResult{}

		err = rows.Scan(&res.Name, &res.PlayCount, &groupConcat)
		if err != nil {
			return artists, err
		}

		res.ImageURLs = strings.Split(groupConcat, ",")
		artists = append(artists, res)
	}

	return artists, nil
}

// TopNewArtists finds the most popular new artists by play count over
// a bounded time period. "new" means the artist was first played during
// this time period
func TopNewArtists(db *sql.DB, start string, end string, limit int) ([]ArtistResult, error) {

	var artists []ArtistResult

	query := `select a.artist, a.plays, min(a2.dt) initial,
	a.images
	from
	(
	  select a.artist, a.artist_id,
	  count(*) as plays,
	  group_concat(distinct i.url) as images
	  from activity a
	  join image i on a.image_id = i.id
	  where a.dt >= ? and a.dt < ?
	  group by a.artist, a.artist_id
	  order by plays desc
	  limit ?
	) a
	join activity a2 on
	a.artist_id = a2.artist_id
	group by a.artist_id
	having initial >= ? and initial < ?;`
	params := []interface{}{start, end, limit, start, end}

	rows, err := db.Query(query, params...)
	if err != nil {
		return artists, err
	}
	defer rows.Close()

	for rows.Next() {
		tmp := ""         // ignore the initial date for now
		groupConcat := "" // temp var for comma separated group concat result
		res := ArtistResult{}

		err = rows.Scan(&res.Name, &res.PlayCount, &tmp, &groupConcat)
		if err != nil {
			return artists, err
		}

		res.ImageURLs = strings.Split(groupConcat, ",")
		artists = append(artists, res)
	}

	return artists, nil
}
