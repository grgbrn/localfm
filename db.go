package localfm

import (
	"database/sql"
	"fmt"
	"time"

	// blank import just to load drivers
	_ "github.com/mattn/go-sqlite3"
)

type Artist struct {
	ID   int64
	Name string
	MBID sql.NullString
}

type Album struct {
	ID   int64
	Name string
	MBID sql.NullString
}

type Image struct {
	ID  int64
	URL string
}

type Activity struct {
	ID  int64
	UTS int
	DT  time.Time

	Title string
	MBID  string
	URL   string

	Artist Artist
	Album  Album

	Duplicate bool

	// don't go crazy with denormalization just yet...
	ArtistName string
	AlbumName  string
}

func toNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func getOrCreateArtist(tx *sql.Tx, name string, mbid string) (Artist, error) {

	var artist Artist
	var err error

	nullMBID := toNullString(mbid)

	// select query depends on whether mbid value is null
	var selQuery string
	if nullMBID.Valid { // not null
		selQuery = `SELECT id, name, mbid FROM artist WHERE name=? and mbid=?`
		err = tx.QueryRow(selQuery, name, nullMBID).Scan(&artist.ID, &artist.Name, &artist.MBID)
	} else {
		selQuery = `SELECT id, name, mbid FROM artist WHERE name=? and mbid is null`
		err = tx.QueryRow(selQuery, name).Scan(&artist.ID, &artist.Name, &artist.MBID)
	}
	if err == nil { // found existing entry
		return artist, nil
	}

	// otherwise have to create a new one
	insQuery := `INSERT INTO artist(name, mbid) values (?,?)`
	res, err := tx.Exec(insQuery, name, nullMBID)
	if err != nil {
		// error creating new row
		return artist, err
	}
	// need to return an artist struct with newly created ID
	lastID, err := res.LastInsertId()
	if err != nil {
		return artist, err
	}
	artist.ID = lastID
	artist.Name = name
	artist.MBID = nullMBID

	return artist, nil
}

func getOrCreateAlbum(tx *sql.Tx, name string, mbid string) (Album, error) {

	var album Album
	var err error

	nullMBID := toNullString(mbid)

	// select query depends on whether mbid value is null
	var selQuery string
	if nullMBID.Valid { // not null
		selQuery = `SELECT id, name, mbid FROM album WHERE name=? and mbid=?`
		err = tx.QueryRow(selQuery, name, nullMBID).Scan(&album.ID, &album.Name, &album.MBID)
	} else {
		selQuery = `SELECT id, name, mbid FROM album WHERE name=? and mbid is null`
		err = tx.QueryRow(selQuery, name).Scan(&album.ID, &album.Name, &album.MBID)
	}
	if err == nil { // found existing entry
		return album, nil
	}

	// otherwise have to create a new one
	insQuery := `INSERT INTO album(name, mbid) values (?,?)`
	res, err := tx.Exec(insQuery, name, nullMBID)
	if err != nil {
		// error creating new row
		return album, err
	}
	// need to return an album struct with newly created ID
	lastID, err := res.LastInsertId()
	if err != nil {
		return album, err
	}
	album.ID = lastID
	album.Name = name
	album.MBID = nullMBID

	return album, nil
}

// InitDB opens a database at a given path and tests the connection
// Currently nonexistent sqlite file doesn't trigger an error
// (won't happen until your first query)
func InitDB(filepath string) (*sql.DB, error) {
	// this driver can use urls to pass magic params to the driver
	// which in this case we need to tweak txn isolation to work
	// with golang's connection pool
	dburl := fmt.Sprintf("file:%s?cache=shared", filepath)
	fmt.Println(dburl)
	db, err := sql.Open("sqlite3", dburl)
	if err != nil {
		return db, err
	}
	err = db.Ping()
	if err != nil {
		return db, err
	}
	return db, nil
}

// FindLatestTimestamp looks up the epoch time of the most recentl db entry
// returns 0 from an empty database
func FindLatestTimestamp(db *sql.DB) (int64, error) {

	/* XXX update for new schema
	var maxTime int64

	// avoid an error with the findmax query on an empty db
	checkdb := `SELECT count(*) FROM lastfm_activity`

	var tmp int
	err := db.QueryRow(checkdb, 1).Scan(&tmp)
	if err != nil {
		return maxTime, err
	}
	if tmp == 0 {
		return maxTime, nil // maxtime is already zero'd
	}

	// cast the datetime into an epoch int
	findmax := `SELECT CAST(strftime('%s', MAX(dt)) as integer) FROM lastfm_activity`

	err = db.QueryRow(findmax, 1).Scan(&maxTime)
	if err != nil {
		return maxTime, err
	}
	return maxTime, nil
	*/
	return 0, nil
}

