package query

import (
	"database/sql"
	"strings"
)

type ArtistResult struct {
	Name      string   `json:"artist"`
	Count     int      `json:"count"`
	ImageURLs []string `json:"urls"`
}

// XXX rename this: artist populalarity over a time period?
// XXX start & end should probably be time.Time?
func Artists(db *sql.DB, start string, end string, limit int) ([]ArtistResult, error) {

	var artists []ArtistResult

	query := `select a.artist, count(*) as c, group_concat(distinct i.url)
	from activity a
	join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist
	order by c desc limit ?;`

	rows, err := db.Query(query, start, end, limit)
	if err != nil {
		return artists, err
	}
	defer rows.Close()

	for rows.Next() {
		groupConcat := "" // temp var for comma separated group concat result
		res := ArtistResult{}

		err = rows.Scan(&res.Name, &res.Count, &groupConcat)
		if err != nil {
			return artists, err
		}

		res.ImageURLs = strings.Split(groupConcat, ",")
		artists = append(artists, res)
	}

	return artists, nil
}
