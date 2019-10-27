package main

import (
	"flag"
	"os"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/update"
)

func main() {

	delayPtr := flag.Int("delay", 5, "Delay in seconds between API calls")
	limitPtr := flag.Int("limit", 0, "Limit number of API calls")

	dupePtr := flag.Bool("duplicates", false, "Check entire database for duplicates")

	flag.Parse()

	//
	// database init
	//
	DSN := os.Getenv("DSN")
	if DSN == "" {
		panic("Must set DSN environment var")
	}
	db, err := m.Open(DSN)
	if err != nil {
		panic(err)
	}

	update.FetchLatestScrobbles(db, *delayPtr, *limitPtr, *dupePtr)
}
