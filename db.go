package localfm

import (
	"database/sql"
	"fmt"
	"time"

	// blank import just to load drivers
	_ "github.com/mattn/go-sqlite3"
)

/*
CREATE TABLE lastfm_activity (
	id INTEGER NOT NULL,
	doc JSON,
	created DATETIME,
	artist VARCHAR(255),
	album VARCHAR(255),
	title VARCHAR(255),
	dt DATETIME,
	PRIMARY KEY (id)
);
*/

type LastFMActivity struct {
	ID int
	//Doc     string // nee json
	Created time.Time
	Artist  string
	Album   string
	Title   string
	Dt      time.Time
}

func InitDB(filepath string) *sql.DB {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		panic(err)
	}
	if db == nil {
		panic("db nil")
	}
	return db
}

// FindLatestTimestamp looks up the timestamp of the most recently recorded
// entry in the database
func FindLatestTimestamp(db *sql.DB) (time.Time, error) {

	findmax := `SELECT MAX(dt) FROM lastfm_activity`

	var maxTime time.Time

	// max() expression isn't typed sufficiently for the sql driver
	// so retrieve as a string and then parse that
	// https://github.com/mattn/go-sqlite3/issues/190
	var maxStr string
	err := db.QueryRow(findmax, 1).Scan(&maxStr)
	if err != nil {
		return maxTime, err
	}

	// returned strings look like "2019-01-23 07:40:08.000000"
	// the ms/ns part probably isn't important since the API is
	// based on epoch time, so 1 second granularity, but include
	// it in the format string anyway
	fmt.Println(maxStr)

	maxTime, err = time.Parse("2006-01-02 15:04:05.000000", maxStr)
	if err != nil {
		return maxTime, err
	}
	return maxTime, nil
}

// ReadItem loads a series of records from the db using timestamp offset
// and count. Not currently used or very well vetted.
func ReadItem(db *sql.DB, from int, count int) ([]LastFMActivity, error) {
	readquery := `
	SELECT id, created, artist, album, title, dt
	from lastfm_activity
	where dt >= ?
	limit ?`

	var result []LastFMActivity

	rows, err := db.Query(readquery, from, count)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		item := LastFMActivity{}
		err2 := rows.Scan(&item.ID, &item.Created, &item.Artist, &item.Album, &item.Title, &item.Dt)
		if err2 != nil {
			return result, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func StoreActivity(db *sql.DB, rows []LastFMActivity) error {
	additem := `
	INSERT INTO lastfm_activity(
		created,
		artist,
		album,
		title,
		dt
	) values (CURRENT_TIMESTAMP, ?, ?, ?, ?)
	`

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(additem)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var insertErr error
	for _, r := range rows {
		_, insertErr = stmt.Exec(r.Artist, r.Album, r.Title, r.Dt)
		if insertErr != nil {
			break
		}
	}
	if insertErr != nil {
		tx.Rollback()
		return insertErr
	} else {
		tx.Commit()
		return nil
	}
}

/*
func main() {
	db := InitDB("/tmp/foo.db")

	max, err := FindLatestTimestamp(db)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("max timestamp:%v [%d]\n", max, max.Unix())

	res := ReadItem(db, 0, 10)
	for _, item := range res {
		fmt.Printf("%+v\n", item)
	}
}
*/
