package model

import (
	"fmt"
	"strings"
)

/*

duplicate detection code
doesn't acutally work very well, probably needs review and some tests

*/

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
func (db *Database) findDuplicates(since int64, diff int64) (duplicateTrackResult, error) {

	var res = duplicateTrackResult{}

	count := 0

	readquery := `
	SELECT id, uts, title, artist, album
	from activity
	where uts >= ?
	order by uts desc`

	rows, err := db.SQL.Query(readquery, since)
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
			// XXX will be returning bogus count number here
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
func (db *Database) FlagDuplicates(since int64, diff int64) (int, error) {

	fmt.Printf("Checking for duplicates with diff=%d\n", diff)
	dupResult, err := db.findDuplicates(since, diff)
	if err != nil {
		return 0, err
	}
	fmt.Println(dupResult)

	if dupResult.DuplicateCount == 0 { // nothing to do
		return 0, nil
	}

	tx, err := db.SQL.Begin()
	if err != nil {
		return 0, err
	}

	// sql lib can't use int slices as parameters so construct a parameter
	// string that matches the number of items in our set
	update := `UPDATE activity SET duplicate=true WHERE ID IN (?` + strings.Repeat(",?", dupResult.DuplicateCount-1) + `)`

	// fmt.Println(update)
	// fmt.Println(dupResult.DuplicateIDs)

	updResult, err := tx.Exec(update, interfaceSliceInt64(dupResult.DuplicateIDs)...)
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
		return 0, err
	}
	fmt.Printf("flagged %d duplicate activity entries\n", rowsAffected)
	return int(rowsAffected), nil
}

// InterfaceSliceInt64 allows []int64 to be used as varargs
// (definitely an ugly corner of go). This is more complicated than
// a simple cast, as explained:
// https://github.com/golang/go/wiki/InterfaceSlice
func interfaceSliceInt64(data []int64) []interface{} {
	var interfaceSlice = make([]interface{}, len(data))
	for i, d := range data {
		interfaceSlice[i] = d
	}
	return interfaceSlice
}
