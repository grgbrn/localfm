package model

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"bitbucket.org/grgbrn/localfm/pkg/util"

	_ "github.com/mattn/go-sqlite3" // blank import just to load driver
)

// database model structs
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

// Database represents an open connection to a database
type Database struct {
	SQL  *sql.DB
	Path string
	// could probably have a logger too
}

func Open(DSN string) (*Database, error) {
	// mimic DSN format from earlier python version of this tool
	// "sqlite:///foo.db"
	if !strings.HasPrefix(DSN, "sqlite://") {
		return nil, errors.New("DSN var must be of the format 'sqlite:///foo.db'")
	}
	dbPath := DSN[9:]

	// sqlite database drivers will automatically create empty databases
	// if the file doesn't exist, so stat the file first and abort
	// if there's no database (must be manually created with schema)
	if !util.FileExists(dbPath) {
		return nil, errors.New("Can't open database [0]")
	}

	// Open the database and test the connection
	// Currently nonexistent sqlite file doesn't trigger an error
	// (won't happen until the first query)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return &Database{
		SQL:  db,
		Path: dbPath,
	}, nil
}

// FindLatestTimestamp looks up the epoch time of the most recent db entry
// returns 0 from an empty database
func (db *Database) FindLatestTimestamp() (int64, error) {

	var maxTime int64

	// avoid an error with the findmax query on an empty db
	checkdb := `SELECT count(*) FROM activity`

	var tmp int
	err := db.SQL.QueryRow(checkdb, 1).Scan(&tmp)
	if err != nil {
		return maxTime, err
	}
	if tmp == 0 {
		return maxTime, nil // maxtime is already zero'd
	}

	findmax := `SELECT max(uts) FROM activity`

	err = db.SQL.QueryRow(findmax, 1).Scan(&maxTime)
	if err != nil {
		return maxTime, err
	}
	return maxTime, nil
}

// StoreActivity inserts a list of activity records into the database
// using a transaction. If error is returned the transaction was rolled
// back and no rows were inserted; otherwise all were inserted
func (db *Database) StoreActivity(tracks []TrackInfo) error {

	tx, err := db.SQL.Begin()
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
		album_id,
		image_id
	) values (?,?,?,?,?,?,?,?,?,?)
	`
	stmt, err := tx.Prepare(additem)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var e error
	var artist Artist
	var album Album
	var image Image

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

		u := ChooseImageURL(track)
		image, e = getOrCreateImage(tx, u)
		if e != nil {
			fmt.Printf("error inserting image:%s mbid:%s\n", track.Album.Name, track.Album.Mbid)
			fmt.Println(e)
			break
		}

		uts, e := GetParsedUTS(track)
		if e != nil {
			fmt.Printf("error parsing UTS: %v\n", e)
			break
		}
		dt, e := GetParsedTime(track)
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
			image.ID,
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

func getOrCreateImage(tx *sql.Tx, url string) (Image, error) {

	var image Image
	var err error

	selQuery := `SELECT id, url FROM image WHERE url=?`
	err = tx.QueryRow(selQuery, url).Scan(&image.ID, &image.URL)

	if err == nil { // found existing entry
		return image, nil
	}

	// otherwise have to create a new one
	insQuery := `INSERT INTO image(url) values (?)`
	res, err := tx.Exec(insQuery, url)
	if err != nil {
		// error creating new row
		return image, err
	}
	// need to return an image struct with newly created ID
	lastID, err := res.LastInsertId()
	if err != nil {
		return image, err
	}
	image.ID = lastID
	image.URL = url

	return image, nil
}
