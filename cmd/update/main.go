package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	m "bitbucket.org/grgbrn/localfm/pkg/model"
	"bitbucket.org/grgbrn/localfm/pkg/update"
)

// main entry point for standalone update command
// intended to be called from a cron job
func main() {

	delayPtr := flag.Int("delay", 5, "Delay in seconds between API calls")
	limitPtr := flag.Int("limit", 0, "Limit number of API calls")

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

	//
	// load lastfm credentials
	//
	APIKey := os.Getenv("LASTFM_API_KEY")
	APISecret := os.Getenv("LASTFM_API_SECRET")
	Username := os.Getenv("LASTFM_USERNAME")

	if APIKey == "" || APISecret == "" || Username == "" {
		panic("Must set LASTFM_USERNAME, LASTFM_API_KEY and LASTFM_API_SECRET environment vars")
	}

	//
	// create logger
	//
	log := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	fetcher := update.CreateFetcher(db,
		log,
		update.LastFMCredentials{
			APIKey:    APIKey,
			APISecret: APISecret,
			Username:  Username,
		},
	)

	res, err := fetcher.FetchLatestScrobbles(
		update.FetchOptions{
			APIThrottleDelay: *delayPtr,
			RequestLimit:     *limitPtr,
		},
	)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", res)
}