// StoreActivity inserts a list of activity records into the database
// using a transaction. If error is returned the transaction was rolled
// back and no rows were inserted; otherwise all were inserted
func StoreActivity(db *sql.DB, tracks []TrackInfo) error {

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	additem := `
	INSERT INTO activity(
		uts,
		dt,
		title,
		mbid,
		url,
		artist,
		artist_id,
		album,
		album_id
	) values (?,?,?,?,?,?,?,?,?)
	`
	stmt, err := tx.Prepare(additem)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var e error
	var artist Artist
	var album Album

	for _, track := range tracks {

		// activity row is denormalized and depends on three other rows:
		// - artist
		// - album
		// - url

		// so all three of those must be resolved before the activity row
		// can be created
		artist, e = getOrCreateArtist(tx, track.Artist.Name, track.Artist.Mbid)
		if e != nil {
			fmt.Printf("error inserting artist:%s mbid:%s\n", track.Artist.Name, track.Artist.Mbid)
			fmt.Println(e)
			break
		}
		fmt.Println(artist)

		album, e = getOrCreateAlbum(tx, track.Album.Name, track.Album.Mbid)
		if e != nil {
			fmt.Printf("error inserting album:%s mbid:%s\n", track.Album.Name, track.Album.Mbid)
			fmt.Println(e)
			break
		}

		uts, e := getParsedUTS(track)
		if e != nil {
			fmt.Printf("error parsing UTS: %v\n", e)
			break
		}
		dt, e := getParsedTime(track)
		if e != nil {
			fmt.Printf("error parsing time: %v\n", e)
			break
		}

		_, e = stmt.Exec(
			uts,
			dt,
			track.Name,
			track.Mbid,
			track.Url,
			artist.Name,
			artist.ID,
			album.Name,
			album.ID,
		)
		if e != nil {
			fmt.Println("error inserting activity row")
			fmt.Println(e)
			break
		}
	}

	fmt.Printf("done processing tracks. err=%v\n", e)

	if e != nil {
		fmt.Printf("rolling back after error:%v\n", e)
		tx.Rollback()
		return e
	} else {
		fmt.Println("committing!")
		tx.Commit()
		return nil
	}
}

// FlagDuplicates will scan all records in the database after
// a given timestamp and set the 'duplicate' field on any rows
// that immediately follow an identical record with a dt/uts
// less than 'diff' seconds apart
// XXX no duplicate field yet
/*
func FlagDuplicates(db *sql.DB, since int64, diff int64) (int, error) {

	count := 0
	duplicates := 0

	// XXX select by uts is kind of clumsy
	readquery := `
	SELECT id, artist, album, title, dt
	from lastfm_activity
	where CAST(strftime('%s', dt) as integer) >= ?
	order by dt desc`

	rows, err := db.Query(readquery, since)
	if err != nil {
		return duplicates, err
	}
	defer rows.Close()

	var lastItem *LastFMActivity

	for rows.Next() {
		item := LastFMActivity{}

		err = rows.Scan(&item.ID, &item.Artist, &item.Album, &item.Title, &item.Dt)
		if err != nil {
			return duplicates, err
		}
		count++

		if lastItem != nil && sameTrack(*lastItem, item) {
			// because of query order, lastitem should always be more
			// recent (larger) than item, so no need for abs()
			d := lastItem.Dt.Unix() - item.Dt.Unix()
			// fmt.Printf("diff: %d\n", d)

			if d <= diff {
				// XXX id of the later one (lastItem) needs to be flagged as duplicate
				fmt.Printf("duplicate found (diff=%d)\n", d)
				fmt.Println(item)
				fmt.Println(*lastItem)
				duplicates++
			}
		}
		lastItem = &item
	}

	pct := (float64(duplicates) / float64(count)) * 100
	fmt.Printf("checked %d rows; %d duplicates found (%0.2f%%)\n", count, duplicates, pct)
	return duplicates, nil
}

func sameTrack(a, b LastFMActivity) bool {
	return a.Artist == b.Artist && a.Album == b.Album && a.Title == b.Title
}
*/
