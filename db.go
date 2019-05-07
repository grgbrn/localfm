package localfm

import (
	"database/sql"
	"fmt"
	"strings"
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
	UTS int64
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
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return db, err
	}
	err = db.Ping()
	if err != nil {
		return db, err
	}
	return db, nil
}

// FindLatestTimestamp looks up the epoch time of the most recent db entry
// returns 0 from an empty database
func FindLatestTimestamp(db *sql.DB) (int64, error) {

	var maxTime int64

	// avoid an error with the findmax query on an empty db
	checkdb := `SELECT count(*) FROM activity`

	var tmp int
	err := db.QueryRow(checkdb, 1).Scan(&tmp)
	if err != nil {
		return maxTime, err
	}
	if tmp == 0 {
		return maxTime, nil // maxtime is already zero'd
	}

	findmax := `SELECT max(uts) FROM activity`

	err = db.QueryRow(findmax, 1).Scan(&maxTime)
	if err != nil {
		return maxTime, err
	}
	return maxTime, nil
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

type duplicateTrackResult struct {
	CheckedCount   int
	DuplicateCount int
	DuplicateIDs   []int64
}

func (res duplicateTrackResult) String() string {
	pct := (float64(res.DuplicateCount) / float64(res.CheckedCount)) * 100
	return fmt.Sprintf("checked %d rows; %d duplicates found (%0.2f%%)", res.CheckedCount, res.DuplicateCount, pct)
}

// XXX rename diff to threshold or something
func findDuplicates(db *sql.DB, since int64, diff int64) (duplicateTrackResult, error) {

	var res = duplicateTrackResult{}

	count := 0

	readquery := `
	SELECT id, uts, title, artist, album
	from activity
	where uts >= ?
	order by uts desc`

	rows, err := db.Query(readquery, since)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	var lastItem *Activity
	var duplicateIDs []int64

	for rows.Next() {
		item := Activity{}

		err = rows.Scan(&item.ID, &item.UTS, &item.Title, &item.ArtistName, &item.AlbumName)
		if err != nil {
			return res, err
		}
		count++

		if lastItem != nil && sameTrack(*lastItem, item) {
			// because of query order, lastitem should always be more
			// recent (larger) than item, so no need for abs()
			d := lastItem.UTS - item.UTS
			// fmt.Printf("diff: %d\n", d)

			if d <= diff {
				// consider the later element to be the duplicate
				// fmt.Printf("duplicate found (diff=%d)\n", d)
				// fmt.Println(item)
				// fmt.Println(*lastItem)
				duplicateIDs = append(duplicateIDs, lastItem.ID)
			}
		}
		lastItem = &item
	}

	return duplicateTrackResult{
		CheckedCount:   count,
		DuplicateCount: len(duplicateIDs),
		DuplicateIDs:   duplicateIDs,
	}, nil
}

func sameTrack(a, b Activity) bool {
	return a.Title == b.Title && a.ArtistName == b.ArtistName && a.AlbumName == b.AlbumName
}

// FlagDuplicates will scan all records in the database after
// a given timestamp and set the 'duplicate' field on any rows
// that immediately follow an identical record with a dt/uts
// less than 'diff' seconds apart
func FlagDuplicates(db *sql.DB, since int64, diff int64) (int, error) {

	fmt.Printf("checking for duplicates with diff=%d\n", diff)
	dupResult, err := findDuplicates(db, since, diff)
	if err != nil {
		return 0, err
	}

	fmt.Println(dupResult)

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}

	// sql lib can't use int slices as parameters so construct a parameter
	// string that matches the number of items in our set
	update := `UPDATE activity SET duplicate=true WHERE ID IN (?` + strings.Repeat(",?", dupResult.DuplicateCount-1) + `)`

	// fmt.Println(update)
	// fmt.Println(dupResult.DuplicateIDs)

	updResult, err := tx.Exec(update, InterfaceSliceInt64(dupResult.DuplicateIDs)...)
	if err != nil {
		fmt.Printf("rolling back after error:%v\n", err)
		tx.Rollback()
		return 0, err
	}

	// XXX what even does err mean here?
	rowsAffected, _ := updResult.RowsAffected()
	if rowsAffected != int64(dupResult.DuplicateCount) {
		fmt.Printf("Warning: incomplete update (%d/%d rows)\n", rowsAffected, dupResult.DuplicateCount)
	}

	err = tx.Commit()
	if err != nil {
		fmt.Printf("Error committing update: %v\n", err)
	} else {
		fmt.Printf("flagged %d duplicate activity entries\n", rowsAffected)
	}
	return int(rowsAffected), nil
}
