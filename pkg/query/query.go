package query

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// ArtistResult contains popularity metrics about an artist
type ArtistResult struct {
	Rank      int      `json:"rank"`
	Name      string   `json:"artist"`
	PlayCount int      `json:"count"` // XXX rename in json also
	ImageURLs []string `json:"urls"`
}

// TrackResult contains popularity metrics about a track
type TrackResult struct {
	Rank      int      `json:"rank"`
	Artist    string   `json:"artist"`
	Title     string   `json:"title"`
	PlayCount int      `json:"count"`
	ImageURLs []string `json:"urls"`
}

// ClockResult holds hourly listening metrics, representing what time
// of day most activity takes place
type ClockResult struct {
	Hour      int `json:"hour"`
	PlayCount int `json:"count"`
	AvgCount  int `json:"avgCount"`
}

// TopTracks finds the most popular tracks by play count over
// a bounded time period
// XXX start & end should probably be time.Time?
func TopTracks(db *sql.DB, start string, end string, limit int) ([]TrackResult, error) {
	var tracks []TrackResult

	query := `select a.artist, a.title, count(*) as plays, group_concat(distinct i.url)
	from activity a
	left join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist, a.title
	order by plays desc limit ?;`

	rows, err := db.Query(query, start, end, limit)
	if err != nil {
		return tracks, err
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		i++
		var groupConcat sql.NullString
		res := TrackResult{}

		err = rows.Scan(&res.Artist, &res.Title, &res.PlayCount, &groupConcat)
		if err != nil {
			return tracks, err
		}

		res.Rank = i
		if groupConcat.Valid {
			res.ImageURLs = strings.Split(groupConcat.String, ",")
		} else {
			res.ImageURLs = []string{}
		}

		tracks = append(tracks, res)
	}

	return tracks, nil
}

// TopArtists finds the most popular artists by play count over
// a bounded time period
// XXX start & end should probably be time.Time?
func TopArtists(db *sql.DB, start string, end string, limit int) ([]ArtistResult, error) {

	var artists []ArtistResult

	query := `select a.artist, count(*) as plays, group_concat(distinct i.url)
	from activity a
	left join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist
	order by plays desc limit ?;`

	rows, err := db.Query(query, start, end, limit)
	if err != nil {
		return artists, err
	}
	defer rows.Close()

	for rows.Next() {
		var groupConcat sql.NullString
		res := ArtistResult{}

		err = rows.Scan(&res.Name, &res.PlayCount, &groupConcat)
		if err != nil {
			return artists, err
		}

		if groupConcat.Valid {
			res.ImageURLs = strings.Split(groupConcat.String, ",")
		} else {
			res.ImageURLs = []string{}
		}

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
	  left join image i on a.image_id = i.id
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
		tmp := "" // ignore the initial date for now
		var groupConcat sql.NullString
		res := ArtistResult{}

		err = rows.Scan(&res.Name, &res.PlayCount, &tmp, &groupConcat)
		if err != nil {
			return artists, err
		}

		if groupConcat.Valid {
			res.ImageURLs = strings.Split(groupConcat.String, ",")
		} else {
			res.ImageURLs = []string{}
		}
		artists = append(artists, res)
	}

	return artists, nil
}

// helper function to generate date boundaries for listening clock
func listeningClockDates(month, year int) (s1, e1, s2 string) {

	start := fmt.Sprintf("%d-%02d-01", year, month)
	end := fmt.Sprintf("%d-%02d-01", year, month+1) // XXX

	var avgMonth, avgYear int

	if month < 6 {
		avgMonth = (month - 6) + 12
		avgYear = year - 1
	} else {
		avgMonth = month - 6
		avgYear = year
	}

	avgStart := fmt.Sprintf("%d-%02d-01", avgYear, avgMonth)

	return start, end, avgStart
}

// XXX not sure how timezone offset will work here
func ListeningClock(db *sql.DB, month, year int) (*[24]ClockResult, error) {

	// allocate the memory for the result and fill in the hours
	var res [24]ClockResult
	for i := 0; i < 24; i++ {
		res[i].Hour = i
	}

	query := `select strftime('%H', dt) as hour, count(*) as c
	from activity
	where dt >= ? and dt < ?
	group by 1
	order by 1;`

	start, end, avgStart := listeningClockDates(month, year)

	// this requires the same query to be executed twice, once for the
	// active period, once for the six month average before

	// first query fills in the play counts
	rows, err := db.Query(query, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		hour := ""
		count := 0

		err = rows.Scan(&hour, &count)
		if err != nil {
			return nil, err
		}

		// deal with the possibility of a sparse resultset - don't
		// assume we got an entry for each hour
		ix, err := strconv.Atoi(hour)
		if err != nil {
			return nil, err
		}
		res[ix].PlayCount = count
	}

	// second query fills in the values for the six month average
	rows, err = db.Query(query, avgStart, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		hour := ""
		count := 0

		err = rows.Scan(&hour, &count)
		if err != nil {
			return nil, err
		}

		// deal with the possibility of a sparse resultset - don't
		// assume we got an entry for each hour
		ix, err := strconv.Atoi(hour)
		if err != nil {
			return nil, err
		}
		res[ix].AvgCount = count / 6 // XXX how does this truncate?
	}

	return &res, nil
}
