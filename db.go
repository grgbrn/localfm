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
	MBID string
}

type Album struct {
	ID   int64
	Name string
	MBID string
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

// XXX no caching here... with sqlite it probably doesn't matter much?
func getOrCreateArtist(db *sql.DB, name string, mbid string) (Artist, error) {
	selQuery := `SELECT id, name, mbid FROM artist WHERE mbid=?`
	insQuery := `INSERT INTO artist(name, mbid) values (?,?)`

	// XXX use prepared statements?

	var artist Artist

	err := db.QueryRow(selQuery, mbid).Scan(&artist.ID, &artist.Name, &artist.MBID)
	if err == nil { // found existing entry
		return artist, nil
	}
	// otherwise have to create a new one
	res, err := db.Exec(insQuery, name, mbid)
	if err != nil {
		// error creating new row
		return artist, err
	}
	// need to to update artist with newly created id
	lastId, err := res.LastInsertId()
	if err != nil {
		return artist, err
	}
	// XXX seems a bit janky
	artist.ID = lastId
	artist.Name = name
	artist.MBID = mbid

	return artist, nil
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

	// additem := `
	// INSERT INTO lastfm_activity(
	// 	created,
	// 	artist,
	// 	album,
	// 	title,
	// 	dt
	// ) values (CURRENT_TIMESTAMP, ?, ?, ?, ?)
	// `

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// XXX need a new insert query
	// stmt, err := tx.Prepare(additem)
	// if err != nil {
	// 	return err
	// }
	// defer stmt.Close()

	var e error
	for _, track := range tracks {

		// activity row is denormalized and depends on three other rows:
		// - artist
		// - album
		// - url

		// so all three of those must be looked up or inserted before
		// we can try to deal with the activity row

		artist, e := getOrCreateArtist(db, track.Artist.Name, track.Artist.Mbid)
		if e != nil {
			fmt.Printf("error inserting artist:%s mbid:%s\n", track.Artist.Name, track.Artist.Mbid)
			fmt.Println(e)
			break
		}
		fmt.Println(artist)

		/*
			_, insertErr = stmt.Exec(r.Artist, r.Album, r.Title, r.Dt)
			if insertErr != nil {
				break
			}
		*/
	}
	if e != nil {
		tx.Rollback()
		return e
	} else {
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
