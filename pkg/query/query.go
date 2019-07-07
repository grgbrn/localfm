package query

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DateRangeParams represents query params over a date range
type DateRangeParams struct {
	Mode  string
	Start time.Time
	End   time.Time
	Limit int
	TZ    *time.Location
}

// StartString returns a sqlite-compatible string with second granularity
func (dp DateRangeParams) StartString() string {
	return dp.Start.Format("2006-01-02 15:04:05")
}

// StartString returns a sqlite-compatible string with second granularity
func (dp DateRangeParams) EndString() string {
	return dp.End.Format("2006-01-02 15:04:05")
}

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
func TopTracks(db *sql.DB, params DateRangeParams) ([]TrackResult, error) {
	var tracks []TrackResult

	query := `select a.artist, a.title, count(*) as plays, group_concat(distinct i.url)
	from activity a
	left join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist, a.title
	order by plays desc limit ?;`

	rows, err := db.Query(query, params.Start, params.End, params.Limit)
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
func TopArtists(db *sql.DB, params DateRangeParams) ([]ArtistResult, error) {

	var artists []ArtistResult

	query := `select a.artist, count(*) as plays, group_concat(distinct i.url)
	from activity a
	left join image i on a.image_id = i.id
	where a.dt >= ? and a.dt < ?
	group by a.artist
	order by plays desc limit ?;`

	rows, err := db.Query(query, params.Start, params.End, params.Limit)
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
func TopNewArtists(db *sql.DB, params DateRangeParams) ([]ArtistResult, error) {

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
	queryParams := []interface{}{
		params.Start,
		params.End,
		params.Limit,
		params.Start,
		params.End}

	rows, err := db.Query(query, queryParams...)
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

// XXX this is deprecated!
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

func listeningClockHelper(db *sql.DB, start, end time.Time, tz *time.Location) ([24]int, error) {

	var counts [24]int

	query := `select strftime('%Y-%m-%d %H:00', dt) as hour, count(*) as c
	from activity
	where dt >= ? and dt < ?
	group by 1
	order by 1;`

	rows, err := db.Query(query, start, end)
	if err != nil {
		return counts, err
	}
	defer rows.Close()

	rowCount := 0
	for rows.Next() {
		var hourStr string
		count := 0

		err = rows.Scan(&hourStr, &count)
		if err != nil {
			return counts, err
		}

		// need to manually parse the time string
		hour, err := time.Parse("2006-01-02 15:04", hourStr)
		if err != nil {
			return counts, err
		}
		// and then convert from UTC to the user timezone
		if tz != time.UTC {
			hour = hour.In(tz)
		}

		//fmt.Printf("%v %d\n", hour, count)

		counts[hour.Hour()] += count
		rowCount++
	}
	fmt.Printf("listeningClockHelper processed %d rows\n", rowCount)
	return counts, nil
}

func ListeningClock(db *sql.DB, params DateRangeParams) (*[24]ClockResult, error) {

	// allocate the memory for the result and fill in the hours
	var res [24]ClockResult
	for i := 0; i < 24; i++ {
		res[i].Hour = i
	}

	// execute the first query, which is the regular listening counts
	fmt.Printf("[[ %v - %v ]]\n", params.Start, params.End)
	regularCount, err := listeningClockHelper(db, params.Start, params.End, params.TZ)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 24; i++ {
		res[i].PlayCount = regularCount[i]
	}

	// average the n preceding periods
	// end on the start of the current "regular" period
	const avgPeriod int = 6
	var avgStart time.Time
	if params.Mode == "week" {
		avgStart = params.Start.AddDate(0, 0, -7*avgPeriod)
	} else if params.Mode == "month" {
		avgStart = params.Start.AddDate(0, -avgPeriod, 0)
	} else if params.Mode == "year" {
		avgStart = params.Start.AddDate(-avgPeriod, 0, 0)
	}

	fmt.Printf("[[ %v - %v ]]\n", avgStart, params.Start)
	avgCount, err := listeningClockHelper(db, avgStart, params.Start, params.TZ)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 24; i++ {
		res[i].AvgCount = avgCount[i] / avgPeriod // XXX how is this truncated?
	}

	fmt.Printf("%+v\n", res)

	return &res, nil
}
