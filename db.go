package localfm

import (
	"database/sql"
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

// LastFMActivity represents a database row storing a track play
type LastFMActivity struct {
	ID int
	//Doc     string // nee json
	Created time.Time
	Artist  string
	Album   string
	Title   string
	Dt      time.Time
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

// FindLatestTimestamp looks up the epoch time of the most recentl db entry
// returns 0 from an empty database
func FindLatestTimestamp(db *sql.DB) (int64, error) {

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

// StoreActivity inserts a list of activity records into the database
// using a transaction. If error is returned the transaction was rolled
// back and no rows were inserted; otherwise all were inserted
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
